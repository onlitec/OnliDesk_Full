package remoteaccess

import (
	"fmt"
	"time"
)

type PrivilegeType string

const (
	PrivilegeTypeAdmin    PrivilegeType = "admin"
	PrivilegeTypeElevated PrivilegeType = "elevated"
	PrivilegeTypeRegistry PrivilegeType = "registry"
	PrivilegeTypeServices PrivilegeType = "services"
)

// IsValid checks if the privilege type is valid
func (p PrivilegeType) IsValid() bool {
	switch p {
	case PrivilegeTypeAdmin, PrivilegeTypeElevated, PrivilegeTypeRegistry, PrivilegeTypeServices:
		return true
	default:
		return false
	}
}

// RemoteAccessConfig holds configuration for remote access functionality
type RemoteAccessConfig struct {
	// Server settings
	Enabled                bool          `json:"enabled" yaml:"enabled"`
	MaxConcurrentSessions  int           `json:"max_concurrent_sessions" yaml:"max_concurrent_sessions"`
	SessionTimeout         time.Duration `json:"session_timeout" yaml:"session_timeout"`
	IdleTimeout            time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	CleanupInterval        time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"`

	// WebSocket settings
	WebSocketReadTimeout   time.Duration `json:"websocket_read_timeout" yaml:"websocket_read_timeout"`
	WebSocketWriteTimeout  time.Duration `json:"websocket_write_timeout" yaml:"websocket_write_timeout"`
	WebSocketPingInterval  time.Duration `json:"websocket_ping_interval" yaml:"websocket_ping_interval"`
	WebSocketPongTimeout   time.Duration `json:"websocket_pong_timeout" yaml:"websocket_pong_timeout"`
	MaxMessageSize         int64         `json:"max_message_size" yaml:"max_message_size"`

	// Security settings
	RequireAuthentication  bool          `json:"require_authentication" yaml:"require_authentication"`
	AllowedOrigins         []string      `json:"allowed_origins" yaml:"allowed_origins"`
	RateLimitEnabled       bool          `json:"rate_limit_enabled" yaml:"rate_limit_enabled"`
	RateLimitRequests      int           `json:"rate_limit_requests" yaml:"rate_limit_requests"`
	RateLimitWindow        time.Duration `json:"rate_limit_window" yaml:"rate_limit_window"`
	MaxFailedAttempts      int           `json:"max_failed_attempts" yaml:"max_failed_attempts"`
	LockoutDuration        time.Duration `json:"lockout_duration" yaml:"lockout_duration"`

	// Privilege escalation settings
	PrivilegeEscalation    PrivilegeEscalationConfig `json:"privilege_escalation" yaml:"privilege_escalation"`

	// Audit settings
	AuditEnabled           bool   `json:"audit_enabled" yaml:"audit_enabled"`
	AuditLogDir            string `json:"audit_log_dir" yaml:"audit_log_dir"`
	AuditRetentionDays     int    `json:"audit_retention_days" yaml:"audit_retention_days"`

	// File transfer settings
	FileTransferEnabled    bool  `json:"file_transfer_enabled" yaml:"file_transfer_enabled"`
	MaxFileSize            int64 `json:"max_file_size" yaml:"max_file_size"`
	AllowedFileTypes       []string `json:"allowed_file_types" yaml:"allowed_file_types"`
	BlockedFileTypes       []string `json:"blocked_file_types" yaml:"blocked_file_types"`

	// Screen sharing settings
	ScreenSharingEnabled   bool `json:"screen_sharing_enabled" yaml:"screen_sharing_enabled"`
	MaxScreenshotSize      int  `json:"max_screenshot_size" yaml:"max_screenshot_size"`
	ScreenshotQuality      int  `json:"screenshot_quality" yaml:"screenshot_quality"`
	ScreenshotInterval     time.Duration `json:"screenshot_interval" yaml:"screenshot_interval"`

	// Command execution settings
	CommandExecutionEnabled bool     `json:"command_execution_enabled" yaml:"command_execution_enabled"`
	AllowedCommands        []string `json:"allowed_commands" yaml:"allowed_commands"`
	BlockedCommands        []string `json:"blocked_commands" yaml:"blocked_commands"`
	CommandTimeout         time.Duration `json:"command_timeout" yaml:"command_timeout"`
}

