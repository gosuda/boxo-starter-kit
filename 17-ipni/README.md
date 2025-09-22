# 17-ipni: IPFS Network Indexer Integration

## 🎯 Learning Objectives

By the end of this module, you will understand:
- How IPNI (InterPlanetary Network Indexer) enables content discovery at scale
- Building indexer clients for efficient content lookup
- Working with provider records and transport protocols
- Implementing content planning and scoring algorithms
- Understanding the IPNI ecosystem and architecture
- Optimizing content retrieval through intelligent provider selection
- Integration patterns for production IPFS applications

## 📋 Prerequisites

- Completion of [00-block-cid](../00-block-cid) - Content addressing fundamentals
- Understanding of [02-network](../02-network) - P2P networking concepts
- Familiarity with [15-graphsync](../15-graphsync) - Data transfer protocols
- Familiarity with [16-trustless-gateway](../16-trustless-gateway) - HTTP gateway concepts
- Knowledge of distributed systems and content discovery
- Understanding of DHT and content routing concepts
- Basic knowledge of database storage systems

## 🔑 Core Concepts

### IPNI (InterPlanetary Network Indexer)

**IPNI** is a distributed system for indexing and discovering content in the IPFS network:
- **Content Discovery**: Find providers for specific content hashes (multihashes)
- **Provider Records**: Store information about where content is available
- **Transport Agnostic**: Support multiple retrieval protocols (HTTP, GraphSync, Bitswap)
- **Decentralized**: Distributed indexer network without central authority
- **Efficient**: Fast lookup times for content across the entire IPFS network

### Key Components

#### 1. Indexer Engine
```go
// Core indexing engine with storage and caching
engine := engine.New(store,
    engine.WithCache(cache),
    engine.WithCacheOnPut(true))
```

#### 2. Provider System
```go
// Provider information including transport capabilities
type Provider struct {
    ID         string
    Addrs      []string      // Connection addresses
    Transports []Transport   // Supported protocols
    Region     string        // Geographic region
    Meta       map[string]string
}
```

#### 3. Content Planning
```go
// Intelligent provider selection and scoring
type Planner struct {
    Policy      ScoringPolicy  // Scoring algorithm
    Preferences Prefs          // User preferences
}
```

### IPNI Architecture Benefits

#### Content Discovery
- **Global Index**: Search across entire IPFS network efficiently
- **Multiple Transports**: Support HTTP, GraphSync, Bitswap protocols
- **Provider Metadata**: Rich information about content availability
- **Regional Optimization**: Geographic provider selection

#### Performance Optimization
- **Intelligent Routing**: Score providers based on multiple factors
- **Transport Selection**: Choose optimal retrieval method
- **Caching**: Local caching for frequently accessed content
- **Staggered Requests**: Race multiple providers for optimal performance

## 💻 Code Architecture

### Module Structure
```
17-ipni/
├── pkg/
│   ├── ipni.go          # Main IPNI wrapper
│   ├── planner.go       # Provider planning and scoring
│   └── provider.go      # Provider data structures
└── ipni_test.go         # Test framework
```

### Core Components

#### IPNIWrapper
Main wrapper providing IPNI indexer operations:

```go
type IPNIWrapper struct {
    Engine       *engine.Engine    // Core indexing engine
    Planner      *Planner         // Provider selection logic
    HealthScorer HealthScorer     // Provider health assessment
    DefaultTTL   time.Duration    // Default record TTL
}
```

**Key Methods:**
- `NewIPNIWrapper(path)`: Create indexer with storage path
- `PutMultihashes(ctx, value, mhs...)`: Index content with provider info
- `FindProviders(ctx, mh)`: Discover providers for content
- `PlanRetrieval(ctx, mh, prefs)`: Get optimal retrieval plan

#### Transport System
Support for multiple content retrieval protocols:

```go
type TransportKind string

const (
    TLocal     TransportKind = "local"     // Local storage
    THTTP      TransportKind = "http"      // HTTP/HTTPS
    TGraphSync TransportKind = "graphsync" // GraphSync protocol
    TBitswap   TransportKind = "bitswap"   // Bitswap protocol
)
```

#### Scoring System
Intelligent provider ranking based on multiple factors:

