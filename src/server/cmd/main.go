package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/onlitec/onlidesk-server/internal/filetransfer"
	"github.com/onlitec/onlidesk-server/internal/remoteaccess"
)

// ServerConfig holds the server configuration
type ServerConfig struct {
	Port               string                           `json:"port"`
	Host               string                           `json:"host"`
	TLSEnabled         bool                             `json:"tls_enabled"`
	CertFile           string                           `json:"cert_file"`
	KeyFile            string                           `json:"key_file"`
	TransferConfig     *filetransfer.TransferConfig     `json:"transfer_config"`
	SecurityConfig     *filetransfer.SecurityConfig     `json:"security_config"`
	RemoteAccessConfig *remoteaccess.RemoteAccessConfig `json:"remote_access_config"`
	CORSOrigins        []string                         `json:"cors_origins"`
	LogLevel           string                           `json:"log_level"`
	MaxConnections     int                              `json:"max_connections"`
	ReadTimeout        time.Duration                    `json:"read_timeout"`
	WriteTimeout       time.Duration                    `json:"write_timeout"`
	IdleTimeout        time.Duration                    `json:"idle_timeout"`
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Port:               "8080",
		Host:               "localhost",
		TLSEnabled:         false,
		CertFile:           "./certs/server.crt",
		KeyFile:            "./certs/server.key",
		TransferConfig:     filetransfer.DefaultTransferConfig(),
		SecurityConfig:     filetransfer.DefaultSecurityConfig(),
		RemoteAccessConfig: remoteaccess.DefaultRemoteAccessConfig(),
		CORSOrigins:        []string{"http://localhost:3000", "http://localhost:5173"}, // React dev servers
		LogLevel:           "info",
		MaxConnections:     1000,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        60 * time.Second,
	}
}

// OnlideskServer represents the main server instance
type OnlideskServer struct {
	config                 *ServerConfig
	fileTransferHandler    *filetransfer.WebSocketHandler
	remoteAccessHandler    *remoteaccess.WebSocketHandler
	remoteAccessHTTP       *remoteaccess.HTTPHandlers
	sessionManager         *remoteaccess.SessionManager
	httpServer             *http.Server
	router                 *mux.Router
}

// NewOnlideskServer creates a new server instance
func NewOnlideskServer(configPath string) (*OnlideskServer, error) {
	// Load configuration
	config, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Failed to load config, using defaults: %v", err)
		config = DefaultServerConfig()
	}

	// Create file transfer handler
	fileTransferHandler := filetransfer.NewWebSocketHandler(config.TransferConfig, config.SecurityConfig)

	// Create remote access components
	sessionManager := remoteaccess.NewSessionManager(config.RemoteAccessConfig)
	remoteAccessHandler := remoteaccess.NewWebSocketHandler(config.RemoteAccessConfig)
	remoteAccessHTTP := remoteaccess.NewHTTPHandlers(sessionManager)

	// Create router
	router := mux.NewRouter()

	// Create server instance
	server := &OnlideskServer{
		config:                 config,
		fileTransferHandler:    fileTransferHandler,
		remoteAccessHandler:    remoteAccessHandler,
		remoteAccessHTTP:       remoteAccessHTTP,
		sessionManager:         sessionManager,
		router:                 router,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%s", config.Host, config.Port),
		Handler:      server.setupCORS(),
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return server, nil
}

