# 15-graphsync: Selective Graph Synchronization Protocol

## 🎯 Learning Objectives

By the end of this module, you will understand:
- How GraphSync enables efficient selective data synchronization
- Working with IPLD selectors for precise data fetching
- Building peer-to-peer data exchange systems with GraphSync
- Request/response patterns for distributed data retrieval
- Optimizing network bandwidth with selective sync
- Integration between GraphSync, libp2p, and IPLD
- Error handling and progress monitoring in distributed sync

## 📋 Prerequisites

- Completion of [02-network](../02-network) - libp2p networking fundamentals
- Completion of [12-ipld-prime](../12-ipld-prime) - IPLD-prime operations
- Completion of [14-traversal-selector](../14-traversal-selector) - Selector concepts
- Understanding of peer-to-peer networking concepts
- Familiarity with distributed systems and sync protocols
- Knowledge of asynchronous programming patterns

## 🔑 Core Concepts

### GraphSync Protocol

**GraphSync** is a protocol for synchronizing IPLD graphs across peers:
- **Selective**: Transfer only requested parts of data graphs
- **Efficient**: Minimize network overhead and bandwidth usage
- **Verifiable**: Content-addressed data ensures integrity
- **Pausable**: Support for resumable transfers
- **Extensible**: Custom extensions for advanced features

### Key Benefits

#### 1. Bandwidth Optimization
- Transfer only necessary data using IPLD selectors
- Avoid downloading entire datasets when only parts are needed
- Intelligent deduplication based on content addressing

#### 2. Latency Reduction
- Parallel fetching of independent data chunks
- Streaming data as it becomes available
- Minimal round-trips for linked data structures

#### 3. Reliability
- Content verification through CID validation
- Automatic retry mechanisms for failed transfers
- Graceful handling of peer disconnections

### GraphSync Components

#### Request Model
```go
// GraphSync request specifies what to fetch
type Request {
    Root     cid.Cid        // Starting point for traversal
    Selector ipld.Node      // Which parts to fetch
    Extensions []Extension  // Custom metadata
}
```

#### Response Model
```go
// Streaming responses with progress updates
type ResponseProgress {
    Node     ipld.Node      // Data being transferred
    Path     datamodel.Path // Location in graph
    LastBlock bool          // Transfer completion flag
}
```

## 💻 Code Architecture

### Module Structure
```
15-graphsync/
├── pkg/
│   └── graphsync.go           # Main GraphSync wrapper
└── graphsync_test.go          # Comprehensive tests
```

### Core Components

#### GraphSyncWrapper
Main wrapper providing simplified GraphSync operations:

```go
type GraphSyncWrapper struct {
    Host *network.HostWrapper      // libp2p networking
    Ipld *ipldprime.IpldWrapper   // IPLD data operations
    igs.GraphExchange             // Core GraphSync interface
}
```

**Key Methods:**
- `New(ctx, host, ipld)`: Create GraphSync instance with networking
- `Fetch(ctx, peer, root, selector)`: High-level data fetching
- `Request(ctx, peer, root, selector)`: Low-level request with channels

#### Integration Points
- **libp2p Host**: Peer discovery and connection management
- **IPLD LinkSystem**: Content storage and retrieval
- **Selectors**: Precise specification of required data
- **Extensions**: Protocol customization and metadata

## 🏃‍♂️ Usage Examples

### Basic GraphSync Setup

```go
package main

import (
    "context"
    "fmt"

    graphsync "github.com/gosuda/boxo-starter-kit/15-graphsync/pkg"
    network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
)

func main() {
    ctx := context.Background()

    // Create first peer (data provider)
    provider, err := graphsync.New(ctx, nil, nil)
    if err != nil {
        panic(err)
    }

    // Create second peer (data consumer)
    consumer, err := graphsync.New(ctx, nil, nil)
    if err != nil {
        panic(err)
    }

    // Connect peers
    providerAddr := provider.Host.GetFullAddresses()[0]
    err = consumer.Host.ConnectToPeer(ctx, providerAddr)
    if err != nil {
        panic(err)
    }

    fmt.Println("GraphSync peers connected successfully")
}
```

