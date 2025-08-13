package filetransfer

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// ChunkSize defines the size of each file chunk (64KB)
	ChunkSize = 64 * 1024
	// MaxConcurrentTransfers limits the number of simultaneous transfers
	MaxConcurrentTransfers = 5
	// TransferTimeout defines the timeout for transfer operations
	TransferTimeout = 30 * time.Minute
	// RetryAttempts defines the number of retry attempts for failed chunks
	RetryAttempts = 3
)

// FileStream manages the streaming of file data
type FileStream struct {
	transferID    string
	filePath      string
	file          *os.File
	totalSize     int64
	chunkCount    int
	sentChunks    map[int]bool
	failedChunks  map[int]int // chunk -> retry count
	currentChunk  int
	isUpload      bool
	conn          *websocket.Conn
	progressChan  chan FileTransferProgress
	errorChan     chan error
	completeChan  chan bool
	cancelChan    chan bool
	pauseChan     chan bool
	resumeChan    chan bool
	mutex         sync.RWMutex
	active        bool
	paused        bool
	startTime     time.Time
	lastProgress  time.Time
	bytesPerSec   int64
}

// NewFileStream creates a new file stream instance
func NewFileStream(transferID, filePath string, isUpload bool, conn *websocket.Conn) (*FileStream, error) {
	var file *os.File
	var totalSize int64
	var err error

	if isUpload {
		// For uploads, create the file
		file, err = os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %v", err)
		}
		totalSize = 0 // Will be set when we receive the first chunk with metadata
	} else {
		// For downloads, open existing file
		file, err = os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open file: %v", err)
		}
		
		// Get file size
		stat, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to get file stats: %v", err)
		}
		totalSize = stat.Size()
	}

	chunkCount := int((totalSize + ChunkSize - 1) / ChunkSize) // Ceiling division

	return &FileStream{
		transferID:   transferID,
		filePath:     filePath,
		file:         file,
		totalSize:    totalSize,
		chunkCount:   chunkCount,
		sentChunks:   make(map[int]bool),
		failedChunks: make(map[int]int),
		isUpload:     isUpload,
		conn:         conn,
		progressChan: make(chan FileTransferProgress, 100),
		errorChan:    make(chan error, 10),
		completeChan: make(chan bool, 1),
		cancelChan:   make(chan bool, 1),
		pauseChan:    make(chan bool, 1),
		resumeChan:   make(chan bool, 1),
		startTime:    time.Now(),
		lastProgress: time.Now(),
	}, nil
}

// StartDownload begins downloading a file to the client
func (fs *FileStream) StartDownload() error {
	fs.mutex.Lock()
	fs.active = true
	fs.mutex.Unlock()

	go fs.downloadWorker()
	go fs.progressMonitor()

	return nil
}

// StartUpload begins receiving an uploaded file from the client
func (fs *FileStream) StartUpload() error {
	fs.mutex.Lock()
	fs.active = true
	fs.mutex.Unlock()

	go fs.uploadWorker()
	go fs.progressMonitor()

	return nil
}

// downloadWorker handles the download process
func (fs *FileStream) downloadWorker() {
	defer fs.cleanup()

	reader := bufio.NewReader(fs.file)
	buffer := make([]byte, ChunkSize)

	for chunkIndex := 0; chunkIndex < fs.chunkCount; chunkIndex++ {
		// Check for pause/cancel signals
		select {
		case <-fs.cancelChan:
			log.Printf("Download cancelled: %s", fs.transferID)
			return
		case <-fs.pauseChan:
			fs.mutex.Lock()
			fs.paused = true
			fs.mutex.Unlock()
			
			// Wait for resume signal
			<-fs.resumeChan
			
			fs.mutex.Lock()
			fs.paused = false
			fs.mutex.Unlock()
		default:
		}

		// Read chunk from file
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			fs.errorChan <- fmt.Errorf("error reading file chunk %d: %v", chunkIndex, err)
			return
		}

		if n == 0 {
			break
		}

		// Create chunk
		chunk := FileChunk{
			ID:       fs.transferID,
			Sequence: chunkIndex,
			Data:     buffer[:n],
			Size:     n,
			IsLast:   chunkIndex == fs.chunkCount-1,
			Checksum: fs.calculateChunkChecksum(buffer[:n]),
		}

		// Send chunk with retry logic
		if err := fs.sendChunkWithRetry(chunk); err != nil {
			fs.errorChan <- fmt.Errorf("failed to send chunk %d after retries: %v", chunkIndex, err)
			return
		}

		// Mark chunk as sent
		fs.mutex.Lock()
		fs.sentChunks[chunkIndex] = true
		fs.currentChunk = chunkIndex + 1
		fs.mutex.Unlock()

		// Send progress update
		fs.sendProgress()
	}

	// Signal completion
	fs.completeChan <- true
	log.Printf("Download completed: %s", fs.transferID)
}

