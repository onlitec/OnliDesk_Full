package filetransfer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SessionManager manages all active file transfer sessions
type SessionManager struct {
	sessions        map[string]*TransferSession
	fileStreams     map[string]*FileStream
	config          *TransferConfig
	mutex           sync.RWMutex
	cleanupTicker   *time.Ticker
	shutdownChan    chan bool
	auditLogger     *AuditLogger
}

// TransferConfig holds configuration for file transfers
type TransferConfig struct {
	MaxFileSize      int64             `json:"max_file_size"`
	AllowedTypes     []string          `json:"allowed_types"`
	TempDir          string            `json:"temp_dir"`
	MaxConcurrent    int               `json:"max_concurrent"`
	TransferTimeout  time.Duration     `json:"transfer_timeout"`
	CleanupInterval  time.Duration     `json:"cleanup_interval"`
	RateLimit        int64             `json:"rate_limit"` // bytes per second
	RequireApproval  bool              `json:"require_approval"`
	AuditLog         bool              `json:"audit_log"`
	VirusScan        bool              `json:"virus_scan"`
	EncryptFiles     bool              `json:"encrypt_files"`
	CompressionLevel int               `json:"compression_level"`
	RetryAttempts    int               `json:"retry_attempts"`
	ChunkSize        int               `json:"chunk_size"`
}

// DefaultTransferConfig returns default configuration
func DefaultTransferConfig() *TransferConfig {
	return &TransferConfig{
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		AllowedTypes:     []string{".txt", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".jpg", ".png", ".gif"},
		TempDir:          "./temp/transfers",
		MaxConcurrent:    5,
		TransferTimeout:  30 * time.Minute,
		CleanupInterval:  5 * time.Minute,
		RateLimit:        10 * 1024 * 1024, // 10MB/s
		RequireApproval:  true,
		AuditLog:         true,
		VirusScan:        false,
		EncryptFiles:     true,
		CompressionLevel: 6,
		RetryAttempts:    3,
		ChunkSize:        64 * 1024, // 64KB
	}
}

// AuditLogEntry represents an audit log entry for file transfers
type AuditLogEntry struct {
	Timestamp    time.Time     `json:"timestamp"`
	TransferID   string        `json:"transfer_id"`
	SessionID    string        `json:"session_id"`
	Technician   string        `json:"technician"`
	ClientID     string        `json:"client_id"`
	Action       string        `json:"action"`
	Filename     string        `json:"filename"`
	FileSize     int64         `json:"file_size"`
	TransferType TransferType  `json:"transfer_type"`
	Status       TransferStatus `json:"status"`
	Duration     time.Duration `json:"duration,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	IPAddress    string        `json:"ip_address"`
	UserAgent    string        `json:"user_agent"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *TransferConfig) *SessionManager {
	if config == nil {
		config = DefaultTransferConfig()
	}

	// Ensure temp directory exists
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		log.Printf("Failed to create temp directory: %v", err)
	}

	sm := &SessionManager{
		sessions:      make(map[string]*TransferSession),
		fileStreams:   make(map[string]*FileStream),
		config:        config,
		cleanupTicker: time.NewTicker(config.CleanupInterval),
		shutdownChan:  make(chan bool),
		auditLogger:   NewAuditLogger("./logs/sessions", true),
	}

	// Start cleanup routine
	go sm.cleanupRoutine()

	return sm
}

