package health

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckFunc(t *testing.T) {
	checker := NewHealthCheckFunc("test-checker", func(ctx context.Context) CheckResult {
		return CheckResult{
			ComponentName: "test-checker",
			Status:        StatusHealthy,
			Message:       "All good",
		}
	})

	assert.Equal(t, "test-checker", checker.Name())

	result := checker.Check(context.Background())
	assert.Equal(t, "test-checker", result.ComponentName)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "All good", result.Message)
}

func TestManager_Register(t *testing.T) {
	manager := NewManager(DefaultConfig())

	checker := NewHealthCheckFunc("test", func(ctx context.Context) CheckResult {
		return CheckResult{
			ComponentName: "test",
			Status:        StatusHealthy,
			Message:       "OK",
		}
	})

	manager.Register(checker)

	// Check initial unknown status
	result, exists := manager.GetResult("test")
	require.True(t, exists)
	assert.Equal(t, StatusUnknown, result.Status)
	assert.Equal(t, "Not yet checked", result.Message)
}

func TestManager_CheckOne(t *testing.T) {
	manager := NewManager(DefaultConfig())

	checker := NewHealthCheckFunc("test", func(ctx context.Context) CheckResult {
		return CheckResult{
			ComponentName: "test",
			Status:        StatusHealthy,
			Message:       "All systems operational",
		}
	})

	manager.Register(checker)

	result, err := manager.CheckOne(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, "test", result.ComponentName)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "All systems operational", result.Message)
	assert.False(t, result.LastChecked.IsZero())
}

func TestManager_CheckAll(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// Register multiple checkers
	checker1 := NewHealthCheckFunc("service1", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusHealthy, Message: "Service 1 OK"}
	})

	checker2 := NewHealthCheckFunc("service2", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDegraded, Message: "Service 2 degraded"}
	})

	manager.Register(checker1)
	manager.Register(checker2)

	results := manager.CheckAll(context.Background())
	assert.Len(t, results, 2)

	assert.Contains(t, results, "service1")
	assert.Contains(t, results, "service2")

	assert.Equal(t, StatusHealthy, results["service1"].Status)
	assert.Equal(t, StatusDegraded, results["service2"].Status)
}

func TestManager_GetOverallStatus(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// No components - should be unknown
	assert.Equal(t, StatusUnknown, manager.GetOverallStatus())

	// Add healthy component
	healthyChecker := NewHealthCheckFunc("healthy", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusHealthy, Message: "OK"}
	})
	manager.Register(healthyChecker)
	manager.CheckAll(context.Background())
	assert.Equal(t, StatusHealthy, manager.GetOverallStatus())

	// Add degraded component - overall should be degraded
	degradedChecker := NewHealthCheckFunc("degraded", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusDegraded, Message: "Degraded"}
	})
	manager.Register(degradedChecker)
	manager.CheckAll(context.Background())
	assert.Equal(t, StatusDegraded, manager.GetOverallStatus())

	// Add unhealthy component - overall should be unhealthy
	unhealthyChecker := NewHealthCheckFunc("unhealthy", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusUnhealthy, Message: "Failed"}
	})
	manager.Register(unhealthyChecker)
	manager.CheckAll(context.Background())
	assert.Equal(t, StatusUnhealthy, manager.GetOverallStatus())
}

func TestManager_GetSystemSummary(t *testing.T) {
	manager := NewManager(DefaultConfig())

	// Add various components
	checkers := []struct {
		name   string
		status Status
	}{
		{"healthy1", StatusHealthy},
		{"healthy2", StatusHealthy},
		{"degraded1", StatusDegraded},
		{"unhealthy1", StatusUnhealthy},
		{"unknown1", StatusUnknown},
	}

	for _, c := range checkers {
		checker := NewHealthCheckFunc(c.name, func(status Status) func(ctx context.Context) CheckResult {
			return func(ctx context.Context) CheckResult {
				return CheckResult{Status: status, Message: "Test"}
			}
		}(c.status))
		manager.Register(checker)
	}

	manager.CheckAll(context.Background())

	summary := manager.GetSystemSummary()
	assert.Equal(t, StatusUnhealthy, summary.OverallStatus) // Due to unhealthy component
	assert.Equal(t, 5, summary.TotalComponents)
	assert.Equal(t, 2, summary.HealthyCount)
	assert.Equal(t, 1, summary.DegradedCount)
	assert.Equal(t, 1, summary.UnhealthyCount)
	assert.Equal(t, 1, summary.UnknownCount)
	assert.Len(t, summary.ComponentDetails, 5)
}