// setupRoutes configures all HTTP routes
func (s *OnlideskServer) setupRoutes() {
	// Health check endpoint
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// API info endpoint
	s.router.HandleFunc("/api/info", s.handleAPIInfo).Methods("GET")

	// WebSocket endpoints
	s.router.HandleFunc("/ws/filetransfer", s.fileTransferHandler.HandleWebSocket)
	s.router.HandleFunc("/ws/remoteaccess", s.remoteAccessHandler.HandleWebSocket)

	// File transfer REST API endpoints
	api := s.router.PathPrefix("/api/v1").Subrouter()
	
	// Transfer management endpoints
	api.HandleFunc("/transfers", s.handleGetTransfers).Methods("GET")
	api.HandleFunc("/transfers/{transferId}", s.handleGetTransfer).Methods("GET")
	api.HandleFunc("/transfers/{transferId}/approve", s.handleApproveTransfer).Methods("POST")
	api.HandleFunc("/transfers/{transferId}/control", s.handleControlTransfer).Methods("POST")
	api.HandleFunc("/transfers/{transferId}/progress", s.handleGetProgress).Methods("GET")
	
	// Configuration endpoints
	api.HandleFunc("/config/transfer", s.handleGetTransferConfig).Methods("GET")
	api.HandleFunc("/config/transfer", s.handleUpdateTransferConfig).Methods("PUT")
	
	// Statistics endpoint
	api.HandleFunc("/stats", s.handleGetStatistics).Methods("GET")
	
	// File download endpoint (for completed transfers)
	api.HandleFunc("/files/{transferId}/download", s.handleFileDownload).Methods("GET")

	// Register remote access HTTP routes
	s.remoteAccessHTTP.RegisterRoutes(s.router)

	// Static file serving for the portal (if needed)
	s.router.PathPrefix("/portal/").Handler(http.StripPrefix("/portal/", http.FileServer(http.Dir("./static/portal/"))))

	log.Println("Routes configured successfully")
}

// setupCORS configures CORS middleware
func (s *OnlideskServer) setupCORS() http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   s.config.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	})

	return c.Handler(s.router)
}

// HTTP Handlers