### Simple Data Exchange

```go
// Provider stores some data
data := map[string]any{
    "title": "GraphSync Demo",
    "content": "This data will be synced across peers",
    "metadata": map[string]any{
        "created": "2024-01-01",
        "version": 1,
    },
}

// Store data on provider
dataCID, err := provider.Ipld.PutIPLDAny(ctx, data)
if err != nil {
    panic(err)
}

fmt.Printf("Data stored with CID: %s\n", dataCID)

// Consumer fetches the complete data
progress, err := consumer.Fetch(ctx, provider.Host.ID(), dataCID, nil)
if err != nil {
    panic(err)
}

if progress {
    // Retrieve the synced data
    retrievedData, err := consumer.Ipld.GetIPLDAny(ctx, dataCID)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Successfully synced data: %v\n", retrievedData)
} else {
    fmt.Println("No data was transferred")
}
```

### Selective Data Fetching with Selectors

```go
import (
    ts "github.com/gosuda/boxo-starter-kit/14-traversal-selector/pkg"
    cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

// Create complex linked data structure
userProfile := map[string]any{
    "name": "Alice",
    "bio":  "Software Engineer",
}
profileCID, _ := provider.Ipld.PutIPLDAny(ctx, userProfile)

userPosts := []map[string]any{
    {"title": "GraphSync Introduction", "content": "..."},
    {"title": "IPLD Best Practices", "content": "..."},
}
postsCID, _ := provider.Ipld.PutIPLDAny(ctx, userPosts)

// Root structure linking everything
userData := map[string]any{
    "profile": cidlink.Link{Cid: profileCID},
    "posts":   cidlink.Link{Cid: postsCID},
    "settings": map[string]any{
        "theme": "dark",
        "notifications": true,
    },
}
rootCID, _ := provider.Ipld.PutIPLDAny(ctx, userData)

// Consumer fetches only the profile (not posts or settings)
profileSelector := ts.SelectorField("profile")
progress, err := consumer.Fetch(ctx, provider.Host.ID(), rootCID, profileSelector)
if err != nil {
    panic(err)
}

if progress {
    // Profile data is now available locally
    profile, err := consumer.Ipld.GetIPLDAny(ctx, profileCID)
    if err == nil {
        fmt.Printf("Profile synced: %v\n", profile)
    }

    // Posts were NOT transferred (selective sync)
    _, err = consumer.Ipld.GetIPLDAny(ctx, postsCID)
    if err != nil {
        fmt.Println("Posts not available locally (as expected)")
    }
} else {
    fmt.Println("No profile data transferred")
}
```

### Advanced Request Handling

```go
// Low-level request with channels for fine-grained control
responseChannel, errorChannel, err := consumer.Request(
    ctx,
    provider.Host.ID(),
    rootCID,
    ts.SelectorAll(true), // Fetch everything
)
if err != nil {
    panic(err)
}

// Process responses as they arrive
for responseChannel != nil || errorChannel != nil {
    select {
    case response, ok := <-responseChannel:
        if !ok {
            responseChannel = nil
            continue
        }

        fmt.Printf("Received data chunk at path: %s\n", response.Path)
        if response.LastBlock {
            fmt.Println("Transfer completed")
        }

    case err, ok := <-errorChannel:
        if !ok {
            errorChannel = nil
            continue
        }

        if err != nil {
            fmt.Printf("Transfer error: %v\n", err)
            return
        }

    case <-ctx.Done():
        fmt.Println("Transfer cancelled")
        return
    }
}

fmt.Println("All data successfully transferred")
```

### Batch Fetching Multiple Items

```go
// Fetch multiple independent data items
items := []cid.Cid{cid1, cid2, cid3}

for i, itemCID := range items {
    fmt.Printf("Fetching item %d: %s\n", i+1, itemCID)

    progress, err := consumer.Fetch(ctx, provider.Host.ID(), itemCID, nil)
    if err != nil {
        fmt.Printf("Failed to fetch item %d: %v\n", i+1, err)
        continue
    }

    if progress {
        fmt.Printf("Successfully fetched item %d\n", i+1)
    } else {
        fmt.Printf("No data received for item %d\n", i+1)
    }
}
```