// PrivilegeEscalationConfig holds privilege escalation configuration
type PrivilegeEscalationConfig struct {
	Enabled                bool          `json:"enabled" yaml:"enabled"`
	RequireApproval        bool          `json:"require_approval" yaml:"require_approval"`
	AutoApprovalTimeout    time.Duration `json:"auto_approval_timeout" yaml:"auto_approval_timeout"`
	MaxPrivilegeDuration   time.Duration `json:"max_privilege_duration" yaml:"max_privilege_duration"`
	DefaultPrivilegeDuration time.Duration `json:"default_privilege_duration" yaml:"default_privilege_duration"`
	RequireJustification   bool          `json:"require_justification" yaml:"require_justification"`
	MinJustificationLength int           `json:"min_justification_length" yaml:"min_justification_length"`
	AllowedPrivileges      []PrivilegeType `json:"allowed_privileges" yaml:"allowed_privileges"`
	NotifyOnEscalation     bool          `json:"notify_on_escalation" yaml:"notify_on_escalation"`
	LogAllRequests         bool          `json:"log_all_requests" yaml:"log_all_requests"`
}

// DefaultRemoteAccessConfig returns default configuration
func DefaultRemoteAccessConfig() *RemoteAccessConfig {
	return &RemoteAccessConfig{
		// Server settings
		Enabled:               true,
		MaxConcurrentSessions: 10,
		SessionTimeout:        4 * time.Hour,
		IdleTimeout:           30 * time.Minute,
		CleanupInterval:       5 * time.Minute,

		// WebSocket settings
		WebSocketReadTimeout:  60 * time.Second,
		WebSocketWriteTimeout: 10 * time.Second,
		WebSocketPingInterval: 30 * time.Second,
		WebSocketPongTimeout:  10 * time.Second,
		MaxMessageSize:        1024 * 1024, // 1MB

		// Security settings
		RequireAuthentication: true,
		AllowedOrigins:        []string{"*"},
		RateLimitEnabled:      true,
		RateLimitRequests:     100,
		RateLimitWindow:       time.Minute,
		MaxFailedAttempts:     5,
		LockoutDuration:       15 * time.Minute,

		// Privilege escalation settings
		PrivilegeEscalation: PrivilegeEscalationConfig{
			Enabled:                  true,
			RequireApproval:          true,
			AutoApprovalTimeout:      5 * time.Minute,
			MaxPrivilegeDuration:     2 * time.Hour,
			DefaultPrivilegeDuration: 30 * time.Minute,
			RequireJustification:     true,
			MinJustificationLength:   10,
			AllowedPrivileges: []PrivilegeType{
				PrivilegeTypeElevated,
				PrivilegeTypeRegistry,
				PrivilegeTypeServices,
			},
			NotifyOnEscalation: true,
			LogAllRequests:     true,
		},

		// Audit settings
		AuditEnabled:       true,
		AuditLogDir:        "./logs/audit",
		AuditRetentionDays: 90,

		// File transfer settings
		FileTransferEnabled: true,
		MaxFileSize:        100 * 1024 * 1024, // 100MB
		AllowedFileTypes:   []string{".txt", ".log", ".cfg", ".conf", ".ini", ".xml", ".json", ".yaml", ".yml"},
		BlockedFileTypes:   []string{".exe", ".bat", ".cmd", ".ps1", ".sh", ".scr", ".com", ".pif"},

		// Screen sharing settings
		ScreenSharingEnabled: true,
		MaxScreenshotSize:   1920 * 1080,
		ScreenshotQuality:   80,
		ScreenshotInterval:  time.Second,

		// Command execution settings
		CommandExecutionEnabled: false, // Disabled by default for security
		AllowedCommands:        []string{"dir", "ls", "pwd", "whoami", "hostname", "ipconfig", "ifconfig"},
		BlockedCommands:        []string{"rm", "del", "format", "fdisk", "mkfs", "sudo", "su", "runas"},
		CommandTimeout:         30 * time.Second,
	}
}