// handleHealth returns server health status
func (s *OnlideskServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"uptime":    time.Since(time.Now()), // This would be calculated from server start time
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleAPIInfo returns API information
func (s *OnlideskServer) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"name":        "Onlidesk File Transfer API",
		"version":     "1.0.0",
		"description": "Secure file transfer API for remote desktop sessions",
		"endpoints": map[string]string{
			"websocket":     "/ws/filetransfer",
			"transfers":     "/api/v1/transfers",
			"config":        "/api/v1/config/transfer",
			"statistics":    "/api/v1/stats",
			"health":        "/health",
		},
		"features": []string{
			"Secure file transfer",
			"Real-time progress tracking",
			"Transfer approval workflow",
			"File encryption",
			"Malware scanning",
			"Audit logging",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleGetTransfers returns all active transfers
func (s *OnlideskServer) handleGetTransfers(w http.ResponseWriter, r *http.Request) {
	sessions := s.fileTransferHandler.GetSessionManager().GetActiveSessions()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleGetTransfer returns a specific transfer
func (s *OnlideskServer) handleGetTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transferId"]

	session, exists := s.fileTransferHandler.GetSessionManager().GetSession(transferID)
	if !exists {
		http.Error(w, "Transfer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// handleApproveTransfer approves or rejects a transfer
func (s *OnlideskServer) handleApproveTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transferId"]

	var approval struct {
		Approved bool   `json:"approved"`
		Message  string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&approval); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.fileTransferHandler.GetSessionManager().ApproveTransfer(transferID, approval.Approved, approval.Message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleControlTransfer controls a transfer (pause, resume, cancel)
func (s *OnlideskServer) handleControlTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transferId"]

	var control struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&control); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	switch control.Action {
	case "pause":
		err = s.fileTransferHandler.GetSessionManager().PauseTransfer(transferID)
	case "resume":
		err = s.fileTransferHandler.GetSessionManager().ResumeTransfer(transferID)
	case "cancel":
		err = s.fileTransferHandler.GetSessionManager().CancelTransfer(transferID)
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleGetProgress returns transfer progress
func (s *OnlideskServer) handleGetProgress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transferId"]

	progress, err := s.fileTransferHandler.GetSessionManager().GetTransferProgress(transferID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

// handleGetTransferConfig returns current transfer configuration
func (s *OnlideskServer) handleGetTransferConfig(w http.ResponseWriter, r *http.Request) {
	config := s.fileTransferHandler.GetSessionManager().GetConfig()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// handleUpdateTransferConfig updates transfer configuration
func (s *OnlideskServer) handleUpdateTransferConfig(w http.ResponseWriter, r *http.Request) {
	var config filetransfer.TransferConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.fileTransferHandler.GetSessionManager().UpdateConfig(&config)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleGetStatistics returns server statistics
func (s *OnlideskServer) handleGetStatistics(w http.ResponseWriter, r *http.Request) {
	stats := s.fileTransferHandler.GetStatistics()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleFileDownload serves completed file transfers
func (s *OnlideskServer) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transferId"]

	session, exists := s.fileTransferHandler.GetSessionManager().GetSession(transferID)
	if !exists {
		http.Error(w, "Transfer not found", http.StatusNotFound)
		return
	}

	if session.Status != filetransfer.StatusCompleted {
		http.Error(w, "Transfer not completed", http.StatusBadRequest)
		return
	}

	if session.TempPath == "" {
		http.Error(w, "File not available", http.StatusNotFound)
		return
	}

	// Serve the file
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", session.Request.Filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, session.TempPath)
}

// Start starts the server
func (s *OnlideskServer) Start() error {
	log.Printf("Starting Onlidesk server on %s:%s", s.config.Host, s.config.Port)
	log.Printf("TLS enabled: %t", s.config.TLSEnabled)
	log.Printf("Max file size: %d MB", s.config.TransferConfig.MaxFileSize/(1024*1024))
	log.Printf("Max concurrent transfers: %d", s.config.TransferConfig.MaxConcurrent)

	if s.config.TLSEnabled {
		log.Printf("Starting HTTPS server with cert: %s, key: %s", s.config.CertFile, s.config.KeyFile)
		return s.httpServer.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	} else {
		log.Printf("Starting HTTP server (WARNING: TLS disabled)")
		return s.httpServer.ListenAndServe()
	}
}

// Stop gracefully stops the server
func (s *OnlideskServer) Stop(ctx context.Context) error {
	log.Println("Shutting down server...")

	// Shutdown file transfer handler
	s.fileTransferHandler.Shutdown()

	// Close remote access components
	if s.sessionManager != nil {
		s.sessionManager.Shutdown()
	}
	if s.remoteAccessHandler != nil {
		s.remoteAccessHandler.Shutdown()
	}

	// Shutdown HTTP server
	return s.httpServer.Shutdown(ctx)
}

// loadConfig loads server configuration from file
func loadConfig(configPath string) (*ServerConfig, error) {
	if configPath == "" {
		configPath = "./config/server.json"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Ensure default configurations are set if missing
	if config.TransferConfig == nil {
		config.TransferConfig = filetransfer.DefaultTransferConfig()
	}
	if config.SecurityConfig == nil {
		config.SecurityConfig = filetransfer.DefaultSecurityConfig()
	} else {
		// Generate encryption key if not present (since it's not serialized)
		if len(config.SecurityConfig.EncryptionKey) == 0 {
			defaultSecurity := filetransfer.DefaultSecurityConfig()
			config.SecurityConfig.EncryptionKey = defaultSecurity.EncryptionKey
		}
	}
	if config.RemoteAccessConfig == nil {
		config.RemoteAccessConfig = remoteaccess.DefaultRemoteAccessConfig()
	}

	return &config, nil
}

// saveDefaultConfig saves a default configuration file
func saveDefaultConfig(configPath string) error {
	config := DefaultServerConfig()

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	log.Printf("Default configuration saved to: %s", configPath)
	return nil
}

func main() {
	// Parse command line arguments
	configPath := "./config/server.json"
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--config":
			if len(os.Args) > 2 {
				configPath = os.Args[2]
			}
		case "--generate-config":
			if len(os.Args) > 2 {
				configPath = os.Args[2]
			}
			if err := saveDefaultConfig(configPath); err != nil {
				log.Fatalf("Failed to generate config: %v", err)
			}
			return
		case "--help":
			fmt.Println("Onlidesk Server")
			fmt.Println("Usage:")
			fmt.Println("  server                           Start server with default config")
			fmt.Println("  server --config <path>           Start server with custom config")
			fmt.Println("  server --generate-config <path>  Generate default config file")
			fmt.Println("  server --help                    Show this help")
			return
		}
	}

	// Create server
	server, err := NewOnlideskServer(configPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	log.Println("Server started successfully. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful shutdown
	if err := server.Stop(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	} else {
		log.Println("Server stopped gracefully")
	}
}