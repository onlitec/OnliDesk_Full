package filetransfer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketHandler manages WebSocket connections for file transfers
type WebSocketHandler struct {
	sessionManager *SessionManager
	fileValidator  *FileValidator
	fileEncryptor  *FileEncryptor
	upgrader       websocket.Upgrader
	connections    map[string]*websocket.Conn // sessionID -> connection
	config         *TransferConfig
	auditLogger    *AuditLogger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(config *TransferConfig, securityConfig *SecurityConfig) *WebSocketHandler {
	if config == nil {
		config = DefaultTransferConfig()
	}
	if securityConfig == nil {
		securityConfig = DefaultSecurityConfig()
	}

	return &WebSocketHandler{
		sessionManager: NewSessionManager(config),
		fileValidator:  NewFileValidator(securityConfig),
		fileEncryptor:  NewFileEncryptor(securityConfig.EncryptionKey),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			ReadBufferSize:  1024 * 64,  // 64KB
			WriteBufferSize: 1024 * 64,  // 64KB
		},
		connections: make(map[string]*websocket.Conn),
		config:      config,
		auditLogger: NewAuditLogger("./logs/websocket", true),
	}
}

// HandleWebSocket handles WebSocket connections for file transfers
func (wh *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get client information for audit logging
	ipAddress := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")

	// Upgrade HTTP connection to WebSocket
	conn, err := wh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		// Log connection failure
		wh.auditLogger.LogSecurityViolation("", "", "", fmt.Sprintf("WebSocket upgrade failed: %v", err), ipAddress)
		return
	}
	defer conn.Close()

	// Set connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Handle ping/pong for connection keep-alive
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Log successful WebSocket connection
	wh.auditLogger.LogEvent(&AuditEvent{
		EventType:   "websocket_connected",
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Details:     map[string]interface{}{"connection_type": "websocket"},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("New WebSocket connection established from %s", r.RemoteAddr)

	// Message handling loop
	for {
		// Read message from client
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Handle different message types
		switch messageType {
		case websocket.TextMessage:
			if err := wh.handleTextMessage(conn, message); err != nil {
				log.Printf("Error handling text message: %v", err)
				// Log message handling error
				wh.auditLogger.LogSecurityViolation("", "", "", fmt.Sprintf("Text message error: %v", err), ipAddress)
				wh.sendErrorResponse(conn, "message_error", err.Error())
			}
		case websocket.BinaryMessage:
			if err := wh.handleBinaryMessage(conn, message); err != nil {
				log.Printf("Error handling binary message: %v", err)
				// Log binary message handling error
				wh.auditLogger.LogSecurityViolation("", "", "", fmt.Sprintf("Binary message error: %v", err), ipAddress)
				wh.sendErrorResponse(conn, "binary_error", err.Error())
			}
		case websocket.PingMessage:
			if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				log.Printf("Error sending pong: %v", err)
				return
			}
		}
	}

	// Log WebSocket disconnection
	wh.auditLogger.LogEvent(&AuditEvent{
		EventType:   "websocket_disconnected",
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Details:     map[string]interface{}{"connection_type": "websocket"},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("WebSocket connection closed for %s", r.RemoteAddr)
}

// handleTextMessage processes text-based control messages
func (wh *WebSocketHandler) handleTextMessage(conn *websocket.Conn, message []byte) error {
	// Parse the message as JSON
	var baseMessage struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(message, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse message: %v", err)
	}

	// Handle different message types
	switch baseMessage.Type {
	case "file_transfer_request":
		return wh.handleFileTransferRequest(conn, message)
	case "transfer_approval":
		return wh.handleTransferApproval(conn, message)
	case "transfer_control":
		return wh.handleTransferControl(conn, message)
	case "progress_request":
		return wh.handleProgressRequest(conn, message)
	case "session_register":
		return wh.handleSessionRegister(conn, message)
	case "ping":
		return wh.sendPongResponse(conn)
	default:
		return fmt.Errorf("unknown message type: %s", baseMessage.Type)
	}
}

// handleBinaryMessage processes binary file chunk data
func (wh *WebSocketHandler) handleBinaryMessage(conn *websocket.Conn, message []byte) error {
	// Binary messages should contain file chunks
	// The first part should be a JSON header with transfer info
	if len(message) < 4 {
		return fmt.Errorf("binary message too short")
	}

	// Extract header length (first 4 bytes)
	headerLength := int(message[0])<<24 | int(message[1])<<16 | int(message[2])<<8 | int(message[3])
	if headerLength > len(message)-4 {
		return fmt.Errorf("invalid header length")
	}

	// Parse header
	headerData := message[4 : 4+headerLength]
	chunkData := message[4+headerLength:]

	var chunkHeader FileTransferChunk
	if err := json.Unmarshal(headerData, &chunkHeader); err != nil {
		return fmt.Errorf("failed to parse chunk header: %v", err)
	}

	// Set the actual chunk data
	chunkHeader.Data = chunkData

	// Process the chunk
	return wh.handleFileChunk(conn, &chunkHeader)
}

// handleFileTransferRequest processes file transfer requests
func (wh *WebSocketHandler) handleFileTransferRequest(conn *websocket.Conn, message []byte) error {
	var request FileTransferRequest
	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse transfer request: %v", err)
	}

	log.Printf("Received file transfer request: %s (%d bytes)", request.Filename, request.FileSize)

	// Create transfer session
	session, err := wh.sessionManager.CreateTransferSession(&request, conn, nil)
	if err != nil {
		return fmt.Errorf("failed to create transfer session: %v", err)
	}

	// Send response
	response := FileTransferResponse{
		Type:       "file_transfer_response",
		TransferID: session.ID,
		Status:     string(session.Status),
		Message:    "Transfer request received",
		Timestamp:  time.Now(),
	}

	if wh.config.RequireApproval {
		response.Message = "Transfer request pending approval"
		// In a real implementation, you would notify the portal/technician here
		wh.notifyPortalOfTransferRequest(session)
	} else {
		// Auto-approve if not required
		if err := wh.sessionManager.ApproveTransfer(session.ID, true, "Auto-approved"); err != nil {
			return fmt.Errorf("failed to auto-approve transfer: %v", err)
		}
		response.Status = string(StatusApproved)
		response.Message = "Transfer approved and ready"
	}

	return wh.sendJSONResponse(conn, response)
}

// handleTransferApproval processes transfer approval/rejection from portal
func (wh *WebSocketHandler) handleTransferApproval(conn *websocket.Conn, message []byte) error {
	var approval struct {
		Type       string `json:"type"`
		TransferID string `json:"transfer_id"`
		Approved   bool   `json:"approved"`
		Message    string `json:"message"`
	}

	if err := json.Unmarshal(message, &approval); err != nil {
		return fmt.Errorf("failed to parse approval message: %v", err)
	}

	log.Printf("Transfer approval received: %s - %t", approval.TransferID, approval.Approved)

	// Process approval
	if err := wh.sessionManager.ApproveTransfer(approval.TransferID, approval.Approved, approval.Message); err != nil {
		return fmt.Errorf("failed to process approval: %v", err)
	}

	// Notify client of approval status
	session, exists := wh.sessionManager.GetSession(approval.TransferID)
	if exists {
		response := FileTransferResponse{
			Type:       "transfer_status_update",
			TransferID: approval.TransferID,
			Status:     string(session.Status),
			Message:    approval.Message,
			Timestamp:  time.Now(),
		}

		// Send to client connection
		if session.ClientConn != nil {
			wh.sendJSONResponse(session.ClientConn, response)
		}
	}

	return nil
}

// handleTransferControl processes transfer control commands (pause, resume, cancel)
func (wh *WebSocketHandler) handleTransferControl(conn *websocket.Conn, message []byte) error {
	var control struct {
		Type       string `json:"type"`
		TransferID string `json:"transfer_id"`
		Action     string `json:"action"` // pause, resume, cancel
	}

	if err := json.Unmarshal(message, &control); err != nil {
		return fmt.Errorf("failed to parse control message: %v", err)
	}

	log.Printf("Transfer control received: %s - %s", control.TransferID, control.Action)

	var err error
	switch control.Action {
	case "pause":
		err = wh.sessionManager.PauseTransfer(control.TransferID)
	case "resume":
		err = wh.sessionManager.ResumeTransfer(control.TransferID)
	case "cancel":
		err = wh.sessionManager.CancelTransfer(control.TransferID)
	default:
		return fmt.Errorf("unknown control action: %s", control.Action)
	}

	if err != nil {
		return fmt.Errorf("failed to execute control action: %v", err)
	}

	// Send confirmation
	response := struct {
		Type       string    `json:"type"`
		TransferID string    `json:"transfer_id"`
		Action     string    `json:"action"`
		Status     string    `json:"status"`
		Timestamp  time.Time `json:"timestamp"`
	}{
		Type:       "control_response",
		TransferID: control.TransferID,
		Action:     control.Action,
		Status:     "success",
		Timestamp:  time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handleProgressRequest processes progress information requests
func (wh *WebSocketHandler) handleProgressRequest(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type       string `json:"type"`
		TransferID string `json:"transfer_id"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse progress request: %v", err)
	}

	// Get transfer progress
	progress, err := wh.sessionManager.GetTransferProgress(request.TransferID)
	if err != nil {
		return fmt.Errorf("failed to get transfer progress: %v", err)
	}

	// Send progress response
	response := struct {
		Type     string                `json:"type"`
		Progress *FileTransferProgress `json:"progress"`
	}{
		Type:     "progress_response",
		Progress: progress,
	}

	return wh.sendJSONResponse(conn, response)
}

// handleSessionRegister registers a WebSocket connection with a session ID
func (wh *WebSocketHandler) handleSessionRegister(conn *websocket.Conn, message []byte) error {
	var register struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Role      string `json:"role"` // client, portal
	}

	if err := json.Unmarshal(message, &register); err != nil {
		return fmt.Errorf("failed to parse session register: %v", err)
	}

	// Store connection mapping
	wh.connections[register.SessionID] = conn

	// Log session registration
	wh.auditLogger.LogEvent(&AuditEvent{
		EventType:   "session_registered",
		SessionID:   register.SessionID,
		IPAddress:   conn.RemoteAddr().String(),
		Details:     map[string]interface{}{"role": register.Role, "session_id": register.SessionID},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("WebSocket connection registered for session %s as %s", register.SessionID, register.Role)

	// Send confirmation
	response := struct {
		Type      string    `json:"type"`
		SessionID string    `json:"session_id"`
		Status    string    `json:"status"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "session_registered",
		SessionID: register.SessionID,
		Status:    "success",
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handleFileChunk processes incoming file chunks
func (wh *WebSocketHandler) handleFileChunk(conn *websocket.Conn, chunk *FileTransferChunk) error {
	log.Printf("Received file chunk: transfer=%s, chunk=%d, size=%d", chunk.TransferID, chunk.ChunkIndex, len(chunk.Data))

	// Get the file stream for this transfer
	fileStream, exists := wh.sessionManager.fileStreams[chunk.TransferID]
	if !exists {
		return fmt.Errorf("file stream not found for transfer: %s", chunk.TransferID)
	}

	// Process the chunk
	if err := fileStream.WriteChunk(chunk.ChunkIndex, chunk.Data); err != nil {
		return fmt.Errorf("failed to write chunk: %v", err)
	}

	// Send chunk acknowledgment
	ack := struct {
		Type       string    `json:"type"`
		TransferID string    `json:"transfer_id"`
		ChunkIndex int       `json:"chunk_index"`
		Status     string    `json:"status"`
		Timestamp  time.Time `json:"timestamp"`
	}{
		Type:       "chunk_ack",
		TransferID: chunk.TransferID,
		ChunkIndex: chunk.ChunkIndex,
		Status:     "received",
		Timestamp:  time.Now(),
	}

	return wh.sendJSONResponse(conn, ack)
}

// notifyPortalOfTransferRequest notifies the portal of a new transfer request
func (wh *WebSocketHandler) notifyPortalOfTransferRequest(session *TransferSession) {
	// In a real implementation, this would send a notification to the portal WebSocket connection
	// For now, just log it
	log.Printf("Portal notification: New transfer request %s from %s", session.ID, session.Request.Technician)

	// Find portal connection for this session
	if portalConn, exists := wh.connections[session.Request.SessionID+"_portal"]; exists {
		notification := struct {
			Type    string               `json:"type"`
			Request *FileTransferRequest `json:"request"`
		}{
			Type:    "transfer_request_notification",
			Request: session.Request,
		}

		wh.sendJSONResponse(portalConn, notification)
	}
}

// sendJSONResponse sends a JSON response to a WebSocket connection
func (wh *WebSocketHandler) sendJSONResponse(conn *websocket.Conn, response interface{}) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %v", err)
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// sendErrorResponse sends an error response to a WebSocket connection
func (wh *WebSocketHandler) sendErrorResponse(conn *websocket.Conn, errorType, message string) {
	errorResponse := struct {
		Type      string    `json:"type"`
		Error     string    `json:"error"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "error",
		Error:     errorType,
		Message:   message,
		Timestamp: time.Now(),
	}

	if err := wh.sendJSONResponse(conn, errorResponse); err != nil {
		log.Printf("Failed to send error response: %v", err)
	}
}

// sendPongResponse sends a pong response
func (wh *WebSocketHandler) sendPongResponse(conn *websocket.Conn) error {
	pongResponse := struct {
		Type      string    `json:"type"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "pong",
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, pongResponse)
}

// GetSessionManager returns the session manager (for testing or external access)
func (wh *WebSocketHandler) GetSessionManager() *SessionManager {
	return wh.sessionManager
}

// GetFileValidator returns the file validator (for testing or external access)
func (wh *WebSocketHandler) GetFileValidator() *FileValidator {
	return wh.fileValidator
}

// GetStatistics returns handler statistics
func (wh *WebSocketHandler) GetStatistics() map[string]interface{} {
	stats := wh.sessionManager.GetStatistics()
	stats["active_connections"] = len(wh.connections)
	return stats
}

// Shutdown gracefully shuts down the WebSocket handler
func (wh *WebSocketHandler) Shutdown() {
	log.Println("Shutting down WebSocket handler...")

	// Close all connections
	for sessionID, conn := range wh.connections {
		log.Printf("Closing connection for session: %s", sessionID)
		conn.Close()
	}

	// Shutdown session manager
	wh.sessionManager.Shutdown()

	log.Println("WebSocket handler shutdown complete")
}