// Validate validates the configuration
func (c *RemoteAccessConfig) Validate() error {
	if c.MaxConcurrentSessions <= 0 {
		return fmt.Errorf("max_concurrent_sessions must be greater than 0")
	}

	if c.SessionTimeout <= 0 {
		return fmt.Errorf("session_timeout must be greater than 0")
	}

	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle_timeout must be greater than 0")
	}

	if c.WebSocketReadTimeout <= 0 {
		return fmt.Errorf("websocket_read_timeout must be greater than 0")
	}

	if c.WebSocketWriteTimeout <= 0 {
		return fmt.Errorf("websocket_write_timeout must be greater than 0")
	}

	if c.MaxMessageSize <= 0 {
		return fmt.Errorf("max_message_size must be greater than 0")
	}

	if c.RateLimitEnabled {
		if c.RateLimitRequests <= 0 {
			return fmt.Errorf("rate_limit_requests must be greater than 0 when rate limiting is enabled")
		}
		if c.RateLimitWindow <= 0 {
			return fmt.Errorf("rate_limit_window must be greater than 0 when rate limiting is enabled")
		}
	}

	if c.MaxFailedAttempts <= 0 {
		return fmt.Errorf("max_failed_attempts must be greater than 0")
	}

	if c.LockoutDuration <= 0 {
		return fmt.Errorf("lockout_duration must be greater than 0")
	}

	// Validate privilege escalation config
	if err := c.PrivilegeEscalation.Validate(); err != nil {
		return fmt.Errorf("privilege_escalation config error: %v", err)
	}

	if c.AuditRetentionDays <= 0 {
		return fmt.Errorf("audit_retention_days must be greater than 0")
	}

	if c.FileTransferEnabled {
		if c.MaxFileSize <= 0 {
			return fmt.Errorf("max_file_size must be greater than 0 when file transfer is enabled")
		}
	}

	if c.ScreenSharingEnabled {
		if c.MaxScreenshotSize <= 0 {
			return fmt.Errorf("max_screenshot_size must be greater than 0 when screen sharing is enabled")
		}
		if c.ScreenshotQuality < 1 || c.ScreenshotQuality > 100 {
			return fmt.Errorf("screenshot_quality must be between 1 and 100")
		}
		if c.ScreenshotInterval <= 0 {
			return fmt.Errorf("screenshot_interval must be greater than 0")
		}
	}

	if c.CommandExecutionEnabled {
		if c.CommandTimeout <= 0 {
			return fmt.Errorf("command_timeout must be greater than 0 when command execution is enabled")
		}
	}

	return nil
}

// Validate validates the privilege escalation configuration
func (c *PrivilegeEscalationConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.AutoApprovalTimeout <= 0 {
		return fmt.Errorf("auto_approval_timeout must be greater than 0")
	}

	if c.MaxPrivilegeDuration <= 0 {
		return fmt.Errorf("max_privilege_duration must be greater than 0")
	}

	if c.DefaultPrivilegeDuration <= 0 {
		return fmt.Errorf("default_privilege_duration must be greater than 0")
	}

	if c.DefaultPrivilegeDuration > c.MaxPrivilegeDuration {
		return fmt.Errorf("default_privilege_duration cannot be greater than max_privilege_duration")
	}

	if c.RequireJustification && c.MinJustificationLength <= 0 {
		return fmt.Errorf("min_justification_length must be greater than 0 when justification is required")
	}

	if len(c.AllowedPrivileges) == 0 {
		return fmt.Errorf("at least one privilege type must be allowed")
	}

	// Validate privilege types
	for _, privilege := range c.AllowedPrivileges {
		if !privilege.IsValid() {
			return fmt.Errorf("invalid privilege type: %s", privilege)
		}
	}

	return nil
}

