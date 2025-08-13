package remoteaccess

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SessionManager manages all remote access sessions
type SessionManager struct {
	sessions      map[string]*RemoteAccessSession
	connections   map[string]*websocket.Conn // sessionID -> connection
	config        *RemoteAccessConfig
	mutex         sync.RWMutex
	cleanupTicker *time.Ticker
	shutdownChan  chan bool
	auditLogger   *AuditLogger
}



// NewSessionManager creates a new session manager
func NewSessionManager(config *RemoteAccessConfig) *SessionManager {
	if config == nil {
		config = DefaultRemoteAccessConfig()
	}

	sm := &SessionManager{
		sessions:     make(map[string]*RemoteAccessSession),
		connections:  make(map[string]*websocket.Conn),
		config:       config,
		shutdownChan: make(chan bool),
		auditLogger:  NewAuditLogger("./logs/remoteaccess", true),
	}

	// Start cleanup routine
	sm.startCleanupRoutine()

	return sm
}

// CreateSession creates a new remote access session
func (sm *SessionManager) CreateSession(clientID, portalID string, clientInfo *ClientInfo) (*RemoteAccessSession, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check session limit
	if len(sm.sessions) >= sm.config.MaxConcurrentSessions {
		return nil, fmt.Errorf("maximum number of sessions reached")
	}

	// Create new session
	session := NewRemoteAccessSession(clientID, portalID, clientInfo)
	session.Settings = &SessionSettings{
		AllowFileTransfer:   sm.config.FileTransferEnabled,
		AllowClipboard:      true, // Default value
		AllowPrinting:        true, // Default value
		SessionTimeout:       sm.config.SessionTimeout,
		IdleTimeout:          sm.config.IdleTimeout,
		RecordSession:       false, // Default value
		RequireApproval:     sm.config.PrivilegeEscalation.RequireApproval,
		MaxPrivilegeDuration: sm.config.PrivilegeEscalation.MaxPrivilegeDuration,
	}

	sm.sessions[session.ID] = session

	// Log session creation
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "session_created",
		SessionID:   session.ID,
		ClientID:    clientID,
		Technician:  portalID,
		IPAddress:   clientInfo.IPAddress,
		Details:     map[string]interface{}{"hostname": clientInfo.Hostname, "os": clientInfo.OperatingSystem},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("Created remote access session %s for client %s with technician %s", session.ID, clientID, portalID)

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*RemoteAccessSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

// GetAllSessions returns all active sessions
func (sm *SessionManager) GetAllSessions() []*RemoteAccessSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	sessions := make([]*RemoteAccessSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetActiveSessions returns all active sessions
func (sm *SessionManager) GetActiveSessions() []*RemoteAccessSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var activeSessions []*RemoteAccessSession
	for _, session := range sm.sessions {
		if session.Status == StatusActive || session.Status == StatusPending {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions
}

// GetSessionsByTechnician returns sessions for a specific technician
func (sm *SessionManager) GetSessionsByTechnician(technicianID string) []*RemoteAccessSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var sessions []*RemoteAccessSession
	for _, session := range sm.sessions {
		if session.TechnicianID == technicianID {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetSessionsByClient returns sessions for a specific client
func (sm *SessionManager) GetSessionsByClient(clientID string) []*RemoteAccessSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var sessions []*RemoteAccessSession
	for _, session := range sm.sessions {
		if session.ClientID == clientID {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// RegisterConnection registers a WebSocket connection for a session
func (sm *SessionManager) RegisterConnection(sessionID string, conn *websocket.Conn, role string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	// Store connection based on role
	connectionKey := fmt.Sprintf("%s_%s", sessionID, role)
	sm.connections[connectionKey] = conn

	if role == "client" {
		session.ClientConn = conn
		session.Status = StatusActive
	} else if role == "portal" {
		session.PortalConn = conn
	}

	session.UpdateActivity()

	// Log connection registration
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "connection_registered",
		SessionID:   sessionID,
		IPAddress:   conn.RemoteAddr().String(),
		Details:     map[string]interface{}{"role": role},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("Registered %s connection for session %s", role, sessionID)

	return nil
}

// UnregisterConnection removes a WebSocket connection
func (sm *SessionManager) UnregisterConnection(sessionID, role string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	connectionKey := fmt.Sprintf("%s_%s", sessionID, role)
	delete(sm.connections, connectionKey)

	if session, exists := sm.sessions[sessionID]; exists {
		if role == "client" {
			session.ClientConn = nil
			session.Status = StatusDisconnected
		} else if role == "portal" {
			session.PortalConn = nil
		}
	}

	log.Printf("Unregistered %s connection for session %s", role, sessionID)
}

// TerminateSession terminates a session
func (sm *SessionManager) TerminateSession(sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.Terminate()

	// Remove connections
	delete(sm.connections, fmt.Sprintf("%s_client", sessionID))
	delete(sm.connections, fmt.Sprintf("%s_portal", sessionID))

	// Log session termination
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "session_terminated",
		SessionID:   sessionID,
		Details:     map[string]interface{}{"duration": session.GetDuration().String()},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	log.Printf("Terminated session %s", sessionID)

	return nil
}

// RequestPrivilege requests privilege escalation for a session
func (sm *SessionManager) RequestPrivilege(sessionID string, privilegeType PrivilegeType, justification string, duration time.Duration) (string, error) {
	sm.mutex.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("session not found")
	}

	// Validate duration
	// Use default max duration if not configured
	maxDuration := time.Hour * 24 // Default 24 hours
	if sm.config.PrivilegeEscalation.MaxPrivilegeDuration > 0 {
		maxDuration = sm.config.PrivilegeEscalation.MaxPrivilegeDuration
	}
	if duration > maxDuration {
		duration = maxDuration
	}

	requestID := session.RequestPrivilege(privilegeType, justification, duration)

	// Log privilege request
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "privilege_requested",
		SessionID:   sessionID,
		Details:     map[string]interface{}{"privilege_type": privilegeType, "justification": justification, "duration": duration.String()},
		Severity:    "warning",
		Success:     true,
		Timestamp:   time.Now(),
	})

	// Notify portal if connected
	if session.PortalConn != nil {
		sm.notifyPortalPrivilegeRequest(session, requestID, privilegeType, justification, duration)
	}

	return requestID, nil
}

// ApprovePrivilege approves a privilege request
func (sm *SessionManager) ApprovePrivilege(sessionID, requestID, approvedBy string) error {
	sm.mutex.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found")
	}

	err := session.ApprovePrivilege(requestID, approvedBy)
	if err != nil {
		return err
	}

	// Log privilege approval
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "privilege_approved",
		SessionID:   sessionID,
		Technician:  approvedBy,
		Details:     map[string]interface{}{"request_id": requestID},
		Severity:    "warning",
		Success:     true,
		Timestamp:   time.Now(),
	})

	// Notify client of privilege approval
	if session.ClientConn != nil {
		sm.notifyClientPrivilegeApproved(session, requestID)
	}

	return nil
}

// DenyPrivilege denies a privilege request
func (sm *SessionManager) DenyPrivilege(sessionID, requestID, deniedBy string) error {
	sm.mutex.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found")
	}

	err := session.DenyPrivilege(requestID, deniedBy)
	if err != nil {
		return err
	}

	// Log privilege denial
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "privilege_denied",
		SessionID:   sessionID,
		Technician:  deniedBy,
		Details:     map[string]interface{}{"request_id": requestID},
		Severity:    "info",
		Success:     true,
		Timestamp:   time.Now(),
	})

	// Notify client of privilege denial
	if session.ClientConn != nil {
		sm.notifyClientPrivilegeDenied(session, requestID)
	}

	return nil
}

