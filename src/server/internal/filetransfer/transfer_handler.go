package filetransfer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TransferType defines the type of file transfer
type TransferType string

const (
	TransferTypeUpload   TransferType = "upload"
	TransferTypeDownload TransferType = "download"
)

// TransferStatus defines the current status of a transfer
type TransferStatus string

const (
	StatusPending    TransferStatus = "pending"
	StatusApproved   TransferStatus = "approved"
	StatusRejected   TransferStatus = "rejected"
	StatusInProgress TransferStatus = "in_progress"
	StatusPaused     TransferStatus = "paused"
	StatusCompleted  TransferStatus = "completed"
	StatusCancelled  TransferStatus = "cancelled"
	StatusFailed     TransferStatus = "failed"
)

// FileTransferRequest represents a file transfer request
type FileTransferRequest struct {
	ID          string       `json:"id"`
	SessionID   string       `json:"session_id"`
	Type        TransferType `json:"type"`
	Filename    string       `json:"filename"`
	FileSize    int64        `json:"file_size"`
	Checksum    string       `json:"checksum,omitempty"`
	Timestamp   time.Time    `json:"timestamp"`
	Technician  string       `json:"technician"`
}

// FileTransferResponse represents a response to a transfer request
type FileTransferResponse struct {
	Type       string    `json:"type"`
	TransferID string    `json:"transfer_id"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	Approved   bool      `json:"approved"`
	Timestamp  time.Time `json:"timestamp"`
}

// FileTransferProgress represents transfer progress information
type FileTransferProgress struct {
	ID              string  `json:"id"`
	BytesTransferred int64   `json:"bytes_transferred"`
	TotalBytes      int64   `json:"total_bytes"`
	Percentage      float64 `json:"percentage"`
	Speed           int64   `json:"speed"` // bytes per second
	ETA             int64   `json:"eta"`   // estimated time remaining in seconds
}

// FileChunk represents a chunk of file data
type FileChunk struct {
	ID          string `json:"id"`
	Sequence    int    `json:"sequence"`
	Data        []byte `json:"data"`
	Size        int    `json:"size"`
	IsLast      bool   `json:"is_last"`
	Checksum    string `json:"checksum"`
}

// FileTransferChunk represents a chunk of file data for WebSocket transfer
type FileTransferChunk struct {
	TransferID string `json:"transfer_id"`
	ChunkIndex int    `json:"chunk_index"`
	Data       []byte `json:"data"`
	Checksum   string `json:"checksum"`
	IsLast     bool   `json:"is_last"`
}

// TransferSession manages an active file transfer
type TransferSession struct {
	ID           string
	Request      *FileTransferRequest
	Status       TransferStatus
	StartTime    time.Time
	EndTime      *time.Time
	BytesTransferred int64
	TotalChunks  int
	ReceivedChunks map[int]bool
	File         *os.File
	TempPath     string
	Checksum     string
	ClientConn   *websocket.Conn
	PortalConn   *websocket.Conn
	mutex        sync.RWMutex
}

// TransferHandler manages file transfer operations
type TransferHandler struct {
	activeSessions map[string]*TransferSession
	maxFileSize    int64
	allowedTypes   map[string]bool
	tempDir        string
	auditLogger    *AuditLogger
	mutex          sync.RWMutex
}

// NewTransferHandler creates a new file transfer handler
func NewTransferHandler(maxFileSize int64, allowedTypes []string, tempDir string) *TransferHandler {
	allowedTypesMap := make(map[string]bool)
	for _, ext := range allowedTypes {
		allowedTypesMap[ext] = true
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("Failed to create temp directory: %v", err)
	}

	// Initialize audit logger
	auditLogger := NewAuditLogger("./logs/audit", true)

	return &TransferHandler{
		activeSessions: make(map[string]*TransferSession),
		maxFileSize:    maxFileSize,
		allowedTypes:   allowedTypesMap,
		tempDir:        tempDir,
		auditLogger:    auditLogger,
	}
}

// HandleTransferRequest processes a new file transfer request
func (h *TransferHandler) HandleTransferRequest(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Configure properly for production
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Handle WebSocket messages
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		if messageType == websocket.TextMessage {
			h.handleControlMessage(conn, message)
		} else if messageType == websocket.BinaryMessage {
			h.handleFileChunk(conn, message)
		}
	}
}

// handleControlMessage processes control messages (JSON)
func (h *TransferHandler) handleControlMessage(conn *websocket.Conn, message []byte) {
	var msgType struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(message, &msgType); err != nil {
		log.Printf("Error parsing message type: %v", err)
		return
	}

	switch msgType.Type {
	case "transfer_request":
		h.handleNewTransferRequest(conn, message)
	case "transfer_response":
		h.handleTransferResponse(conn, message)
	case "transfer_control":
		h.handleTransferControl(conn, message)
	case "progress_request":
		h.handleProgressRequest(conn, message)
	default:
		log.Printf("Unknown message type: %s", msgType.Type)
	}
}

// handleNewTransferRequest processes a new transfer request
func (h *TransferHandler) handleNewTransferRequest(conn *websocket.Conn, message []byte) {
	var request FileTransferRequest
	if err := json.Unmarshal(message, &request); err != nil {
		log.Printf("Error parsing transfer request: %v", err)
		return
	}

	// Get client IP for audit logging
	ipAddress := conn.RemoteAddr().String()
	userAgent := "" // Would be extracted from headers in a real implementation

	// Validate file size
	if request.FileSize > h.maxFileSize {
		errorMsg := fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", h.maxFileSize)
		h.auditLogger.LogSecurityViolation(request.ID, request.SessionID, request.Filename, errorMsg, ipAddress)
		h.sendError(conn, request.ID, errorMsg)
		return
	}

	// Validate file type
	ext := filepath.Ext(request.Filename)
	if len(h.allowedTypes) > 0 && !h.allowedTypes[ext] {
		errorMsg := fmt.Sprintf("File type %s is not allowed", ext)
		h.auditLogger.LogSecurityViolation(request.ID, request.SessionID, request.Filename, errorMsg, ipAddress)
		h.sendError(conn, request.ID, errorMsg)
		return
	}

	// Log transfer request
	h.auditLogger.LogTransferRequest(&request, ipAddress, userAgent)

	// Create transfer session
	session := &TransferSession{
		ID:             request.ID,
		Request:        &request,
		Status:         StatusPending,
		StartTime:      time.Now(),
		ReceivedChunks: make(map[int]bool),
		PortalConn:     conn,
	}

	h.mutex.Lock()
	h.activeSessions[request.ID] = session
	h.mutex.Unlock()

	// Forward request to client for approval
	// This would be implemented with the session management from Story 1.1
	log.Printf("Transfer request created: %s", request.ID)

	// Send confirmation to portal
	response := map[string]interface{}{
		"type":    "transfer_request_received",
		"id":      request.ID,
		"status":  StatusPending,
		"message": "Transfer request sent to client for approval",
	}
	h.sendJSON(conn, response)
}

// handleTransferResponse processes client's response to transfer request
func (h *TransferHandler) handleTransferResponse(conn *websocket.Conn, message []byte) {
	var response FileTransferResponse
	if err := json.Unmarshal(message, &response); err != nil {
		log.Printf("Error parsing transfer response: %v", err)
		return
	}

	h.mutex.Lock()
	session, exists := h.activeSessions[response.TransferID]
	h.mutex.Unlock()

	if !exists {
		log.Printf("Transfer session not found: %s", response.TransferID)
		return
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	if response.Approved {
		session.Status = StatusApproved
		session.ClientConn = conn

		// Log transfer approval
		h.auditLogger.LogTransferApproval(response.TransferID, session.Request.SessionID, true, response.Message, "")

		// Create temporary file for transfer
		tempPath := filepath.Join(h.tempDir, fmt.Sprintf("transfer_%s_%s", response.TransferID, session.Request.Filename))
		file, err := os.Create(tempPath)
		if err != nil {
			log.Printf("Error creating temp file: %v", err)
			session.Status = StatusFailed
			// Log transfer failure
			h.auditLogger.LogTransferProgress(response.TransferID, session.Request.SessionID, AuditEventTransferFailed, map[string]interface{}{
				"error": "Failed to create temporary file",
				"details": err.Error(),
			})
			return
		}
		session.File = file
		session.TempPath = tempPath

		log.Printf("Transfer approved: %s", response.TransferID)
	} else {
		session.Status = StatusRejected
		// Log transfer rejection
		h.auditLogger.LogTransferApproval(response.TransferID, session.Request.SessionID, false, response.Message, "")
		log.Printf("Transfer rejected: %s", response.TransferID)
	}

	// Notify portal of the response
	if session.PortalConn != nil {
		notification := map[string]interface{}{
			"type":     "transfer_response",
			"id":       response.TransferID,
			"status":   session.Status,
			"approved": response.Approved,
			"message":  response.Message,
		}
		h.sendJSON(session.PortalConn, notification)
	}
}

// handleFileChunk processes incoming file data chunks
func (h *TransferHandler) handleFileChunk(conn *websocket.Conn, data []byte) {
	// Parse chunk header (first part should contain metadata)
	var chunk FileChunk
	if err := json.Unmarshal(data[:256], &chunk); err != nil {
		log.Printf("Error parsing chunk header: %v", err)
		return
	}

	h.mutex.RLock()
	session, exists := h.activeSessions[chunk.ID]
	h.mutex.RUnlock()

	if !exists {
		log.Printf("Transfer session not found for chunk: %s", chunk.ID)
		return
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.Status != StatusApproved && session.Status != StatusInProgress {
		log.Printf("Transfer not in valid state for chunks: %s", session.Status)
		return
	}

	// Update status to in progress
	if session.Status == StatusApproved {
		session.Status = StatusInProgress
	}

	// Write chunk data to file
	chunkData := data[256:] // Skip header
	if _, err := session.File.Write(chunkData); err != nil {
		log.Printf("Error writing chunk to file: %v", err)
		session.Status = StatusFailed
		return
	}

	// Update progress
	session.BytesTransferred += int64(len(chunkData))
	session.ReceivedChunks[chunk.Sequence] = true

	// Send progress update
	progress := FileTransferProgress{
		ID:               chunk.ID,
		BytesTransferred: session.BytesTransferred,
		TotalBytes:       session.Request.FileSize,
		Percentage:       float64(session.BytesTransferred) / float64(session.Request.FileSize) * 100,
	}

	// Send progress to both client and portal
	progressMsg := map[string]interface{}{
		"type":     "transfer_progress",
		"progress": progress,
	}

	if session.ClientConn != nil {
		h.sendJSON(session.ClientConn, progressMsg)
	}
	if session.PortalConn != nil {
		h.sendJSON(session.PortalConn, progressMsg)
	}

	// Check if transfer is complete
	if chunk.IsLast {
		h.completeTransfer(session)
	}
}

// completeTransfer finalizes a file transfer
func (h *TransferHandler) completeTransfer(session *TransferSession) {
	if session.File != nil {
		session.File.Close()
	}

	// Verify file integrity
	if session.Request.Checksum != "" {
		if err := h.verifyChecksum(session.TempPath, session.Request.Checksum); err != nil {
			log.Printf("Checksum verification failed: %v", err)
			session.Status = StatusFailed
			// Log transfer failure due to checksum mismatch
			h.auditLogger.LogTransferProgress(session.ID, session.Request.SessionID, AuditEventTransferFailed, map[string]interface{}{
				"error": "Checksum verification failed",
				"expected_checksum": session.Request.Checksum,
				"details": err.Error(),
			})
			h.cleanupTransfer(session)
			return
		}
	}

	session.Status = StatusCompleted
	now := time.Now()
	session.EndTime = &now

	// Log successful transfer completion
	duration := now.Sub(session.StartTime)
	h.auditLogger.LogTransferProgress(session.ID, session.Request.SessionID, AuditEventTransferCompleted, map[string]interface{}{
		"filename": session.Request.Filename,
		"file_size": session.Request.FileSize,
		"bytes_transferred": session.BytesTransferred,
		"duration_seconds": duration.Seconds(),
		"transfer_speed": float64(session.BytesTransferred) / duration.Seconds(),
	})

	// Notify completion
	completionMsg := map[string]interface{}{
		"type":    "transfer_completed",
		"id":      session.ID,
		"status":  StatusCompleted,
		"message": "File transfer completed successfully",
	}

	if session.ClientConn != nil {
		h.sendJSON(session.ClientConn, completionMsg)
	}
	if session.PortalConn != nil {
		h.sendJSON(session.PortalConn, completionMsg)
	}

	log.Printf("Transfer completed: %s", session.ID)

	// Clean up after successful transfer
	h.cleanupTransfer(session)
}

// verifyChecksum verifies file integrity using SHA-256
func (h *TransferHandler) verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualChecksum := fmt.Sprintf("%x", hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// cleanupTransfer removes temporary files and session data
func (h *TransferHandler) cleanupTransfer(session *TransferSession) {
	if session.TempPath != "" {
		if err := os.Remove(session.TempPath); err != nil {
			log.Printf("Error removing temp file: %v", err)
		}
	}

	h.mutex.Lock()
	delete(h.activeSessions, session.ID)
	h.mutex.Unlock()
}

// sendJSON sends a JSON message over WebSocket
func (h *TransferHandler) sendJSON(conn *websocket.Conn, data interface{}) {
	message, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// sendError sends an error message
func (h *TransferHandler) sendError(conn *websocket.Conn, transferID, errorMsg string) {
	errorResponse := map[string]interface{}{
		"type":    "transfer_error",
		"id":      transferID,
		"error":   errorMsg,
		"status":  StatusFailed,
	}
	h.sendJSON(conn, errorResponse)
}

// handleTransferControl processes transfer control commands (pause, resume, cancel)
func (h *TransferHandler) handleTransferControl(conn *websocket.Conn, message []byte) {
	var control struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Action    string `json:"action"`
	}

	if err := json.Unmarshal(message, &control); err != nil {
		log.Printf("Error parsing control message: %v", err)
		return
	}

	h.mutex.RLock()
	session, exists := h.activeSessions[control.ID]
	h.mutex.RUnlock()

	if !exists {
		log.Printf("Transfer session not found: %s", control.ID)
		return
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	switch control.Action {
	case "pause":
		if session.Status == StatusInProgress {
			session.Status = StatusPaused
			// Log transfer pause
			h.auditLogger.LogTransferProgress(session.ID, session.Request.SessionID, AuditEventTransferPaused, map[string]interface{}{
				"filename": session.Request.Filename,
				"bytes_transferred": session.BytesTransferred,
				"total_bytes": session.Request.FileSize,
			})
			log.Printf("Transfer paused: %s", control.ID)
		}
	case "resume":
		if session.Status == StatusPaused {
			session.Status = StatusInProgress
			// Log transfer resume
			h.auditLogger.LogTransferProgress(session.ID, session.Request.SessionID, AuditEventTransferResumed, map[string]interface{}{
				"filename": session.Request.Filename,
				"bytes_transferred": session.BytesTransferred,
				"total_bytes": session.Request.FileSize,
			})
			log.Printf("Transfer resumed: %s", control.ID)
		}
	case "cancel":
		session.Status = StatusCancelled
		// Log transfer cancellation
		h.auditLogger.LogTransferProgress(session.ID, session.Request.SessionID, AuditEventTransferCancelled, map[string]interface{}{
			"filename": session.Request.Filename,
			"bytes_transferred": session.BytesTransferred,
			"total_bytes": session.Request.FileSize,
			"reason": "User cancelled",
		})
		h.cleanupTransfer(session)
		log.Printf("Transfer cancelled: %s", control.ID)
	}

	// Notify both connections of status change
	statusMsg := map[string]interface{}{
		"type":   "transfer_status",
		"id":     control.ID,
		"status": session.Status,
		"action": control.Action,
	}

	if session.ClientConn != nil {
		h.sendJSON(session.ClientConn, statusMsg)
	}
	if session.PortalConn != nil {
		h.sendJSON(session.PortalConn, statusMsg)
	}
}

// handleProgressRequest sends current progress information
func (h *TransferHandler) handleProgressRequest(conn *websocket.Conn, message []byte) {
	var request struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		log.Printf("Error parsing progress request: %v", err)
		return
	}

	h.mutex.RLock()
	session, exists := h.activeSessions[request.ID]
	h.mutex.RUnlock()

	if !exists {
		log.Printf("Transfer session not found: %s", request.ID)
		return
	}

	session.mutex.RLock()
	progress := FileTransferProgress{
		ID:               request.ID,
		BytesTransferred: session.BytesTransferred,
		TotalBytes:       session.Request.FileSize,
		Percentage:       float64(session.BytesTransferred) / float64(session.Request.FileSize) * 100,
	}
	session.mutex.RUnlock()

	progressMsg := map[string]interface{}{
		"type":     "transfer_progress",
		"progress": progress,
	}

	h.sendJSON(conn, progressMsg)
}

// GetActiveTransfers returns information about all active transfers
func (h *TransferHandler) GetActiveTransfers() map[string]*TransferSession {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string]*TransferSession)
	for id, session := range h.activeSessions {
		result[id] = session
	}
	return result
}