// uploadWorker handles the upload process
func (fs *FileStream) uploadWorker() {
	defer fs.cleanup()

	writer := bufio.NewWriter(fs.file)
	receivedChunks := make(map[int][]byte)
	expectedChunk := 0

	// Listen for incoming chunks
	for {
		select {
		case <-fs.cancelChan:
			log.Printf("Upload cancelled: %s", fs.transferID)
			return
		case <-fs.pauseChan:
			fs.mutex.Lock()
			fs.paused = true
			fs.mutex.Unlock()
			
			// Wait for resume signal
			<-fs.resumeChan
			
			fs.mutex.Lock()
			fs.paused = false
			fs.mutex.Unlock()
		default:
			// Read message from WebSocket
			messageType, message, err := fs.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fs.errorChan <- fmt.Errorf("websocket error: %v", err)
				}
				return
			}

			if messageType == websocket.BinaryMessage {
				chunk, err := fs.parseChunk(message)
				if err != nil {
					log.Printf("Error parsing chunk: %v", err)
					continue
				}

				// Verify chunk checksum
				if !fs.verifyChunkChecksum(chunk) {
					log.Printf("Chunk checksum verification failed: %d", chunk.Sequence)
					// Request retransmission
					fs.requestChunkRetransmission(chunk.Sequence)
					continue
				}

				// Store chunk
				receivedChunks[chunk.Sequence] = chunk.Data

				// Write chunks in order
				for {
					if data, exists := receivedChunks[expectedChunk]; exists {
						if _, err := writer.Write(data); err != nil {
							fs.errorChan <- fmt.Errorf("error writing chunk %d: %v", expectedChunk, err)
							return
						}

						delete(receivedChunks, expectedChunk)
						expectedChunk++

						// Update progress
						fs.mutex.Lock()
						fs.currentChunk = expectedChunk
						fs.mutex.Unlock()

						fs.sendProgress()

						// Check if this was the last chunk
						if chunk.IsLast {
							writer.Flush()
							fs.completeChan <- true
							log.Printf("Upload completed: %s", fs.transferID)
							return
						}
					} else {
						break
					}
				}
			}
		}
	}
}

// sendChunkWithRetry sends a chunk with retry logic
func (fs *FileStream) sendChunkWithRetry(chunk FileChunk) error {
	retryCount := 0
	for retryCount < RetryAttempts {
		if err := fs.sendChunk(chunk); err != nil {
			retryCount++
			log.Printf("Failed to send chunk %d, attempt %d: %v", chunk.Sequence, retryCount, err)
			
			if retryCount < RetryAttempts {
				time.Sleep(time.Duration(retryCount) * time.Second) // Exponential backoff
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("max retry attempts exceeded")
}

// sendChunk sends a single chunk over WebSocket
func (fs *FileStream) sendChunk(chunk FileChunk) error {
	// Create chunk message with header + data
	header, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("error marshaling chunk header: %v", err)
	}

	// Pad header to fixed size (256 bytes)
	headerPadded := make([]byte, 256)
	copy(headerPadded, header)

	// Combine header and data
	message := append(headerPadded, chunk.Data...)

	// Send as binary message
	if err := fs.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
		return fmt.Errorf("error sending chunk: %v", err)
	}

	return nil
}

// parseChunk parses a received chunk from binary data
func (fs *FileStream) parseChunk(data []byte) (FileChunk, error) {
	var chunk FileChunk

	if len(data) < 256 {
		return chunk, fmt.Errorf("invalid chunk data: too short")
	}

	// Parse header
	header := data[:256]
	if err := json.Unmarshal(header, &chunk); err != nil {
		return chunk, fmt.Errorf("error parsing chunk header: %v", err)
	}

	// Extract data
	chunk.Data = data[256:]
	chunk.Size = len(chunk.Data)

	return chunk, nil
}

// calculateChunkChecksum calculates SHA-256 checksum for a chunk
func (fs *FileStream) calculateChunkChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// verifyChunkChecksum verifies the checksum of a received chunk
func (fs *FileStream) verifyChunkChecksum(chunk FileChunk) bool {
	expectedChecksum := fs.calculateChunkChecksum(chunk.Data)
	return expectedChecksum == chunk.Checksum
}

// requestChunkRetransmission requests retransmission of a failed chunk
func (fs *FileStream) requestChunkRetransmission(sequence int) {
	retransmissionRequest := map[string]interface{}{
		"type":     "chunk_retransmission_request",
		"id":       fs.transferID,
		"sequence": sequence,
	}

	message, err := json.Marshal(retransmissionRequest)
	if err != nil {
		log.Printf("Error marshaling retransmission request: %v", err)
		return
	}

	if err := fs.conn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Printf("Error sending retransmission request: %v", err)
	}
}

