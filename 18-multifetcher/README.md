# 18-multifetcher: Parallel Multi-Protocol Content Fetching

A sophisticated content fetching system that orchestrates parallel retrieval across multiple IPFS protocols (Bitswap, GraphSync, HTTP Gateways) with intelligent provider selection and performance optimization.

## 🎯 Learning Objectives

- Master parallel content fetching strategies in IPFS
- Understand protocol selection and optimization techniques
- Learn to implement resilient content retrieval systems
- Explore performance monitoring and metrics collection
- Understand integration with IPNI (InterPlanetary Network Indexer)

## 📋 Prerequisites

- **Previous Chapters**: 04-bitswap, 15-graphsync, 16-trustless-gateway, 17-ipni (understanding of protocols)
- **Technical Knowledge**: Concurrency patterns, HTTP protocols, performance optimization
- **Go Experience**: Goroutines, channels, context handling, synchronization

## 🔑 Core Concepts

### What is Multi-Protocol Fetching?

Multi-protocol fetching is a strategy that leverages multiple content retrieval protocols simultaneously to:

1. **Maximize Success Rate**: If one protocol fails, others can succeed
2. **Optimize Performance**: Choose the fastest available protocol
3. **Improve Resilience**: Reduce dependency on single points of failure
4. **Enable Smart Routing**: Use network intelligence to select optimal providers

### Key Features

- **Protocol Racing**: Run multiple fetchers concurrently and use the fastest response
- **IPNI Integration**: Use network indexing to find optimal providers
- **Intelligent Staggering**: Start fetchers with delays to reduce resource waste
- **Comprehensive Metrics**: Track performance across all protocols
- **Configurable Behavior**: Customize concurrency, timeouts, and strategies

### Supported Protocols

| Protocol | Best For | Characteristics |
|----------|----------|----------------|
| **Bitswap** | Single blocks | P2P, good for popular content |
| **GraphSync** | DAG structures | Efficient for large datasets |
| **HTTP Gateway** | Web integration | Reliable, works through firewalls |

## 💻 Code Analysis

### Core Structure

```go
type MultiFetcher struct {
    config       FetcherConfig          // Behavior configuration
    ipni         *ipni.IPNIWrapper      // Provider discovery
    graphsync    *graphsync.GraphSyncWrapper
    bitswap      *bitswap.BitswapWrapper
    httpFetcher  *HTTPFetcher
    metrics      *Metrics               // Performance tracking
}
```

### Key Methods

#### 1. Single Block Fetching

```go
func (mf *MultiFetcher) FetchBlock(ctx context.Context, c cid.Cid) (*FetchResult, error)
```
- Uses IPNI to discover optimal providers
- Falls back to direct Bitswap if no providers found
- Returns fastest successful result

#### 2. DAG Fetching

```go
func (mf *MultiFetcher) FetchDAG(ctx context.Context, root cid.Cid, selector ipld.Node) (*FetchResult, error)
```
- Encodes selector to CBOR for IPNI
- Optimized for structured data retrieval
- Supports selective DAG traversal

#### 3. Protocol Racing

```go
func (mf *MultiFetcher) raceProtocols(ctx context.Context, c cid.Cid, fetchers []ipni.RankedFetcher, selector ipld.Node) (*FetchResult, error)
```
- Runs multiple fetchers concurrently
- Implements staggered start to reduce resource waste
- Cancels other fetchers on first success (configurable)

### Configuration Options

```go
type FetcherConfig struct {
    MaxConcurrent    int           // Maximum concurrent fetchers (default: 3)
    Timeout          time.Duration // Overall timeout (default: 30s)
    StaggerDelay     time.Duration // Delay between starts (default: 150ms)
    CancelOnFirstWin bool          // Cancel others on success (default: true)
}
```

### Performance Metrics

```go
type Metrics struct {
    TotalRequests      int64
    SuccessfulRequests int64
    FailedRequests     int64
    ProtocolStats      map[string]*ProtocolMetrics
}

type ProtocolMetrics struct {
    Attempts        int64
    Successes       int64
    Failures        int64
    AvgLatency      time.Duration
    BytesTransferred int64
}
```

## 🏃‍♂️ Practical Usage

### Example 1: Basic Block Fetching

```bash
cd 18-multifetcher
go test -v -run TestMultiFetcher_FetchBlock
```

**Expected Output:**
```
=== MultiFetcher Block Fetching Demo ===

🔧 Creating multifetcher with components:
   • Bitswap wrapper (P2P block exchange)
   • GraphSync wrapper (structured data sync)
   • IPNI wrapper (provider discovery)
   • HTTP fetcher (gateway access)

📦 Fetching block: bafkreiabcd1234...
   Strategy: Query IPNI for ranked providers
   Protocols: [bitswap, graphsync, http]

🏁 Racing protocols:
   • Bitswap: Started at 0ms
   • HTTP: Started at 150ms (staggered)
   • GraphSync: Started at 300ms (staggered)

✅ Result:
   Winner: HTTP gateway (420ms)
   Provider: gateway.ipfs.io
   Size: 1.2 KB
   Other protocols cancelled
```

