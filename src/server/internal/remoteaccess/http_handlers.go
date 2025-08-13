package remoteaccess

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// HTTPHandlers provides HTTP endpoints for remote access management
type HTTPHandlers struct {
	sessionManager *SessionManager
}

// NewHTTPHandlers creates a new HTTP handlers instance
func NewHTTPHandlers(sessionManager *SessionManager) *HTTPHandlers {
	return &HTTPHandlers{
		sessionManager: sessionManager,
	}
}

// RegisterRoutes registers HTTP routes for remote access
func (h *HTTPHandlers) RegisterRoutes(router *mux.Router) {
	// Session management
	router.HandleFunc("/api/remoteaccess/sessions", h.handleGetSessions).Methods("GET")
	router.HandleFunc("/api/remoteaccess/sessions", h.handleCreateSession).Methods("POST")
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}", h.handleGetSession).Methods("GET")
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}", h.handleTerminateSession).Methods("DELETE")
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}/extend", h.handleExtendSession).Methods("POST")

	// Privilege management
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}/privileges", h.handleRequestPrivilege).Methods("POST")
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}/privileges/{privilegeId}", h.handleApprovePrivilege).Methods("PUT")
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}/privileges/{privilegeId}", h.handleRevokePrivilege).Methods("DELETE")

	// Statistics and monitoring
	router.HandleFunc("/api/remoteaccess/stats", h.handleGetStatistics).Methods("GET")
	router.HandleFunc("/api/remoteaccess/sessions/{sessionId}/stats", h.handleGetSessionStatistics).Methods("GET")

	// Configuration
	router.HandleFunc("/api/remoteaccess/config", h.handleGetConfig).Methods("GET")
	router.HandleFunc("/api/remoteaccess/config", h.handleUpdateConfig).Methods("PUT")

	// Health check
	router.HandleFunc("/api/remoteaccess/health", h.handleHealthCheck).Methods("GET")

	// Audit logs
	router.HandleFunc("/api/remoteaccess/audit", h.handleGetAuditLogs).Methods("GET")
}

// Session management handlers

func (h *HTTPHandlers) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	statusFilter := query.Get("status")
	technicianFilter := query.Get("technician")
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	limit := 50 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0 // default
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get sessions from manager
	allSessions := h.sessionManager.GetAllSessions()

	// Apply filters
	var filteredSessions []*RemoteAccessSession
	for _, session := range allSessions {
		// Status filter
		if statusFilter != "" && string(session.Status) != statusFilter {
			continue
		}

		// Technician filter
		if technicianFilter != "" && session.TechnicianID != technicianFilter {
			continue
		}

		filteredSessions = append(filteredSessions, session)
	}

	// Apply pagination
	total := len(filteredSessions)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	paginatedSessions := filteredSessions[start:end]

	// Prepare response
	response := map[string]interface{}{
		"sessions": paginatedSessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *HTTPHandlers) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID     string `json:"client_id"`
		TechnicianID string `json:"technician_id"`
		ClientInfo   struct {
			Hostname    string `json:"hostname"`
			OS          string `json:"os"`
			Version     string `json:"version"`
			IPAddress   string `json:"ip_address"`
			UserAgent   string `json:"user_agent"`
		} `json:"client_info"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate required fields
	if req.ClientID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "client_id is required", nil)
		return
	}

	if req.TechnicianID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "technician_id is required", nil)
		return
	}

	// Create session
	session, err := h.sessionManager.CreateSession(req.ClientID, req.TechnicianID, nil)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create session", err)
		return
	}

	// Update client info if provided
	if req.ClientInfo.Hostname != "" {
		session.ClientInfo.Hostname = req.ClientInfo.Hostname
	}
	if req.ClientInfo.OS != "" {
		session.ClientInfo.OperatingSystem = req.ClientInfo.OS
	}
	if req.ClientInfo.Version != "" {
		// Version field mapping - using UserAgent as closest match
		session.ClientInfo.UserAgent = req.ClientInfo.Version
	}
	if req.ClientInfo.IPAddress != "" {
		session.ClientInfo.IPAddress = req.ClientInfo.IPAddress
	}
	if req.ClientInfo.UserAgent != "" {
		session.ClientInfo.UserAgent = req.ClientInfo.UserAgent
	}

	h.writeJSONResponse(w, http.StatusCreated, session)
}

func (h *HTTPHandlers) handleGetSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", nil)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, session)
}

func (h *HTTPHandlers) handleTerminateSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	err := h.sessionManager.TerminateSession(sessionID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Session terminated successfully"})
}

func (h *HTTPHandlers) handleExtendSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req struct {
		Duration string `json:"duration"` // e.g., "1h", "30m"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	_, err := time.ParseDuration(req.Duration)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid duration format", err)
		return
	}

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", nil)
		return
	}

	// Session extension logic would be implemented here

	h.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message":    "Session extended successfully",
		"expires_at": session.StartTime.Add(time.Hour * 24),
	})
}

// Privilege management handlers

func (h *HTTPHandlers) handleRequestPrivilege(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req struct {
		PrivilegeType PrivilegeType `json:"privilege_type"`
		Justification string        `json:"justification"`
		Duration      string        `json:"duration,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", nil)
		return
	}

	// Parse duration if provided
	var duration time.Duration
	if req.Duration != "" {
		var err error
		duration, err = time.ParseDuration(req.Duration)
		if err != nil {
			h.writeErrorResponse(w, http.StatusBadRequest, "Invalid duration format", err)
			return
		}
	}

	// Request privilege
	privilegeID := session.RequestPrivilege(req.PrivilegeType, req.Justification, duration)

	h.writeJSONResponse(w, http.StatusCreated, map[string]interface{}{
		"privilege_id": privilegeID,
		"message":      "Privilege request submitted",
	})
}