// CreateTransferSession creates a new transfer session
func (sm *SessionManager) CreateTransferSession(request *FileTransferRequest, clientConn, portalConn *websocket.Conn) (*TransferSession, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check if we've reached the maximum concurrent transfers
	if len(sm.sessions) >= sm.config.MaxConcurrent {
		return nil, fmt.Errorf("maximum concurrent transfers reached (%d)", sm.config.MaxConcurrent)
	}

	// Validate file size
	if request.FileSize > sm.config.MaxFileSize {
		return nil, fmt.Errorf("file size (%d bytes) exceeds maximum allowed size (%d bytes)", request.FileSize, sm.config.MaxFileSize)
	}

	// Validate file type
	if len(sm.config.AllowedTypes) > 0 {
		ext := filepath.Ext(request.Filename)
		allowed := false
		for _, allowedType := range sm.config.AllowedTypes {
			if ext == allowedType {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("file type %s is not allowed", ext)
		}
	}

	// Generate unique transfer ID if not provided
	if request.ID == "" {
		request.ID = uuid.New().String()
	}

	// Create transfer session
	session := &TransferSession{
		ID:             request.ID,
		Request:        request,
		Status:         StatusPending,
		StartTime:      time.Now(),
		ReceivedChunks: make(map[int]bool),
		ClientConn:     clientConn,
		PortalConn:     portalConn,
	}

	// Store session
	sm.sessions[request.ID] = session

	// Log audit entry
	if sm.config.AuditLog {
		sm.logAuditEntry(&AuditLogEntry{
			Timestamp:    time.Now(),
			TransferID:   request.ID,
			SessionID:    request.SessionID,
			Technician:   request.Technician,
			Action:       "transfer_request_created",
			Filename:     request.Filename,
			FileSize:     request.FileSize,
			TransferType: request.Type,
			Status:       StatusPending,
		})
	}

	return session, nil
}

// GetSession retrieves a transfer session by ID
func (sm *SessionManager) GetSession(transferID string) (*TransferSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[transferID]
	return session, exists
}

// ApproveTransfer approves a pending transfer
func (sm *SessionManager) ApproveTransfer(transferID string, approved bool, message string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[transferID]
	if !exists {
		return fmt.Errorf("transfer session not found: %s", transferID)
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.Status != StatusPending {
		return fmt.Errorf("transfer is not in pending state: %s", session.Status)
	}

	if approved {
		session.Status = StatusApproved

		// Create temporary file path
		tempPath := filepath.Join(sm.config.TempDir, fmt.Sprintf("transfer_%s_%s", transferID, session.Request.Filename))
		session.TempPath = tempPath

		// Create file stream
		fileStream, err := NewFileStream(transferID, tempPath, session.Request.Type == TransferTypeUpload, session.ClientConn)
		if err != nil {
			return fmt.Errorf("failed to create file stream: %v", err)
		}

		sm.fileStreams[transferID] = fileStream

		// Start the appropriate transfer process
		if session.Request.Type == TransferTypeUpload {
			if err := fileStream.StartUpload(); err != nil {
				return fmt.Errorf("failed to start upload: %v", err)
			}
		} else {
			if err := fileStream.StartDownload(); err != nil {
				return fmt.Errorf("failed to start download: %v", err)
			}
		}

		log.Printf("Transfer approved and started: %s", transferID)
	} else {
		session.Status = StatusRejected
		log.Printf("Transfer rejected: %s", transferID)
	}

	// Log audit entry using new audit system
	if approved {
		sm.auditLogger.LogTransferApproval(transferID, session.Request.SessionID, true, message, session.Request.Technician)
		sm.auditLogger.LogTransferProgress(transferID, session.Request.SessionID, AuditEventTransferStarted, map[string]interface{}{
			"filename":      session.Request.Filename,
			"file_size":     session.Request.FileSize,
			"transfer_type": session.Request.Type,
			"technician":    session.Request.Technician,
			"temp_path":     session.TempPath,
		})
	} else {
		sm.auditLogger.LogTransferApproval(transferID, session.Request.SessionID, false, message, session.Request.Technician)
	}

	return nil
}

// PauseTransfer pauses an active transfer
func (sm *SessionManager) PauseTransfer(transferID string) error {
	sm.mutex.RLock()
	fileStream, exists := sm.fileStreams[transferID]
	sm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("file stream not found: %s", transferID)
	}

	fileStream.Pause()

	sm.mutex.Lock()
	if session, exists := sm.sessions[transferID]; exists {
		session.mutex.Lock()
		session.Status = StatusPaused
		session.mutex.Unlock()
		
		// Log audit entry using new audit system
		sm.auditLogger.LogTransferProgress(transferID, session.Request.SessionID, AuditEventTransferPaused, map[string]interface{}{
			"filename":      session.Request.Filename,
			"file_size":     session.Request.FileSize,
			"transfer_type": session.Request.Type,
			"technician":    session.Request.Technician,
		})
	}
	sm.mutex.Unlock()

	log.Printf("Transfer paused: %s", transferID)
	return nil
}

// ResumeTransfer resumes a paused transfer
func (sm *SessionManager) ResumeTransfer(transferID string) error {
	sm.mutex.RLock()
	fileStream, exists := sm.fileStreams[transferID]
	sm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("file stream not found: %s", transferID)
	}

	fileStream.Resume()

	sm.mutex.Lock()
	if session, exists := sm.sessions[transferID]; exists {
		session.mutex.Lock()
		session.Status = StatusInProgress
		session.mutex.Unlock()
		
		// Log audit entry using new audit system
		sm.auditLogger.LogTransferProgress(transferID, session.Request.SessionID, AuditEventTransferResumed, map[string]interface{}{
			"filename":      session.Request.Filename,
			"file_size":     session.Request.FileSize,
			"transfer_type": session.Request.Type,
			"technician":    session.Request.Technician,
		})
	}
	sm.mutex.Unlock()

	log.Printf("Transfer resumed: %s", transferID)
	return nil
}

