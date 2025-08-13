package remoteaccess

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketHandler manages WebSocket connections for remote access
type WebSocketHandler struct {
	sessionManager *SessionManager
	upgrader       websocket.Upgrader
	config         *RemoteAccessConfig
	auditLogger    *AuditLogger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(config *RemoteAccessConfig) *WebSocketHandler {
	if config == nil {
		config = DefaultRemoteAccessConfig()
	}

	return &WebSocketHandler{
		sessionManager: NewSessionManager(config),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			ReadBufferSize:  1024 * 64,  // 64KB
			WriteBufferSize: 1024 * 64,  // 64KB
		},
		config:      config,
		auditLogger: NewAuditLogger("./logs/remoteaccess", true),
	}
}

// HandleWebSocket handles WebSocket connections for remote access
func (wh *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get client information for audit logging
	ipAddress := r.RemoteAddr
	userAgent := r.Header.Get("User-Agent")

	// Upgrade HTTP connection to WebSocket
	conn, err := wh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
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
	wh.auditLogger.LogEvent(AuditEvent{
		EventType:   "websocket_connected",
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Details:     map[string]interface{}{"connection_type": "remoteaccess"},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("New remote access WebSocket connection established from %s", r.RemoteAddr)

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

		if messageType == websocket.TextMessage {
			if err := wh.handleMessage(conn, message); err != nil {
				log.Printf("Error handling message: %v", err)
				wh.sendErrorResponse(conn, err.Error())
			}
		}
	}

	log.Printf("Remote access WebSocket connection closed from %s", r.RemoteAddr)
}

// handleMessage processes incoming WebSocket messages
func (wh *WebSocketHandler) handleMessage(conn *websocket.Conn, message []byte) error {
	var baseMessage struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(message, &baseMessage); err != nil {
		return fmt.Errorf("failed to parse message: %v", err)
	}

	switch baseMessage.Type {
	case "session_register":
		return wh.handleSessionRegister(conn, message)
	case "session_create":
		return wh.handleSessionCreate(conn, message)
	case "session_join":
		return wh.handleSessionJoin(conn, message)
	case "session_terminate":
		return wh.handleSessionTerminate(conn, message)
	case "privilege_request":
		return wh.handlePrivilegeRequest(conn, message)
	case "privilege_response":
		return wh.handlePrivilegeResponse(conn, message)
	case "privilege_revoke":
		return wh.handlePrivilegeRevoke(conn, message)
	case "control_command":
		return wh.handleControlCommand(conn, message)
	case "screen_capture":
		return wh.handleScreenCapture(conn, message)
	case "input_event":
		return wh.handleInputEvent(conn, message)
	case "file_transfer_request":
		return wh.handleFileTransferRequest(conn, message)
	case "heartbeat":
		return wh.handleHeartbeat(conn, message)
	default:
		return fmt.Errorf("unknown message type: %s", baseMessage.Type)
	}
}

// handleSessionRegister registers a WebSocket connection with a session
func (wh *WebSocketHandler) handleSessionRegister(conn *websocket.Conn, message []byte) error {
	var register struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Role      string `json:"role"` // client, portal
		ClientID  string `json:"client_id,omitempty"`
		Technician string `json:"technician,omitempty"`
	}

	if err := json.Unmarshal(message, &register); err != nil {
		return fmt.Errorf("failed to parse session register: %v", err)
	}

	// Register connection with session manager
	err := wh.sessionManager.RegisterConnection(register.SessionID, conn, register.Role)
	if err != nil {
		return fmt.Errorf("failed to register connection: %v", err)
	}

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

// handleSessionCreate creates a new remote access session
func (wh *WebSocketHandler) handleSessionCreate(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type         string      `json:"type"`
		ClientID     string      `json:"client_id"`
		TechnicianID string      `json:"technician_id"`
		ClientInfo   *ClientInfo `json:"client_info"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse session create request: %v", err)
	}

	// Create new session
	session, err := wh.sessionManager.CreateSession(request.ClientID, request.TechnicianID, request.ClientInfo)
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}

	// Register the connection
	err = wh.sessionManager.RegisterConnection(session.ID, conn, "client")
	if err != nil {
		return fmt.Errorf("failed to register connection: %v", err)
	}

	// Send response
	response := struct {
		Type      string                   `json:"type"`
		SessionID string                   `json:"session_id"`
		Session   *RemoteAccessSession     `json:"session"`
		Status    string                   `json:"status"`
		Message   string                   `json:"message"`
		Timestamp time.Time               `json:"timestamp"`
	}{
		Type:      "session_created",
		SessionID: session.ID,
		Session:   session,
		Status:    "success",
		Message:   "Remote access session created successfully",
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handleSessionJoin handles portal joining an existing session
func (wh *WebSocketHandler) handleSessionJoin(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type         string `json:"type"`
		SessionID    string `json:"session_id"`
		TechnicianID string `json:"technician_id"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse session join request: %v", err)
	}

	// Get session
	session, exists := wh.sessionManager.GetSession(request.SessionID)
	if !exists {
		return fmt.Errorf("session not found")
	}

	// Register portal connection
	err := wh.sessionManager.RegisterConnection(request.SessionID, conn, "portal")
	if err != nil {
		return fmt.Errorf("failed to register portal connection: %v", err)
	}

	// Send response
	response := struct {
		Type      string               `json:"type"`
		SessionID string               `json:"session_id"`
		Session   *RemoteAccessSession `json:"session"`
		Status    string               `json:"status"`
		Message   string               `json:"message"`
		Timestamp time.Time            `json:"timestamp"`
	}{
		Type:      "session_joined",
		SessionID: session.ID,
		Session:   session,
		Status:    "success",
		Message:   "Successfully joined remote access session",
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handleSessionTerminate terminates a session
func (wh *WebSocketHandler) handleSessionTerminate(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Reason    string `json:"reason,omitempty"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse session terminate request: %v", err)
	}

	// Terminate session
	err := wh.sessionManager.TerminateSession(request.SessionID)
	if err != nil {
		return fmt.Errorf("failed to terminate session: %v", err)
	}

	// Send response
	response := struct {
		Type      string    `json:"type"`
		SessionID string    `json:"session_id"`
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "session_terminated",
		SessionID: request.SessionID,
		Status:    "success",
		Message:   "Session terminated successfully",
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handlePrivilegeRequest handles privilege escalation requests
func (wh *WebSocketHandler) handlePrivilegeRequest(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type          string        `json:"type"`
		SessionID     string        `json:"session_id"`
		PrivilegeType PrivilegeType `json:"privilege_type"`
		Justification string        `json:"justification"`
		Duration      time.Duration `json:"duration"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse privilege request: %v", err)
	}

	// Request privilege
	requestID, err := wh.sessionManager.RequestPrivilege(request.SessionID, request.PrivilegeType, request.Justification, request.Duration)
	if err != nil {
		return fmt.Errorf("failed to request privilege: %v", err)
	}

	// Send response
	response := struct {
		Type      string    `json:"type"`
		RequestID string    `json:"request_id"`
		SessionID string    `json:"session_id"`
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "privilege_requested",
		RequestID: requestID,
		SessionID: request.SessionID,
		Status:    "pending",
		Message:   "Privilege request submitted for approval",
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handlePrivilegeResponse handles privilege approval/denial responses
func (wh *WebSocketHandler) handlePrivilegeResponse(conn *websocket.Conn, message []byte) error {
	var response struct {
		Type       string `json:"type"`
		SessionID  string `json:"session_id"`
		RequestID  string `json:"request_id"`
		Approved   bool   `json:"approved"`
		ApprovedBy string `json:"approved_by"`
		Reason     string `json:"reason,omitempty"`
	}

	if err := json.Unmarshal(message, &response); err != nil {
		return fmt.Errorf("failed to parse privilege response: %v", err)
	}

	var err error
	if response.Approved {
		err = wh.sessionManager.ApprovePrivilege(response.SessionID, response.RequestID, response.ApprovedBy)
	} else {
		err = wh.sessionManager.DenyPrivilege(response.SessionID, response.RequestID, response.ApprovedBy)
	}

	if err != nil {
		return fmt.Errorf("failed to process privilege response: %v", err)
	}

	// Send confirmation
	confirmation := struct {
		Type      string    `json:"type"`
		RequestID string    `json:"request_id"`
		SessionID string    `json:"session_id"`
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "privilege_response_processed",
		RequestID: response.RequestID,
		SessionID: response.SessionID,
		Status:    "success",
		Timestamp: time.Now(),
	}

	if response.Approved {
		confirmation.Message = "Privilege request approved"
	} else {
		confirmation.Message = "Privilege request denied"
	}

	return wh.sendJSONResponse(conn, confirmation)
}

// handlePrivilegeRevoke handles privilege revocation
func (wh *WebSocketHandler) handlePrivilegeRevoke(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type          string        `json:"type"`
		SessionID     string        `json:"session_id"`
		PrivilegeType PrivilegeType `json:"privilege_type"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse privilege revoke request: %v", err)
	}

	// Revoke privilege
	err := wh.sessionManager.RevokePrivilege(request.SessionID, request.PrivilegeType)
	if err != nil {
		return fmt.Errorf("failed to revoke privilege: %v", err)
	}

	// Send response
	response := struct {
		Type      string    `json:"type"`
		SessionID string    `json:"session_id"`
		Status    string    `json:"status"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "privilege_revoked",
		SessionID: request.SessionID,
		Status:    "success",
		Message:   fmt.Sprintf("Privilege %s revoked successfully", request.PrivilegeType),
		Timestamp: time.Now(),
	}

	return wh.sendJSONResponse(conn, response)
}

// handleControlCommand handles remote control commands
func (wh *WebSocketHandler) handleControlCommand(conn *websocket.Conn, message []byte) error {
	var command struct {
		Type      string                 `json:"type"`
		SessionID string                 `json:"session_id"`
		Command   string                 `json:"command"`
		Params    map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.Unmarshal(message, &command); err != nil {
		return fmt.Errorf("failed to parse control command: %v", err)
	}

	// Get session and update activity
	session, exists := wh.sessionManager.GetSession(command.SessionID)
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.IncrementCommand(command.Command)

	// Forward command to client if this is from portal
	if session.ClientConn != nil && session.ClientConn != conn {
		return wh.sendJSONResponse(session.ClientConn, command)
	}

	return nil
}

// handleScreenCapture handles screen capture requests
func (wh *WebSocketHandler) handleScreenCapture(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Quality   int    `json:"quality,omitempty"`
		Format    string `json:"format,omitempty"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse screen capture request: %v", err)
	}

	// Get session and update activity
	session, exists := wh.sessionManager.GetSession(request.SessionID)
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.IncrementScreenshot()

	// Forward request to client
	if session.ClientConn != nil {
		return wh.sendJSONResponse(session.ClientConn, request)
	}

	return fmt.Errorf("client not connected")
}

// handleInputEvent handles input events (mouse, keyboard)
func (wh *WebSocketHandler) handleInputEvent(conn *websocket.Conn, message []byte) error {
	var event struct {
		Type      string                 `json:"type"`
		SessionID string                 `json:"session_id"`
		EventType string                 `json:"event_type"`
		Data      map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(message, &event); err != nil {
		return fmt.Errorf("failed to parse input event: %v", err)
	}

	// Get session and update activity
	session, exists := wh.sessionManager.GetSession(event.SessionID)
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.UpdateActivity()

	// Forward event to client
	if session.ClientConn != nil {
		return wh.sendJSONResponse(session.ClientConn, event)
	}

	return fmt.Errorf("client not connected")
}

// handleFileTransferRequest handles file transfer requests
func (wh *WebSocketHandler) handleFileTransferRequest(conn *websocket.Conn, message []byte) error {
	var request struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id"`
		Action    string `json:"action"` // start, approve, deny
		Filename  string `json:"filename,omitempty"`
		FileSize  int64  `json:"file_size,omitempty"`
	}

	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to parse file transfer request: %v", err)
	}

	// Get session and update activity
	session, exists := wh.sessionManager.GetSession(request.SessionID)
	if !exists {
		return fmt.Errorf("session not found")
	}

	if request.Action == "start" {
		session.AddFileTransfer(request.FileSize)
	}

	// Forward to appropriate connection
	if session.ClientConn != nil && session.ClientConn != conn {
		return wh.sendJSONResponse(session.ClientConn, request)
	} else if session.PortalConn != nil && session.PortalConn != conn {
		return wh.sendJSONResponse(session.PortalConn, request)
	}

	return nil
}

// handleHeartbeat handles heartbeat messages
func (wh *WebSocketHandler) handleHeartbeat(conn *websocket.Conn, message []byte) error {
	var heartbeat struct {
		Type      string `json:"type"`
		SessionID string `json:"session_id,omitempty"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(message, &heartbeat); err != nil {
		return fmt.Errorf("failed to parse heartbeat: %v", err)
	}

	// Update session activity if session ID provided
	if heartbeat.SessionID != "" {
		if session, exists := wh.sessionManager.GetSession(heartbeat.SessionID); exists {
			session.UpdateActivity()
		}
	}

	// Send heartbeat response
	response := struct {
		Type      string `json:"type"`
		Timestamp int64  `json:"timestamp"`
	}{
		Type:      "heartbeat_response",
		Timestamp: time.Now().Unix(),
	}

	return wh.sendJSONResponse(conn, response)
}

// sendJSONResponse sends a JSON response to the WebSocket connection
func (wh *WebSocketHandler) sendJSONResponse(conn *websocket.Conn, response interface{}) error {
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %v", err)
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// sendErrorResponse sends an error response to the WebSocket connection
func (wh *WebSocketHandler) sendErrorResponse(conn *websocket.Conn, errorMessage string) {
	errorResponse := struct {
		Type      string    `json:"type"`
		Error     string    `json:"error"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Type:      "error",
		Error:     errorMessage,
		Timestamp: time.Now(),
	}

	wh.sendJSONResponse(conn, errorResponse)
}

// GetSessionManager returns the session manager
func (wh *WebSocketHandler) GetSessionManager() *SessionManager {
	return wh.sessionManager
}

// GetStatistics returns WebSocket handler statistics
func (wh *WebSocketHandler) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"session_stats": wh.sessionManager.GetStatistics(),
		"config":        wh.config,
	}
}

// Shutdown gracefully shuts down the WebSocket handler
func (wh *WebSocketHandler) Shutdown() {
	log.Println("Shutting down WebSocket handler...")
	
	// Shutdown session manager
	if wh.sessionManager != nil {
		wh.sessionManager.Shutdown()
	}
	
	log.Println("WebSocket handler shutdown complete")
}