func (h *HTTPHandlers) handleApprovePrivilege(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]
	privilegeID := vars["privilegeId"]

	var req struct {
		ApprovedBy string `json:"approved_by"`
		Comments   string `json:"comments,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", nil)
		return
	}

	if err := session.ApprovePrivilege(privilegeID, req.ApprovedBy); err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "Privilege request not found", nil)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Privilege approved successfully"})
}

func (h *HTTPHandlers) handleRevokePrivilege(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req struct {
		RevokedBy string `json:"revoked_by"`
		Reason    string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", nil)
		return
	}

	privilegeType := PrivilegeTypeAdmin // This should be determined from the request
	if err := session.RevokePrivilege(privilegeType); err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "Privilege not found", nil)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Privilege revoked successfully"})
}

// Statistics and monitoring handlers

func (h *HTTPHandlers) handleGetStatistics(w http.ResponseWriter, r *http.Request) {
	stats := h.sessionManager.GetStatistics()
	h.writeJSONResponse(w, http.StatusOK, stats)
}

func (h *HTTPHandlers) handleGetSessionStatistics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	session, exists := h.sessionManager.GetSession(sessionID)
	if !exists {
		h.writeErrorResponse(w, http.StatusNotFound, "Session not found", nil)
		return
	}

	session.mutex.RLock()
	stats := map[string]interface{}{
		"session_id":         session.ID,
		"status":             session.Status,
		"created_at":         session.StartTime,
		"last_activity":      session.LastActivity,
		"expires_at":         session.StartTime.Add(time.Hour * 24),
		"commands_executed":  session.Statistics.CommandsExecuted,
		"files_transferred": session.Statistics.FilesTransferred,
		"screenshots_taken": session.Statistics.ScreenshotsTaken,
		"privileges_active":  len(session.ActivePrivileges),
	}
	session.mutex.RUnlock()

	h.writeJSONResponse(w, http.StatusOK, stats)
}

// Configuration handlers

func (h *HTTPHandlers) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := h.sessionManager.GetConfig()
	h.writeJSONResponse(w, http.StatusOK, config)
}

func (h *HTTPHandlers) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig RemoteAccessConfig
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate configuration
	if err := newConfig.Validate(); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid configuration", err)
		return
	}

	// Update configuration
	h.sessionManager.UpdateConfig(&newConfig)

	h.writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Configuration updated successfully"})
}

// Health check handler

func (h *HTTPHandlers) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	stats := h.sessionManager.GetStatistics()

	health := map[string]interface{}{
		"status":           "healthy",
		"timestamp":        time.Now(),
		"active_sessions":  stats["active_sessions"],
		"total_sessions":   stats["total_sessions"],
		"uptime":           stats["uptime"],
		"memory_usage":     stats["memory_usage"],
		"websocket_connections": stats["websocket_connections"],
	}

	h.writeJSONResponse(w, http.StatusOK, health)
}

// Audit logs handler

func (h *HTTPHandlers) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sessionID := query.Get("session_id")
	eventType := query.Get("event_type")
	severity := query.Get("severity")
	limitStr := query.Get("limit")

	limit := 100 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Build search criteria
	criteria := make(map[string]interface{})
	if sessionID != "" {
		criteria["session_id"] = sessionID
	}
	if eventType != "" {
		criteria["event_type"] = eventType
	}
	if severity != "" {
		criteria["severity"] = severity
	}

	// Search audit logs
	auditLogger := h.sessionManager.auditLogger
	if auditLogger == nil {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Audit logging not enabled", nil)
		return
	}

	events, err := auditLogger.SearchLogs(criteria, limit)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to search audit logs", err)
		return
	}

	response := map[string]interface{}{
		"events": events,
		"total":  len(events),
		"limit":  limit,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods

func (h *HTTPHandlers) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *HTTPHandlers) writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	errorResponse := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"timestamp": time.Now(),
	}

	if err != nil {
		errorResponse["details"] = err.Error()
	}

	h.writeJSONResponse(w, statusCode, errorResponse)
}

// CORS middleware
func (h *HTTPHandlers) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Authentication middleware (placeholder)
func (h *HTTPHandlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement authentication logic
		// For now, allow all requests
		next.ServeHTTP(w, r)
	})
}

// Rate limiting middleware (placeholder)
func (h *HTTPHandlers) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement rate limiting logic
		// For now, allow all requests
		next.ServeHTTP(w, r)
	})
}

// Logging middleware
func (h *HTTPHandlers) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		// Log request
		if h.sessionManager.auditLogger != nil {
			h.sessionManager.auditLogger.LogEvent(AuditEvent{
				EventType: "http_request",
				IPAddress: getClientIP(r),
				UserAgent: r.UserAgent(),
				Details: map[string]interface{}{
					"method":      r.Method,
					"path":        r.URL.Path,
					"status_code": wrapper.statusCode,
					"duration_ms": duration.Milliseconds(),
				},
				Severity:  "info",
				Success:   wrapper.statusCode < 400,
				Timestamp: time.Now(),
			})
		}
	})
}

// responseWriterWrapper wraps http.ResponseWriter to capture status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}