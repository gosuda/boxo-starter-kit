package main

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	bitswap "github.com/gosuda/boxo-starter-kit/04-bitswap/pkg"
	ipni "github.com/gosuda/boxo-starter-kit/17-ipni/pkg"

	. "github.com/gosuda/boxo-starter-kit/18-multifetcher/pkg"
)

func TestMultiFetcher_Configuration(t *testing.T) {
	// Test default configuration
	config := DefaultConfig()
	assert.Equal(t, 3, config.MaxConcurrent)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 150*time.Millisecond, config.StaggerDelay)
	assert.True(t, config.CancelOnFirstWin)
}

func TestMultiFetcher_Creation(t *testing.T) {
	ctx := context.Background()

	// Create dependencies
	host, err := network.New(nil)
	require.NoError(t, err)
	defer host.Close()

	store, err := persistent.New(persistent.Memory, "")
	require.NoError(t, err)
	defer store.Close()

	bs, err := bitswap.NewBitswap(ctx, host, store)
	require.NoError(t, err)
	defer bs.Close()

	ipniWrapper, err := ipni.NewIPNIWrapper("")
	require.NoError(t, err)
	defer ipniWrapper.Close()

	// Create multifetcher
	mf := NewMultiFetcher(ipniWrapper, nil, bs, nil)
	require.NotNil(t, mf)
	defer mf.Close()

	// Test metrics initialization
	metrics := mf.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, int64(0), metrics.SuccessfulRequests)
	assert.Equal(t, int64(0), metrics.FailedRequests)
	assert.NotNil(t, metrics.ProtocolStats["bitswap"])
	assert.NotNil(t, metrics.ProtocolStats["graphsync"])
	assert.NotNil(t, metrics.ProtocolStats["http"])
}

func TestMultiFetcher_MetricsTracking(t *testing.T) {
	ctx := context.Background()

	// Create minimal multifetcher for testing
	ipniWrapper, err := ipni.NewIPNIWrapper("")
	require.NoError(t, err)
	defer ipniWrapper.Close()

	mf := NewMultiFetcher(ipniWrapper, nil, nil, nil)
	require.NotNil(t, mf)
	defer mf.Close()

	// Test request tracking
	mf.recordRequest()
	metrics := mf.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalRequests)

	// Test failure tracking
	mf.recordFailure()
	metrics = mf.GetMetrics()
	assert.Equal(t, int64(1), metrics.FailedRequests)

	// Test result tracking
	result := &FetchResult{
		Protocol: "bitswap",
		Duration: 100 * time.Millisecond,
		Data:     []byte("test data"),
		Error:    nil,
	}
	mf.recordResult(result)

	metrics = mf.GetMetrics()
	assert.Equal(t, int64(1), metrics.SuccessfulRequests)

	bitswapStats := metrics.ProtocolStats["bitswap"]
	assert.Equal(t, int64(1), bitswapStats.Attempts)
	assert.Equal(t, int64(1), bitswapStats.Successes)
	assert.Equal(t, int64(0), bitswapStats.Failures)
	assert.Equal(t, int64(9), bitswapStats.BytesTransferred) // len("test data")
	assert.Equal(t, 100*time.Millisecond, bitswapStats.AvgLatency)
}

func TestMultiFetcher_HTTPFetcher(t *testing.T) {
	fetcher := NewHTTPFetcher()
	require.NotNil(t, fetcher)
	defer fetcher.Close()

	// Test invalid URL handling
	ctx := context.Background()
	c, err := cid.Parse("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	require.NoError(t, err)

	_, err = fetcher.Fetch(ctx, "invalid-url", c, false)
	assert.Error(t, err)
}

func TestMultiFetcher_ConfigValidation(t *testing.T) {
	ipniWrapper, err := ipni.NewIPNIWrapper("")
	require.NoError(t, err)
	defer ipniWrapper.Close()

	// Test with custom config
	customConfig := FetcherConfig{
		MaxConcurrent:    5,
		Timeout:          60 * time.Second,
		StaggerDelay:     200 * time.Millisecond,
		CancelOnFirstWin: false,
	}

	mf := NewMultiFetcher(ipniWrapper, nil, nil, &customConfig)
	require.NotNil(t, mf)
	defer mf.Close()

	assert.Equal(t, 5, mf.config.MaxConcurrent)
	assert.Equal(t, 60*time.Second, mf.config.Timeout)
	assert.Equal(t, 200*time.Millisecond, mf.config.StaggerDelay)
	assert.False(t, mf.config.CancelOnFirstWin)
}

func TestFetchResult_Validation(t *testing.T) {
	c, err := cid.Parse("QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	require.NoError(t, err)

	result := &FetchResult{
		Protocol: "bitswap",
		Provider: "test-provider",
		Data:     []byte("test"),
		Error:    nil,
		Duration: 50 * time.Millisecond,
		CID:      c,
	}

	assert.Equal(t, "bitswap", result.Protocol)
	assert.Equal(t, "test-provider", result.Provider)
	assert.Equal(t, []byte("test"), result.Data)
	assert.NoError(t, result.Error)
	assert.Equal(t, 50*time.Millisecond, result.Duration)
	assert.Equal(t, c, result.CID)
}

// Integration test placeholder - requires actual network setup
func TestMultiFetcher_Integration(t *testing.T) {
	t.Skip("Integration test requires network setup")

	// This test would require:
	// 1. Two connected libp2p hosts
	// 2. One host providing content via bitswap
	// 3. IPNI setup with provider information
	// 4. Actual fetch operations
}

// Benchmark tests for performance measurement
func BenchmarkMultiFetcher_MetricsTracking(b *testing.B) {
	ipniWrapper, err := ipni.NewIPNIWrapper("")
	require.NoError(b, err)
	defer ipniWrapper.Close()

	mf := NewMultiFetcher(ipniWrapper, nil, nil, nil)
	defer mf.Close()

	result := &FetchResult{
		Protocol: "bitswap",
		Duration: 100 * time.Millisecond,
		Data:     make([]byte, 1024), // 1KB data
		Error:    nil,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mf.recordResult(result)
	}
}