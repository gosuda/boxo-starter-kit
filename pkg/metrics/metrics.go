package metrics

import (
	"sync"
	"time"
)

// ComponentMetrics tracks performance metrics for a component
type ComponentMetrics struct {
	mu                 sync.RWMutex
	ComponentName      string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalLatency       time.Duration
	AverageLatency     time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	BytesProcessed     int64
	ErrorsByType       map[string]int64
	LastResetTime      time.Time
}

// NewComponentMetrics creates a new metrics tracker
func NewComponentMetrics(componentName string) *ComponentMetrics {
	return &ComponentMetrics{
		ComponentName: componentName,
		ErrorsByType:  make(map[string]int64),
		LastResetTime: time.Now(),
		MinLatency:    time.Duration(1<<63 - 1), // Max duration
	}
}

// RecordRequest increments the total request counter
func (m *ComponentMetrics) RecordRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRequests++
}

// RecordSuccess records a successful operation with its duration and bytes processed
func (m *ComponentMetrics) RecordSuccess(duration time.Duration, bytesProcessed int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SuccessfulRequests++
	m.BytesProcessed += bytesProcessed
	m.recordLatency(duration)
}

// RecordFailure records a failed operation with its duration and error type
func (m *ComponentMetrics) RecordFailure(duration time.Duration, errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FailedRequests++
	m.recordLatency(duration)

	if errorType != "" {
		m.ErrorsByType[errorType]++
	}
}

// recordLatency updates latency statistics (must be called with lock held)
func (m *ComponentMetrics) recordLatency(duration time.Duration) {
	m.TotalLatency += duration

	if duration < m.MinLatency {
		m.MinLatency = duration
	}
	if duration > m.MaxLatency {
		m.MaxLatency = duration
	}

	if m.TotalRequests > 0 {
		m.AverageLatency = m.TotalLatency / time.Duration(m.TotalRequests)
	}
}

// GetSnapshot returns a snapshot of current metrics
func (m *ComponentMetrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Deep copy error map
	errorsCopy := make(map[string]int64)
	for k, v := range m.ErrorsByType {
		errorsCopy[k] = v
	}

	return MetricsSnapshot{
		ComponentName:      m.ComponentName,
		TotalRequests:      m.TotalRequests,
		SuccessfulRequests: m.SuccessfulRequests,
		FailedRequests:     m.FailedRequests,
		SuccessRate:        m.calculateSuccessRate(),
		AverageLatency:     m.AverageLatency,
		MinLatency:         m.MinLatency,
		MaxLatency:         m.MaxLatency,
		BytesProcessed:     m.BytesProcessed,
		ErrorsByType:       errorsCopy,
		UptimeSince:        m.LastResetTime,
	}
}

// calculateSuccessRate computes success rate (must be called with lock held)
func (m *ComponentMetrics) calculateSuccessRate() float64 {
	if m.TotalRequests == 0 {
		return 0.0
	}
	return float64(m.SuccessfulRequests) / float64(m.TotalRequests) * 100.0
}

// Reset clears all metrics
func (m *ComponentMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests = 0
	m.SuccessfulRequests = 0
	m.FailedRequests = 0
	m.TotalLatency = 0
	m.AverageLatency = 0
	m.MinLatency = time.Duration(1<<63 - 1)
	m.MaxLatency = 0
	m.BytesProcessed = 0
	m.ErrorsByType = make(map[string]int64)
	m.LastResetTime = time.Now()
}

// MetricsSnapshot represents a point-in-time view of metrics
type MetricsSnapshot struct {
	ComponentName      string           `json:"component_name"`
	TotalRequests      int64            `json:"total_requests"`
	SuccessfulRequests int64            `json:"successful_requests"`
	FailedRequests     int64            `json:"failed_requests"`
	SuccessRate        float64          `json:"success_rate_percent"`
	AverageLatency     time.Duration    `json:"average_latency"`
	MinLatency         time.Duration    `json:"min_latency"`
	MaxLatency         time.Duration    `json:"max_latency"`
	BytesProcessed     int64            `json:"bytes_processed"`
	ErrorsByType       map[string]int64 `json:"errors_by_type"`
	UptimeSince        time.Time        `json:"uptime_since"`
}

// MetricsCollector aggregates metrics from multiple components
type MetricsCollector struct {
	mu         sync.RWMutex
	components map[string]*ComponentMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		components: make(map[string]*ComponentMetrics),
	}
}

// RegisterComponent adds a component to the collector
func (c *MetricsCollector) RegisterComponent(component *ComponentMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.components[component.ComponentName] = component
}

// UnregisterComponent removes a component from the collector
func (c *MetricsCollector) UnregisterComponent(componentName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.components, componentName)
}

// GetComponent returns a component's metrics
func (c *MetricsCollector) GetComponent(componentName string) *ComponentMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.components[componentName]
}

// GetAllSnapshots returns snapshots of all registered components
func (c *MetricsCollector) GetAllSnapshots() map[string]MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshots := make(map[string]MetricsSnapshot)
	for name, component := range c.components {
		snapshots[name] = component.GetSnapshot()
	}
	return snapshots
}

// GetAggregatedSnapshot returns combined metrics across all components
func (c *MetricsCollector) GetAggregatedSnapshot() AggregatedSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agg := AggregatedSnapshot{
		TotalComponents: len(c.components),
		ComponentStats:  make(map[string]ComponentStats),
	}

	for name, component := range c.components {
		snapshot := component.GetSnapshot()
		agg.TotalRequests += snapshot.TotalRequests
		agg.TotalSuccesses += snapshot.SuccessfulRequests
		agg.TotalFailures += snapshot.FailedRequests
		agg.TotalBytesProcessed += snapshot.BytesProcessed

		agg.ComponentStats[name] = ComponentStats{
			SuccessRate:    snapshot.SuccessRate,
			AverageLatency: snapshot.AverageLatency,
			RequestCount:   snapshot.TotalRequests,
		}
	}

	if agg.TotalRequests > 0 {
		agg.OverallSuccessRate = float64(agg.TotalSuccesses) / float64(agg.TotalRequests) * 100.0
	}

	return agg
}

// AggregatedSnapshot represents system-wide metrics
type AggregatedSnapshot struct {
	TotalComponents     int                       `json:"total_components"`
	TotalRequests       int64                     `json:"total_requests"`
	TotalSuccesses      int64                     `json:"total_successes"`
	TotalFailures       int64                     `json:"total_failures"`
	OverallSuccessRate  float64                   `json:"overall_success_rate_percent"`
	TotalBytesProcessed int64                     `json:"total_bytes_processed"`
	ComponentStats      map[string]ComponentStats `json:"component_stats"`
}

// ComponentStats represents summarized stats for a component
type ComponentStats struct {
	SuccessRate    float64       `json:"success_rate_percent"`
	AverageLatency time.Duration `json:"average_latency"`
	RequestCount   int64         `json:"request_count"`
}

// Global metrics collector instance
var globalCollector = NewMetricsCollector()

// RegisterGlobalComponent registers a component with the global collector
func RegisterGlobalComponent(component *ComponentMetrics) {
	globalCollector.RegisterComponent(component)
}

// GetGlobalSnapshot returns a snapshot of all global metrics
func GetGlobalSnapshot() map[string]MetricsSnapshot {
	return globalCollector.GetAllSnapshots()
}

// GetGlobalAggregatedSnapshot returns aggregated global metrics
func GetGlobalAggregatedSnapshot() AggregatedSnapshot {
	return globalCollector.GetAggregatedSnapshot()
}
