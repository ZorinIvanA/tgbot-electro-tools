package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ZorinIvanA/tgbot-electro-tools/internal/metrics"
	"github.com/ZorinIvanA/tgbot-electro-tools/internal/storage"
)

// Server represents HTTP API server
type Server struct {
	storage          storage.Storage
	metricsCollector *metrics.Collector
	adminToken       string
	port             string
}

// NewServer creates a new HTTP API server
func NewServer(storage storage.Storage, metricsCollector *metrics.Collector, adminToken, port string) *Server {
	return &Server{
		storage:          storage,
		metricsCollector: metricsCollector,
		adminToken:       adminToken,
		port:             port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
	mux.HandleFunc("/api/v1/settings", s.handleSettings)
	mux.HandleFunc("/health", s.handleHealth)

	addr := ":" + s.port
	log.Printf("Starting HTTP API server on %s", addr)

	return http.ListenAndServe(addr, mux)
}

// handleMetrics returns Prometheus-format metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metricsText, err := s.metricsCollector.Export()
	if err != nil {
		log.Printf("Error exporting metrics: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(metricsText))
}

// handleSettings handles GET and PUT requests for settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	if !s.authenticate(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetSettings(w, r)
	case http.MethodPut:
		s.handleUpdateSettings(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetSettings returns current settings
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.storage.GetSettings()
	if err != nil {
		log.Printf("Error getting settings: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"trigger_message_count": settings.TriggerMessageCount,
		"site_url":              settings.SiteURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleUpdateSettings updates settings
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TriggerMessageCount int    `json:"trigger_message_count"`
		SiteURL             string `json:"site_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Bad request: invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate input
	if request.TriggerMessageCount < 1 {
		http.Error(w, "Bad request: trigger_message_count must be at least 1", http.StatusBadRequest)
		return
	}

	if request.SiteURL == "" {
		http.Error(w, "Bad request: site_url is required", http.StatusBadRequest)
		return
	}

	// Update settings
	settings := &storage.Settings{
		ID:                  1, // Always ID 1 (single row)
		TriggerMessageCount: request.TriggerMessageCount,
		SiteURL:             request.SiteURL,
	}

	if err := s.storage.UpdateSettings(settings); err != nil {
		log.Printf("Error updating settings: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"trigger_message_count": settings.TriggerMessageCount,
		"site_url":              settings.SiteURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleHealth returns health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]string{
		"status": "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// authenticate checks if request has valid admin token
func (s *Server) authenticate(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}

	return parts[1] == s.adminToken
}

// GetSettingsResponse represents settings response
type GetSettingsResponse struct {
	TriggerMessageCount int    `json:"trigger_message_count"`
	SiteURL             string `json:"site_url"`
}

// UpdateSettingsRequest represents settings update request
type UpdateSettingsRequest struct {
	TriggerMessageCount int    `json:"trigger_message_count"`
	SiteURL             string `json:"site_url"`
}

// ValidateUpdateSettingsRequest validates settings update request
func ValidateUpdateSettingsRequest(req *UpdateSettingsRequest) error {
	if req.TriggerMessageCount < 1 {
		return fmt.Errorf("trigger_message_count must be at least 1")
	}
	if req.SiteURL == "" {
		return fmt.Errorf("site_url is required")
	}
	return nil
}
