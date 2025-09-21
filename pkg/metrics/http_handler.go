package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPHandler provides HTTP endpoints for metrics
type HTTPHandler struct {
	collector *MetricsCollector
}

// NewHTTPHandler creates a new HTTP handler for metrics
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		collector: globalCollector,
	}
}

// ServeHTTP implements http.Handler
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	switch r.URL.Path {
	case "/metrics":
		h.handleMetrics(w, r)
	case "/metrics/components":
		h.handleComponents(w, r)
	case "/metrics/aggregated":
		h.handleAggregated(w, r)
	case "/metrics/health":
		h.handleHealth(w, r)
	default:
		h.handleIndex(w, r)
	}
}

// handleMetrics returns all metrics data
func (h *HTTPHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"components": h.collector.GetAllSnapshots(),
		"aggregated": h.collector.GetAggregatedSnapshot(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleComponents returns individual component metrics
func (h *HTTPHandler) handleComponents(w http.ResponseWriter, r *http.Request) {
	componentName := r.URL.Query().Get("name")

	if componentName != "" {
		// Return specific component
		component := h.collector.GetComponent(componentName)
		if component == nil {
			http.Error(w, fmt.Sprintf("Component '%s' not found", componentName), http.StatusNotFound)
			return
		}

		response := map[string]interface{}{
			"timestamp": time.Now().UTC(),
			"component": component.GetSnapshot(),
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// Return all components
	response := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"components": h.collector.GetAllSnapshots(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleAggregated returns aggregated metrics
func (h *HTTPHandler) handleAggregated(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"timestamp":  time.Now().UTC(),
		"aggregated": h.collector.GetAggregatedSnapshot(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleHealth returns system health status
func (h *HTTPHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	agg := h.collector.GetAggregatedSnapshot()

	health := "healthy"
	if agg.TotalComponents == 0 {
		health = "no_components"
	} else if agg.OverallSuccessRate < 50.0 && agg.TotalRequests > 10 {
		health = "degraded"
	} else if agg.OverallSuccessRate < 10.0 && agg.TotalRequests > 10 {
		health = "unhealthy"
	}

	response := map[string]interface{}{
		"timestamp":           time.Now().UTC(),
		"status":              health,
		"total_components":    agg.TotalComponents,
		"total_requests":      agg.TotalRequests,
		"overall_success_rate": agg.OverallSuccessRate,
		"total_bytes_processed": agg.TotalBytesProcessed,
	}

	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	if health == "degraded" {
		statusCode = http.StatusPartialContent
	} else if health == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleIndex returns API documentation
func (h *HTTPHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"title":       "Boxo Metrics API",
		"version":     "1.0.0",
		"timestamp":   time.Now().UTC(),
		"endpoints": map[string]string{
			"GET /metrics":            "All metrics data (components + aggregated)",
			"GET /metrics/components": "Individual component metrics (use ?name=component_name for specific component)",
			"GET /metrics/aggregated": "System-wide aggregated metrics",
			"GET /metrics/health":     "System health status",
		},
		"examples": map[string]string{
			"all_metrics":        "/metrics",
			"specific_component": "/metrics/components?name=network",
			"health_check":       "/metrics/health",
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// StartMetricsServer starts an HTTP server for metrics on the specified port
func StartMetricsServer(port int) error {
	handler := NewHTTPHandler()
	addr := fmt.Sprintf(":%d", port)

	fmt.Printf("Starting metrics server on http://localhost%s\n", addr)
	fmt.Printf("Available endpoints:\n")
	fmt.Printf("  - http://localhost%s/metrics\n", addr)
	fmt.Printf("  - http://localhost%s/metrics/components\n", addr)
	fmt.Printf("  - http://localhost%s/metrics/aggregated\n", addr)
	fmt.Printf("  - http://localhost%s/metrics/health\n", addr)

	return http.ListenAndServe(addr, handler)
}

// MetricsMiddleware is an HTTP middleware that tracks request metrics
func MetricsMiddleware(componentName string) func(http.Handler) http.Handler {
	metrics := NewComponentMetrics(componentName + "-http")
	RegisterGlobalComponent(metrics)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			metrics.RecordRequest()

			// Wrap ResponseWriter to capture status code
			wrapper := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapper, r)

			duration := time.Since(start)

			if wrapper.statusCode >= 200 && wrapper.statusCode < 400 {
				metrics.RecordSuccess(duration, int64(wrapper.bytesWritten))
			} else {
				errorType := fmt.Sprintf("http_%d", wrapper.statusCode)
				metrics.RecordFailure(duration, errorType)
			}
		})
	}
}

// responseWrapper wraps http.ResponseWriter to capture response data
type responseWrapper struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (w *responseWrapper) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWrapper) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}