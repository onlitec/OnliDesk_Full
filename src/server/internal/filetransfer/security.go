package filetransfer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	EncryptionKey       []byte   `json:"-"` // Never serialize the key
	AllowedMimeTypes    []string `json:"allowed_mime_types"`
	BlockedExtensions   []string `json:"blocked_extensions"`
	MaxFilenameLength   int      `json:"max_filename_length"`
	ScanForMalware      bool     `json:"scan_for_malware"`
	QuarantineDir       string   `json:"quarantine_dir"`
	RequireChecksum     bool     `json:"require_checksum"`
	ChecksumAlgorithm   string   `json:"checksum_algorithm"`
	EncryptionEnabled   bool     `json:"encryption_enabled"`
	CompressionEnabled  bool     `json:"compression_enabled"`
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	// Generate a random encryption key (in production, this should be loaded from secure storage)
	key := make([]byte, 32) // 256-bit key for AES-256
	if _, err := rand.Read(key); err != nil {
		log.Printf("Warning: Failed to generate random encryption key: %v", err)
	}

	return &SecurityConfig{
		EncryptionKey: key,
		AllowedMimeTypes: []string{
			"text/plain",
			"text/csv",
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/zip",
			"application/x-rar-compressed",
			"image/jpeg",
			"image/png",
			"image/gif",
			"image/bmp",
			"image/webp",
		},
		BlockedExtensions: []string{
			".exe", ".bat", ".cmd", ".com", ".scr", ".pif",
			".vbs", ".js", ".jar", ".msi", ".dll", ".sys",
			".ps1", ".sh", ".php", ".asp", ".jsp",
		},
		MaxFilenameLength:   255,
		ScanForMalware:      false, // Would require integration with antivirus
		QuarantineDir:       "./quarantine",
		RequireChecksum:     true,
		ChecksumAlgorithm:   "SHA256",
		EncryptionEnabled:   true,
		CompressionEnabled:  false,
	}
}

// FileValidator handles file validation and security checks
type FileValidator struct {
	config      *SecurityConfig
	auditLogger *AuditLogger
}

// NewFileValidator creates a new file validator
func NewFileValidator(config *SecurityConfig) *FileValidator {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	// Ensure quarantine directory exists
	if err := os.MkdirAll(config.QuarantineDir, 0755); err != nil {
		log.Printf("Failed to create quarantine directory: %v", err)
	}

	return &FileValidator{
		config:      config,
		auditLogger: NewAuditLogger("./logs/security", true),
	}
}

