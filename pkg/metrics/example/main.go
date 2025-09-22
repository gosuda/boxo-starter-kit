package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

func main() {
	fmt.Println("🚀 Starting Boxo Metrics System Demo")

	// Create network component to demonstrate metrics
	host, err := network.New(nil)
	if err != nil {
		log.Fatalf("Failed to create network host: %v", err)
	}
	defer host.Close()

	fmt.Printf("📡 Network host created with ID: %s\n", host.ID())

	// Start metrics server in background
	go func() {
		fmt.Println("📊 Starting metrics server on port 8080...")
		if err := metrics.StartMetricsServer(8080); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	// Simulate some network operations to generate metrics
	fmt.Println("🔄 Simulating network operations...")

	ctx := context.Background()

	// Simulate sending some data (will generate metrics)
	for i := 0; i < 10; i++ {
		payload := fmt.Sprintf("Hello from operation %d", i)

		// This will record metrics for the send operation
		_, err := host.Send(ctx, host.ID(), []byte(payload))
		if err != nil {
			fmt.Printf("❌ Send operation %d failed: %v\n", i, err)
		} else {
			fmt.Printf("✅ Send operation %d completed\n", i)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Print current metrics
	fmt.Println("\n📈 Current Network Metrics:")
	networkMetrics := host.GetMetrics()
	fmt.Printf("  Total Requests: %d\n", networkMetrics.TotalRequests)
	fmt.Printf("  Successful: %d\n", networkMetrics.SuccessfulRequests)
	fmt.Printf("  Failed: %d\n", networkMetrics.FailedRequests)
	fmt.Printf("  Success Rate: %.2f%%\n", networkMetrics.SuccessRate)
	fmt.Printf("  Average Latency: %v\n", networkMetrics.AverageLatency)
	fmt.Printf("  Bytes Processed: %d\n", networkMetrics.BytesProcessed)

	if len(networkMetrics.ErrorsByType) > 0 {
		fmt.Println("  Errors by Type:")
		for errorType, count := range networkMetrics.ErrorsByType {
			fmt.Printf("    %s: %d\n", errorType, count)
		}
	}

	// Print global aggregated metrics
	fmt.Println("\n🌍 Global Aggregated Metrics:")
	globalMetrics := metrics.GetGlobalAggregatedSnapshot()
	fmt.Printf("  Total Components: %d\n", globalMetrics.TotalComponents)
	fmt.Printf("  Total Requests: %d\n", globalMetrics.TotalRequests)
	fmt.Printf("  Overall Success Rate: %.2f%%\n", globalMetrics.OverallSuccessRate)
	fmt.Printf("  Total Bytes Processed: %d\n", globalMetrics.TotalBytesProcessed)

	fmt.Println("\n📊 Component Breakdown:")
	for name, stats := range globalMetrics.ComponentStats {
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Success Rate: %.2f%%\n", stats.SuccessRate)
		fmt.Printf("    Average Latency: %v\n", stats.AverageLatency)
		fmt.Printf("    Request Count: %d\n", stats.RequestCount)
	}

	// Demonstrate HTTP endpoints
	fmt.Println("\n🌐 Available HTTP Endpoints:")
	fmt.Println("  📊 All metrics: http://localhost:8080/metrics")
	fmt.Println("  🔧 Components: http://localhost:8080/metrics/components")
	fmt.Println("  📈 Aggregated: http://localhost:8080/metrics/aggregated")
	fmt.Println("  🏥 Health: http://localhost:8080/metrics/health")

	// Test HTTP endpoints
	fmt.Println("\n🧪 Testing HTTP endpoints...")

	endpoints := []string{
		"http://localhost:8080/metrics/health",
		"http://localhost:8080/metrics/aggregated",
		"http://localhost:8080/metrics/components",
	}

	for _, endpoint := range endpoints {
		resp, err := http.Get(endpoint)
		if err != nil {
			fmt.Printf("❌ Failed to access %s: %v\n", endpoint, err)
			continue
		}
		fmt.Printf("✅ %s - Status: %d\n", endpoint, resp.StatusCode)
		resp.Body.Close()
	}

	fmt.Println("\n🎉 Demo completed! Metrics server continues running...")
	fmt.Println("💡 Try accessing the endpoints in your browser:")
	fmt.Println("   curl http://localhost:8080/metrics/health")
	fmt.Println("   curl http://localhost:8080/metrics/aggregated")
	fmt.Println("\n⏹️  Press Ctrl+C to stop the metrics server")

	// Keep the server running
	select {}
}

// Example of creating custom component metrics
func demonstrateCustomMetrics() {
	fmt.Println("\n🔧 Demonstrating Custom Component Metrics:")

	// Create a custom component
	customMetrics := metrics.NewComponentMetrics("custom-service")
	metrics.RegisterGlobalComponent(customMetrics)

	// Simulate operations
	operations := []struct {
		duration time.Duration
		success  bool
		bytes    int64
		error    string
	}{
		{50 * time.Millisecond, true, 1024, ""},
		{30 * time.Millisecond, true, 512, ""},
		{100 * time.Millisecond, false, 0, "timeout"},
		{75 * time.Millisecond, true, 2048, ""},
		{200 * time.Millisecond, false, 0, "network_error"},
	}

	for i, op := range operations {
		customMetrics.RecordRequest()

		if op.success {
			customMetrics.RecordSuccess(op.duration, op.bytes)
			fmt.Printf("  ✅ Operation %d: Success (%v, %d bytes)\n", i+1, op.duration, op.bytes)
		} else {
			customMetrics.RecordFailure(op.duration, op.error)
			fmt.Printf("  ❌ Operation %d: Failed (%v, %s)\n", i+1, op.duration, op.error)
		}
	}

	// Print results
	snapshot := customMetrics.GetSnapshot()
	fmt.Printf("  📊 Results: %d/%d successful (%.1f%%)\n",
		snapshot.SuccessfulRequests, snapshot.TotalRequests, snapshot.SuccessRate)
}

// Example HTTP handler with metrics middleware
func createSampleHTTPServer() {
	mux := http.NewServeMux()

	// Add sample endpoints
	mux.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Simulate slow operation
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Slow response"))
	})

	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error occurred"))
	})

	// Wrap with metrics middleware
	handler := metrics.MetricsMiddleware("sample-api")(mux)

	fmt.Println("🌐 Sample API server would start on :8081 with metrics")
	fmt.Println("   Endpoints: /api/hello, /api/slow, /api/error")

	// Note: Server is not actually started in this demo
	_ = handler
}