```go
type ScoringPolicy struct {
    TransportBase  map[TransportKind]float64  // Base scores per transport
    HealthWeight   float64                    // Health score influence
    RegionBonus    float64                    // Geographic proximity bonus
    PartialBonus   float64                    // Partial content support bonus
    LocalBias      float64                    // Local provider preference
    DefaultStagger time.Duration             // Request staggering
}
```

## 🏃‍♂️ Usage Examples

### Basic IPNI Setup

```go
package main

import (
    "context"
    "fmt"
    "log"

    ipni "github.com/gosuda/boxo-starter-kit/17-ipni/pkg"
    "github.com/multiformats/go-multihash"
)

func main() {
    ctx := context.Background()

    // Create IPNI wrapper with local storage
    indexer, err := ipni.NewIPNIWrapper("./ipni-data")
    if err != nil {
        log.Fatal(err)
    }
    defer indexer.Close()

    // Create some test content hash
    hash, err := multihash.Sum([]byte("Hello IPNI!"), multihash.SHA2_256, -1)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Looking up providers for: %s\\n", hash.B58String())

    // Find providers (initially empty for new content)
    providers, err := indexer.FindProviders(ctx, hash, 10)
    if err != nil {
        log.Printf("Provider search error: %v\\n", err)
    } else {
        fmt.Printf("Found %d providers\\n", len(providers))
    }
}
```

### Provider Registration

```go
import (
    "github.com/ipni/go-indexer-core"
    "github.com/libp2p/go-libp2p/core/peer"
)

func registerProvider(indexer *ipni.IPNIWrapper) error {
    ctx := context.Background()

    // Create provider record
    provider := &ipni.Provider{
        ID:    "provider-id-123",
        Addrs: []string{
            "/ip4/192.168.1.100/tcp/4001",
            "https://gateway.example.com",
        },
        Transports: []ipni.Transport{
            {
                Kind:       ipni.THTTP,
                PartialCAR: true,
                Auth:       false,
            },
            {
                Kind: ipni.TBitswap,
            },
        },
        Region: "us-west",
        Meta: map[string]string{
            "name":        "Example Provider",
            "description": "Test IPNI provider",
        },
    }

    // Create value record for indexing
    value := indexer.Value{
        ProviderID:    peer.ID("provider-id-123"),
        ContextID:     []byte("context-123"),
        MetadataBytes: []byte("provider metadata"),
    }

    // Register content with provider
    hash, _ := multihash.Sum([]byte("content to index"), multihash.SHA2_256, -1)

    err := indexer.PutMultihashes(ctx, value, hash)
    if err != nil {
        return fmt.Errorf("failed to register content: %w", err)
    }

    fmt.Printf("Successfully registered content with provider\\n")
    return nil
}
```

### Content Discovery and Planning

```go
func discoverAndPlan(indexer *ipni.IPNIWrapper, contentHash multihash.Multihash) {
    ctx := context.Background()

    // Set up user preferences
    prefs := ipni.Prefs{
        PreferredTransports: []ipni.TransportKind{
            ipni.THTTP,      // Prefer HTTP first
            ipni.TGraphSync, // Then GraphSync
            ipni.TBitswap,   // Finally Bitswap
        },
        Region:         "us-west",
        RequireAuth:    false,
        PartialCAR:     true,
        MaxProviders:   5,
        RequestTimeout: 30 * time.Second,
    }

    // Find providers
    providers, err := indexer.FindProviders(ctx, contentHash, 20)
    if err != nil {
        log.Printf("Provider discovery failed: %v\\n", err)
        return
    }

    fmt.Printf("Found %d providers\\n", len(providers))

    // Plan optimal retrieval strategy
    plan, err := indexer.Planner.PlanRetrieval(ctx, contentHash, prefs, providers)
    if err != nil {
        log.Printf("Planning failed: %v\\n", err)
        return
    }

    // Execute plan
    fmt.Printf("Retrieval Plan:\\n")
    for i, step := range plan.Steps {
        fmt.Printf("  Step %d: Provider %s via %s (score: %.2f)\\n",
            i+1, step.Provider.ID, step.Transport.Kind, step.Score)
    }

    // Execute retrieval with staggered requests
    err = executeRetrievalPlan(ctx, plan)
    if err != nil {
        log.Printf("Retrieval failed: %v\\n", err)
    } else {
        fmt.Printf("Content retrieved successfully\\n")
    }
}
```

