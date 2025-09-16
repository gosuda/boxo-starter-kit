# 02-network-bitswap: P2P Networking and Block Exchange

## 🎯 Learning Objectives

Through this module, you will learn:
- **P2P networking** fundamentals and IPFS network architecture
- **Bitswap protocol** for efficient block exchange between peers
- **Want-list** management and block request/response mechanisms
- **DHT (Distributed Hash Table)** for peer discovery and content routing
- **Network optimization** strategies for better performance
- **Real-world deployment** considerations for production networks

## 📋 Prerequisites

- **00-block-cid** module completion (Block and CID understanding)
- **01-persistent** module completion (Data persistence concepts)
- Basic understanding of networking concepts (TCP/IP, P2P)
- Knowledge of Go concurrency patterns (goroutines, channels)

## 🔑 Key Concepts

### What is Bitswap?

**Bitswap** is IPFS's block exchange protocol that enables peers to efficiently trade data blocks:

```
Traditional Client-Server:
Client → Server: "Give me file.txt"
Server → Client: [entire file]

IPFS Bitswap:
Peer A → Network: "I want blocks: [CID1, CID2, CID3]"
Peer B → Peer A: "I have CID1" → [block data]
Peer C → Peer A: "I have CID2" → [block data]
```

### Key Components

1. **Want-list**: List of blocks a peer is seeking
2. **Have-list**: List of blocks a peer can provide
3. **Bitswap Ledger**: Credit/debt tracking between peers
4. **Session**: Context for related block requests
5. **Strategy**: Algorithm for prioritizing requests and responses

### Network Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    Peer A   │    │    Peer B   │    │    Peer C   │
│ ┌─────────┐ │    │ ┌─────────┐ │    │ ┌─────────┐ │
│ │Bitswap  │←┼────┼→│Bitswap  │←┼────┼→│Bitswap  │ │
│ └─────────┘ │    │ └─────────┘ │    │ └─────────┘ │
│ ┌─────────┐ │    │ ┌─────────┐ │    │ ┌─────────┐ │
│ │ DHT     │←┼────┼→│ DHT     │←┼────┼→│ DHT     │ │
│ └─────────┘ │    │ └─────────┘ │    │ └─────────┘ │
└─────────────┘    └─────────────┘    └─────────────┘
```

## 💻 Code Analysis

### 1. Network Manager Structure

```go
// pkg/network.go:20-35
type NetworkManager struct {
    host      host.Host
    bitswap   exchange.Interface
    dht       *dht.IpfsDHT
    blockstore blockstore.Blockstore
    sessions  map[string]*BitswapSession
    ctx       context.Context
    cancel    context.CancelFunc
    mutex     sync.RWMutex
}

func NewNetworkManager(bs blockstore.Blockstore) (*NetworkManager, error) {
    ctx, cancel := context.WithCancel(context.Background())

    // Create libp2p host
    h, err := libp2p.New()
    if err != nil {
        cancel()
        return nil, fmt.Errorf("failed to create libp2p host: %w", err)
    }

    return &NetworkManager{
        host:       h,
        blockstore: bs,
        sessions:   make(map[string]*BitswapSession),
        ctx:        ctx,
        cancel:     cancel,
    }, nil
}
```

**Design Features**:
- Centralized network management
- Session-based block retrieval
- Context-based lifecycle management
- Thread-safe session tracking

### 2. Bitswap Integration

```go
// pkg/network.go:65-85
func (nm *NetworkManager) StartBitswap() error {
    // Initialize DHT for peer discovery
    dhtInstance, err := dht.New(nm.ctx, nm.host)
    if err != nil {
        return fmt.Errorf("failed to create DHT: %w", err)
    }
    nm.dht = dhtInstance

    // Bootstrap DHT
    if err := nm.dht.Bootstrap(nm.ctx); err != nil {
        return fmt.Errorf("failed to bootstrap DHT: %w", err)
    }

    // Create Bitswap network adapter
    network := bsnet.NewFromIpfsHost(nm.host, nm.dht)

    // Initialize Bitswap exchange
    nm.bitswap = bitswap.New(nm.ctx, network, nm.blockstore)

    fmt.Printf("Bitswap started on peer: %s\n", nm.host.ID().Pretty())
    return nil
}
```

**Integration Process**:
1. **DHT Initialization**: Enable peer and content discovery
2. **DHT Bootstrap**: Connect to known peers in network
3. **Network Adapter**: Bridge libp2p and Bitswap protocols
4. **Bitswap Creation**: Initialize block exchange engine

### 3. Block Request Implementation

```go
// pkg/network.go:120-150
func (nm *NetworkManager) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
    // Check local blockstore first
    if has, err := nm.blockstore.Has(ctx, c); err == nil && has {
        return nm.blockstore.Get(ctx, c)
    }

    // Create or get existing session
    sessionID := "default"
    session := nm.getOrCreateSession(sessionID)

    // Request block through Bitswap
    block, err := session.GetBlock(ctx, c)
    if err != nil {
        return nil, fmt.Errorf("failed to get block %s: %w", c, err)
    }

    // Store in local blockstore
    if err := nm.blockstore.Put(ctx, block); err != nil {
        fmt.Printf("Warning: failed to store block locally: %v\n", err)
    }

    return block, nil
}