func TestManager_Timeout(t *testing.T) {
	config := DefaultConfig()
	config.Timeout = 100 * time.Millisecond
	manager := NewManager(config)

	// Create a slow checker
	slowChecker := NewHealthCheckFunc("slow", func(ctx context.Context) CheckResult {
		time.Sleep(200 * time.Millisecond) // Longer than timeout
		return CheckResult{Status: StatusHealthy, Message: "Should not reach here"}
	})

	manager.Register(slowChecker)

	result, err := manager.CheckOne(context.Background(), "slow")
	require.NoError(t, err)

	// Should timeout and be marked unhealthy
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "timed out")
}

func TestManager_UnregisterComponent(t *testing.T) {
	manager := NewManager(DefaultConfig())

	checker := NewHealthCheckFunc("test", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusHealthy, Message: "OK"}
	})

	manager.Register(checker)
	_, exists := manager.GetResult("test")
	assert.True(t, exists)

	manager.Unregister("test")
	_, exists = manager.GetResult("test")
	assert.False(t, exists)
}

func TestCustomFunctionCheck(t *testing.T) {
	// Test healthy check
	healthyCheck := CustomFunctionCheck("custom-healthy", func() (bool, string, map[string]string) {
		return true, "Everything is fine", map[string]string{"version": "1.0"}
	})

	result := healthyCheck.Check(context.Background())
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "Everything is fine", result.Message)
	assert.Equal(t, "1.0", result.Metadata["version"])

	// Test unhealthy check
	unhealthyCheck := CustomFunctionCheck("custom-unhealthy", func() (bool, string, map[string]string) {
		return false, "Something went wrong", map[string]string{"error": "connection_failed"}
	})

	result = unhealthyCheck.Check(context.Background())
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Equal(t, "Something went wrong", result.Message)
	assert.Equal(t, "connection_failed", result.Metadata["error"])
}

func TestTimeoutCheck(t *testing.T) {
	// Create a slow checker
	slowChecker := NewHealthCheckFunc("slow", func(ctx context.Context) CheckResult {
		time.Sleep(200 * time.Millisecond)
		return CheckResult{Status: StatusHealthy, Message: "Completed"}
	})

	// Wrap with timeout
	timeoutChecker := TimeoutCheck(slowChecker, 50*time.Millisecond)

	result := timeoutChecker.Check(context.Background())
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "timed out")
	assert.Equal(t, "timeout", result.Metadata["error"])
}

func TestRetryCheck(t *testing.T) {
	attempts := 0

	// Checker that fails first two times, then succeeds
	flakyChecker := NewHealthCheckFunc("flaky", func(ctx context.Context) CheckResult {
		attempts++
		if attempts < 3 {
			return CheckResult{Status: StatusUnhealthy, Message: "Failed"}
		}
		return CheckResult{Status: StatusHealthy, Message: "Success"}
	})

	retryChecker := RetryCheck(flakyChecker, 3, 10*time.Millisecond)

	result := retryChecker.Check(context.Background())
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "succeeded after 2 retries")
	assert.Equal(t, "2", result.Metadata["retries"])
}

func TestGlobalHealthFunctions(t *testing.T) {
	// Register a global checker
	checker := NewHealthCheckFunc("global-test", func(ctx context.Context) CheckResult {
		return CheckResult{Status: StatusHealthy, Message: "Global OK"}
	})

	RegisterGlobal(checker)

	// Check global health
	results := CheckGlobal(context.Background())
	assert.Contains(t, results, "global-test")
	assert.Equal(t, StatusHealthy, results["global-test"].Status)

	// Get global summary
	summary := GetGlobalSummary()
	assert.GreaterOrEqual(t, summary.TotalComponents, 1)
}

// Benchmark tests
func BenchmarkManager_CheckAll(b *testing.B) {
	manager := NewManager(DefaultConfig())

	// Add multiple checkers
	for i := 0; i < 10; i++ {
		checker := NewHealthCheckFunc(fmt.Sprintf("service%d", i), func(ctx context.Context) CheckResult {
			time.Sleep(1 * time.Millisecond) // Simulate work
			return CheckResult{Status: StatusHealthy, Message: "OK"}
		})
		manager.Register(checker)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CheckAll(context.Background())
	}
}

func BenchmarkManager_GetOverallStatus(b *testing.B) {
	manager := NewManager(DefaultConfig())

	// Add checkers and run initial check
	for i := 0; i < 100; i++ {
		checker := NewHealthCheckFunc(fmt.Sprintf("service%d", i), func(ctx context.Context) CheckResult {
			return CheckResult{Status: StatusHealthy, Message: "OK"}
		})
		manager.Register(checker)
	}
	manager.CheckAll(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetOverallStatus()
	}
}