### Example 2: DAG Fetching with Selector

```go
// Create IPLD selector for specific parts of DAG
selector := basicnode.Prototype.Map.NewBuilder()
// ... build selector

// Fetch DAG with selector
result, err := multiFetcher.FetchDAG(ctx, rootCID, selector)
if err != nil {
    log.Fatalf("DAG fetch failed: %v", err)
}

fmt.Printf("Fetched %d bytes via %s\n", len(result.Data), result.Protocol)
```

### Example 3: Performance Monitoring

```go
// Get current metrics
metrics := multiFetcher.GetMetrics()

fmt.Printf("Overall Stats:\n")
fmt.Printf("  Total Requests: %d\n", metrics.TotalRequests)
fmt.Printf("  Success Rate: %.2f%%\n",
    float64(metrics.SuccessfulRequests)/float64(metrics.TotalRequests)*100)

fmt.Printf("\nProtocol Performance:\n")
for protocol, stats := range metrics.ProtocolStats {
    fmt.Printf("  %s:\n", protocol)
    fmt.Printf("    Success Rate: %.2f%%\n",
        float64(stats.Successes)/float64(stats.Attempts)*100)
    fmt.Printf("    Avg Latency: %v\n", stats.AvgLatency)
    fmt.Printf("    Bytes Transferred: %d\n", stats.BytesTransferred)
}
```

## 🔍 Key Features Demonstrated

### 1. **IPNI Integration**
- Discover optimal providers using network indexing
- Rank providers by performance and availability
- Handle provider metadata (gateway URLs, capabilities)

### 2. **Protocol Racing**
- Run multiple protocols concurrently
- Implement staggered starts to reduce resource waste
- Cancel redundant operations on first success

### 3. **Smart Fallbacks**
- Direct Bitswap when no IPNI providers found
- HTTP gateway as reliable fallback
- Graceful degradation on protocol failures

### 4. **Performance Optimization**
- Configurable concurrency limits
- Timeout management across protocols
- Metrics collection for optimization

### 5. **Error Handling**
- Comprehensive error reporting per protocol
- Aggregated failure analysis
- Retry strategies and circuit breaker patterns

## 🧪 Running Tests

```bash
# Run all multifetcher tests
go test ./...

# Run specific functionality tests
go test -v -run TestMultiFetcher_Configuration
go test -v -run TestMultiFetcher_FetchBlock
go test -v -run TestMultiFetcher_Metrics

# Test with race detection
go test -race ./...

# Benchmark performance
go test -bench=. -benchmem
```

### Test Coverage

The test suite covers:
- ✅ Configuration validation and defaults
- ✅ Protocol racing mechanics
- ✅ IPNI integration
- ✅ Metrics collection and reporting
- ✅ Error handling and fallbacks
- ✅ Performance benchmarking

## 🔗 Integration Examples

### With Gateway (10-gateway)

```go
// Use multifetcher as backend for gateway
func (gw *Gateway) handleIPFSRequest(w http.ResponseWriter, r *http.Request) {
    cid := extractCIDFromPath(r.URL.Path)

    // Use multifetcher for resilient content retrieval
    result, err := gw.multiFetcher.FetchBlock(r.Context(), cid)
    if err != nil {
        http.Error(w, "Content not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/octet-stream")
    w.Write(result.Data)
}
```

### With IPNI (17-ipni)

```go
// Configure multifetcher with IPNI provider discovery
ipniWrapper := ipni.NewIPNIWrapper("/tmp/ipni-index")
defer ipniWrapper.Close()

// Add content to IPNI index
ipniWrapper.PutBitswap(ctx, peerID, contextID, multihashes...)
ipniWrapper.PutHTTP(ctx, peerID, contextID, gatewayURLs, partialCAR, auth, multihashes...)

// Use in multifetcher for provider discovery
multiFetcher := NewMultiFetcher(ipniWrapper, graphsyncWrapper, bitswapWrapper, nil)
```

### With Bitswap (04-bitswap)

```go
// Enhanced bitswap integration with peer-specific requests
bitswapWrapper := bitswap.NewBitswap(ctx, dhtWrapper, hostWrapper, persistentWrapper)

// Use multifetcher's enhanced bitswap capabilities
result, err := multiFetcher.FetchBlock(ctx, targetCID)
if err == nil && result.Protocol == "bitswap" {
    fmt.Printf("Retrieved via Bitswap from peer: %s\n", result.Provider)
}
```

## 🎯 Use Cases

### 1. **Content Delivery Networks**
- Multi-protocol content retrieval
- Automatic failover between providers
- Performance optimization for global access

### 2. **IPFS Gateways**
- Resilient backend for public gateways
- Load balancing across protocols
- Intelligent caching strategies

### 3. **Data Synchronization**
- Large dataset synchronization
- Selective DAG fetching
- Bandwidth optimization

### 4. **Mobile and Edge Applications**
- Network-aware protocol selection
- Battery-efficient fetching
- Offline-first strategies

## 🔧 Advanced Configuration

### Custom HTTP Fetcher