// RevokePrivilege revokes an active privilege
func (sm *SessionManager) RevokePrivilege(sessionID string, privilegeType PrivilegeType) error {
	sm.mutex.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found")
	}

	err := session.RevokePrivilege(privilegeType)
	if err != nil {
		return err
	}

	// Log privilege revocation
	sm.auditLogger.LogEvent(AuditEvent{
		EventType:   "privilege_revoked",
		SessionID:   sessionID,
		Details:     map[string]interface{}{"privilege_type": privilegeType},
		Severity:    "warning",
		Success:     true,
		Timestamp:   time.Now(),
	})

	// Notify client of privilege revocation
	if session.ClientConn != nil {
		sm.notifyClientPrivilegeRevoked(session, privilegeType)
	}

	return nil
}

// GenerateSessionID generates a unique session ID
func (sm *SessionManager) GenerateSessionID() string {
	return uuid.New().String()
}

// ValidateSessionID validates a session ID format
func (sm *SessionManager) ValidateSessionID(sessionID string) bool {
	_, err := uuid.Parse(sessionID)
	return err == nil
}

// GetStatistics returns session statistics
func (sm *SessionManager) GetStatistics() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_sessions":   len(sm.sessions),
		"active_sessions":  0,
		"pending_sessions": 0,
		"total_connections": len(sm.connections),
		"config":           sm.config,
	}

	for _, session := range sm.sessions {
		if session.Status == StatusActive {
			stats["active_sessions"] = stats["active_sessions"].(int) + 1
		} else if session.Status == StatusPending {
			stats["pending_sessions"] = stats["pending_sessions"].(int) + 1
		}
	}

	return stats
}