func (nm *NetworkManager) GetBlocks(ctx context.Context, cids []cid.Cid) (<-chan blocks.Block, error) {
    sessionID := "batch-" + uuid.New().String()
    session := nm.getOrCreateSession(sessionID)

    blockCh := make(chan blocks.Block, len(cids))

    go func() {
        defer close(blockCh)
        defer nm.closeSession(sessionID)

        for block := range session.GetBlocks(ctx, cids) {
            // Store locally
            nm.blockstore.Put(ctx, block)
            blockCh <- block
        }
    }()

    return blockCh, nil
}
```

**Request Strategy**:
1. **Local Check**: Always check local storage first
2. **Session Management**: Use sessions for efficient batch requests
3. **Network Request**: Leverage Bitswap for missing blocks
4. **Local Caching**: Store retrieved blocks for future use

### 4. Want-list Management

```go
// pkg/session.go:25-50
type BitswapSession struct {
    exchange   exchange.SessionExchange
    wantlist   *Wantlist
    peers      map[peer.ID]*PeerContext
    strategy   RequestStrategy
    mutex      sync.RWMutex
}

type Wantlist struct {
    wants  map[cid.Cid]*WantEntry
    mutex  sync.RWMutex
}

type WantEntry struct {
    Cid      cid.Cid
    Priority int32
    WantType pb.Message_Wantlist_WantType
    SendDontHave bool
    Created  time.Time
}

func (w *Wantlist) Add(c cid.Cid, priority int32, wantType pb.Message_Wantlist_WantType) {
    w.mutex.Lock()
    defer w.mutex.Unlock()

    w.wants[c] = &WantEntry{
        Cid:      c,
        Priority: priority,
        WantType: wantType,
        SendDontHave: true,
        Created:  time.Now(),
    }
}

func (w *Wantlist) Remove(c cid.Cid) {
    w.mutex.Lock()
    defer w.mutex.Unlock()
    delete(w.wants, c)
}
```

**Want-list Features**:
- **Priority System**: Higher priority requests processed first
- **Want Types**: Block vs Have (existence check)
- **Timeout Handling**: Remove stale requests
- **Thread Safety**: Concurrent access protection

## 🏃‍♂️ Practice Guide

### 1. Basic Network Setup

```bash
cd 02-network
go run main.go
```

**Expected Output**:
```
=== IPFS P2P Network Demo ===

1. Initializing Network Manager:
   ✅ LibP2P host created: 12D3KooWBhvWaF...
   ✅ DHT initialized and bootstrapped
   ✅ Bitswap exchange started
   🌐 Connected to 0 peers initially

2. Connecting to Bootstrap Peers:
   🔄 Attempting to connect to bootstrap peers...
   ✅ Connected to peer: 12D3KooWLRhRdq...
   ✅ Connected to peer: 12D3KooWMySNaP...
   🌐 Total connected peers: 2

3. Testing Block Exchange:
   📤 Publishing test blocks to network...
   ✅ Block published: bafkreigh2akiscaid6...
   📥 Requesting blocks from network...
   ✅ Block retrieved: bafkreigh2akiscaid6...
   🔍 Block validation: PASSED
```

### 2. Two-Peer Communication Test

```bash
# Terminal 1: Start first peer
PEER_PORT=4001 go run main.go

