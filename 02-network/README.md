# 02-network: P2P Networking using libp2p

This module demonstrates the simplest possible P2P communication using libp2p:
- A single custom protocol
- Length-prefixed payload transmission
- Receiving side computes a CID and delivers the message only when the requested CID arrives

This is a pure networking demo. Bitswap, DHT, wantlists, sessions, and advanced strategies are not included.

## 🎯 Learning Objectives

Through this module, you will learn:
- **P2P networking** Create and configure a libp2p host
- **Peer connections** Establish connections between peers using multiaddresses
- **Custom protocols** Implement a simple protocol for data exchange
- **Message transmission** Send and receive length-prefixed messages

## 📋 Prerequisites

- **00-block-cid** module completion (Block and CID understanding)
- **01-persistent** module completion (Data persistence concepts)
- Basic understanding of networking concepts (TCP/IP, P2P)
- Knowledge of Go concurrency patterns (goroutines, channels)

## 🔑 Key Concepts

### What is libp2p?

**libp2p** is a modular peer-to-peer networking stack. It provides the building blocks for peers to discover each other, establish encrypted connections, and exchange messages over custom protocols:

```
Traditional Client-Server:
Client → Server: "Give me file.txt"
Server → Client: [entire file]

libp2p Peer-to-Peer:
Peer A → Peer B: "Open a stream using /custom/proto/1.0.0"
Peer B → Peer A: [response data]
Peer A ↔ Peer C: [parallel stream using the same or another protocol]
```

### Key Components

1. **Peer Identity**: Each peer has a cryptographic ID (peer.ID) derived from its public key.
2. **Multiaddress**: Unified addressing format (/ip4/127.0.0.1/tcp/4001/p2p/Qm...) that encodes transport + peer ID.
3. **Transport**: Underlying protocol (TCP, WebRTC, etc.) used for connections.
4. **Streaming Multiplexing**: Bidirectional channel over a connection for protocol-specific communication.

### Network Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│    Peer A   │    │    Peer B   │    │    Peer C   │
│ ┌─────────┐ │    │ ┌─────────┐ │    │ ┌─────────┐ │
│ │ Protocol│←┼────┼→│ Protocol│←┼────┼→│ Protocol│ │
│ │ Handler │ │    │ │ Handler │ │    │ │ Handler │ │
│ └─────────┘ │    │ └─────────┘ │    │ └─────────┘ │
│ ┌─────────┐ │    │ ┌─────────┐ │    │ ┌─────────┐ │
│ │Transport│←┼────┼→│Transport│←┼────┼→│Transport│ │
│ └─────────┘ │    │ └─────────┘ │    │ └─────────┘ │
└─────────────┘    └─────────────┘    └─────────────┘
```

## 🏃‍♂️ Practice Guide

### 1. Basic Network Setup

```bash
cd 02-network
go run main.go
```

**Expected Output**:
```
=== IPFS P2P Network Demo ===

1. Creating libp2p nodes:
   ✅ Node 1 created: 12D3KooWP6uX...
   ✅ Node 2 created: 12D3KooWDhod...

2. Connecting nodes:
   ✅ Node 1 connected to Node 2

3. Storing content in Node 1:
   ✅ Content stored...

4. Retrieving content from Node 2:
   ✅ Content retrieved: Hello, libp2p World! This is a test message for P2P block exchange.

5. Multiple block exchange test:
   ✅ Stored block 1: bafkreigb7tfrfwhxfyd...
   ✅ Stored block 2: bafkreicnzymjdhro2wc...
   ✅ Stored block 3: bafkreig6ruxlrdbe23s...

   Attempting to retrieve blocks from Node 2:
   ✅ Retrieved block 1: First message for libp2p exchange
   ✅ Retrieved block 2: Second message with different content
   ✅ Retrieved block 3: Third message to test multiple blocks
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
- ✅ Session lifecycle management

## ⚠️ Best Practices and Considerations

### 1. Resource Management

```go
// ✅ Always clean up network resources
func (n *HostWrapper) Close() error {
    // Signal the dispatcher to exit.
    // Safe to close multiple times thanks to the nil-check and host.Close semantics.
    close(n.done)

    if n.Host != nil {
        return n.Host.Close()
    }
    return nil
}
```

### 2. Error Handling and Retries

```go
func backoff(attempt int) time.Duration {
    return time.Duration(attempt*attempt) * time.Second
}

// ✅ Retry Send with exponential backoff
func SendWithRetry(ctx context.Context, n *HostWrapper, to peer.ID, payload []byte, maxRetries int) (cid.Cid, error) {
    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        c, err := n.Send(ctx, to, payload)
        if err == nil {
            return c, nil
        }
        lastErr = err

        // Wait unless context is already done
        select {
        case <-time.After(backoff(attempt)):
        case <-ctx.Done():
            return cid.Undef, ctx.Err()
        }
    }
    return cid.Undef, fmt.Errorf("send failed after %d attempts: %w", maxRetries, lastErr)
}

// ✅ Retry Receive with exponential backoff on timeout
func ReceiveWithRetry(ctx context.Context, n *HostWrapper, want cid.Cid, maxRetries int) (peer.ID, []byte, error) {
    var lastErr error
    for attempt := 0; attempt < maxRetries; attempt++ {
        from, data, err := n.Receive(ctx, want)
        if err == nil {
            return from, data, nil
        }
        lastErr = err

        // If the context was canceled by caller, bubble up immediately.
        if ctx.Err() != nil {
            return "", nil, ctx.Err()
        }
        // Small pause before re-waiting; upstream sender may not have pushed yet.
        select {
        case <-time.After(backoff(attempt)):
        case <-ctx.Done():
            return "", nil, ctx.Err()
        }
    }
    return "", nil, fmt.Errorf("receive failed after %d attempts: %w", maxRetries, lastErr)
}
```

### 3. Security Considerations

```go
func ValidatePayloadAgainstCID(expected cid.Cid, payload []byte) error {
    // Recompute using your canonical prefix; this must match ComputeCID used in Send/handle.
    got, err := block.ComputeCID(payload, nil)
    if err != nil {
        return fmt.Errorf("compute cid: %w", err)
    }
    if got != expected {
        return fmt.Errorf("cid mismatch: expected %s, got %s", expected, got)
    }
    return nil
}

// Optional: guard against self-dial at call sites.
func AvoidSelfDial(self, target peer.ID) error {
    if self == target {
        return fmt.Errorf("dial to self attempted")
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
1. **03-dht-router**: Implement a distributed hash table for peer and content routing
2. **04-bitswap**: Dive deeper into the Bitswap protocol for block exchange

### Advanced Topics
- Network topology optimization
- Content routing algorithms
- DHT performance tuning

## 🎓 Practice Exercises

### Basic Exercises
1. Create a simple file sharing application using libp2p
2. Implement a custom want-list prioritization algorithm
3. Build a network monitor that displays real-time peer connections

### Advanced Exercises
1. Design a content distribution network using libp2p
2. Implement a reputation system for peer selection
3. Create a caching layer that optimizes block retrieval patterns

You now understand how libp2p uses P2P networking for efficient block exchange. The next modules will show you how to structure and organize this networked data! 🚀