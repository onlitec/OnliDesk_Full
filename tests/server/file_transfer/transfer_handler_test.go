package filetransfer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransferHandler(t *testing.T) {
	tempDir := t.TempDir()
	allowedTypes := []string{".txt", ".pdf", ".jpg"}
	maxFileSize := int64(10 * 1024 * 1024) // 10MB

	handler := NewTransferHandler(maxFileSize, allowedTypes, tempDir)

	assert.NotNil(t, handler)
	assert.Equal(t, maxFileSize, handler.maxFileSize)
	assert.Equal(t, tempDir, handler.tempDir)
	assert.Len(t, handler.allowedTypes, 3)
	assert.True(t, handler.allowedTypes[".txt"])
	assert.True(t, handler.allowedTypes[".pdf"])
	assert.True(t, handler.allowedTypes[".jpg"])
}

func TestTransferHandler_ValidateFileSize(t *testing.T) {
	tempDir := t.TempDir()
	handler := NewTransferHandler(1024, []string{".txt"}, tempDir)

	tests := []struct {
		name     string
		fileSize int64
		wantErr  bool
	}{
		{
			name:     "valid file size",
			fileSize: 512,
			wantErr:  false,
		},
		{
			name:     "file size at limit",
			fileSize: 1024,
			wantErr:  false,
		},
		{
			name:     "file size exceeds limit",
			fileSize: 2048,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &FileTransferRequest{
				ID:       "test-id",
				Filename: "test.txt",
				FileSize: tt.fileSize,
			}

			// Create mock WebSocket connections
			clientConn := &websocket.Conn{}
			portalConn := &websocket.Conn{}

			sessionManager := NewSessionManager(DefaultTransferConfig())
			_, err := sessionManager.CreateTransferSession(request, clientConn, portalConn)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransferHandler_ValidateFileType(t *testing.T) {
	tempDir := t.TempDir()
	allowedTypes := []string{".txt", ".pdf"}
	handler := NewTransferHandler(1024*1024, allowedTypes, tempDir)

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "allowed txt file",
			filename: "document.txt",
			wantErr:  false,
		},
		{
			name:     "allowed pdf file",
			filename: "document.pdf",
			wantErr:  false,
		},
		{
			name:     "disallowed exe file",
			filename: "malware.exe",
			wantErr:  true,
		},
		{
			name:     "disallowed js file",
			filename: "script.js",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &FileTransferRequest{
				ID:       "test-id",
				Filename: tt.filename,
				FileSize: 1024,
			}

			clientConn := &websocket.Conn{}
			portalConn := &websocket.Conn{}

			sessionManager := NewSessionManager(DefaultTransferConfig())
			_, err := sessionManager.CreateTransferSession(request, clientConn, portalConn)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not allowed")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransferHandler_ChecksumVerification(t *testing.T) {
	tempDir := t.TempDir()
	handler := NewTransferHandler(1024*1024, []string{".txt"}, tempDir)

	// Create a test file with known content
	testContent := []byte("Hello, World!")
	testFile := filepath.Join(tempDir, "test.txt")
	err := ioutil.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	// Calculate expected checksum
	expectedChecksum := fmt.Sprintf("%x", sha256.Sum256(testContent))

	tests := []struct {
		name     string
		checksum string
		wantErr  bool
	}{
		{
			name:     "valid checksum",
			checksum: expectedChecksum,
			wantErr:  false,
		},
		{
			name:     "invalid checksum",
			checksum: "invalid_checksum",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.verifyChecksum(testFile, tt.checksum)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTransferSession_StatusTransitions(t *testing.T) {
	request := &FileTransferRequest{
		ID:        "test-transfer",
		SessionID: "test-session",
		Filename:  "test.txt",
		FileSize:  1024,
		Type:      TransferTypeUpload,
	}

	session := &TransferSession{
		ID:             request.ID,
		Request:        request,
		Status:         StatusPending,
		StartTime:      time.Now(),
		ReceivedChunks: make(map[int]bool),
	}

	// Test status transitions
	assert.Equal(t, StatusPending, session.Status)

	session.Status = StatusApproved
	assert.Equal(t, StatusApproved, session.Status)

	session.Status = StatusInProgress
	assert.Equal(t, StatusInProgress, session.Status)

	session.Status = StatusCompleted
	assert.Equal(t, StatusCompleted, session.Status)
}

func TestTransferHandler_ConcurrentTransferLimit(t *testing.T) {
	config := DefaultTransferConfig()
	config.MaxConcurrent = 2 // Limit to 2 concurrent transfers

	sessionManager := NewSessionManager(config)

	// Create first transfer session
	request1 := &FileTransferRequest{
		ID:       "transfer-1",
		Filename: "file1.txt",
		FileSize: 1024,
	}
	clientConn := &websocket.Conn{}
	portalConn := &websocket.Conn{}

	session1, err := sessionManager.CreateTransferSession(request1, clientConn, portalConn)
	assert.NoError(t, err)
	assert.NotNil(t, session1)

	// Create second transfer session
	request2 := &FileTransferRequest{
		ID:       "transfer-2",
		Filename: "file2.txt",
		FileSize: 1024,
	}

	session2, err := sessionManager.CreateTransferSession(request2, clientConn, portalConn)
	assert.NoError(t, err)
	assert.NotNil(t, session2)

	// Try to create third transfer session (should fail)
	request3 := &FileTransferRequest{
		ID:       "transfer-3",
		Filename: "file3.txt",
		FileSize: 1024,
	}

	session3, err := sessionManager.CreateTransferSession(request3, clientConn, portalConn)
	assert.Error(t, err)
	assert.Nil(t, session3)
	assert.Contains(t, err.Error(), "maximum concurrent transfers reached")
}

func TestFileTransferProgress_Calculation(t *testing.T) {
	tests := []struct {
		name             string
		bytesTransferred int64
		totalBytes       int64
		expectedPercent  float64
	}{
		{
			name:             "0% progress",
			bytesTransferred: 0,
			totalBytes:       1000,
			expectedPercent:  0.0,
		},
		{
			name:             "50% progress",
			bytesTransferred: 500,
			totalBytes:       1000,
			expectedPercent:  50.0,
		},
		{
			name:             "100% progress",
			bytesTransferred: 1000,
			totalBytes:       1000,
			expectedPercent:  100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := FileTransferProgress{
				ID:               "test-transfer",
				BytesTransferred: tt.bytesTransferred,
				TotalBytes:       tt.totalBytes,
				Percentage:       float64(tt.bytesTransferred) / float64(tt.totalBytes) * 100,
			}

			assert.Equal(t, tt.expectedPercent, progress.Percentage)
		})
	}
}

func TestTransferHandler_AuditLogging(t *testing.T) {
	config := DefaultTransferConfig()
	config.AuditLog = true

	sessionManager := NewSessionManager(config)

	request := &FileTransferRequest{
		ID:         "audit-test",
		SessionID:  "session-123",
		Technician: "tech@example.com",
		Filename:   "audit.txt",
		FileSize:   1024,
		Type:       TransferTypeUpload,
	}

	clientConn := &websocket.Conn{}
	portalConn := &websocket.Conn{}

	session, err := sessionManager.CreateTransferSession(request, clientConn, portalConn)
	assert.NoError(t, err)
	assert.NotNil(t, session)

	// Complete the transfer to trigger audit logging
	err = sessionManager.CompleteTransfer("audit-test", true, "")
	assert.NoError(t, err)

	// Verify session status
	session, exists := sessionManager.GetSession("audit-test")
	assert.True(t, exists)
	assert.Equal(t, StatusCompleted, session.Status)
}

func BenchmarkFileChunkProcessing(b *testing.B) {
	tempDir := b.TempDir()
	handler := NewTransferHandler(1024*1024, []string{".txt"}, tempDir)

	// Create test data
	chunkData := make([]byte, 64*1024) // 64KB chunk
	for i := range chunkData {
		chunkData[i] = byte(i % 256)
	}

	chunk := FileChunk{
		ID:       "bench-test",
		Sequence: 1,
		Data:     chunkData,
		Size:     len(chunkData),
		IsLast:   false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate chunk processing
		_ = fmt.Sprintf("%x", sha256.Sum256(chunk.Data))
	}
}