// ValidationResult represents the result of file validation
type ValidationResult struct {
	Valid        bool     `json:"valid"`
	Errors       []string `json:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	MimeType     string   `json:"mime_type"`
	FileSize     int64    `json:"file_size"`
	Checksum     string   `json:"checksum"`
	Quarantined  bool     `json:"quarantined"`
	ScanResults  string   `json:"scan_results,omitempty"`
}

// ValidateFile performs comprehensive file validation
func (fv *FileValidator) ValidateFile(filePath, originalFilename string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %v", err)
	}

	result.FileSize = fileInfo.Size()

	// Validate filename
	if err := fv.validateFilename(originalFilename); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		// Log security violation for invalid filename
		fv.auditLogger.LogSecurityViolation("", "", originalFilename, "Invalid filename: "+err.Error(), "")
	}

	// Validate file extension
	if err := fv.validateFileExtension(originalFilename); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		// Log security violation for blocked extension
		fv.auditLogger.LogSecurityViolation("", "", originalFilename, "Blocked file extension: "+err.Error(), "")
	}

	// Detect and validate MIME type
	mimeType, err := fv.detectMimeType(filePath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to detect MIME type: %v", err))
	} else {
		result.MimeType = mimeType
		if err := fv.validateMimeType(mimeType); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err.Error())
			// Log security violation for invalid MIME type
			fv.auditLogger.LogSecurityViolation("", "", originalFilename, "Invalid MIME type: "+mimeType, "")
		}
	}

	// Calculate checksum
	if fv.config.RequireChecksum {
		checksum, err := fv.calculateChecksum(filePath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to calculate checksum: %v", err))
		} else {
			result.Checksum = checksum
		}
	}

	// Scan for malware (if enabled)
	if fv.config.ScanForMalware {
		scanResult, err := fv.scanForMalware(filePath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Malware scan failed: %v", err))
		} else if !scanResult.Clean {
			result.Valid = false
			result.Errors = append(result.Errors, "File failed malware scan")
			result.ScanResults = scanResult.Details
			
			// Log security violation for malware detection
			fv.auditLogger.LogSecurityViolation("", "", originalFilename, "Malware detected: "+scanResult.Details, "")
			
			// Quarantine the file
			if err := fv.quarantineFile(filePath, originalFilename); err != nil {
				log.Printf("Failed to quarantine file: %v", err)
			} else {
				result.Quarantined = true
				// Log file quarantine
				fv.auditLogger.LogTransferProgress("", "", AuditEventFileQuarantined, map[string]interface{}{
					"filename": originalFilename,
					"file_size": result.FileSize,
					"reason": "Malware detected",
					"scan_results": scanResult.Details,
				})
			}
		}
	}

	// Log successful file validation if no errors
	if result.Valid {
		fv.auditLogger.LogTransferProgress("", "", AuditEventFileValidated, map[string]interface{}{
			"filename": originalFilename,
			"file_size": result.FileSize,
			"mime_type": result.MimeType,
			"checksum": result.Checksum,
		})
	}

	return result, nil
}

// validateFilename checks if the filename is valid
func (fv *FileValidator) validateFilename(filename string) error {
	if len(filename) == 0 {
		return fmt.Errorf("filename cannot be empty")
	}

	if len(filename) > fv.config.MaxFilenameLength {
		return fmt.Errorf("filename too long (max %d characters)", fv.config.MaxFilenameLength)
	}

	// Check for dangerous characters
	dangerousChars := []string{"<", ">", ":", "\"", "|", "?", "*", "\x00"}
	for _, char := range dangerousChars {
		if strings.Contains(filename, char) {
			return fmt.Errorf("filename contains dangerous character: %s", char)
		}
	}

	// Check for reserved names (Windows)
	reservedNames := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	baseFilename := strings.ToUpper(strings.TrimSuffix(filename, filepath.Ext(filename)))
	for _, reserved := range reservedNames {
		if baseFilename == reserved {
			return fmt.Errorf("filename uses reserved name: %s", reserved)
		}
	}

	return nil
}

// validateFileExtension checks if the file extension is allowed
func (fv *FileValidator) validateFileExtension(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	
	// Check blocked extensions
	for _, blocked := range fv.config.BlockedExtensions {
		if ext == strings.ToLower(blocked) {
			return fmt.Errorf("file extension %s is blocked", ext)
		}
	}

	return nil
}

// detectMimeType detects the MIME type of a file
func (fv *FileValidator) detectMimeType(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read first 512 bytes for MIME type detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Use Go's built-in MIME type detection
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		// Fallback to content-based detection
		mimeType = "application/octet-stream" // Default binary type
		
		// Simple content-based detection
		content := buffer[:n]
		if len(content) >= 4 {
			// PDF
			if string(content[:4]) == "%PDF" {
				mimeType = "application/pdf"
			}
			// ZIP
			if content[0] == 0x50 && content[1] == 0x4B {
				mimeType = "application/zip"
			}
			// JPEG
			if content[0] == 0xFF && content[1] == 0xD8 {
				mimeType = "image/jpeg"
			}
			// PNG
			if len(content) >= 8 && content[0] == 0x89 && string(content[1:4]) == "PNG" {
				mimeType = "image/png"
			}
		}
	}

	return mimeType, nil
}

// validateMimeType checks if the MIME type is allowed
func (fv *FileValidator) validateMimeType(mimeType string) error {
	if len(fv.config.AllowedMimeTypes) == 0 {
		return nil // No restrictions
	}

	for _, allowed := range fv.config.AllowedMimeTypes {
		if mimeType == allowed {
			return nil
		}
	}

	return fmt.Errorf("MIME type %s is not allowed", mimeType)
}

// calculateChecksum calculates the file checksum
func (fv *FileValidator) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	switch fv.config.ChecksumAlgorithm {
	case "SHA256":
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(hash.Sum(nil)), nil
	default:
		return "", fmt.Errorf("unsupported checksum algorithm: %s", fv.config.ChecksumAlgorithm)
	}
}

// MalwareScanResult represents the result of a malware scan
type MalwareScanResult struct {
	Clean   bool   `json:"clean"`
	Details string `json:"details"`
	Scanner string `json:"scanner"`
}

// scanForMalware performs malware scanning (placeholder implementation)
func (fv *FileValidator) scanForMalware(filePath string) (*MalwareScanResult, error) {
	// This is a placeholder implementation
	// In production, this would integrate with antivirus engines like ClamAV
	
	// For now, just check file size and some basic heuristics
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// Simple heuristic: files larger than 100MB might be suspicious
	if fileInfo.Size() > 100*1024*1024 {
		return &MalwareScanResult{
			Clean:   false,
			Details: "File size exceeds safety threshold",
			Scanner: "basic_heuristic",
		}, nil
	}

	return &MalwareScanResult{
		Clean:   true,
		Details: "No threats detected",
		Scanner: "basic_heuristic",
	}, nil
}

// quarantineFile moves a file to quarantine
func (fv *FileValidator) quarantineFile(filePath, originalFilename string) error {
	timestamp := time.Now().Format("20060102_150405")
	quarantinePath := filepath.Join(fv.config.QuarantineDir, fmt.Sprintf("%s_%s", timestamp, originalFilename))

	return os.Rename(filePath, quarantinePath)
}

// FileEncryptor handles file encryption and decryption
type FileEncryptor struct {
	key []byte
}

// NewFileEncryptor creates a new file encryptor
func NewFileEncryptor(key []byte) *FileEncryptor {
	if len(key) != 32 {
		panic("encryption key must be 32 bytes for AES-256")
	}

	return &FileEncryptor{
		key: key,
	}
}

// EncryptFile encrypts a file using AES-256-GCM
func (fe *FileEncryptor) EncryptFile(inputPath, outputPath string) error {
	// Read input file
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(fe.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Write encrypted file
	if err := os.WriteFile(outputPath, ciphertext, 0644); err != nil {
		return fmt.Errorf("failed to write encrypted file: %v", err)
	}

	return nil
}

// DecryptFile decrypts a file using AES-256-GCM
func (fe *FileEncryptor) DecryptFile(inputPath, outputPath string) error {
	// Read encrypted file
	ciphertext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %v", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(fe.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %v", err)
	}

	// Write decrypted file
	if err := os.WriteFile(outputPath, plaintext, 0644); err != nil {
		return fmt.Errorf("failed to write decrypted file: %v", err)
	}

	return nil
}

// EncryptChunk encrypts a data chunk
func (fe *FileEncryptor) EncryptChunk(data []byte) ([]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(fe.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

// DecryptChunk decrypts a data chunk
func (fe *FileEncryptor) DecryptChunk(ciphertext []byte) ([]byte, error) {
	// Create AES cipher
	block, err := aes.NewCipher(fe.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %v", err)
	}

	return plaintext, nil
}

// GenerateFileChecksum generates a checksum for file integrity verification
func GenerateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyFileChecksum verifies a file's checksum
func VerifyFileChecksum(filePath, expectedChecksum string) (bool, error) {
	actualChecksum, err := GenerateFileChecksum(filePath)
	if err != nil {
		return false, err
	}

	return actualChecksum == expectedChecksum, nil
}

// SecureDelete securely deletes a file by overwriting it multiple times
func SecureDelete(filePath string) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	fileSize := fileInfo.Size()

	// Overwrite with random data 3 times
	for i := 0; i < 3; i++ {
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}

		// Write random data
		buffer := make([]byte, 4096)
		for written := int64(0); written < fileSize; {
			toWrite := int64(len(buffer))
			if written+toWrite > fileSize {
				toWrite = fileSize - written
			}

			if _, err := rand.Read(buffer[:toWrite]); err != nil {
				return err
			}

			if _, err := file.Write(buffer[:toWrite]); err != nil {
				return err
			}

			written += toWrite
		}

		if err := file.Sync(); err != nil {
			return err
		}
	}

	// Finally, delete the file
	return os.Remove(filePath)
}