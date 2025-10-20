package ipni

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// MonitoringManager handles metrics collection and health checks
type MonitoringManager struct {
	metrics     *IPNIMetrics
	healthCheck *HealthChecker
	server      *http.Server
	config      *MonitoringConfig
	running     bool
	mutex       sync.RWMutex
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	MetricsPort    int           `json:"metrics_port"`
	HealthPort     int           `json:"health_port"`
	UpdateInterval time.Duration `json:"update_interval"`
	EnableHTTP     bool          `json:"enable_http"`
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		MetricsPort:    9090,
		HealthPort:     8080,
		UpdateInterval: 30 * time.Second,
		EnableHTTP:     true,
	}
}

// IPNIMetrics collects comprehensive IPNI metrics
type IPNIMetrics struct {
	// Index metrics
	TotalProviders   int64 `json:"total_providers"`
	TotalEntries     int64 `json:"total_entries"`
	TotalMultihashes int64 `json:"total_multihashes"`
	IndexSizeBytes   int64 `json:"index_size_bytes"`

	// Query metrics
	QueriesTotal      int64   `json:"queries_total"`
	QueriesSuccessful int64   `json:"queries_successful"`
	QueryLatencyMS    float64 `json:"query_latency_ms"`
	CacheHitRate      float64 `json:"cache_hit_rate"`

	// Network metrics
	PeersConnected   int     `json:"peers_connected"`
	MessagesReceived int64   `json:"messages_received"`
	MessagesSent     int64   `json:"messages_sent"`
	NetworkLatencyMS float64 `json:"network_latency_ms"`

	// Security metrics
	SignaturesVerified int64 `json:"signatures_verified"`
	TrustedProviders   int64 `json:"trusted_providers"`
	SpamBlocked        int64 `json:"spam_blocked"`
	RateLimitHits      int64 `json:"rate_limit_hits"`

	// Advertisement chain metrics
	ChainLength         int   `json:"chain_length"`
	ChainSizeBytes      int64 `json:"chain_size_bytes"`
	AdvertisementsAdded int64 `json:"advertisements_added"`

	// System metrics
	MemoryUsageBytes int64     `json:"memory_usage_bytes"`
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`
	GoroutineCount   int       `json:"goroutine_count"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	LastUpdate       time.Time `json:"last_update"`
}

