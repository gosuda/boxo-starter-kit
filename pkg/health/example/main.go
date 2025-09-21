package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gosuda/boxo-starter-kit/pkg/health"
	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
)

func main() {
	fmt.Println("🏥 Starting Boxo Health Check System Demo")

	// Create a health manager
	healthManager := health.NewManager(health.DefaultConfig())

	// Create network component for demonstration
	host, err := network.New(nil)
	if err != nil {
		log.Fatalf("Failed to create network host: %v", err)
	}
	defer host.Close()

	fmt.Printf("📡 Network host created with ID: %s\n", host.ID())

	// Register various health checks
	registerHealthChecks(healthManager, host)

	// Start automatic health checking
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("🔄 Starting automatic health checks...")
	go healthManager.Start(ctx)

	// Start health check HTTP server
	go func() {
		fmt.Println("🏥 Starting health check server on port 8081...")
		if err := health.StartHealthServer(8081, healthManager); err != nil {
			log.Printf("Health server error: %v", err)
		}
	}()

	// Start metrics server for comparison
	go func() {
		fmt.Println("📊 Starting metrics server on port 8080...")
		if err := metrics.StartMetricsServer(8080); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Wait for servers to start
	time.Sleep(2 * time.Second)

	// Simulate some network operations to generate data
	simulateOperations(host)

	// Demonstrate health checking functionality
	demonstrateHealthChecks(healthManager)

	// Test HTTP endpoints
	testHTTPEndpoints()

	fmt.Println("\n🎉 Health check demo completed!")
	fmt.Println("💡 Available endpoints:")
	fmt.Println("   Health: http://localhost:8081/health")
	fmt.Println("   Metrics: http://localhost:8080/metrics")
	fmt.Println("\n⏹️  Press Ctrl+C to stop servers")

	// Keep servers running
	select {}
}

func registerHealthChecks(manager *health.Manager, host *network.HostWrapper) {
	fmt.Println("📋 Registering health checks...")

	// 1. Network connectivity check
	connectivityCheck := health.NetworkConnectivityCheck(5 * time.Second)
	manager.Register(connectivityCheck)
	fmt.Println("  ✅ Network connectivity check registered")

	// 2. Metrics-based health check for network component
	networkMetricsCheck := health.MetricsBasedHealthCheck("network", health.DefaultMetricsHealthConfig())
	manager.Register(networkMetricsCheck)
	fmt.Println("  ✅ Network metrics health check registered")

	// 3. Custom component connectivity check
	componentCheck := health.ComponentConnectivityCheck("libp2p-host", func(ctx context.Context) error {
		// Test if host is responsive
		if host.ID().String() == "" {
			return fmt.Errorf("host ID is empty")
		}

		// Test if host can listen
		addrs := host.Addrs()
		if len(addrs) == 0 {
			return fmt.Errorf("no listening addresses")
		}

		return nil
	})
	manager.Register(componentCheck)
	fmt.Println("  ✅ LibP2P host connectivity check registered")

	// 4. Custom business logic check
	businessLogicCheck := health.CustomFunctionCheck("business-logic", func() (bool, string, map[string]string) {
		// Simulate business logic validation
		currentTime := time.Now()
		metadata := map[string]string{
			"check_time": currentTime.Format(time.RFC3339),
			"version":    "1.0.0",
		}

		// Simulate occasional issues
		if currentTime.Second()%10 == 0 {
			return false, "Business logic validation failed", metadata
		}

		return true, "Business logic is healthy", metadata
	})
	manager.Register(businessLogicCheck)
	fmt.Println("  ✅ Business logic check registered")

	// 5. Memory usage check (simulated)
	memoryCheck := health.MemoryUsageCheck()
	manager.Register(memoryCheck)
	fmt.Println("  ✅ Memory usage check registered")

	// 6. Disk space check (simulated)
	diskCheck := health.DiskSpaceCheck("/tmp", health.DiskSpaceThresholds{
		UnhealthyPercent: 95.0,
		DegradedPercent:  85.0,
	})
	manager.Register(diskCheck)
	fmt.Println("  ✅ Disk space check registered")

	// 7. Wrapped check with timeout
	slowCheck := health.NewHealthCheckFunc("slow-service", func(ctx context.Context) health.CheckResult {
		// Simulate a potentially slow operation
		time.Sleep(100 * time.Millisecond)
		return health.CheckResult{
			Status:  health.StatusHealthy,
			Message: "Slow service is responsive",
		}
	})
	timeoutWrappedCheck := health.TimeoutCheck(slowCheck, 2*time.Second)
	manager.Register(timeoutWrappedCheck)
	fmt.Println("  ✅ Timeout-wrapped check registered")

	// 8. Retry-wrapped check
	flakyCheck := health.NewHealthCheckFunc("flaky-service", func(ctx context.Context) health.CheckResult {
		// Simulate flaky service (fails ~30% of the time)
		if time.Now().UnixNano()%10 < 3 {
			return health.CheckResult{
				Status:  health.StatusUnhealthy,
				Message: "Flaky service is temporarily unavailable",
			}
		}
		return health.CheckResult{
			Status:  health.StatusHealthy,
			Message: "Flaky service is working",
		}
	})
	retryWrappedCheck := health.RetryCheck(flakyCheck, 2, 100*time.Millisecond)
	manager.Register(retryWrappedCheck)
	fmt.Println("  ✅ Retry-wrapped check registered")

	fmt.Printf("📋 Total health checks registered: %d\n", 8)
}

func simulateOperations(host *network.HostWrapper) {
	fmt.Println("\n🔄 Simulating network operations to generate metrics...")

	ctx := context.Background()

	// Simulate successful operations
	for i := 0; i < 5; i++ {
		payload := fmt.Sprintf("test-message-%d", i)
		_, err := host.Send(ctx, host.ID(), []byte(payload))
		if err != nil {
			fmt.Printf("❌ Operation %d failed: %v\n", i, err)
		} else {
			fmt.Printf("✅ Operation %d completed\n", i)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Simulate some failures
	for i := 0; i < 2; i++ {
		_, err := host.Send(ctx, "", []byte("empty-peer-id"))
		if err != nil {
			fmt.Printf("❌ Expected failure %d: %v\n", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println("🔄 Operations simulation completed")
}

func demonstrateHealthChecks(manager *health.Manager) {
	fmt.Println("\n🏥 Demonstrating health check functionality...")

	ctx := context.Background()

	// Run all health checks
	fmt.Println("📊 Running all health checks...")
	results := manager.CheckAll(ctx)

	fmt.Printf("🔍 Health check results (%d components):\n", len(results))
	for name, result := range results {
		statusEmoji := getStatusEmoji(result.Status)
		fmt.Printf("  %s %s: %s (%v)\n", statusEmoji, name, result.Message, result.Duration)

		if len(result.Metadata) > 0 {
			fmt.Printf("    Metadata: %v\n", result.Metadata)
		}
	}

	// Get overall system status
	fmt.Println("\n🌍 Overall system health:")
	overallStatus := manager.GetOverallStatus()
	statusEmoji := getStatusEmoji(overallStatus)
	fmt.Printf("  %s Status: %s\n", statusEmoji, overallStatus)

	// Get detailed summary
	summary := manager.GetSystemSummary()
	fmt.Printf("  📊 Summary:\n")
	fmt.Printf("    Total: %d, Healthy: %d, Degraded: %d, Unhealthy: %d, Unknown: %d\n",
		summary.TotalComponents, summary.HealthyCount, summary.DegradedCount,
		summary.UnhealthyCount, summary.UnknownCount)

	// Test individual component check
	fmt.Println("\n🔍 Testing individual component check...")
	result, err := manager.CheckOne(ctx, "network-connectivity")
	if err != nil {
		fmt.Printf("❌ Failed to check network connectivity: %v\n", err)
	} else {
		statusEmoji := getStatusEmoji(result.Status)
		fmt.Printf("  %s network-connectivity: %s\n", statusEmoji, result.Message)
	}
}

func testHTTPEndpoints() {
	fmt.Println("\n🌐 Testing health check HTTP endpoints...")

	endpoints := []struct {
		url         string
		description string
	}{
		{"http://localhost:8081/health", "Overall health status"},
		{"http://localhost:8081/health/summary", "Detailed health summary"},
		{"http://localhost:8081/health/components", "All component details"},
		{"http://localhost:8081/health/components?name=network-connectivity", "Specific component"},
		{"http://localhost:8081/health/live", "Liveness probe"},
		{"http://localhost:8081/health/ready", "Readiness probe"},
	}

	for _, endpoint := range endpoints {
		resp, err := http.Get(endpoint.url)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", endpoint.description, err)
			continue
		}

		statusEmoji := "✅"
		if resp.StatusCode >= 400 {
			statusEmoji = "❌"
		} else if resp.StatusCode >= 300 {
			statusEmoji = "⚠️"
		}

		fmt.Printf("  %s %s: HTTP %d\n", statusEmoji, endpoint.description, resp.StatusCode)
		resp.Body.Close()
	}
}

func getStatusEmoji(status health.Status) string {
	switch status {
	case health.StatusHealthy:
		return "✅"
	case health.StatusDegraded:
		return "⚠️"
	case health.StatusUnhealthy:
		return "❌"
	case health.StatusUnknown:
		return "❓"
	default:
		return "❓"
	}
}

// Example of integrating health checks with existing application
func createSampleApplication() {
	fmt.Println("\n🏗️ Example: Integrating health checks with existing application")

	// Create custom health manager for this application
	appHealthManager := health.NewManager(health.Config{
		CheckInterval:      10 * time.Second, // Check every 10 seconds
		Timeout:            3 * time.Second,   // 3 second timeout
		UnhealthyThreshold: 2,                 // 2 consecutive failures = unhealthy
		EnableAutoCheck:    true,
	})

	// Register application-specific health checks
	dbCheck := health.CustomFunctionCheck("database", func() (bool, string, map[string]string) {
		// Simulate database connectivity check
		// In real application, this would be actual DB ping
		return true, "Database connection is healthy", map[string]string{
			"driver":      "postgres",
			"connections": "5/20",
		}
	})
	appHealthManager.Register(dbCheck)

	cacheCheck := health.CustomFunctionCheck("redis", func() (bool, string, map[string]string) {
		// Simulate Redis connectivity check
		return true, "Redis cache is responsive", map[string]string{
			"memory_usage": "45%",
			"keyspace":     "db0:keys=1234",
		}
	})
	appHealthManager.Register(cacheCheck)

	// Create HTTP server with health middleware
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Application is running"))
	})

	// Add health check middleware
	handler := health.HealthCheckMiddleware(appHealthManager)(mux)

	fmt.Println("🌐 Sample application would be available with health headers")
	fmt.Println("   Application: http://localhost:8082/")
	fmt.Println("   Headers: X-Health-Status, X-Health-Last-Updated")

	// Note: Server is not actually started in this demo
	_ = handler
}