package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPHandler provides HTTP endpoints for health checks
type HTTPHandler struct {
	manager *Manager
}

// NewHTTPHandler creates a new HTTP handler for health checks
func NewHTTPHandler(manager *Manager) *HTTPHandler {
	if manager == nil {
		manager = globalManager
	}
	return &HTTPHandler{manager: manager}
}

// ServeHTTP implements http.Handler
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.URL.Path {
	case "/health":
		h.handleOverallHealth(w, r)
	case "/health/summary":
		h.handleSummary(w, r)
	case "/health/components":
		h.handleComponents(w, r)
	case "/health/check":
		h.handleManualCheck(w, r)
	case "/health/live":
		h.handleLiveness(w, r)
	case "/health/ready":
		h.handleReadiness(w, r)
	default:
		h.handleIndex(w, r)
	}
}

// handleOverallHealth returns the overall system health status
func (h *HTTPHandler) handleOverallHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary := h.manager.GetSystemSummary()

	// Set HTTP status code based on health
	statusCode := http.StatusOK
	switch summary.OverallStatus {
	case StatusDegraded:
		statusCode = http.StatusPartialContent
	case StatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	case StatusUnknown:
		statusCode = http.StatusPreconditionFailed
	}

	response := map[string]interface{}{
		"status":           summary.OverallStatus,
		"timestamp":        time.Now().UTC(),
		"total_components": summary.TotalComponents,
		"healthy":          summary.HealthyCount,
		"degraded":         summary.DegradedCount,
		"unhealthy":        summary.UnhealthyCount,
		"unknown":          summary.UnknownCount,
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleSummary returns detailed health summary
func (h *HTTPHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary := h.manager.GetSystemSummary()

	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"summary":   summary,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleComponents returns individual component health
func (h *HTTPHandler) handleComponents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	componentName := r.URL.Query().Get("name")

	if componentName != "" {
		// Return specific component
		result, exists := h.manager.GetResult(componentName)
		if !exists {
			http.Error(w, fmt.Sprintf("Component '%s' not found", componentName), http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"timestamp": time.Now().UTC(),
			"component": result,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Return all components
	results := h.manager.GetResults()
	response := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"components": results,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleManualCheck triggers manual health checks
func (h *HTTPHandler) handleManualCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	componentName := r.URL.Query().Get("name")
	ctx := r.Context()

	if componentName != "" {
		// Check specific component
		result, err := h.manager.CheckOne(ctx, componentName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"timestamp": time.Now().UTC(),
			"result":    result,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Check all components
	results := h.manager.CheckAll(ctx)
	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"results":   results,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleLiveness implements Kubernetes-style liveness probe
func (h *HTTPHandler) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Liveness check - basic "is the service running"
	overallStatus := h.manager.GetOverallStatus()

	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
		"health":    overallStatus,
	}

	// Liveness should only fail if the service is completely down
	// We return 200 even for degraded services
	statusCode := http.StatusOK
	if overallStatus == StatusUnknown {
		statusCode = http.StatusServiceUnavailable
		response["status"] = "unknown"
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleReadiness implements Kubernetes-style readiness probe
func (h *HTTPHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Readiness check - "is the service ready to handle traffic"
	overallStatus := h.manager.GetOverallStatus()

	response := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"health":    overallStatus,
	}

	// Readiness is more strict - degraded services should not receive traffic
	statusCode := http.StatusOK
	switch overallStatus {
	case StatusHealthy:
		response["status"] = "ready"
	case StatusDegraded:
		statusCode = http.StatusServiceUnavailable
		response["status"] = "not_ready"
		response["reason"] = "service_degraded"
	case StatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
		response["status"] = "not_ready"
		response["reason"] = "service_unhealthy"
	case StatusUnknown:
		statusCode = http.StatusServiceUnavailable
		response["status"] = "not_ready"
		response["reason"] = "health_unknown"
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleIndex returns API documentation
func (h *HTTPHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"title":     "Boxo Health Check API",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC(),
		"endpoints": map[string]string{
			"GET /health":            "Overall system health status",
			"GET /health/summary":    "Detailed health summary with all components",
			"GET /health/components": "Individual component health (use ?name=component_name for specific)",
			"POST /health/check":     "Trigger manual health checks (use ?name=component_name for specific)",
			"GET /health/live":       "Kubernetes-style liveness probe",
			"GET /health/ready":      "Kubernetes-style readiness probe",
		},
		"examples": map[string]string{
			"overall_health":   "/health",
			"component_health": "/health/components?name=network",
			"manual_check":     "POST /health/check",
			"liveness_probe":   "/health/live",
			"readiness_probe":  "/health/ready",
		},
		"status_codes": map[string]string{
			"200": "Healthy",
			"206": "Degraded (partial content)",
			"412": "Unknown health status",
			"503": "Unhealthy/Not ready",
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// StartHealthServer starts an HTTP server for health checks on the specified port
func StartHealthServer(port int, manager *Manager) error {
	if manager == nil {
		manager = globalManager
	}

	handler := NewHTTPHandler(manager)
	addr := fmt.Sprintf(":%d", port)

	fmt.Printf("Starting health check server on http://localhost%s\n", addr)
	fmt.Printf("Available endpoints:\n")
	fmt.Printf("  - http://localhost%s/health (overall status)\n", addr)
	fmt.Printf("  - http://localhost%s/health/summary (detailed summary)\n", addr)
	fmt.Printf("  - http://localhost%s/health/components (component details)\n", addr)
	fmt.Printf("  - http://localhost%s/health/live (liveness probe)\n", addr)
	fmt.Printf("  - http://localhost%s/health/ready (readiness probe)\n", addr)

	return http.ListenAndServe(addr, handler)
}

// HealthCheckMiddleware is middleware that adds health status headers to HTTP responses
func HealthCheckMiddleware(manager *Manager) func(http.Handler) http.Handler {
	if manager == nil {
		manager = globalManager
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add health status as response header
			overallStatus := manager.GetOverallStatus()
			w.Header().Set("X-Health-Status", string(overallStatus))

			// Add last check timestamp
			summary := manager.GetSystemSummary()
			w.Header().Set("X-Health-Last-Updated", summary.LastUpdated.Format(time.RFC3339))

			next.ServeHTTP(w, r)
		})
	}
}
