package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComponentMetrics_Basic(t *testing.T) {
	metrics := NewComponentMetrics("test-component")
	require.NotNil(t, metrics)

	// Test initial state
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, "test-component", snapshot.ComponentName)
	assert.Equal(t, int64(0), snapshot.TotalRequests)
	assert.Equal(t, int64(0), snapshot.SuccessfulRequests)
	assert.Equal(t, int64(0), snapshot.FailedRequests)
	assert.Equal(t, 0.0, snapshot.SuccessRate)
}

func TestComponentMetrics_RequestTracking(t *testing.T) {
	metrics := NewComponentMetrics("test")

	// Record some requests
	metrics.RecordRequest()
	metrics.RecordRequest()
	metrics.RecordRequest()

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(3), snapshot.TotalRequests)
}

func TestComponentMetrics_SuccessTracking(t *testing.T) {
	metrics := NewComponentMetrics("test")

	// Record successful operations
	metrics.RecordRequest()
	metrics.RecordSuccess(100*time.Millisecond, 1024)

	metrics.RecordRequest()
	metrics.RecordSuccess(200*time.Millisecond, 2048)

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(2), snapshot.TotalRequests)
	assert.Equal(t, int64(2), snapshot.SuccessfulRequests)
	assert.Equal(t, int64(0), snapshot.FailedRequests)
	assert.Equal(t, 100.0, snapshot.SuccessRate)
	assert.Equal(t, int64(3072), snapshot.BytesProcessed)          // 1024 + 2048
	assert.Equal(t, 150*time.Millisecond, snapshot.AverageLatency) // (100+200)/2
}

func TestComponentMetrics_FailureTracking(t *testing.T) {
	metrics := NewComponentMetrics("test")

	// Record failures
	metrics.RecordRequest()
	metrics.RecordFailure(50*time.Millisecond, "network_error")

	metrics.RecordRequest()
	metrics.RecordFailure(75*time.Millisecond, "timeout_error")

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(2), snapshot.TotalRequests)
	assert.Equal(t, int64(0), snapshot.SuccessfulRequests)
	assert.Equal(t, int64(2), snapshot.FailedRequests)
	assert.Equal(t, 0.0, snapshot.SuccessRate)
	assert.Equal(t, int64(1), snapshot.ErrorsByType["network_error"])
	assert.Equal(t, int64(1), snapshot.ErrorsByType["timeout_error"])
}

func TestComponentMetrics_MixedOperations(t *testing.T) {
	metrics := NewComponentMetrics("test")

	// Mix of success and failure
	metrics.RecordRequest()
	metrics.RecordSuccess(100*time.Millisecond, 500)

	metrics.RecordRequest()
	metrics.RecordFailure(50*time.Millisecond, "error")

	metrics.RecordRequest()
	metrics.RecordSuccess(150*time.Millisecond, 1000)

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(3), snapshot.TotalRequests)
	assert.Equal(t, int64(2), snapshot.SuccessfulRequests)
	assert.Equal(t, int64(1), snapshot.FailedRequests)
	assert.InDelta(t, 66.67, snapshot.SuccessRate, 0.01) // 2/3 * 100
	assert.Equal(t, int64(1500), snapshot.BytesProcessed)
	assert.Equal(t, 100*time.Millisecond, snapshot.AverageLatency) // (100+50+150)/3
}

func TestComponentMetrics_Reset(t *testing.T) {
	metrics := NewComponentMetrics("test")

	// Add some data
	metrics.RecordRequest()
	metrics.RecordSuccess(100*time.Millisecond, 1024)

	// Verify data exists
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(1), snapshot.TotalRequests)

	// Reset and verify clean state
	metrics.Reset()
	snapshot = metrics.GetSnapshot()
	assert.Equal(t, int64(0), snapshot.TotalRequests)
	assert.Equal(t, int64(0), snapshot.SuccessfulRequests)
	assert.Equal(t, int64(0), snapshot.FailedRequests)
	assert.Equal(t, int64(0), snapshot.BytesProcessed)
	assert.Equal(t, time.Duration(0), snapshot.AverageLatency)
	assert.Len(t, snapshot.ErrorsByType, 0)
}