// GetConfig returns the current configuration
func (sm *SessionManager) GetConfig() *RemoteAccessConfig {
	return sm.config
}

// UpdateConfig updates the configuration
func (sm *SessionManager) UpdateConfig(config *RemoteAccessConfig) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.config = config
}

// Shutdown gracefully shuts down the session manager
func (sm *SessionManager) Shutdown() {
	log.Println("Shutting down session manager...")

	// Stop cleanup routine
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	close(sm.shutdownChan)

	// Terminate all sessions
	sm.mutex.Lock()
	for sessionID := range sm.sessions {
		sm.sessions[sessionID].Terminate()
	}
	sm.mutex.Unlock()

	log.Println("Session manager shutdown complete")
}

// startCleanupRoutine starts the cleanup routine for expired sessions
func (sm *SessionManager) startCleanupRoutine() {
	sm.cleanupTicker = time.NewTicker(sm.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-sm.cleanupTicker.C:
				sm.cleanupExpiredSessions()
			case <-sm.shutdownChan:
				return
			}
		}
	}()
}

// cleanupExpiredSessions removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	var expiredSessions []string

	for sessionID, session := range sm.sessions {
		if session.IsExpired() {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	for _, sessionID := range expiredSessions {
		session := sm.sessions[sessionID]
		session.Status = StatusExpired
		session.Terminate()

		// Remove connections
		delete(sm.connections, fmt.Sprintf("%s_client", sessionID))
		delete(sm.connections, fmt.Sprintf("%s_portal", sessionID))

		// Log session expiration
		sm.auditLogger.LogEvent(AuditEvent{
			EventType:   "session_expired",
			SessionID:   sessionID,
			Details:     map[string]interface{}{"duration": session.GetDuration().String()},
			Severity:    "info",
			Success:     true,
			Timestamp:   time.Now(),
		})

		log.Printf("Expired session %s cleaned up", sessionID)
	}
}

// Notification methods
func (sm *SessionManager) notifyPortalPrivilegeRequest(session *RemoteAccessSession, requestID string, privilegeType PrivilegeType, justification string, duration time.Duration) {
	// Implementation for notifying portal of privilege request
	// This would send a WebSocket message to the portal
}

func (sm *SessionManager) notifyClientPrivilegeApproved(session *RemoteAccessSession, requestID string) {
	// Implementation for notifying client of privilege approval
	// This would send a WebSocket message to the client
}

func (sm *SessionManager) notifyClientPrivilegeDenied(session *RemoteAccessSession, requestID string) {
	// Implementation for notifying client of privilege denial
	// This would send a WebSocket message to the client
}

func (sm *SessionManager) notifyClientPrivilegeRevoked(session *RemoteAccessSession, privilegeType PrivilegeType) {
	// Implementation for notifying client of privilege revocation
	// This would send a WebSocket message to the client
}