### Advanced Scoring Configuration

```go
func setupCustomScoring(indexer *ipni.IPNIWrapper) {
    // Configure custom scoring policy
    policy := ipni.ScoringPolicy{
        TransportBase: map[ipni.TransportKind]float64{
            ipni.TLocal:     1.0,  // Highest priority for local
            ipni.THTTP:      0.8,  // High priority for HTTP
            ipni.TGraphSync: 0.6,  // Medium for GraphSync
            ipni.TBitswap:   0.3,  // Lower for Bitswap
        },
        HealthWeight:   0.7,  // High influence of provider health
        RegionBonus:    0.3,  // Significant regional preference
        PartialBonus:   0.4,  // Good bonus for partial CAR support
        LocalBias:      0.5,  // Moderate local preference
        DefaultStagger: 100 * time.Millisecond, // Fast staggering
    }

    // Update planner with custom policy
    indexer.Planner = ipni.NewPlanner(&policy)

    // Set up health scorer
    healthScorer := &ipni.BasicHealthScorer{
        SuccessWeight: 0.7,
        LatencyWeight: 0.3,
        TimeWindow:    24 * time.Hour,
    }
    indexer.SetHealthScorer(healthScorer)

    fmt.Println("Custom scoring configuration applied")
}
```

### Batch Operations

```go
func batchIndexing(indexer *ipni.IPNIWrapper) error {
    ctx := context.Background()

    // Prepare batch of content to index
    contentBatch := []string{
        "content item 1",
        "content item 2",
        "content item 3",
        "content item 4",
        "content item 5",
    }

    // Create provider value
    value := indexer.Value{
        ProviderID:    peer.ID("batch-provider"),
        ContextID:     []byte("batch-context"),
        MetadataBytes: []byte("batch indexing metadata"),
    }

    // Generate hashes for all content
    var hashes []multihash.Multihash
    for _, content := range contentBatch {
        hash, err := multihash.Sum([]byte(content), multihash.SHA2_256, -1)
        if err != nil {
            return err
        }
        hashes = append(hashes, hash)
    }

    // Batch index all content
    start := time.Now()
    err := indexer.PutMultihashes(ctx, value, hashes...)
    duration := time.Since(start)

    if err != nil {
        return fmt.Errorf("batch indexing failed: %w", err)
    }

    fmt.Printf("Successfully indexed %d items in %v\\n", len(hashes), duration)

    // Verify indexing
    for i, hash := range hashes {
        providers, err := indexer.FindProviders(ctx, hash, 1)
        if err != nil || len(providers) == 0 {
            fmt.Printf("Warning: Content %d not found in index\\n", i+1)
        } else {
            fmt.Printf("✅ Content %d indexed successfully\\n", i+1)
        }
    }

    return nil
}
```

## 🏃‍♂️ Running and Testing

### Basic Testing
```bash
cd 17-ipni

# Run any existing tests
go test -v

# Build and test integration
go build ./pkg
```

### Integration Testing
```go
func TestIPNIIntegration(t *testing.T) {
    ctx := context.Background()

    // Create temporary indexer
    tmpDir, err := os.MkdirTemp("", "ipni-test-*")
    require.NoError(t, err)
    defer os.RemoveAll(tmpDir)

    indexer, err := ipni.NewIPNIWrapper(tmpDir)
    require.NoError(t, err)
    defer indexer.Close()

    // Test content indexing
    content := "test content for indexing"
    hash, err := multihash.Sum([]byte(content), multihash.SHA2_256, -1)
    require.NoError(t, err)

    // Create provider value
    value := indexer.Value{
        ProviderID:    peer.ID("test-provider"),
        ContextID:     []byte("test-context"),
        MetadataBytes: []byte("test metadata"),
    }

    // Index content
    err = indexer.PutMultihashes(ctx, value, hash)
    require.NoError(t, err)

    // Verify content can be found
    providers, err := indexer.FindProviders(ctx, hash, 10)
    require.NoError(t, err)
    require.Greater(t, len(providers), 0)

    t.Logf("Successfully indexed and retrieved content")
}
```