```go
// Configure HTTP fetcher with custom transport
httpFetcher := &HTTPFetcher{
    client: &http.Client{
        Timeout: 60 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:          20,
            IdleConnTimeout:       60 * time.Second,
            MaxConnsPerHost:       10,
            ResponseHeaderTimeout: 15 * time.Second,
        },
    },
}
```

### Performance Tuning

```go
// Optimize for high-throughput scenarios
config := FetcherConfig{
    MaxConcurrent:    10,              // More concurrent fetchers
    Timeout:          2 * time.Minute, // Longer timeout for large content
    StaggerDelay:     50 * time.Millisecond, // Faster staggering
    CancelOnFirstWin: false,           // Collect multiple results
}
```

### Custom Metrics Collection

```go
// Implement custom metrics handler
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        metrics := multiFetcher.GetMetrics()

        // Export metrics to monitoring system
        exportMetrics(metrics)

        // Log performance warnings
        for protocol, stats := range metrics.ProtocolStats {
            if stats.Successes > 0 && stats.AvgLatency > 5*time.Second {
                log.Printf("Warning: %s protocol showing high latency: %v",
                    protocol, stats.AvgLatency)
            }
        }
    }
}()
```

## 📚 Next Steps

### Immediate Next Steps
Congratulations! You've mastered the advanced IPFS content retrieval stack. Your next steps involve specialization and production deployment:

1. **Production Implementation**: Real-World Applications
   - Deploy multifetcher in production environments
   - Implement comprehensive monitoring and optimization
   - Build custom protocol selection strategies for your use case

2. **Advanced Specialization Paths**: Choose your expertise:
   - **Performance Engineering**: Custom optimization and profiling systems
   - **Network Architecture**: Large-scale distributed content systems
   - **Protocol Development**: Contributing to IPFS protocol evolution

### Related Modules
**Prerequisites (Complete foundation achieved):**
- [04-bitswap](../04-bitswap): Block exchange protocol mastery
- [15-graphsync](../15-graphsync): Selective graph synchronization
- [16-trustless-gateway](../16-trustless-gateway): HTTP gateway patterns
- [17-ipni](../17-ipni): Content indexing and discovery

**Supporting Advanced Technologies:**
- [12-ipld-prime](../12-ipld-prime): High-performance IPLD operations
- [13-dasl](../13-dasl): Schema-based development patterns
- [14-traversal-selector](../14-traversal-selector): Advanced data navigation
- [02-network](../02-network): P2P networking fundamentals

**Production Integration:**
- Enterprise Systems: Large-scale IPFS deployment patterns
- Monitoring: Advanced observability and performance tracking
- Security: Production security and compliance requirements

### Alternative Learning Paths

**For Systems Architecture:**
18-multifetcher → Enterprise IPFS Design → Large-Scale System Implementation

**For Performance Engineering:**
18-multifetcher → Custom Protocol Optimization → High-Performance Computing

**For Protocol Development:**
18-multifetcher → IPFS Core Development → Standards and Research

**For Web3 Applications:**
18-multifetcher → Decentralized Application Backends → Blockchain Integration

**For Research and Innovation:**
18-multifetcher → Novel Distributed Systems → Academic Research → Future Protocols

## 🐛 Troubleshooting

### Common Issues

1. **No Providers Found**
   ```
   Error: failed to get providers from IPNI
   ```
   - Ensure IPNI wrapper is properly initialized
   - Check if content is indexed in IPNI
   - Verify network connectivity

2. **High Latency**
   ```
   Warning: Average latency >5s for HTTP protocol
   ```
   - Check gateway performance
   - Consider adjusting timeout values
   - Monitor network conditions

3. **Protocol Failures**
   ```
   Error: all fetchers failed
   ```
   - Check individual protocol health
   - Verify peer connectivity for Bitswap
   - Test gateway accessibility for HTTP

### Debug Tips

```go
// Enable detailed logging
config := FetcherConfig{
    MaxConcurrent:    1, // Reduce concurrency for debugging
    Timeout:          5 * time.Minute,
    StaggerDelay:     1 * time.Second, // Slow down for observation
    CancelOnFirstWin: false, // See all protocol results
}

// Monitor individual protocol performance
metrics := multiFetcher.GetMetrics()
for protocol, stats := range metrics.ProtocolStats {
    if stats.Failures > stats.Successes {
        log.Printf("Protocol %s failing: %d failures vs %d successes",
            protocol, stats.Failures, stats.Successes)
    }
}
```

## 📚 Additional Resources

- [IPNI Specification](https://specs.ipfs.tech/ipni/)
- [Bitswap Protocol](https://docs.ipfs.tech/concepts/bitswap/)
- [GraphSync Protocol](https://specs.ipfs.tech/ipld/graphsync/)
- [IPFS Gateway Specification](https://specs.ipfs.tech/http-gateways/)
- [Go Concurrency Patterns](https://go.dev/doc/codewalk/sharemem/)

---

The MultiFetcher represents the pinnacle of IPFS content retrieval optimization, combining multiple protocols, intelligent provider selection, and performance monitoring into a single, powerful system.