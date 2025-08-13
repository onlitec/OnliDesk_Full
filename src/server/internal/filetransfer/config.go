package filetransfer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ConfigManager handles dynamic configuration updates
type ConfigManager struct {
	transferConfig *TransferConfig
	securityConfig *SecurityConfig
	configFile     string
	updateCallbacks []func(*TransferConfig, *SecurityConfig)
	auditLogger    *AuditLogger
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configFile string) *ConfigManager {
	cm := &ConfigManager{
		configFile: configFile,
		updateCallbacks: make([]func(*TransferConfig, *SecurityConfig), 0),
		auditLogger: NewAuditLogger("./logs/config", true),
	}
	
	// Load configuration from file or use defaults
	if err := cm.LoadConfig(); err != nil {
		log.Printf("Failed to load config from %s, using defaults: %v", configFile, err)
		cm.transferConfig = DefaultTransferConfig()
		cm.securityConfig = DefaultSecurityConfig()
		
		// Save default configuration
		if err := cm.SaveConfig(); err != nil {
			log.Printf("Failed to save default config: %v", err)
		}
	}
	
	return cm
}

// LoadConfig loads configuration from file
func (cm *ConfigManager) LoadConfig() error {
	data, err := ioutil.ReadFile(cm.configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}
	
	var config struct {
		Transfer *TransferConfig `json:"transfer"`
		Security *SecurityConfig `json:"security"`
	}
	
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}
	
	if config.Transfer == nil {
		config.Transfer = DefaultTransferConfig()
	}
	if config.Security == nil {
		config.Security = DefaultSecurityConfig()
	}
	
	cm.transferConfig = config.Transfer
	cm.securityConfig = config.Security
	
	return nil
}