// IsFileTypeAllowed checks if a file type is allowed for transfer
func (c *RemoteAccessConfig) IsFileTypeAllowed(filename string) bool {
	if !c.FileTransferEnabled {
		return false
	}

	// Extract file extension
	ext := ""
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			ext = filename[i:]
			break
		}
	}

	if ext == "" {
		return false // No extension
	}

	// Convert to lowercase for comparison
	extLower := ""
	for _, r := range ext {
		if r >= 'A' && r <= 'Z' {
			extLower += string(r + 32)
		} else {
			extLower += string(r)
		}
	}

	// Check blocked types first
	for _, blocked := range c.BlockedFileTypes {
		if extLower == blocked {
			return false
		}
	}

	// If no allowed types specified, allow all (except blocked)
	if len(c.AllowedFileTypes) == 0 {
		return true
	}

	// Check allowed types
	for _, allowed := range c.AllowedFileTypes {
		if extLower == allowed {
			return true
		}
	}

	return false
}

// IsCommandAllowed checks if a command is allowed for execution
func (c *RemoteAccessConfig) IsCommandAllowed(command string) bool {
	if !c.CommandExecutionEnabled {
		return false
	}

	// Extract command name (first word)
	cmdName := ""
	for i, r := range command {
		if r == ' ' || r == '\t' {
			cmdName = command[:i]
			break
		}
	}
	if cmdName == "" {
		cmdName = command
	}

	// Convert to lowercase for comparison
	cmdLower := ""
	for _, r := range cmdName {
		if r >= 'A' && r <= 'Z' {
			cmdLower += string(r + 32)
		} else {
			cmdLower += string(r)
		}
	}

	// Check blocked commands first
	for _, blocked := range c.BlockedCommands {
		if cmdLower == blocked {
			return false
		}
	}

	// If no allowed commands specified, allow all (except blocked)
	if len(c.AllowedCommands) == 0 {
		return true
	}

	// Check allowed commands
	for _, allowed := range c.AllowedCommands {
		if cmdLower == allowed {
			return true
		}
	}

	return false
}

// GetPrivilegeTimeout returns the timeout for a privilege type
func (c *RemoteAccessConfig) GetPrivilegeTimeout(privilegeType PrivilegeType, requestedDuration time.Duration) time.Duration {
	if requestedDuration <= 0 {
		return c.PrivilegeEscalation.DefaultPrivilegeDuration
	}

	if requestedDuration > c.PrivilegeEscalation.MaxPrivilegeDuration {
		return c.PrivilegeEscalation.MaxPrivilegeDuration
	}

	return requestedDuration
}

// IsPrivilegeAllowed checks if a privilege type is allowed
func (c *RemoteAccessConfig) IsPrivilegeAllowed(privilegeType PrivilegeType) bool {
	if !c.PrivilegeEscalation.Enabled {
		return false
	}

	for _, allowed := range c.PrivilegeEscalation.AllowedPrivileges {
		if allowed == privilegeType {
			return true
		}
	}

	return false
}

// Clone creates a deep copy of the configuration
func (c *RemoteAccessConfig) Clone() *RemoteAccessConfig {
	clone := *c

	// Clone slices
	clone.AllowedOrigins = make([]string, len(c.AllowedOrigins))
	copy(clone.AllowedOrigins, c.AllowedOrigins)

	clone.AllowedFileTypes = make([]string, len(c.AllowedFileTypes))
	copy(clone.AllowedFileTypes, c.AllowedFileTypes)

	clone.BlockedFileTypes = make([]string, len(c.BlockedFileTypes))
	copy(clone.BlockedFileTypes, c.BlockedFileTypes)

	clone.AllowedCommands = make([]string, len(c.AllowedCommands))
	copy(clone.AllowedCommands, c.AllowedCommands)

	clone.BlockedCommands = make([]string, len(c.BlockedCommands))
	copy(clone.BlockedCommands, c.BlockedCommands)

	clone.PrivilegeEscalation.AllowedPrivileges = make([]PrivilegeType, len(c.PrivilegeEscalation.AllowedPrivileges))
	copy(clone.PrivilegeEscalation.AllowedPrivileges, c.PrivilegeEscalation.AllowedPrivileges)

	return &clone
}