// sendProgress sends progress updates
func (fs *FileStream) sendProgress() {
	fs.mutex.RLock()
	currentChunk := fs.currentChunk
	fs.mutex.RUnlock()

	bytesTransferred := int64(currentChunk) * ChunkSize
	if bytesTransferred > fs.totalSize {
		bytesTransferred = fs.totalSize
	}

	percentage := float64(bytesTransferred) / float64(fs.totalSize) * 100

	// Calculate transfer speed
	now := time.Now()
	elapsed := now.Sub(fs.lastProgress).Seconds()
	if elapsed > 0 {
		fs.bytesPerSec = int64(float64(ChunkSize) / elapsed)
		fs.lastProgress = now
	}

	// Calculate ETA
	remainingBytes := fs.totalSize - bytesTransferred
	var eta int64
	if fs.bytesPerSec > 0 {
		eta = remainingBytes / fs.bytesPerSec
	}

	progress := FileTransferProgress{
		ID:               fs.transferID,
		BytesTransferred: bytesTransferred,
		TotalBytes:       fs.totalSize,
		Percentage:       percentage,
		Speed:            fs.bytesPerSec,
		ETA:              eta,
	}

	select {
	case fs.progressChan <- progress:
	default:
		// Channel full, skip this update
	}
}

// progressMonitor monitors and broadcasts progress updates
func (fs *FileStream) progressMonitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fs.sendProgress()
		case progress := <-fs.progressChan:
			// Send progress to WebSocket
			progressMsg := map[string]interface{}{
				"type":     "transfer_progress",
				"progress": progress,
			}

			message, err := json.Marshal(progressMsg)
			if err != nil {
				log.Printf("Error marshaling progress: %v", err)
				continue
			}

			if err := fs.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Error sending progress: %v", err)
				return
			}
		case <-fs.completeChan:
			return
		case err := <-fs.errorChan:
			log.Printf("File stream error: %v", err)
			return
		}
	}
}

// Pause pauses the file transfer
func (fs *FileStream) Pause() {
	fs.mutex.RLock()
	active := fs.active
	paused := fs.paused
	fs.mutex.RUnlock()

	if active && !paused {
		select {
		case fs.pauseChan <- true:
		default:
		}
	}
}

// Resume resumes the file transfer
func (fs *FileStream) Resume() {
	fs.mutex.RLock()
	active := fs.active
	paused := fs.paused
	fs.mutex.RUnlock()

	if active && paused {
		select {
		case fs.resumeChan <- true:
		default:
		}
	}
}

// Cancel cancels the file transfer
func (fs *FileStream) Cancel() {
	fs.mutex.RLock()
	active := fs.active
	fs.mutex.RUnlock()

	if active {
		select {
		case fs.cancelChan <- true:
		default:
		}
	}
}

// cleanup performs cleanup operations
func (fs *FileStream) cleanup() {
	fs.mutex.Lock()
	fs.active = false
	fs.mutex.Unlock()

	if fs.file != nil {
		fs.file.Close()
	}

	// Close channels
	close(fs.progressChan)
	close(fs.errorChan)
	close(fs.completeChan)
	close(fs.cancelChan)
	close(fs.pauseChan)
	close(fs.resumeChan)
}

// GetProgress returns the current transfer progress
func (fs *FileStream) GetProgress() FileTransferProgress {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	bytesTransferred := int64(fs.currentChunk) * ChunkSize
	if bytesTransferred > fs.totalSize {
		bytesTransferred = fs.totalSize
	}

	percentage := float64(bytesTransferred) / float64(fs.totalSize) * 100

	return FileTransferProgress{
		ID:               fs.transferID,
		BytesTransferred: bytesTransferred,
		TotalBytes:       fs.totalSize,
		Percentage:       percentage,
		Speed:            fs.bytesPerSec,
	}
}

// IsActive returns whether the stream is currently active
func (fs *FileStream) IsActive() bool {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.active
}

// IsPaused returns whether the stream is currently paused
func (fs *FileStream) IsPaused() bool {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.paused
}

// GetTransferInfo returns basic information about the transfer
func (fs *FileStream) GetTransferInfo() map[string]interface{} {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	return map[string]interface{}{
		"transfer_id":   fs.transferID,
		"file_path":     fs.filePath,
		"total_size":    fs.totalSize,
		"chunk_count":   fs.chunkCount,
		"current_chunk": fs.currentChunk,
		"is_upload":     fs.isUpload,
		"active":        fs.active,
		"paused":        fs.paused,
		"start_time":    fs.startTime,
		"bytes_per_sec": fs.bytesPerSec,
	}
}

// WriteChunk writes a chunk of data to the file
func (fs *FileStream) WriteChunk(chunkIndex int, data []byte) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if !fs.active {
		return fmt.Errorf("file stream is not active")
	}

	if fs.paused {
		return fmt.Errorf("file stream is paused")
	}

	// Calculate the offset for this chunk
	offset := int64(chunkIndex) * ChunkSize

	// Seek to the correct position in the file
	if _, err := fs.file.Seek(offset, 0); err != nil {
		return fmt.Errorf("failed to seek to chunk position: %v", err)
	}

	// Write the chunk data
	if _, err := fs.file.Write(data); err != nil {
		return fmt.Errorf("failed to write chunk data: %v", err)
	}

	// Mark this chunk as received
	fs.sentChunks[chunkIndex] = true
	fs.currentChunk = chunkIndex + 1

	// Update progress
	fs.sendProgress()

	return nil
}