func TestMetricsCollector_Basic(t *testing.T) {
	collector := NewMetricsCollector()
	require.NotNil(t, collector)

	// Register components
	comp1 := NewComponentMetrics("component1")
	comp2 := NewComponentMetrics("component2")

	collector.RegisterComponent(comp1)
	collector.RegisterComponent(comp2)

	// Add some data
	comp1.RecordRequest()
	comp1.RecordSuccess(100*time.Millisecond, 1024)

	comp2.RecordRequest()
	comp2.RecordFailure(50*time.Millisecond, "error")

	// Get all snapshots
	snapshots := collector.GetAllSnapshots()
	assert.Len(t, snapshots, 2)
	assert.Contains(t, snapshots, "component1")
	assert.Contains(t, snapshots, "component2")

	// Check aggregated snapshot
	agg := collector.GetAggregatedSnapshot()
	assert.Equal(t, 2, agg.TotalComponents)
	assert.Equal(t, int64(2), agg.TotalRequests)
	assert.Equal(t, int64(1), agg.TotalSuccesses)
	assert.Equal(t, int64(1), agg.TotalFailures)
	assert.Equal(t, 50.0, agg.OverallSuccessRate)
	assert.Equal(t, int64(1024), agg.TotalBytesProcessed)
}

func TestMetricsCollector_Unregister(t *testing.T) {
	collector := NewMetricsCollector()

	comp := NewComponentMetrics("test")
	collector.RegisterComponent(comp)

	snapshots := collector.GetAllSnapshots()
	assert.Len(t, snapshots, 1)

	collector.UnregisterComponent("test")
	snapshots = collector.GetAllSnapshots()
	assert.Len(t, snapshots, 0)
}

func TestGlobalMetrics(t *testing.T) {
	// Test global metrics functions
	comp := NewComponentMetrics("global-test")
	RegisterGlobalComponent(comp)

	comp.RecordRequest()
	comp.RecordSuccess(100*time.Millisecond, 1024)

	snapshots := GetGlobalSnapshot()
	assert.Contains(t, snapshots, "global-test")

	agg := GetGlobalAggregatedSnapshot()
	assert.GreaterOrEqual(t, agg.TotalComponents, 1)
	assert.GreaterOrEqual(t, agg.TotalRequests, int64(1))
}

func TestLatencyTracking(t *testing.T) {
	metrics := NewComponentMetrics("latency-test")

	// Record operations with different latencies
	latencies := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
	}

	for _, latency := range latencies {
		metrics.RecordRequest()
		metrics.RecordSuccess(latency, 100)
	}

	snapshot := metrics.GetSnapshot()
	assert.Equal(t, 50*time.Millisecond, snapshot.MinLatency)
	assert.Equal(t, 200*time.Millisecond, snapshot.MaxLatency)
	assert.Equal(t, 125*time.Millisecond, snapshot.AverageLatency) // (50+100+200+150)/4
}

// Benchmark tests
func BenchmarkMetrics_RecordSuccess(b *testing.B) {
	metrics := NewComponentMetrics("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordSuccess(100*time.Millisecond, 1024)
	}
}

func BenchmarkMetrics_RecordFailure(b *testing.B) {
	metrics := NewComponentMetrics("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordFailure(100*time.Millisecond, "test_error")
	}
}

func BenchmarkMetrics_GetSnapshot(b *testing.B) {
	metrics := NewComponentMetrics("benchmark")

	// Add some data
	for i := 0; i < 1000; i++ {
		metrics.RecordRequest()
		if i%2 == 0 {
			metrics.RecordSuccess(100*time.Millisecond, 1024)
		} else {
			metrics.RecordFailure(50*time.Millisecond, "error")
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.GetSnapshot()
	}
}