// HealthChecker performs health checks on IPNI components
type HealthChecker struct {
	checks  map[string]HealthCheck
	results map[string]HealthResult
	config  *HealthConfig
	mutex   sync.RWMutex
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	Interval         time.Duration `json:"interval"`
	Timeout          time.Duration `json:"timeout"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
}

// HealthCheck interface for component health checks
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) HealthResult
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SystemHealth represents overall system health
type SystemHealth struct {
	Overall    HealthStatus            `json:"overall"`
	Components map[string]HealthResult `json:"components"`
	Metrics    *IPNIMetrics            `json:"metrics"`
	Uptime     time.Duration           `json:"uptime"`
	Version    string                  `json:"version"`
	Timestamp  time.Time               `json:"timestamp"`
}

// NewMonitoringManager creates a new monitoring manager
func NewMonitoringManager(config *MonitoringConfig) *MonitoringManager {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	healthConfig := &HealthConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
	}

	return &MonitoringManager{
		metrics: &IPNIMetrics{
			LastUpdate: time.Now(),
		},
		healthCheck: &HealthChecker{
			checks:  make(map[string]HealthCheck),
			results: make(map[string]HealthResult),
			config:  healthConfig,
		},
		config: config,
	}
}

// Start starts the monitoring manager
func (mm *MonitoringManager) Start(ctx context.Context) error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if mm.running {
		return fmt.Errorf("monitoring manager already running")
	}

	mm.running = true

	// Start metrics collection
	go mm.metricsCollectionLoop(ctx)

	// Start health checks
	go mm.healthCheckLoop(ctx)

	// Start HTTP server if enabled
	if mm.config.EnableHTTP {
		go mm.startHTTPServer()
	}

	fmt.Printf("📊 Monitoring started on ports %d (metrics) and %d (health)\n",
		mm.config.MetricsPort, mm.config.HealthPort)
	return nil
}

// Stop stops the monitoring manager
func (mm *MonitoringManager) Stop() error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.running {
		return nil
	}

	mm.running = false

	if mm.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mm.server.Shutdown(ctx)
	}

	fmt.Println("📊 Monitoring stopped")
	return nil
}

// UpdateMetrics updates IPNI metrics
func (mm *MonitoringManager) UpdateMetrics(stats *IndexStats, chainStats *ChainStats, pubsubMetrics *PubSubMetrics) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	// Update index metrics
	if stats != nil {
		mm.metrics.TotalProviders = stats.TotalProviders
		mm.metrics.TotalEntries = stats.TotalEntries
		mm.metrics.TotalMultihashes = stats.TotalMultihashes
		mm.metrics.QueriesTotal = stats.QueryCount
	}

	// Update chain metrics
	if chainStats != nil {
		mm.metrics.ChainLength = chainStats.ChainLength
		mm.metrics.ChainSizeBytes = chainStats.ChainSize
		mm.metrics.AdvertisementsAdded = chainStats.TotalAdvertisements
	}

	// Update pubsub metrics
	if pubsubMetrics != nil {
		mm.metrics.MessagesReceived = pubsubMetrics.MessagesReceived
		mm.metrics.MessagesSent = pubsubMetrics.MessagesSent
		mm.metrics.PeersConnected = pubsubMetrics.SubscriberCount
	}

	// Update system metrics
	mm.updateSystemMetrics()

	mm.metrics.LastUpdate = time.Now()
}

// GetMetrics returns current metrics
func (mm *MonitoringManager) GetMetrics() *IPNIMetrics {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Return a copy
	metrics := *mm.metrics
	return &metrics
}

// GetSystemHealth returns overall system health
func (mm *MonitoringManager) GetSystemHealth() *SystemHealth {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	// Calculate overall health
	overallHealth := HealthHealthy
	components := make(map[string]HealthResult)

	for name, result := range mm.healthCheck.results {
		components[name] = result
		if result.Status == HealthUnhealthy {
			overallHealth = HealthUnhealthy
		} else if result.Status == HealthDegraded && overallHealth == HealthHealthy {
			overallHealth = HealthDegraded
		}
	}

	return &SystemHealth{
		Overall:    overallHealth,
		Components: components,
		Metrics:    mm.GetMetrics(),
		Uptime:     time.Since(time.Now().Add(-time.Duration(mm.metrics.UptimeSeconds) * time.Second)),
		Version:    "demo-v1.0.0",
		Timestamp:  time.Now(),
	}
}

// RegisterHealthCheck registers a health check
func (mm *MonitoringManager) RegisterHealthCheck(check HealthCheck) {
	mm.healthCheck.mutex.Lock()
	defer mm.healthCheck.mutex.Unlock()

	mm.healthCheck.checks[check.Name()] = check
}

// metricsCollectionLoop collects metrics periodically
func (mm *MonitoringManager) metricsCollectionLoop(ctx context.Context) {
	ticker := time.NewTicker(mm.config.UpdateInterval)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mm.mutex.Lock()
			mm.updateSystemMetrics()
			mm.metrics.UptimeSeconds = int64(time.Since(startTime).Seconds())
			mm.metrics.LastUpdate = time.Now()
			mm.mutex.Unlock()
		}
	}
}

// healthCheckLoop performs health checks periodically
func (mm *MonitoringManager) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(mm.healthCheck.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mm.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks runs all registered health checks
func (mm *MonitoringManager) performHealthChecks(ctx context.Context) {
	mm.healthCheck.mutex.Lock()
	defer mm.healthCheck.mutex.Unlock()

	for name, check := range mm.healthCheck.checks {
		checkCtx, cancel := context.WithTimeout(ctx, mm.healthCheck.config.Timeout)

		start := time.Now()
		result := check.Check(checkCtx)
		result.Duration = time.Since(start)
		result.Timestamp = time.Now()

		mm.healthCheck.results[name] = result
		cancel()
	}
}

// updateSystemMetrics updates system-level metrics
func (mm *MonitoringManager) updateSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mm.metrics.MemoryUsageBytes = int64(m.Alloc)
	mm.metrics.GoroutineCount = runtime.NumGoroutine()

	// CPU usage would require additional monitoring in a real implementation
	mm.metrics.CPUUsagePercent = 15.5 // Mock value for demo
}

// startHTTPServer starts the HTTP monitoring server
func (mm *MonitoringManager) startHTTPServer() {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.HandleFunc("/metrics", mm.handleMetrics)

	// Health endpoints
	mux.HandleFunc("/health", mm.handleHealth)
	mux.HandleFunc("/ready", mm.handleReadiness)
	mux.HandleFunc("/live", mm.handleLiveness)

	mm.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", mm.config.HealthPort),
		Handler: mux,
	}

	if err := mm.server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("❌ HTTP server error: %v\n", err)
	}
}

// HTTP handlers

func (mm *MonitoringManager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := mm.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (mm *MonitoringManager) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := mm.GetSystemHealth()

	w.Header().Set("Content-Type", "application/json")

	if health.Overall == HealthUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(health)
}

func (mm *MonitoringManager) handleReadiness(w http.ResponseWriter, r *http.Request) {
	health := mm.GetSystemHealth()

	w.Header().Set("Content-Type", "application/json")

	if health.Overall == HealthUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready"}`))
	} else {
		w.Write([]byte(`{"status":"ready"}`))
	}
}