// CancelTransfer cancels an active transfer
func (sm *SessionManager) CancelTransfer(transferID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Cancel file stream
	if fileStream, exists := sm.fileStreams[transferID]; exists {
		fileStream.Cancel()
		delete(sm.fileStreams, transferID)
	}

	// Update session status
	if session, exists := sm.sessions[transferID]; exists {
		session.mutex.Lock()
		session.Status = StatusCancelled
		now := time.Now()
		session.EndTime = &now
		session.mutex.Unlock()

		// Clean up temporary files
		if session.TempPath != "" {
			if err := os.Remove(session.TempPath); err != nil {
				log.Printf("Error removing temp file: %v", err)
			}
		}

		// Log audit entry using new audit system
		sm.auditLogger.LogTransferProgress(transferID, session.Request.SessionID, AuditEventTransferCancelled, map[string]interface{}{
			"filename":      session.Request.Filename,
			"file_size":     session.Request.FileSize,
			"transfer_type": session.Request.Type,
			"technician":    session.Request.Technician,
			"duration":      time.Since(session.StartTime).String(),
			"reason":        "User cancelled",
		})

		delete(sm.sessions, transferID)
	}

	log.Printf("Transfer cancelled: %s", transferID)
	return nil
}

// CompleteTransfer marks a transfer as completed
func (sm *SessionManager) CompleteTransfer(transferID string, success bool, errorMessage string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[transferID]
	if !exists {
		return fmt.Errorf("transfer session not found: %s", transferID)
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	now := time.Now()
	session.EndTime = &now

	if success {
		session.Status = StatusCompleted
		log.Printf("Transfer completed successfully: %s", transferID)
	} else {
		session.Status = StatusFailed
		log.Printf("Transfer failed: %s - %s", transferID, errorMessage)
	}

	// Clean up file stream
	if fileStream, exists := sm.fileStreams[transferID]; exists {
		fileStream.Cancel() // This will trigger cleanup
		delete(sm.fileStreams, transferID)
	}

	// Log audit entry using new audit system
	if success {
		sm.auditLogger.LogTransferProgress(transferID, session.Request.SessionID, AuditEventTransferCompleted, map[string]interface{}{
			"filename":        session.Request.Filename,
			"file_size":       session.Request.FileSize,
			"transfer_type":   session.Request.Type,
			"technician":      session.Request.Technician,
			"duration":        time.Since(session.StartTime).String(),
			"bytes_transferred": session.Request.FileSize,
		})
	} else {
		sm.auditLogger.LogTransferProgress(transferID, session.Request.SessionID, AuditEventTransferFailed, map[string]interface{}{
			"filename":        session.Request.Filename,
			"file_size":       session.Request.FileSize,
			"transfer_type":   session.Request.Type,
			"technician":      session.Request.Technician,
			"duration":        time.Since(session.StartTime).String(),
			"error_message":   errorMessage,
		})
	}

	// Keep completed sessions for a while for audit purposes
	// They will be cleaned up by the cleanup routine

	return nil
}

// GetTransferProgress returns the current progress of a transfer
func (sm *SessionManager) GetTransferProgress(transferID string) (*FileTransferProgress, error) {
	sm.mutex.RLock()
	fileStream, exists := sm.fileStreams[transferID]
	sm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("file stream not found: %s", transferID)
	}

	progress := fileStream.GetProgress()
	return &progress, nil
}

// GetActiveSessions returns all active transfer sessions
func (sm *SessionManager) GetActiveSessions() map[string]*TransferSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]*TransferSession)
	for id, session := range sm.sessions {
		session.mutex.RLock()
		if session.Status == StatusInProgress || session.Status == StatusPaused || session.Status == StatusPending || session.Status == StatusApproved {
			result[id] = session
		}
		session.mutex.RUnlock()
	}

	return result
}

// GetSessionsByUser returns all sessions for a specific technician
func (sm *SessionManager) GetSessionsByUser(technician string) map[string]*TransferSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]*TransferSession)
	for id, session := range sm.sessions {
		if session.Request.Technician == technician {
			result[id] = session
		}
	}

	return result
}

// UpdateConfig updates the transfer configuration
func (sm *SessionManager) UpdateConfig(config *TransferConfig) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.config = config

	// Restart cleanup ticker with new interval
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	sm.cleanupTicker = time.NewTicker(config.CleanupInterval)
}

