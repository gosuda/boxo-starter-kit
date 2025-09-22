package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// NetworkConnectivityCheck checks if the host can establish network connections
func NetworkConnectivityCheck(timeout time.Duration) HealthChecker {
	return NewHealthCheckFunc("network-connectivity", func(ctx context.Context) CheckResult {
		result := CheckResult{
			ComponentName: "network-connectivity",
			Status:        StatusHealthy,
			Message:       "Network connectivity is working",
			Metadata:      make(map[string]string),
		}

		// Test DNS resolution
		_, err := net.LookupHost("google.com")
		if err != nil {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("DNS resolution failed: %v", err)
			result.Metadata["error"] = "dns_resolution_failed"
			return result
		}

		// Test HTTP connectivity
		client := &http.Client{Timeout: timeout}
		resp, err := client.Get("https://httpbin.org/status/200")
		if err != nil {
			result.Status = StatusDegraded
			result.Message = fmt.Sprintf("HTTP connectivity degraded: %v", err)
			result.Metadata["error"] = "http_connectivity_failed"
			return result
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			result.Status = StatusDegraded
			result.Message = fmt.Sprintf("HTTP test returned status %d", resp.StatusCode)
			result.Metadata["http_status"] = fmt.Sprintf("%d", resp.StatusCode)
		}

		result.Metadata["dns_status"] = "ok"
		result.Metadata["http_status"] = "ok"
		return result
	})
}

// MetricsBasedHealthCheck creates a health check based on component metrics
func MetricsBasedHealthCheck(componentName string, config MetricsHealthConfig) HealthChecker {
	return NewHealthCheckFunc(fmt.Sprintf("metrics-%s", componentName), func(ctx context.Context) CheckResult {
		result := CheckResult{
			ComponentName: fmt.Sprintf("metrics-%s", componentName),
			Status:        StatusHealthy,
			Message:       "Component metrics are healthy",
			Metadata:      make(map[string]string),
		}

		// Get global metrics
		snapshots := metrics.GetGlobalSnapshot()
		snapshot, exists := snapshots[componentName]
		if !exists {
			result.Status = StatusUnknown
			result.Message = fmt.Sprintf("No metrics found for component '%s'", componentName)
			result.Metadata["error"] = "no_metrics_found"
			return result
		}

		// Check success rate
		if snapshot.TotalRequests > config.MinRequests {
			if snapshot.SuccessRate < config.UnhealthySuccessRate {
				result.Status = StatusUnhealthy
				result.Message = fmt.Sprintf("Success rate %.2f%% is below threshold %.2f%%",
					snapshot.SuccessRate, config.UnhealthySuccessRate)
				result.Metadata["success_rate"] = fmt.Sprintf("%.2f", snapshot.SuccessRate)
				result.Metadata["threshold"] = fmt.Sprintf("%.2f", config.UnhealthySuccessRate)
				return result
			} else if snapshot.SuccessRate < config.DegradedSuccessRate {
				result.Status = StatusDegraded
				result.Message = fmt.Sprintf("Success rate %.2f%% is below optimal threshold %.2f%%",
					snapshot.SuccessRate, config.DegradedSuccessRate)
				result.Metadata["success_rate"] = fmt.Sprintf("%.2f", snapshot.SuccessRate)
			}
		}

		// Check average latency
		if snapshot.AverageLatency > config.UnhealthyLatency {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("Average latency %v exceeds unhealthy threshold %v",
				snapshot.AverageLatency, config.UnhealthyLatency)
			result.Metadata["avg_latency"] = snapshot.AverageLatency.String()
			result.Metadata["latency_threshold"] = config.UnhealthyLatency.String()
			return result
		} else if snapshot.AverageLatency > config.DegradedLatency {
			if result.Status == StatusHealthy {
				result.Status = StatusDegraded
				result.Message = fmt.Sprintf("Average latency %v exceeds optimal threshold %v",
					snapshot.AverageLatency, config.DegradedLatency)
			}
			result.Metadata["avg_latency"] = snapshot.AverageLatency.String()
		}

		// Add metadata
		result.Metadata["total_requests"] = fmt.Sprintf("%d", snapshot.TotalRequests)
		result.Metadata["success_rate"] = fmt.Sprintf("%.2f", snapshot.SuccessRate)
		result.Metadata["avg_latency"] = snapshot.AverageLatency.String()
		result.Metadata["bytes_processed"] = fmt.Sprintf("%d", snapshot.BytesProcessed)

		return result
	})
}

// MetricsHealthConfig configures metrics-based health checks
type MetricsHealthConfig struct {
	MinRequests          int64         // Minimum requests before checking success rate
	UnhealthySuccessRate float64       // Success rate below this is unhealthy
	DegradedSuccessRate  float64       // Success rate below this is degraded
	UnhealthyLatency     time.Duration // Latency above this is unhealthy
	DegradedLatency      time.Duration // Latency above this is degraded
}

// DefaultMetricsHealthConfig returns sensible defaults
func DefaultMetricsHealthConfig() MetricsHealthConfig {
	return MetricsHealthConfig{
		MinRequests:          10,
		UnhealthySuccessRate: 50.0,
		DegradedSuccessRate:  90.0,
		UnhealthyLatency:     5 * time.Second,
		DegradedLatency:      1 * time.Second,
	}
}