# Terminal 2: Start second peer and connect
PEER_PORT=4002 CONNECT_TO=/ip4/127.0.0.1/tcp/4001/p2p/12D3KooW... go run main.go
```

**Observe**:
- Peer discovery and connection establishment
- Bitswap want-list propagation
- Block request and response cycles
- DHT content provider announcements

### 3. Network Performance Testing

```bash
# Benchmark block exchange performance
BENCHMARK=true BLOCK_COUNT=1000 go run main.go
```

### 4. Running Tests

```bash
go test -v ./...
```

**Test Coverage**:
- ✅ Network initialization and cleanup
- ✅ Peer discovery and connection
- ✅ Block publishing and retrieval
- ✅ Want-list management
- ✅ Session lifecycle management

## 🚀 Advanced Features

### 1. Custom Request Strategy

```go
type PriorityStrategy struct {
    highPriority map[cid.Cid]bool
    peers        map[peer.ID]*PeerReputation
}

type PeerReputation struct {
    ResponseTime    time.Duration
    SuccessRate     float64
    LastInteraction time.Time
}

func (ps *PriorityStrategy) SelectPeers(c cid.Cid, available []peer.ID) []peer.ID {
    // Sort peers by reputation
    sort.Slice(available, func(i, j int) bool {
        repI := ps.peers[available[i]]
        repJ := ps.peers[available[j]]

        if repI.SuccessRate != repJ.SuccessRate {
            return repI.SuccessRate > repJ.SuccessRate
        }

        return repI.ResponseTime < repJ.ResponseTime
    })

    // Return top 3 peers
    if len(available) > 3 {
        available = available[:3]
    }

    return available
}
```

### 2. Content Routing Optimization

```go
func (nm *NetworkManager) AnnounceContent(ctx context.Context, c cid.Cid) error {
    // Announce to DHT that we have this content
    if err := nm.dht.Provide(ctx, c, true); err != nil {
        return fmt.Errorf("failed to announce content: %w", err)
    }

    // Also announce to connected Bitswap peers
    nm.bitswap.NotifyNewBlocks(ctx, blocks.NewBlock([]byte{}, c))

    return nil
}

func (nm *NetworkManager) FindProviders(ctx context.Context, c cid.Cid, maxProviders int) ([]peer.AddrInfo, error) {
    // Query DHT for content providers
    providers := nm.dht.FindProviders(ctx, c)

    var result []peer.AddrInfo
    for provider := range providers {
        result = append(result, provider)
        if len(result) >= maxProviders {
            break
        }
    }

    return result, nil
}
```

### 3. Network Monitoring

```go
type NetworkStats struct {
    ConnectedPeers    int           `json:"connected_peers"`
    BlocksRequested   int64         `json:"blocks_requested"`
    BlocksProvided    int64         `json:"blocks_provided"`
    AverageLatency    time.Duration `json:"average_latency"`
    WantlistSize      int           `json:"wantlist_size"`
    ActiveSessions    int           `json:"active_sessions"`
}

func (nm *NetworkManager) GetNetworkStats() *NetworkStats {
    nm.mutex.RLock()
    defer nm.mutex.RUnlock()

    return &NetworkStats{
        ConnectedPeers:  len(nm.host.Network().Peers()),
        ActiveSessions:  len(nm.sessions),
        WantlistSize:    nm.getCurrentWantlistSize(),
    }
}