func (mm *MonitoringManager) handleLiveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"alive"}`))
}

// Built-in health checks

// ProviderHealthCheck checks provider component health
type ProviderHealthCheck struct {
	provider *Provider
}

func NewProviderHealthCheck(provider *Provider) *ProviderHealthCheck {
	return &ProviderHealthCheck{provider: provider}
}

func (c *ProviderHealthCheck) Name() string {
	return "provider"
}

func (c *ProviderHealthCheck) Check(ctx context.Context) HealthResult {
	if c.provider == nil {
		return HealthResult{
			Status:  HealthUnhealthy,
			Message: "provider is nil",
		}
	}

	stats := c.provider.GetStats()

	status := HealthHealthy
	message := "provider operational"

	if stats.TotalEntries == 0 {
		status = HealthDegraded
		message = "no entries in index"
	}

	return HealthResult{
		Status:  status,
		Message: message,
		Metadata: map[string]interface{}{
			"entries":   stats.TotalEntries,
			"providers": stats.TotalProviders,
		},
	}
}

// SecurityHealthCheck checks security component health
type SecurityHealthCheck struct {
	security *Security
}

func NewSecurityHealthCheck(security *Security) *SecurityHealthCheck {
	return &SecurityHealthCheck{security: security}
}

func (c *SecurityHealthCheck) Name() string {
	return "security"
}

func (c *SecurityHealthCheck) Check(ctx context.Context) HealthResult {
	if c.security == nil {
		return HealthResult{
			Status:  HealthUnhealthy,
			Message: "security manager is nil",
		}
	}

	// Test signature functionality
	testData := []byte("health check test")
	signature, err := c.security.SignData(testData)
	if err != nil {
		return HealthResult{
			Status:  HealthUnhealthy,
			Message: fmt.Sprintf("signature test failed: %v", err),
		}
	}

	if !c.security.VerifySignature(testData, signature, c.security.GetPublicKey()) {
		return HealthResult{
			Status:  HealthUnhealthy,
			Message: "signature verification failed",
		}
	}

	return HealthResult{
		Status:  HealthHealthy,
		Message: "security operational",
		Metadata: map[string]interface{}{
			"peer_id": c.security.GetPeerID().String(),
		},
	}
}
