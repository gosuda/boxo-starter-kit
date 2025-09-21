package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
	StatusUnknown   Status = "unknown"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	ComponentName string            `json:"component_name"`
	Status        Status            `json:"status"`
	Message       string            `json:"message"`
	LastChecked   time.Time         `json:"last_checked"`
	Duration      time.Duration     `json:"duration"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HealthChecker defines the interface for health checks
type HealthChecker interface {
	// Check performs the health check and returns the result
	Check(ctx context.Context) CheckResult

	// Name returns the name of this health checker
	Name() string
}

// HealthCheckFunc is a function type that implements HealthChecker
type HealthCheckFunc struct {
	name string
	fn   func(ctx context.Context) CheckResult
}

func (h HealthCheckFunc) Check(ctx context.Context) CheckResult {
	return h.fn(ctx)
}

func (h HealthCheckFunc) Name() string {
	return h.name
}

// NewHealthCheckFunc creates a new HealthChecker from a function
func NewHealthCheckFunc(name string, fn func(ctx context.Context) CheckResult) HealthChecker {
	return HealthCheckFunc{name: name, fn: fn}
}

// Manager manages multiple health checkers
type Manager struct {
	mu       sync.RWMutex
	checkers map[string]HealthChecker
	results  map[string]CheckResult
	config   Config
}

// Config holds configuration for the health manager
type Config struct {
	CheckInterval    time.Duration // How often to run checks
	Timeout          time.Duration // Timeout for individual checks
	UnhealthyThreshold int         // Number of consecutive failures to mark as unhealthy
	EnableAutoCheck  bool          // Whether to run checks automatically
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		CheckInterval:      30 * time.Second,
		Timeout:            5 * time.Second,
		UnhealthyThreshold: 3,
		EnableAutoCheck:    true,
	}
}

// NewManager creates a new health check manager
func NewManager(config Config) *Manager {
	if config.CheckInterval == 0 {
		config = DefaultConfig()
	}

	return &Manager{
		checkers: make(map[string]HealthChecker),
		results:  make(map[string]CheckResult),
		config:   config,
	}
}

// Register adds a health checker
func (m *Manager) Register(checker HealthChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := checker.Name()
	m.checkers[name] = checker

	// Initialize with unknown status
	m.results[name] = CheckResult{
		ComponentName: name,
		Status:        StatusUnknown,
		Message:       "Not yet checked",
		LastChecked:   time.Time{},
	}
}

// Unregister removes a health checker
func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checkers, name)
	delete(m.results, name)
}

// CheckAll runs all registered health checks
func (m *Manager) CheckAll(ctx context.Context) map[string]CheckResult {
	m.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range m.checkers {
		checkers[name] = checker
	}
	m.mu.RUnlock()

	results := make(map[string]CheckResult)
	var wg sync.WaitGroup

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
			defer cancel()

			result := m.runSingleCheck(checkCtx, checker)

			m.mu.Lock()
			m.results[name] = result
			m.mu.Unlock()

			results[name] = result
		}(name, checker)
	}

	wg.Wait()
	return results
}

// CheckOne runs a specific health check
func (m *Manager) CheckOne(ctx context.Context, name string) (CheckResult, error) {
	m.mu.RLock()
	checker, exists := m.checkers[name]
	m.mu.RUnlock()

	if !exists {
		return CheckResult{}, fmt.Errorf("health checker '%s' not found", name)
	}

	checkCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)
	defer cancel()

	result := m.runSingleCheck(checkCtx, checker)

	m.mu.Lock()
	m.results[name] = result
	m.mu.Unlock()

	return result, nil
}

// runSingleCheck executes a single health check with error handling
func (m *Manager) runSingleCheck(ctx context.Context, checker HealthChecker) CheckResult {
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			// Handle panics in health checks
			fmt.Printf("Health check panic for %s: %v\n", checker.Name(), r)
		}
	}()

	// Run the check
	result := checker.Check(ctx)

	// Ensure required fields are set
	if result.ComponentName == "" {
		result.ComponentName = checker.Name()
	}
	result.LastChecked = start
	result.Duration = time.Since(start)

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Health check timed out after %v", m.config.Timeout)
	}

	return result
}

// GetResults returns the current health check results
func (m *Manager) GetResults() map[string]CheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]CheckResult)
	for name, result := range m.results {
		results[name] = result
	}
	return results
}

// GetResult returns the result for a specific component
func (m *Manager) GetResult(name string) (CheckResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.results[name]
	return result, exists
}

// GetOverallStatus returns the overall system health status
func (m *Manager) GetOverallStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.results) == 0 {
		return StatusUnknown
	}

	hasUnhealthy := false
	hasDegraded := false
	hasUnknown := false

	for _, result := range m.results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		case StatusUnknown:
			hasUnknown = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	if hasUnknown {
		return StatusUnknown
	}

	return StatusHealthy
}

// Start begins automatic health checking
func (m *Manager) Start(ctx context.Context) {
	if !m.config.EnableAutoCheck {
		return
	}

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	// Run initial check
	m.CheckAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CheckAll(ctx)
		}
	}
}

// SystemSummary provides a high-level view of system health
type SystemSummary struct {
	OverallStatus    Status                    `json:"overall_status"`
	TotalComponents  int                       `json:"total_components"`
	HealthyCount     int                       `json:"healthy_count"`
	DegradedCount    int                       `json:"degraded_count"`
	UnhealthyCount   int                       `json:"unhealthy_count"`
	UnknownCount     int                       `json:"unknown_count"`
	LastUpdated      time.Time                 `json:"last_updated"`
	ComponentDetails map[string]CheckResult    `json:"component_details"`
}

// GetSystemSummary returns a comprehensive health summary
func (m *Manager) GetSystemSummary() SystemSummary {
	results := m.GetResults()

	summary := SystemSummary{
		OverallStatus:    m.GetOverallStatus(),
		TotalComponents:  len(results),
		LastUpdated:      time.Now(),
		ComponentDetails: results,
	}

	for _, result := range results {
		switch result.Status {
		case StatusHealthy:
			summary.HealthyCount++
		case StatusDegraded:
			summary.DegradedCount++
		case StatusUnhealthy:
			summary.UnhealthyCount++
		case StatusUnknown:
			summary.UnknownCount++
		}
	}

	return summary
}

// Global health manager instance
var globalManager = NewManager(DefaultConfig())

// RegisterGlobal registers a health checker with the global manager
func RegisterGlobal(checker HealthChecker) {
	globalManager.Register(checker)
}

// CheckGlobal runs all global health checks
func CheckGlobal(ctx context.Context) map[string]CheckResult {
	return globalManager.CheckAll(ctx)
}

// GetGlobalSummary returns the global health summary
func GetGlobalSummary() SystemSummary {
	return globalManager.GetSystemSummary()
}

// StartGlobalHealthChecks starts the global health checking
func StartGlobalHealthChecks(ctx context.Context) {
	go globalManager.Start(ctx)
}