// SaveConfig saves current configuration to file
func (cm *ConfigManager) SaveConfig() error {
	// Ensure directory exists
	dir := filepath.Dir(cm.configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	
	config := struct {
		Transfer *TransferConfig `json:"transfer"`
		Security *SecurityConfig `json:"security"`
		Updated  time.Time       `json:"updated"`
	}{
		Transfer: cm.transferConfig,
		Security: cm.securityConfig,
		Updated:  time.Now(),
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	
	if err := ioutil.WriteFile(cm.configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	
	return nil
}

// GetTransferConfig returns the current transfer configuration
func (cm *ConfigManager) GetTransferConfig() *TransferConfig {
	return cm.transferConfig
}

// GetSecurityConfig returns the current security configuration
func (cm *ConfigManager) GetSecurityConfig() *SecurityConfig {
	return cm.securityConfig
}

// UpdateTransferConfig updates the transfer configuration
func (cm *ConfigManager) UpdateTransferConfig(config *TransferConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	// Validate configuration
	if err := cm.validateTransferConfig(config); err != nil {
		return fmt.Errorf("invalid transfer config: %v", err)
	}
	
	oldConfig := cm.transferConfig
	cm.transferConfig = config
	
	// Save to file
	if err := cm.SaveConfig(); err != nil {
		// Rollback on save failure
		cm.transferConfig = oldConfig
		return fmt.Errorf("failed to save config: %v", err)
	}
	
	// Log configuration update
	changes := map[string]interface{}{
		"config_type": "transfer",
		"old_max_file_size": oldConfig.MaxFileSize,
		"new_max_file_size": config.MaxFileSize,
		"old_max_concurrent": oldConfig.MaxConcurrent,
		"new_max_concurrent": config.MaxConcurrent,
		"old_require_approval": oldConfig.RequireApproval,
		"new_require_approval": config.RequireApproval,
	}
	cm.auditLogger.LogConfigUpdate("system", "transfer_config", changes)
	
	// Notify callbacks
	for _, callback := range cm.updateCallbacks {
		callback(cm.transferConfig, cm.securityConfig)
	}
	
	log.Printf("Transfer configuration updated successfully")
	return nil
}

// UpdateSecurityConfig updates the security configuration
func (cm *ConfigManager) UpdateSecurityConfig(config *SecurityConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	// Validate configuration
	if err := cm.validateSecurityConfig(config); err != nil {
		return fmt.Errorf("invalid security config: %v", err)
	}
	
	oldConfig := cm.securityConfig
	cm.securityConfig = config
	
	// Save to file
	if err := cm.SaveConfig(); err != nil {
		// Rollback on save failure
		cm.securityConfig = oldConfig
		return fmt.Errorf("failed to save config: %v", err)
	}
	
	// Log configuration update
	changes := map[string]interface{}{
		"config_type": "security",
		"old_encryption_enabled": oldConfig.EncryptionEnabled,
		"new_encryption_enabled": config.EncryptionEnabled,
		"old_require_checksum": oldConfig.RequireChecksum,
		"new_require_checksum": config.RequireChecksum,
		"old_scan_for_malware": oldConfig.ScanForMalware,
		"new_scan_for_malware": config.ScanForMalware,
		"allowed_mime_types_count": len(config.AllowedMimeTypes),
		"blocked_extensions_count": len(config.BlockedExtensions),
	}
	cm.auditLogger.LogConfigUpdate("system", "security_config", changes)
	
	// Notify callbacks
	for _, callback := range cm.updateCallbacks {
		callback(cm.transferConfig, cm.securityConfig)
	}
	
	log.Printf("Security configuration updated successfully")
	return nil
}

// RegisterUpdateCallback registers a callback for configuration updates
func (cm *ConfigManager) RegisterUpdateCallback(callback func(*TransferConfig, *SecurityConfig)) {
	cm.updateCallbacks = append(cm.updateCallbacks, callback)
}

// validateTransferConfig validates transfer configuration
func (cm *ConfigManager) validateTransferConfig(config *TransferConfig) error {
	if config.MaxFileSize <= 0 {
		return fmt.Errorf("max file size must be positive")
	}
	if config.MaxFileSize > 10*1024*1024*1024 { // 10GB limit
		return fmt.Errorf("max file size cannot exceed 10GB")
	}
	if config.MaxConcurrent <= 0 {
		return fmt.Errorf("max concurrent transfers must be positive")
	}
	if config.MaxConcurrent > 100 {
		return fmt.Errorf("max concurrent transfers cannot exceed 100")
	}
	if config.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}
	if config.ChunkSize > 10*1024*1024 { // 10MB max chunk
		return fmt.Errorf("chunk size cannot exceed 10MB")
	}
	if config.TransferTimeout <= 0 {
		return fmt.Errorf("transfer timeout must be positive")
	}
	if config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}
	if config.RetryAttempts > 10 {
		return fmt.Errorf("retry attempts cannot exceed 10")
	}
	
	return nil
}

// validateSecurityConfig validates security configuration
func (cm *ConfigManager) validateSecurityConfig(config *SecurityConfig) error {
	if config.MaxFilenameLength <= 0 {
		return fmt.Errorf("max filename length must be positive")
	}
	if config.MaxFilenameLength > 1000 {
		return fmt.Errorf("max filename length cannot exceed 1000")
	}
	if config.EncryptionEnabled && len(config.EncryptionKey) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes for AES-256")
	}
	if config.QuarantineDir == "" {
		return fmt.Errorf("quarantine directory cannot be empty")
	}
	
	return nil
}

// GetConfigSummary returns a summary of current configuration
func (cm *ConfigManager) GetConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"transfer": map[string]interface{}{
			"max_file_size":    cm.transferConfig.MaxFileSize,
			"max_concurrent":   cm.transferConfig.MaxConcurrent,
			"chunk_size":       cm.transferConfig.ChunkSize,
			"require_approval": cm.transferConfig.RequireApproval,
			"encrypt_files":    cm.transferConfig.EncryptFiles,
			"allowed_types":    cm.transferConfig.AllowedTypes,
		},
		"security": map[string]interface{}{
			"encryption_enabled":  cm.securityConfig.EncryptionEnabled,
			"require_checksum":    cm.securityConfig.RequireChecksum,
			"scan_for_malware":    cm.securityConfig.ScanForMalware,
			"max_filename_length": cm.securityConfig.MaxFilenameLength,
			"allowed_mime_types":  cm.securityConfig.AllowedMimeTypes,
			"blocked_extensions": cm.securityConfig.BlockedExtensions,
		},
		"config_file": cm.configFile,
	}
}

// ResetToDefaults resets configuration to default values
func (cm *ConfigManager) ResetToDefaults() error {
	cm.transferConfig = DefaultTransferConfig()
	cm.securityConfig = DefaultSecurityConfig()
	
	if err := cm.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save default config: %v", err)
	}
	
	// Notify callbacks
	for _, callback := range cm.updateCallbacks {
		callback(cm.transferConfig, cm.securityConfig)
	}
	
	log.Printf("Configuration reset to defaults")
	return nil
}