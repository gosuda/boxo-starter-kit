package benchmarks

import (
	"time"
)

// BenchmarkConfig contains configuration for all benchmarks
type BenchmarkConfig struct {
	// Data sizes for testing (in bytes)
	SmallBlockSize  int
	MediumBlockSize int
	LargeBlockSize  int

	// Number of operations for throughput tests
	SmallOpCount  int
	MediumOpCount int
	LargeOpCount  int

	// Concurrency levels
	LowConcurrency    int
	MediumConcurrency int
	HighConcurrency   int

	// Timeouts and durations
	OperationTimeout time.Duration
	BenchmarkTimeout time.Duration

	// Network settings
	NetworkLatency time.Duration
	MaxPeers       int

	// Memory limits (in MB)
	MemoryLimit int
}

// DefaultConfig returns the default benchmark configuration
func DefaultConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		// Block sizes
		SmallBlockSize:  1024,        // 1KB
		MediumBlockSize: 1024 * 1024, // 1MB
		LargeBlockSize:  10 * 1024 * 1024, // 10MB

		// Operation counts
		SmallOpCount:  1000,
		MediumOpCount: 100,
		LargeOpCount:  10,

		// Concurrency
		LowConcurrency:    10,
		MediumConcurrency: 100,
		HighConcurrency:   1000,

		// Timeouts
		OperationTimeout: 30 * time.Second,
		BenchmarkTimeout: 5 * time.Minute,

		// Network
		NetworkLatency: 10 * time.Millisecond,
		MaxPeers:       50,

		// Memory
		MemoryLimit: 512, // 512MB
	}
}

// TestData generates test data of the specified size
func (c *BenchmarkConfig) TestData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

// SmallTestData returns small test data
func (c *BenchmarkConfig) SmallTestData() []byte {
	return c.TestData(c.SmallBlockSize)
}

// MediumTestData returns medium test data
func (c *BenchmarkConfig) MediumTestData() []byte {
	return c.TestData(c.MediumBlockSize)
}

// LargeTestData returns large test data
func (c *BenchmarkConfig) LargeTestData() []byte {
	return c.TestData(c.LargeBlockSize)
}