// DiskSpaceCheck checks available disk space
func DiskSpaceCheck(path string, thresholds DiskSpaceThresholds) HealthChecker {
	return NewHealthCheckFunc("disk-space", func(ctx context.Context) CheckResult {
		result := CheckResult{
			ComponentName: "disk-space",
			Status:        StatusHealthy,
			Message:       "Disk space is adequate",
			Metadata:      make(map[string]string),
		}

		// Note: This is a simplified implementation
		// In production, you'd use syscall.Statfs or similar

		// For demonstration, we'll simulate disk space check
		// In real implementation, you would check actual disk usage

		result.Metadata["path"] = path
		result.Metadata["check_type"] = "simulated"
		result.Message = "Disk space check is simulated (not implemented for cross-platform compatibility)"

		return result
	})
}

// DiskSpaceThresholds defines disk space warning levels
type DiskSpaceThresholds struct {
	UnhealthyPercent float64 // Percentage used above which is unhealthy
	DegradedPercent  float64 // Percentage used above which is degraded
}

// MemoryUsageCheck checks memory usage patterns
func MemoryUsageCheck() HealthChecker {
	return NewHealthCheckFunc("memory-usage", func(ctx context.Context) CheckResult {
		result := CheckResult{
			ComponentName: "memory-usage",
			Status:        StatusHealthy,
			Message:       "Memory usage is normal",
			Metadata:      make(map[string]string),
		}

		// Note: This is a simplified implementation
		// In production, you'd use runtime.MemStats and system-specific calls

		result.Metadata["check_type"] = "simulated"
		result.Message = "Memory usage check is simulated"

		return result
	})
}

// CustomFunctionCheck creates a health check from a custom function
func CustomFunctionCheck(name string, checkFn func() (bool, string, map[string]string)) HealthChecker {
	return NewHealthCheckFunc(name, func(ctx context.Context) CheckResult {
		result := CheckResult{
			ComponentName: name,
			Status:        StatusUnknown,
			Message:       "Check failed to complete",
			Metadata:      make(map[string]string),
		}

		healthy, message, metadata := checkFn()

		if healthy {
			result.Status = StatusHealthy
		} else {
			result.Status = StatusUnhealthy
		}

		result.Message = message
		if metadata != nil {
			result.Metadata = metadata
		}

		return result
	})
}

// ComponentConnectivityCheck checks if a component can establish connections
func ComponentConnectivityCheck(componentName string, testFn func(ctx context.Context) error) HealthChecker {
	return NewHealthCheckFunc(fmt.Sprintf("%s-connectivity", componentName), func(ctx context.Context) CheckResult {
		result := CheckResult{
			ComponentName: fmt.Sprintf("%s-connectivity", componentName),
			Status:        StatusHealthy,
			Message:       fmt.Sprintf("%s connectivity is working", componentName),
			Metadata:      make(map[string]string),
		}

		err := testFn(ctx)
		if err != nil {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("%s connectivity failed: %v", componentName, err)
			result.Metadata["error"] = err.Error()
		}

		return result
	})
}

// TimeoutCheck wraps another health check with a timeout
func TimeoutCheck(checker HealthChecker, timeout time.Duration) HealthChecker {
	return NewHealthCheckFunc(fmt.Sprintf("%s-timeout", checker.Name()), func(ctx context.Context) CheckResult {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		done := make(chan CheckResult, 1)
		go func() {
			done <- checker.Check(timeoutCtx)
		}()

		select {
		case result := <-done:
			return result
		case <-timeoutCtx.Done():
			return CheckResult{
				ComponentName: checker.Name(),
				Status:        StatusUnhealthy,
				Message:       fmt.Sprintf("Health check timed out after %v", timeout),
				Metadata: map[string]string{
					"error":   "timeout",
					"timeout": timeout.String(),
				},
			}
		}
	})
}

// RetryCheck wraps another health check with retry logic
func RetryCheck(checker HealthChecker, maxRetries int, retryDelay time.Duration) HealthChecker {
	return NewHealthCheckFunc(fmt.Sprintf("%s-retry", checker.Name()), func(ctx context.Context) CheckResult {
		var lastResult CheckResult

		for attempt := 0; attempt <= maxRetries; attempt++ {
			lastResult = checker.Check(ctx)

			if lastResult.Status == StatusHealthy {
				if attempt > 0 {
					lastResult.Message += fmt.Sprintf(" (succeeded after %d retries)", attempt)
					if lastResult.Metadata == nil {
						lastResult.Metadata = make(map[string]string)
					}
					lastResult.Metadata["retries"] = fmt.Sprintf("%d", attempt)
				}
				return lastResult
			}

			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return lastResult
				case <-time.After(retryDelay):
					// Continue to next retry
				}
			}
		}

		lastResult.Message += fmt.Sprintf(" (failed after %d retries)", maxRetries)
		if lastResult.Metadata == nil {
			lastResult.Metadata = make(map[string]string)
		}
		lastResult.Metadata["retries"] = fmt.Sprintf("%d", maxRetries)
		return lastResult
	})
}