## 🏃‍♂️ Running the Examples

### Run Tests
```bash
cd 15-graphsync
go test -v
```

### Expected Output
```
=== RUN   TestGraphSyncPubsub
--- PASS: TestGraphSyncPubsub (1.23s)
=== RUN   TestGraphSyncPubsubWithSelector
--- PASS: TestGraphSyncPubsubWithSelector (2.45s)
PASS
```

### Test Explanations

1. **TestGraphSyncPubsub**: Basic end-to-end data transfer
   - Creates two GraphSync peers
   - Connects them over libp2p
   - Transfers simple string data
   - Verifies data integrity after transfer

2. **TestGraphSyncPubsubWithSelector**: Selective synchronization
   - Creates linked data structure (left/right references)
   - Uses selector to fetch only "left" branch initially
   - Verifies selective transfer worked correctly
   - Fetches remaining data with complete selector

## 🔧 Configuration and Optimization

### Custom Request Hooks
```go
// Register custom request validation
gs.RegisterIncomingRequestHook(func(
    p peer.ID,
    request graphsync.RequestData,
    hookActions graphsync.IncomingRequestHookActions,
) {
    // Custom validation logic
    if isValidRequest(request) {
        hookActions.ValidateRequest()
    } else {
        hookActions.TerminateWithError(errors.New("invalid request"))
    }
})
```

### Response Processing Hooks
```go
// Monitor outgoing responses
gs.RegisterOutgoingResponseHook(func(
    p peer.ID,
    response graphsync.ResponseData,
    hookActions graphsync.OutgoingResponseHookActions,
) {
    // Log or modify responses
    log.Printf("Sending response to %s: %d blocks", p, response.BlockCount())
})
```

### Network Optimization
```go
// Configure connection limits and timeouts
networkOptions := []network.Option{
    network.WithConnectionTimeout(30 * time.Second),
    network.WithMaxConnections(100),
}

host, err := network.New(networkOptions)
if err != nil {
    panic(err)
}

gs, err := graphsync.New(ctx, host, nil)
```

## 🧪 Testing Patterns

### Creating Test Networks
```go
func createTestPeers(t *testing.T, count int) []*graphsync.GraphSyncWrapper {
    ctx := context.Background()
    peers := make([]*graphsync.GraphSyncWrapper, count)

    for i := 0; i < count; i++ {
        peer, err := graphsync.New(ctx, nil, nil)
        require.NoError(t, err)
        peers[i] = peer
    }

    // Connect all peers to each other
    for i := 0; i < count; i++ {
        for j := i + 1; j < count; j++ {
            addr := peers[j].Host.GetFullAddresses()[0]
            err := peers[i].Host.ConnectToPeer(ctx, addr)
            require.NoError(t, err)
        }
    }

    return peers
}
```