// GetConfig returns the current configuration
func (sm *SessionManager) GetConfig() *TransferConfig {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.config
}

// cleanupRoutine periodically cleans up old sessions and temporary files
func (sm *SessionManager) cleanupRoutine() {
	for {
		select {
		case <-sm.cleanupTicker.C:
			sm.performCleanup()
		case <-sm.shutdownChan:
			return
		}
	}
}

// performCleanup removes old completed/failed sessions and temporary files
func (sm *SessionManager) performCleanup() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	cutoffTime := time.Now().Add(-1 * time.Hour) // Keep sessions for 1 hour after completion

	for id, session := range sm.sessions {
		session.mutex.RLock()
		shoudCleanup := (session.Status == StatusCompleted || session.Status == StatusFailed || session.Status == StatusCancelled) &&
			session.EndTime != nil && session.EndTime.Before(cutoffTime)
		tempPath := session.TempPath
		session.mutex.RUnlock()

		if shoudCleanup {
			// Remove temporary file
			if tempPath != "" {
				if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
					log.Printf("Error removing temp file during cleanup: %v", err)
				}
			}

			// Remove session
			delete(sm.sessions, id)
			log.Printf("Cleaned up old transfer session: %s", id)
		}
	}

	// Clean up orphaned file streams
	for id, fileStream := range sm.fileStreams {
		if !fileStream.IsActive() {
			delete(sm.fileStreams, id)
			log.Printf("Cleaned up inactive file stream: %s", id)
		}
	}

	// Clean up orphaned temporary files
	sm.cleanupOrphanedTempFiles()
}

// cleanupOrphanedTempFiles removes temporary files that are no longer associated with active sessions
func (sm *SessionManager) cleanupOrphanedTempFiles() {
	files, err := os.ReadDir(sm.config.TempDir)
	if err != nil {
		log.Printf("Error reading temp directory: %v", err)
		return
	}

	activeFiles := make(map[string]bool)
	for _, session := range sm.sessions {
		if session.TempPath != "" {
			activeFiles[filepath.Base(session.TempPath)] = true
		}
	}

	for _, file := range files {
		if !file.IsDir() && !activeFiles[file.Name()] {
			filePath := filepath.Join(sm.config.TempDir, file.Name())
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}

			// Remove files older than 1 hour
			if time.Since(fileInfo.ModTime()) > time.Hour {
				if err := os.Remove(filePath); err != nil {
					log.Printf("Error removing orphaned temp file: %v", err)
				} else {
					log.Printf("Removed orphaned temp file: %s", file.Name())
				}
			}
		}
	}
}

// logAuditEntry logs an audit entry (in production, this would write to a database or log file)
func (sm *SessionManager) logAuditEntry(entry *AuditLogEntry) {
	// For now, just log to console. In production, this should write to a database or structured log file
	auditJSON, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Error marshaling audit entry: %v", err)
		return
	}

	log.Printf("AUDIT: %s", string(auditJSON))
}

// GetAuditLogs returns audit logs for a specific time range (placeholder implementation)
func (sm *SessionManager) GetAuditLogs(startTime, endTime time.Time, technician string) ([]AuditLogEntry, error) {
	// This is a placeholder implementation. In production, this would query a database
	return []AuditLogEntry{}, fmt.Errorf("audit log retrieval not implemented")
}

// GetStatistics returns transfer statistics
func (sm *SessionManager) GetStatistics() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_sessions":    len(sm.sessions),
		"active_streams":    len(sm.fileStreams),
		"max_concurrent":    sm.config.MaxConcurrent,
		"temp_dir":          sm.config.TempDir,
		"max_file_size":     sm.config.MaxFileSize,
		"allowed_types":     sm.config.AllowedTypes,
	}

	// Count sessions by status
	statusCounts := make(map[TransferStatus]int)
	for _, session := range sm.sessions {
		session.mutex.RLock()
		statusCounts[session.Status]++
		session.mutex.RUnlock()
	}
	stats["status_counts"] = statusCounts

	return stats
}

// Shutdown gracefully shuts down the session manager
func (sm *SessionManager) Shutdown() {
	log.Println("Shutting down transfer session manager...")

	// Signal cleanup routine to stop
	close(sm.shutdownChan)

	// Stop cleanup ticker
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}

	// Cancel all active transfers
	sm.mutex.Lock()
	for id := range sm.sessions {
		if fileStream, exists := sm.fileStreams[id]; exists {
			fileStream.Cancel()
		}
	}
	sm.mutex.Unlock()

	// Perform final cleanup
	sm.performCleanup()

	log.Println("Transfer session manager shutdown complete")
}