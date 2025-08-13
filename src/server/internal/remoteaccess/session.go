package remoteaccess

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// RemoteAccessSession represents an active remote access session
type RemoteAccessSession struct {
	ID              string                 `json:"id"`
	ClientID        string                 `json:"client_id"`
	TechnicianID    string                 `json:"technician_id"`
	Status          SessionStatus          `json:"status"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         *time.Time             `json:"end_time,omitempty"`
	ClientInfo      *ClientInfo            `json:"client_info"`
	Privileges      []PrivilegeRequest     `json:"privileges"`
	ActivePrivileges map[string]*ActivePrivilege `json:"active_privileges"`
	ClientConn      *websocket.Conn        `json:"-"`
	PortalConn      *websocket.Conn        `json:"-"`
	LastActivity    time.Time              `json:"last_activity"`
	Settings        *SessionSettings       `json:"settings"`
	Statistics      *SessionStatistics     `json:"statistics"`
	mutex           sync.RWMutex           `json:"-"`
}

// SessionStatus represents the status of a remote access session
type SessionStatus string

const (
	StatusPending     SessionStatus = "pending"
	StatusActive      SessionStatus = "active"
	StatusDisconnected SessionStatus = "disconnected"
	StatusTerminated  SessionStatus = "terminated"
	StatusExpired     SessionStatus = "expired"
)

// ClientInfo contains information about the client machine
type ClientInfo struct {
	Hostname        string            `json:"hostname"`
	OperatingSystem string            `json:"operating_system"`
	IPAddress       string            `json:"ip_address"`
	UserAgent       string            `json:"user_agent"`
	ScreenResolution string           `json:"screen_resolution"`
	CurrentUser     string            `json:"current_user"`
	SystemInfo      map[string]string `json:"system_info"`
}

// PrivilegeRequest represents a request for elevated privileges
type PrivilegeRequest struct {
	ID          string        `json:"id"`
	Type        PrivilegeType `json:"type"`
	Justification string      `json:"justification"`
	Duration    time.Duration `json:"duration"`
	RequestedAt time.Time     `json:"requested_at"`
	Status      string        `json:"status"` // pending, approved, denied
	ApprovedBy  string        `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time    `json:"approved_at,omitempty"`
}