### Performance Testing
```go
func BenchmarkGraphSyncTransfer(b *testing.B) {
    ctx := context.Background()
    peers := createTestPeers(b, 2)
    provider, consumer := peers[0], peers[1]

    // Create test data
    data := make([]byte, 1024*1024) // 1MB
    dataCID, _ := provider.Ipld.PutIPLDAny(ctx, data)

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, err := consumer.Fetch(ctx, provider.Host.ID(), dataCID, nil)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## 🔍 Troubleshooting

### Common Issues

1. **Peer Connection Failures**
   ```
   Error: failed to connect to peer
   Solution: Ensure peers are reachable and have valid addresses
   ```

2. **Selector Compilation Errors**
   ```
   Error: invalid selector specification
   Solution: Validate selector syntax using CompileSelector()
   ```

3. **Data Not Found**
   ```
   Error: block not found during transfer
   Solution: Verify data exists on provider before requesting
   ```

4. **Transfer Timeouts**
   ```
   Error: context deadline exceeded
   Solution: Increase timeout or check network connectivity
   ```

### Performance Debugging

- **Monitor Transfer Progress**: Use response channels to track transfer
- **Check Network Stats**: Monitor connection quality and bandwidth
- **Validate Selectors**: Ensure selectors are optimized for your use case
- **Profile Memory Usage**: Large transfers may require memory optimization

### Network Diagnostics
```go
// Check peer connectivity
func diagnosePeerConnection(gs *graphsync.GraphSyncWrapper, peerID peer.ID) {
    peers := gs.Host.Network().Peers()
    connected := false

    for _, p := range peers {
        if p == peerID {
            connected = true
            break
        }
    }

    if connected {
        fmt.Printf("Peer %s is connected\n", peerID)
    } else {
        fmt.Printf("Peer %s is not connected\n", peerID)
    }
}
```

## 📊 Performance Characteristics

### Transfer Efficiency
- **Selective Sync**: Only transfer requested data portions
- **Deduplication**: Automatic block-level deduplication
- **Streaming**: Progressive data availability during transfer
- **Parallel**: Concurrent transfer of independent data

### Network Utilization
- **Bandwidth**: Optimized for minimal data transfer
- **Latency**: Reduced round-trips through batching
- **Reliability**: Built-in retry and error recovery
- **Scalability**: Efficient for large distributed networks

### Best Practices
- Use **specific selectors** instead of fetching entire graphs
- **Batch requests** for multiple small items
- **Monitor transfer progress** for large datasets
- **Cache frequently accessed** data locally
- **Set appropriate timeouts** for network conditions

## 📚 Next Steps

### Immediate Next Steps
With GraphSync mastery, expand to comprehensive content discovery and optimization:

1. **[17-ipni](../17-ipni)**: Content Indexing and Discovery
   - Integrate GraphSync with IPNI for intelligent provider selection
   - Build scalable content discovery systems
   - Master network indexing for efficient data location

2. **Advanced Integration Paths**: Choose your specialization:
   - **[18-multifetcher](../18-multifetcher)**: Multi-protocol optimization with GraphSync
   - **[16-trustless-gateway](../16-trustless-gateway)**: Trustless HTTP delivery systems

### Related Modules
**Prerequisites (Essential foundation):**
- [02-network](../02-network): libp2p networking fundamentals
- [12-ipld-prime](../12-ipld-prime): Advanced IPLD operations
- [14-traversal-selector](../14-traversal-selector): Selector patterns for selective sync

**Complementary Protocols:**
- [04-bitswap](../04-bitswap): Block exchange protocol comparison
- [06-unixfs-car](../06-unixfs-car): CAR file operations for efficient transfer
- [10-gateway](../10-gateway): HTTP gateway integration patterns

**Advanced Applications:**
- [17-ipni](../17-ipni): Content discovery and provider selection
- [16-trustless-gateway](../16-trustless-gateway): Trustless content delivery
- [18-multifetcher](../18-multifetcher): Multi-source content retrieval

### Alternative Learning Paths

**For Distributed Systems Architecture:**
15-graphsync → 17-ipni → 18-multifetcher → Large-Scale Deployment

**For Protocol Engineering:**
15-graphsync → Custom Protocol Development → 02-network (advanced) → P2P Innovation

**For Content Delivery Networks:**
15-graphsync → 16-trustless-gateway → 17-ipni → CDN Implementation

**For Performance Optimization:**
15-graphsync → 18-multifetcher → Performance Tuning → High-Throughput Systems

## 📚 Further Reading

- [GraphSync Specification](https://github.com/ipld/specs/blob/master/block-layer/graphsync/graphsync.md)
- [Go-GraphSync Documentation](https://pkg.go.dev/github.com/ipfs/go-graphsync)
- [IPLD Selectors in GraphSync](https://ipld.io/specs/selectors/)
- [Libp2p Network Layer](https://docs.libp2p.io/)
- [IPFS Data Exchange](https://docs.ipfs.tech/concepts/bitswap/)

---

This module demonstrates how GraphSync enables efficient, selective synchronization of IPLD data across peer-to-peer networks. Master these patterns to build high-performance distributed data systems.