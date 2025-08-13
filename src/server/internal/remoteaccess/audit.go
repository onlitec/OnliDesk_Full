package remoteaccess

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent represents an audit log event
type AuditEvent struct {
	EventType   string                 `json:"event_type"`
	SessionID   string                 `json:"session_id,omitempty"`
	ClientID    string                 `json:"client_id,omitempty"`
	Technician  string                 `json:"technician,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Severity    string                 `json:"severity"`
	Success     bool                   `json:"success"`
	Timestamp   time.Time              `json:"timestamp"`
	CorrelationID string               `json:"correlation_id,omitempty"`
}

// AuditLogger handles audit logging for remote access events
type AuditLogger struct {
	logDir      string
	enabled     bool
	file        *os.File
	mutex       sync.Mutex
	rotateSize  int64
	maxFiles    int
	currentSize int64
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logDir string, enabled bool) *AuditLogger {
	logger := &AuditLogger{
		logDir:     logDir,
		enabled:    enabled,
		rotateSize: 100 * 1024 * 1024, // 100MB
		maxFiles:   10,
	}

	if enabled {
		if err := logger.initLogFile(); err != nil {
			log.Printf("Failed to initialize audit log file: %v", err)
			logger.enabled = false
		}
	}

	return logger
}

// initLogFile initializes the log file
func (al *AuditLogger) initLogFile() error {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(al.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFile := filepath.Join(al.logDir, fmt.Sprintf("remoteaccess_audit_%s.log", timestamp))

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	al.file = file
	al.currentSize = 0

	// Get current file size
	if stat, err := file.Stat(); err == nil {
		al.currentSize = stat.Size()
	}

	return nil
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event AuditEvent) {
	if !al.enabled {
		return
	}

	al.mutex.Lock()
	defer al.mutex.Unlock()

	// Check if log rotation is needed
	if al.currentSize > al.rotateSize {
		al.rotateLog()
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal audit event: %v", err)
		return
	}

	// Write to log file
	logLine := fmt.Sprintf("%s\n", string(eventJSON))
	n, err := al.file.WriteString(logLine)
	if err != nil {
		log.Printf("Failed to write audit event: %v", err)
		return
	}

	al.currentSize += int64(n)

	// Force sync to disk for critical events
	if event.Severity == "critical" || event.Severity == "error" {
		al.file.Sync()
	}
}

// LogSecurityViolation logs a security violation event
func (al *AuditLogger) LogSecurityViolation(sessionID, clientID, technician, violation, ipAddress string) {
	event := AuditEvent{
		EventType:  "security_violation",
		SessionID:  sessionID,
		ClientID:   clientID,
		Technician: technician,
		IPAddress:  ipAddress,
		Details: map[string]interface{}{
			"violation": violation,
		},
		Severity:  "critical",
		Success:   false,
		Timestamp: time.Now(),
	}

	al.LogEvent(event)
}

// LogPrivilegeEscalation logs privilege escalation events
func (al *AuditLogger) LogPrivilegeEscalation(sessionID, technician string, privilegeType PrivilegeType, approved bool, approvedBy string) {
	eventType := "privilege_escalation_denied"
	severity := "warning"
	if approved {
		eventType = "privilege_escalation_approved"
		severity = "error"
	}

	event := AuditEvent{
		EventType:  eventType,
		SessionID:  sessionID,
		Technician: technician,
		Details: map[string]interface{}{
			"privilege_type": privilegeType,
			"approved_by":    approvedBy,
		},
		Severity:  severity,
		Success:   approved,
		Timestamp: time.Now(),
	}

	al.LogEvent(event)
}

// LogSessionActivity logs session activity
func (al *AuditLogger) LogSessionActivity(sessionID, technician, activity string, details map[string]interface{}) {
	event := AuditEvent{
		EventType:  "session_activity",
		SessionID:  sessionID,
		Technician: technician,
		Details: map[string]interface{}{
			"activity": activity,
		},
		Severity:  "info",
		Success:   true,
		Timestamp: time.Now(),
	}

	// Merge additional details
	if details != nil {
		for k, v := range details {
			event.Details[k] = v
		}
	}

	al.LogEvent(event)
}

// LogFileTransfer logs file transfer events
func (al *AuditLogger) LogFileTransfer(sessionID, technician, filename string, fileSize int64, direction string, success bool) {
	event := AuditEvent{
		EventType:  "file_transfer",
		SessionID:  sessionID,
		Technician: technician,
		Details: map[string]interface{}{
			"filename":  filename,
			"file_size": fileSize,
			"direction": direction, // upload, download
		},
		Severity:  "info",
		Success:   success,
		Timestamp: time.Now(),
	}

	if !success {
		event.Severity = "warning"
	}

	al.LogEvent(event)
}

// LogCommandExecution logs command execution events
func (al *AuditLogger) LogCommandExecution(sessionID, technician, command string, success bool, output string) {
	event := AuditEvent{
		EventType:  "command_execution",
		SessionID:  sessionID,
		Technician: technician,
		Details: map[string]interface{}{
			"command": command,
			"output":  output,
		},
		Severity:  "info",
		Success:   success,
		Timestamp: time.Now(),
	}

	// Increase severity for certain commands
	if isHighRiskCommand(command) {
		event.Severity = "warning"
	}

	al.LogEvent(event)
}

// rotateLog rotates the log file when it gets too large
func (al *AuditLogger) rotateLog() {
	if al.file != nil {
		al.file.Close()
	}

	// Clean up old log files
	al.cleanupOldLogs()

	// Create new log file
	if err := al.initLogFile(); err != nil {
		log.Printf("Failed to rotate audit log: %v", err)
		al.enabled = false
	}
}

// cleanupOldLogs removes old log files beyond the retention limit
func (al *AuditLogger) cleanupOldLogs() {
	files, err := filepath.Glob(filepath.Join(al.logDir, "remoteaccess_audit_*.log"))
	if err != nil {
		log.Printf("Failed to list log files for cleanup: %v", err)
		return
	}

	if len(files) <= al.maxFiles {
		return
	}

	// Sort files by modification time and remove oldest
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		if stat, err := os.Stat(file); err == nil {
			fileInfos = append(fileInfos, fileInfo{
				path:    file,
				modTime: stat.ModTime(),
			})
		}
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(fileInfos)-1; i++ {
		for j := i + 1; j < len(fileInfos); j++ {
			if fileInfos[i].modTime.After(fileInfos[j].modTime) {
				fileInfos[i], fileInfos[j] = fileInfos[j], fileInfos[i]
			}
		}
	}

	// Remove oldest files
	filesToRemove := len(fileInfos) - al.maxFiles
	for i := 0; i < filesToRemove; i++ {
		if err := os.Remove(fileInfos[i].path); err != nil {
			log.Printf("Failed to remove old log file %s: %v", fileInfos[i].path, err)
		}
	}
}

// isHighRiskCommand checks if a command is considered high risk
func isHighRiskCommand(command string) bool {
	highRiskCommands := []string{
		"rm", "del", "format", "fdisk", "mkfs",
		"sudo", "su", "runas",
		"net user", "useradd", "userdel",
		"reg add", "reg delete", "regedit",
		"sc create", "sc delete", "systemctl",
		"iptables", "netsh", "route",
		"chmod", "chown", "icacls",
	}

	for _, riskCmd := range highRiskCommands {
		if len(command) >= len(riskCmd) && command[:len(riskCmd)] == riskCmd {
			return true
		}
	}

	return false
}

// Close closes the audit logger
func (al *AuditLogger) Close() {
	al.mutex.Lock()
	defer al.mutex.Unlock()

	if al.file != nil {
		al.file.Close()
		al.file = nil
	}
}

// GetLogFiles returns a list of audit log files
func (al *AuditLogger) GetLogFiles() ([]string, error) {
	return filepath.Glob(filepath.Join(al.logDir, "remoteaccess_audit_*.log"))
}

// SearchLogs searches audit logs for specific criteria
func (al *AuditLogger) SearchLogs(criteria map[string]interface{}, limit int) ([]AuditEvent, error) {
	files, err := al.GetLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %v", err)
	}

	var events []AuditEvent
	count := 0

	// Search through log files (newest first)
	for i := len(files) - 1; i >= 0 && count < limit; i-- {
		file, err := os.Open(files[i])
		if err != nil {
			continue
		}

		// Read file line by line
		// This is a simplified implementation - in production you might want to use a more efficient approach
		file.Close()
	}

	return events, nil
}

// GetStatistics returns audit logging statistics
func (al *AuditLogger) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":      al.enabled,
		"log_dir":      al.logDir,
		"current_size": al.currentSize,
		"rotate_size":  al.rotateSize,
		"max_files":    al.maxFiles,
	}

	if files, err := al.GetLogFiles(); err == nil {
		stats["log_files_count"] = len(files)
	}

	return stats
}