// ActivePrivilege represents an active privilege with expiration
type ActivePrivilege struct {
	Type      PrivilegeType `json:"type"`
	GrantedAt time.Time     `json:"granted_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	GrantedBy string        `json:"granted_by"`
}

// SessionSettings contains session configuration
type SessionSettings struct {
	AllowFileTransfer   bool          `json:"allow_file_transfer"`
	AllowClipboard      bool          `json:"allow_clipboard"`
	AllowPrinting       bool          `json:"allow_printing"`
	SessionTimeout      time.Duration `json:"session_timeout"`
	IdleTimeout         time.Duration `json:"idle_timeout"`
	RecordSession       bool          `json:"record_session"`
	RequireApproval     bool          `json:"require_approval"`
	MaxPrivilegeDuration time.Duration `json:"max_privilege_duration"`
}

// SessionStatistics contains session usage statistics
type SessionStatistics struct {
	CommandsExecuted    int           `json:"commands_executed"`
	FilesTransferred    int           `json:"files_transferred"`
	BytesTransferred    int64         `json:"bytes_transferred"`
	ScreenshotsTaken    int           `json:"screenshots_taken"`
	PrivilegeEscalations int          `json:"privilege_escalations"`
	Duration            time.Duration `json:"duration"`
	LastCommand         string        `json:"last_command,omitempty"`
	LastCommandTime     *time.Time    `json:"last_command_time,omitempty"`
}

// NewRemoteAccessSession creates a new remote access session
func NewRemoteAccessSession(clientID, technicianID string, clientInfo *ClientInfo) *RemoteAccessSession {
	return &RemoteAccessSession{
		ID:               uuid.New().String(),
		ClientID:         clientID,
		TechnicianID:     technicianID,
		Status:           StatusPending,
		StartTime:        time.Now(),
		ClientInfo:       clientInfo,
		Privileges:       make([]PrivilegeRequest, 0),
		ActivePrivileges: make(map[string]*ActivePrivilege),
		LastActivity:     time.Now(),
		Settings:         DefaultSessionSettings(),
		Statistics:       &SessionStatistics{},
	}
}

// DefaultSessionSettings returns default session settings
func DefaultSessionSettings() *SessionSettings {
	return &SessionSettings{
		AllowFileTransfer:    true,
		AllowClipboard:       true,
		AllowPrinting:        false,
		SessionTimeout:       4 * time.Hour,
		IdleTimeout:          30 * time.Minute,
		RecordSession:        true,
		RequireApproval:      true,
		MaxPrivilegeDuration: 1 * time.Hour,
	}
}

// UpdateActivity updates the last activity timestamp
func (s *RemoteAccessSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.LastActivity = time.Now()
}

// IsExpired checks if the session has expired
func (s *RemoteAccessSession) IsExpired() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	if s.Status == StatusTerminated || s.Status == StatusExpired {
		return true
	}
	
	// Check session timeout
	if time.Since(s.StartTime) > s.Settings.SessionTimeout {
		return true
	}
	
	// Check idle timeout
	if time.Since(s.LastActivity) > s.Settings.IdleTimeout {
		return true
	}
	
	return false
}

// RequestPrivilege adds a new privilege request
func (s *RemoteAccessSession) RequestPrivilege(privilegeType PrivilegeType, justification string, duration time.Duration) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	request := PrivilegeRequest{
		ID:            uuid.New().String(),
		Type:          privilegeType,
		Justification: justification,
		Duration:      duration,
		RequestedAt:   time.Now(),
		Status:        "pending",
	}
	
	s.Privileges = append(s.Privileges, request)
	s.Statistics.PrivilegeEscalations++
	
	return request.ID
}

// ApprovePrivilege approves a privilege request
func (s *RemoteAccessSession) ApprovePrivilege(requestID, approvedBy string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for i, request := range s.Privileges {
		if request.ID == requestID {
			if request.Status != "pending" {
				return fmt.Errorf("privilege request is not pending")
			}
			
			// Update request status
			now := time.Now()
			s.Privileges[i].Status = "approved"
			s.Privileges[i].ApprovedBy = approvedBy
			s.Privileges[i].ApprovedAt = &now
			
			// Add to active privileges
			activePrivilege := &ActivePrivilege{
				Type:      request.Type,
				GrantedAt: now,
				ExpiresAt: now.Add(request.Duration),
				GrantedBy: approvedBy,
			}
			
			s.ActivePrivileges[string(request.Type)] = activePrivilege
			
			log.Printf("Privilege %s approved for session %s", request.Type, s.ID)
			return nil
		}
	}
	
	return fmt.Errorf("privilege request not found")
}

// DenyPrivilege denies a privilege request
func (s *RemoteAccessSession) DenyPrivilege(requestID, deniedBy string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for i, request := range s.Privileges {
		if request.ID == requestID {
			if request.Status != "pending" {
				return fmt.Errorf("privilege request is not pending")
			}
			
			s.Privileges[i].Status = "denied"
			s.Privileges[i].ApprovedBy = deniedBy
			
			log.Printf("Privilege %s denied for session %s", request.Type, s.ID)
			return nil
		}
	}
	
	return fmt.Errorf("privilege request not found")
}

// RevokePrivilege revokes an active privilege
func (s *RemoteAccessSession) RevokePrivilege(privilegeType PrivilegeType) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if _, exists := s.ActivePrivileges[string(privilegeType)]; !exists {
		return fmt.Errorf("privilege not active")
	}
	
	delete(s.ActivePrivileges, string(privilegeType))
	log.Printf("Privilege %s revoked for session %s", privilegeType, s.ID)
	
	return nil
}

// HasActivePrivilege checks if a privilege is currently active
func (s *RemoteAccessSession) HasActivePrivilege(privilegeType PrivilegeType) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	privilege, exists := s.ActivePrivileges[string(privilegeType)]
	if !exists {
		return false
	}
	
	// Check if privilege has expired
	if time.Now().After(privilege.ExpiresAt) {
		// Remove expired privilege
		s.mutex.RUnlock()
		s.mutex.Lock()
		delete(s.ActivePrivileges, string(privilegeType))
		s.mutex.Unlock()
		s.mutex.RLock()
		return false
	}
	
	return true
}

// Terminate terminates the session
func (s *RemoteAccessSession) Terminate() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.Status = StatusTerminated
	now := time.Now()
	s.EndTime = &now
	s.Statistics.Duration = now.Sub(s.StartTime)
	
	// Close connections
	if s.ClientConn != nil {
		s.ClientConn.Close()
	}
	if s.PortalConn != nil {
		s.PortalConn.Close()
	}
	
	log.Printf("Session %s terminated", s.ID)
}

// ToJSON converts the session to JSON (excluding connections)
func (s *RemoteAccessSession) ToJSON() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return json.Marshal(s)
}

// GetDuration returns the session duration
func (s *RemoteAccessSession) GetDuration() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	if s.EndTime != nil {
		return s.EndTime.Sub(s.StartTime)
	}
	return time.Since(s.StartTime)
}

// IncrementCommand increments the command counter
func (s *RemoteAccessSession) IncrementCommand(command string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.Statistics.CommandsExecuted++
	s.Statistics.LastCommand = command
	now := time.Now()
	s.Statistics.LastCommandTime = &now
	s.LastActivity = now
}

// AddFileTransfer adds file transfer statistics
func (s *RemoteAccessSession) AddFileTransfer(bytes int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.Statistics.FilesTransferred++
	s.Statistics.BytesTransferred += bytes
	s.LastActivity = time.Now()
}

// IncrementScreenshot increments the screenshot counter
func (s *RemoteAccessSession) IncrementScreenshot() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.Statistics.ScreenshotsTaken++
	s.LastActivity = time.Now()
}