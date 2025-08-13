package filetransfer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEventType defines the type of audit event
type AuditEventType string

const (
	AuditEventTransferRequested AuditEventType = "transfer_requested"
	AuditEventTransferApproved  AuditEventType = "transfer_approved"
	AuditEventTransferRejected  AuditEventType = "transfer_rejected"
	AuditEventTransferStarted   AuditEventType = "transfer_started"
	AuditEventTransferPaused    AuditEventType = "transfer_paused"
	AuditEventTransferResumed   AuditEventType = "transfer_resumed"
	AuditEventTransferCompleted AuditEventType = "transfer_completed"
	AuditEventTransferFailed    AuditEventType = "transfer_failed"
	AuditEventTransferCancelled AuditEventType = "transfer_cancelled"
	AuditEventFileValidated     AuditEventType = "file_validated"
	AuditEventFileQuarantined   AuditEventType = "file_quarantined"
	AuditEventConfigUpdated     AuditEventType = "config_updated"
	AuditEventSecurityViolation AuditEventType = "security_violation"
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   AuditEventType         `json:"event_type"`
	SessionID   string                 `json:"session_id,omitempty"`
	TransferID  string                 `json:"transfer_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	Technician  string                 `json:"technician,omitempty"`
	Filename    string                 `json:"filename,omitempty"`
	FileSize    int64                  `json:"file_size,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Severity    string                 `json:"severity"`
	Success     bool                   `json:"success"`
	ErrorMsg    string                 `json:"error_message,omitempty"`
}

// AuditLogger handles audit logging for file transfers
type AuditLogger struct {
	logFile    string
	logDir     string
	maxLogSize int64
	maxLogAge  time.Duration
	enabled    bool
	mutex      sync.RWMutex
	logChan    chan *AuditEvent
	stopChan   chan bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logDir string, enabled bool) *AuditLogger {
	if logDir == "" {
		logDir = "./logs/audit"
	}
	
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Failed to create audit log directory: %v", err)
		enabled = false
	}
	
	logger := &AuditLogger{
		logDir:     logDir,
		maxLogSize: 100 * 1024 * 1024, // 100MB
		maxLogAge:  30 * 24 * time.Hour, // 30 days
		enabled:    enabled,
		logChan:    make(chan *AuditEvent, 1000),
		stopChan:   make(chan bool),
	}
	
	if enabled {
		logger.logFile = filepath.Join(logDir, fmt.Sprintf("audit_%s.log", time.Now().Format("2006-01-02")))
		go logger.processLogs()
		go logger.rotateLogsDaily()
	}
	
	return logger
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event *AuditEvent) {
	if !al.enabled {
		return
	}
	
	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Severity == "" {
		event.Severity = al.determineSeverity(event.EventType)
	}
	
	select {
	case al.logChan <- event:
		// Event queued successfully
	default:
		// Channel full, log to stderr
		log.Printf("Audit log channel full, dropping event: %s", event.EventType)
	}
}

// LogTransferRequest logs a transfer request event
func (al *AuditLogger) LogTransferRequest(request *FileTransferRequest, ipAddress, userAgent string) {
	event := &AuditEvent{
		EventType:  AuditEventTransferRequested,
		SessionID:  request.SessionID,
		TransferID: request.ID,
		Technician: request.Technician,
		Filename:   request.Filename,
		FileSize:   request.FileSize,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Success:    true,
		Details: map[string]interface{}{
			"transfer_type": request.Type,
			"checksum":     request.Checksum,
		},
	}
	al.LogEvent(event)
}

// LogTransferApproval logs a transfer approval/rejection event
func (al *AuditLogger) LogTransferApproval(transferID, sessionID string, approved bool, message, userID string) {
	eventType := AuditEventTransferApproved
	if !approved {
		eventType = AuditEventTransferRejected
	}
	
	event := &AuditEvent{
		EventType:  eventType,
		SessionID:  sessionID,
		TransferID: transferID,
		UserID:     userID,
		Success:    true,
		Details: map[string]interface{}{
			"approved": approved,
			"message":  message,
		},
	}
	al.LogEvent(event)
}

// LogTransferProgress logs transfer progress events
func (al *AuditLogger) LogTransferProgress(transferID, sessionID string, eventType AuditEventType, details map[string]interface{}) {
	event := &AuditEvent{
		EventType:  eventType,
		SessionID:  sessionID,
		TransferID: transferID,
		Success:    true,
		Details:    details,
	}
	al.LogEvent(event)
}