### Performance Testing
```go
func BenchmarkIPNILookup(b *testing.B) {
    ctx := context.Background()
    indexer, _ := ipni.NewIPNIWrapper("")
    defer indexer.Close()

    // Pre-populate with test data
    for i := 0; i < 1000; i++ {
        content := fmt.Sprintf("benchmark content %d", i)
        hash, _ := multihash.Sum([]byte(content), multihash.SHA2_256, -1)
        value := indexer.Value{ProviderID: peer.ID("bench-provider")}
        indexer.PutMultihashes(ctx, value, hash)
    }

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        content := fmt.Sprintf("benchmark content %d", i%1000)
        hash, _ := multihash.Sum([]byte(content), multihash.SHA2_256, -1)
        indexer.FindProviders(ctx, hash, 5)
    }
}
```

## 🔧 Configuration and Optimization

### Storage Configuration
```go
// Configure Pebble storage options
storeOptions := &pebble.Options{
    CacheSize:               64 << 20, // 64MB cache
    WriteBufferSize:         32 << 20, // 32MB write buffer
    MaxOpenFiles:            1000,
    L0CompactionThreshold:   4,
    L0StopWritesThreshold:   12,
}

store, err := pebble.New(path, storeOptions)
```

### Cache Tuning
```go
// Configure radix cache
cacheSize := 16 * 1024 * 1024 // 16MB
cache := radixcache.New(cacheSize)

engine := engine.New(store,
    engine.WithCache(cache),
    engine.WithCacheOnPut(true),
    engine.WithCacheOnFind(true),
)
```

### Health Monitoring
```go
type HealthMonitor struct {
    indexer  *ipni.IPNIWrapper
    interval time.Duration
}

func (h *HealthMonitor) Start(ctx context.Context) {
    ticker := time.NewTicker(h.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            h.checkHealth(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (h *HealthMonitor) checkHealth(ctx context.Context) {
    stats, err := h.indexer.Stats()
    if err != nil {
        log.Printf("Health check failed: %v", err)
        return
    }

    log.Printf("IPNI Health: %d providers, %d records, %.2fMB storage",
        stats.NumProviders, stats.NumRecords, float64(stats.StorageSize)/(1024*1024))
}
```

## 🔍 Troubleshooting

### Common Issues

1. **Storage Errors**
   ```
   Error: failed to open database
   Solution: Check file permissions and disk space
   ```

2. **Provider Discovery Failures**
   ```
   Error: no providers found
   Solution: Verify content is indexed and network connectivity
   ```

3. **Performance Issues**
   ```
   Error: slow lookup times
   Solution: Tune cache size and storage configuration
   ```

4. **Memory Usage**
   ```
   Error: high memory consumption
   Solution: Adjust cache sizes and implement periodic cleanup
   ```

### Performance Optimization

- **Cache Sizing**: Balance memory usage vs. lookup performance
- **Storage Tuning**: Configure Pebble for your workload
- **Batch Operations**: Use batch indexing for multiple items
- **Health Scoring**: Implement effective provider scoring
- **Request Staggering**: Optimize concurrent request timing

## 📊 Performance Characteristics

### Indexer Performance
- **Lookup Time**: Typically <10ms for cached records
- **Indexing Speed**: 1000+ records/second for batch operations
- **Storage Efficiency**: Compressed record storage
- **Memory Usage**: Configurable cache with reasonable defaults

### Network Integration
- **Provider Discovery**: Fast lookup across distributed index
- **Transport Selection**: Intelligent protocol choosing
- **Geographic Optimization**: Regional provider preference
- **Fault Tolerance**: Multiple provider fallbacks

## 🔗 Related Modules

- **[18-multifetcher](../18-multifetcher)**: Multifetcher using Bitswap, GraphSync, and HTTP in parallel

## 📚 Further Reading

- [IPNI Specification](https://github.com/ipni/specs)
- [Go-Indexer-Core Documentation](https://pkg.go.dev/github.com/ipni/go-indexer-core)
- [IPFS Provider Records](https://docs.ipfs.tech/concepts/dht/#provider-records)
- [Content Routing in IPFS](https://docs.ipfs.tech/concepts/content-routing/)
- [IPNI Network Architecture](https://blog.ipfs.tech/2022-09-02-introducing-ipni/)

---

This module demonstrates how to integrate with IPNI for efficient content discovery and retrieval planning in the IPFS network. Master these patterns to build scalable content discovery systems.