func (nm *NetworkManager) StartMonitoring(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            stats := nm.GetNetworkStats()
            fmt.Printf("Network Stats: %+v\n", stats)

        case <-nm.ctx.Done():
            return
        }
    }
}
```

## ⚠️ Best Practices and Considerations

### 1. Resource Management

```go
// ✅ Always clean up network resources
func (nm *NetworkManager) Close() error {
    nm.cancel() // Cancel context

    // Close Bitswap
    if nm.bitswap != nil {
        nm.bitswap.Close()
    }

    // Close DHT
    if nm.dht != nil {
        nm.dht.Close()
    }

    // Close libp2p host
    if nm.host != nil {
        nm.host.Close()
    }

    return nil
}
```

### 2. Error Handling and Retries

```go
// ✅ Implement exponential backoff for network requests
func (nm *NetworkManager) GetBlockWithRetry(ctx context.Context, c cid.Cid, maxRetries int) (blocks.Block, error) {
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        block, err := nm.GetBlock(ctx, c)
        if err == nil {
            return block, nil
        }

        lastErr = err

        // Exponential backoff
        delay := time.Duration(attempt*attempt) * time.Second
        select {
        case <-time.After(delay):
            continue
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
```

### 3. Security Considerations

```go
// ✅ Validate blocks before accepting
func (nm *NetworkManager) validateBlock(block blocks.Block) error {
    // Verify CID matches content
    expectedCID, err := block.Cid().Prefix().Sum(block.RawData())
    if err != nil {
        return fmt.Errorf("failed to calculate CID: %w", err)
    }

    if !expectedCID.Equals(block.Cid()) {
        return fmt.Errorf("block CID mismatch: expected %s, got %s",
            expectedCID, block.Cid())
    }

    // Check block size limits
    if len(block.RawData()) > MaxBlockSize {
        return fmt.Errorf("block too large: %d bytes > %d",
            len(block.RawData()), MaxBlockSize)
    }

    return nil
}
```

## 🔧 Troubleshooting

### Problem 1: No Peers Connected

**Cause**: Bootstrap peers unreachable or firewall issues
```go
// Solution: Add more bootstrap peers and check connectivity
bootstrapPeers := []string{
    "/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
    "/ip4/104.236.179.241/tcp/4001/p2p/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
}

for _, peerAddr := range bootstrapPeers {
    if err := nm.ConnectToPeer(ctx, peerAddr); err != nil {
        fmt.Printf("Failed to connect to %s: %v\n", peerAddr, err)
    }
}
```

### Problem 2: Blocks Not Found

**Cause**: Content not announced or providers offline
```go
// Solution: Implement provider search and fallback
func (nm *NetworkManager) GetBlockWithFallback(ctx context.Context, c cid.Cid) (blocks.Block, error) {
    // Try local first
    if block, err := nm.blockstore.Get(ctx, c); err == nil {
        return block, nil
    }

    // Try Bitswap
    if block, err := nm.GetBlock(ctx, c); err == nil {
        return block, nil
    }

    // Search for providers
    providers, err := nm.FindProviders(ctx, c, 10)
    if err != nil || len(providers) == 0 {
        return nil, fmt.Errorf("no providers found for %s", c)
    }

    // Try connecting to providers
    for _, provider := range providers {
        if err := nm.host.Connect(ctx, provider); err != nil {
            continue
        }

        if block, err := nm.GetBlock(ctx, c); err == nil {
            return block, nil
        }
    }

    return nil, fmt.Errorf("failed to retrieve block from any provider")
}
```

### Problem 3: High Latency

**Cause**: Poor peer selection or network congestion
```go
// Solution: Implement peer scoring and selection
type PeerScore struct {
    Latency     time.Duration
    Reliability float64
    Bandwidth   int64
}

func (nm *NetworkManager) selectBestPeers(candidates []peer.ID) []peer.ID {
    scores := make(map[peer.ID]*PeerScore)

    for _, peerID := range candidates {
        scores[peerID] = nm.calculatePeerScore(peerID)
    }

    // Sort by score
    sort.Slice(candidates, func(i, j int) bool {
        scoreI := scores[candidates[i]]
        scoreJ := scores[candidates[j]]
        return scoreI.Reliability/float64(scoreI.Latency) >
               scoreJ.Reliability/float64(scoreJ.Latency)
    })

    // Return top 5 peers
    if len(candidates) > 5 {
        candidates = candidates[:5]
    }

    return candidates
}
```

## 📚 Next Steps

### Related Modules
1. **03-dag-ipld**: Learn how blocks connect to form complex structures
2. **04-unixfs**: Understand file system abstraction over the network
3. **05-pin-gc**: Learn data lifecycle management in networked environment

### Advanced Topics
- Custom Bitswap strategies
- Network topology optimization
- Content routing algorithms
- DHT performance tuning

## 🎓 Practice Exercises

### Basic Exercises
1. Create a simple file sharing application using Bitswap
2. Implement a custom want-list prioritization algorithm
3. Build a network monitor that displays real-time peer connections

### Advanced Exercises
1. Design a content distribution network using IPFS
2. Implement a reputation system for peer selection
3. Create a caching layer that optimizes block retrieval patterns

You now understand how IPFS uses P2P networking and Bitswap for efficient block exchange. The next modules will show you how to structure and organize this networked data! 🚀