// LogSecurityViolation logs security violation events
func (al *AuditLogger) LogSecurityViolation(transferID, sessionID, filename, violation, ipAddress string) {
	event := &AuditEvent{
		EventType:  AuditEventSecurityViolation,
		SessionID:  sessionID,
		TransferID: transferID,
		Filename:   filename,
		IPAddress:  ipAddress,
		Success:    false,
		ErrorMsg:   violation,
		Severity:   "HIGH",
		Details: map[string]interface{}{
			"violation_type": "security",
			"description":    violation,
		},
	}
	al.LogEvent(event)
}

// LogConfigUpdate logs configuration update events
func (al *AuditLogger) LogConfigUpdate(userID, configType string, changes map[string]interface{}) {
	event := &AuditEvent{
		EventType: AuditEventConfigUpdated,
		UserID:    userID,
		Success:   true,
		Details: map[string]interface{}{
			"config_type": configType,
			"changes":     changes,
		},
	}
	al.LogEvent(event)
}

// processLogs processes audit events from the channel
func (al *AuditLogger) processLogs() {
	for {
		select {
		case event := <-al.logChan:
			al.writeEvent(event)
		case <-al.stopChan:
			// Process remaining events
			for len(al.logChan) > 0 {
				event := <-al.logChan
				al.writeEvent(event)
			}
			return
		}
	}
}

// writeEvent writes an audit event to the log file
func (al *AuditLogger) writeEvent(event *AuditEvent) {
	al.mutex.Lock()
	defer al.mutex.Unlock()
	
	// Check if log rotation is needed
	if al.needsRotation() {
		al.rotateLog()
	}
	
	// Open log file
	file, err := os.OpenFile(al.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open audit log file: %v", err)
		return
	}
	defer file.Close()
	
	// Write event as JSON
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal audit event: %v", err)
		return
	}
	
	if _, err := file.Write(append(data, '\n')); err != nil {
		log.Printf("Failed to write audit event: %v", err)
	}
}

// needsRotation checks if log rotation is needed
func (al *AuditLogger) needsRotation() bool {
	stat, err := os.Stat(al.logFile)
	if err != nil {
		return false
	}
	return stat.Size() > al.maxLogSize
}

// rotateLog rotates the current log file
func (al *AuditLogger) rotateLog() {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	rotatedFile := filepath.Join(al.logDir, fmt.Sprintf("audit_%s.log", timestamp))
	
	if err := os.Rename(al.logFile, rotatedFile); err != nil {
		log.Printf("Failed to rotate audit log: %v", err)
		return
	}
	
	log.Printf("Audit log rotated to: %s", rotatedFile)
}

// rotateLogsDaily rotates logs daily and cleans up old logs
func (al *AuditLogger) rotateLogsDaily() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			al.cleanupOldLogs()
			// Update log file name for new day
			al.mutex.Lock()
			al.logFile = filepath.Join(al.logDir, fmt.Sprintf("audit_%s.log", time.Now().Format("2006-01-02")))
			al.mutex.Unlock()
		case <-al.stopChan:
			return
		}
	}
}

// cleanupOldLogs removes old log files
func (al *AuditLogger) cleanupOldLogs() {
	cutoff := time.Now().Add(-al.maxLogAge)
	
	files, err := filepath.Glob(filepath.Join(al.logDir, "audit_*.log"))
	if err != nil {
		log.Printf("Failed to list audit log files: %v", err)
		return
	}
	
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		if stat.ModTime().Before(cutoff) {
			if err := os.Remove(file); err != nil {
				log.Printf("Failed to remove old audit log %s: %v", file, err)
			} else {
				log.Printf("Removed old audit log: %s", file)
			}
		}
	}
}

// determineSeverity determines the severity level for an event type
func (al *AuditLogger) determineSeverity(eventType AuditEventType) string {
	switch eventType {
	case AuditEventSecurityViolation:
		return "HIGH"
	case AuditEventTransferFailed, AuditEventFileQuarantined:
		return "MEDIUM"
	case AuditEventTransferRejected, AuditEventTransferCancelled:
		return "LOW"
	default:
		return "INFO"
	}
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000)
}

// Stop stops the audit logger
func (al *AuditLogger) Stop() {
	if al.enabled {
		close(al.stopChan)
	}
}

// GetAuditSummary returns a summary of audit events
func (al *AuditLogger) GetAuditSummary(since time.Time) (map[string]interface{}, error) {
	if !al.enabled {
		return map[string]interface{}{"enabled": false}, nil
	}
	
	// This is a simplified implementation
	// In a production system, you might want to use a database for better querying
	return map[string]interface{}{
		"enabled":     true,
		"log_dir":     al.logDir,
		"log_file":    al.logFile,
		"max_log_age": al.maxLogAge.String(),
		"since":       since,
	}, nil
}