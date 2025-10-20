# 17-ipni: Simple Provider/Subscriber Pattern

**Educational IPFS content indexing made easy - understand in 10 minutes!**

> ⚠️ **Note**: This module has been completely rewritten (2025-10-19) to focus on educational clarity.
> The previous complex implementation (~2,600 lines) has been replaced with this simplified version (~750 lines).
> The old version is preserved in `17-ipni-old/` for reference.

## 🎯 What is this?

This module demonstrates the **Provider/Subscriber pattern** used in IPFS for content discovery:

- **Providers**: "I have this content! Come get it from me"
- **Index**: "Here's who has what content" (like a phonebook)
- **Subscribers**: "Who has the content I need?"

Think of it as a **phonebook for IPFS content** - instead of mapping names to phone numbers, it maps Content IDs (CIDs) to providers.

## 🌟 Why This Simplified Version?

### Previous Version (now in 17-ipni-old/)
- **2,600+ lines** of complex code
- 8 different files with heavy abstractions
- Takes hours to understand
- Difficult for beginners

### Current Version (Simplified)
- **~750 lines** of clear, focused code
- 4 simple files
- **Understand in 10 minutes**
- Perfect for learning the core concepts

## 📚 Core Concepts

### 1. Provider
A peer that **HAS** content and announces it:

```go
provider := indexer.NewSimpleProvider(host, index)

// Announce: "I have this CID via these protocols"
provider.Announce(ctx, myCID, []Protocol{
    ProtocolBitswap,
    ProtocolHTTP,
})
```

### 2. Index
The "phonebook" that **STORES** the mappings:

```go
index := indexer.NewSimpleIndex()

// Internally maintains:
// CID -> [Provider1, Provider2, ...]
// ProviderID -> [CID1, CID2, ...]
```

### 3. Subscriber
A peer that **NEEDS** content and queries for it:

```go
subscriber := indexer.NewSimpleSubscriber(host, index)

// Find who has this content
providers, _ := subscriber.FindProviders(ctx, targetCID)

// Find providers with specific protocol
httpProviders, _ := subscriber.FindProvidersByProtocol(ctx, targetCID, ProtocolHTTP)

// Get the "best" provider (most protocols, most recent)
best, _ := subscriber.GetBestProvider(ctx, targetCID)
```

## 🚀 Quick Start

### Run the Demo

```bash
cd 17-ipni
go run main.go
```

### What the Demo Shows

1. **Creating the Index** - the central phonebook
2. **Creating Providers** - peers who have content
3. **Announcements** - providers announce their content
4. **Finding Providers** - subscribers query for content
5. **Protocol Filtering** - find providers by specific protocol
6. **Best Provider Selection** - smart provider ranking
7. **TTL Expiration** - automatic cleanup of old records

### Example Output

```
📢 Step 3: Providers Announce Content
   ✅ Provider 1 announced: bafkreifzjut3te2nhye...
      Protocols: bitswap, http
   ✅ Provider 2 announced: bafkreifzjut3te2nhye...
      Protocols: graphsync, http

❓ Step 6: Finding Providers for Content
   Query: Who has bafkreifzjut3te2nhye...?
   Answer: 2 providers found
      1. Provider 12D3KooWBhHAYo7c7N7B...
         Protocols: [bitswap http]
      2. Provider 12D3KooWEikbNqiKV8X9...
         Protocols: [graphsync http]
```

## 📖 API Reference

### SimpleProvider

```go
// Create a provider
provider := NewSimpleProvider(host, index)

// Announce content
err := provider.Announce(ctx, cid, []Protocol{ProtocolBitswap})

// Remove announcement
err := provider.Remove(ctx, cid)

// List all content
cids := provider.ListContent()

// Check if has content
hasIt := provider.HasContent(cid)

// Set TTL for announcements
provider.SetTTL(24 * time.Hour)

// Get statistics
stats := provider.Stats()
```

### SimpleIndex

```go
// Create index
index := NewSimpleIndex()

// Query for providers
providers, err := index.Query(cid)

// List all content
allCIDs := index.ListContent()

// List all providers
allProviders := index.ListProviders()

// Get provider's content
content, err := index.GetProviderContent(providerID)

// Remove provider
err := index.RemoveProvider(providerID)

// Cleanup expired records
removed := index.CleanupExpired()

// Get statistics
stats := index.Stats()
```

### SimpleSubscriber

```go
// Create subscriber
subscriber := NewSimpleSubscriber(host, index)

// Find all providers for content
providers, err := subscriber.FindProviders(ctx, cid)

// Find providers by protocol
httpProviders, err := subscriber.FindProvidersByProtocol(ctx, cid, ProtocolHTTP)

// Get best provider
best, err := subscriber.GetBestProvider(ctx, cid)

// Subscribe to updates (streaming)
ch, err := subscriber.Subscribe(ctx, cid)

// Cache management
subscriber.ClearCache()
subscriber.ClearCacheFor(cid)

// Get statistics
stats := subscriber.Stats()
```

## 🏗️ Architecture

```
┌─────────────┐         ┌─────────────┐         ┌──────────────┐
│  Provider 1 │────────▶│             │◀────────│ Subscriber A │
│             │ Announce│             │  Query  │              │
│ "I have X"  │         │   Index     │         │ "Who has X?" │
└─────────────┘         │             │         └──────────────┘
                        │  (Phonebook)│
┌─────────────┐         │             │         ┌──────────────┐
│  Provider 2 │────────▶│  CID -> []  │◀────────│ Subscriber B │
│             │ Announce│  Providers  │  Query  │              │
│ "I have Y"  │         │             │         │ "Who has Y?" │
└─────────────┘         └─────────────┘         └──────────────┘
```

## 🔄 Typical Workflow

### Provider Side

```go
// 1. Create provider
provider := indexer.NewSimpleProvider(host, index)

// 2. When you add content to your node
localStore.Add(myData)
myCID := myData.Cid()

// 3. Announce it
provider.Announce(ctx, myCID, []Protocol{
    ProtocolBitswap,
    ProtocolHTTP,
})

// 4. Optional: remove when deleted
localStore.Delete(myCID)
provider.Remove(ctx, myCID)
```

### Subscriber Side

```go
// 1. Create subscriber
subscriber := indexer.NewSimpleSubscriber(host, index)

// 2. When you need content
targetCID := cid.Parse("bafybeiabc123...")

// 3. Find providers
providers, err := subscriber.FindProviders(ctx, targetCID)

// 4. Try to fetch from best provider
best, _ := subscriber.GetBestProvider(ctx, targetCID)
data, err := fetchFrom(best.ProviderID, best.Addresses, targetCID)
```

## 📊 Key Features

### ✅ Multi-Protocol Support
- Bitswap
- HTTP
- GraphSync
- Easy to add more

### ✅ Smart Provider Selection
- Ranks by number of protocols
- Prefers more recent announcements
- Filters expired records

### ✅ Automatic Cleanup
- TTL-based expiration
- Background cleanup goroutine
- Configurable cleanup interval

### ✅ Thread-Safe
- All operations use proper locking
- Safe for concurrent use
- No data races

### ✅ Efficient Indexing
- Bidirectional mapping (CID ↔ Provider)
- Fast lookups
- Memory efficient

## 🔍 Real-World Use Cases

### 1. Content Discovery
```go
// User wants to download a file
cid := userRequestedCID
providers, _ := subscriber.FindProviders(ctx, cid)

if len(providers) == 0 {
    return errors.New("content not available")
}

// Try each provider until success
for _, p := range providers {
    if data, err := fetchFrom(p); err == nil {
        return data
    }
}
```

### 2. Protocol Preference
```go
// Prefer HTTP for browser compatibility
httpProviders, _ := subscriber.FindProvidersByProtocol(ctx, cid, ProtocolHTTP)

if len(httpProviders) > 0 {
    return fetchViaHTTP(httpProviders[0])
}

// Fallback to Bitswap
bitswapProviders, _ := subscriber.FindProvidersByProtocol(ctx, cid, ProtocolBitswap)
return fetchViaBitswap(bitswapProviders[0])
```

### 3. Load Balancing
```go
// Get all providers
providers, _ := subscriber.FindProviders(ctx, cid)

// Round-robin selection
selected := providers[requestCount % len(providers)]

// Fetch from selected provider
return fetchFrom(selected)
```

## 🆚 Comparison: New vs Old Implementation

| Feature | 17-ipni (Current) | 17-ipni-old (Previous) |
|---------|-------------------|------------------------|
| **Lines of Code** | ~750 | 2,600+ |
| **Files** | 4 | 8 |
| **Learning Time** | 10 minutes | Hours |
| **Use Case** | Education | Production |
| **Complexity** | Minimal | High |
| **Features** | Core concepts | Full-featured |
| **Dependencies** | Minimal | Many |

## 🎓 Learning Path

1. **Start Here**: Run `main.go` and observe output
2. **Read Code**: `types.go` → `index.go` → `provider.go` → `subscriber.go`
3. **Experiment**: Modify the demo to add more providers
4. **Build**: Create your own app using these primitives
5. **Graduate**: Check out `17-ipni-old/` for production-grade features

## 💡 Design Decisions

### Why In-Memory Only?
- **Educational focus**: Easier to understand
- **No external dependencies**: Just Go standard library + libp2p
- **Fast iteration**: No database setup needed

For production, add persistence layer:
```go
type PersistentIndex struct {
    *SimpleIndex
    db Database
}
```

### Why No PubSub?
- **Simplicity**: Direct index access is easier to understand
- **Local first**: Perfect for single-node or small cluster setups

For distributed systems, add gossip layer:
```go
type GossipIndex struct {
    *SimpleIndex
    pubsub *pubsub.PubSub
}
```

### Why Separate Provider/Subscriber?
- **Clear roles**: Easier to understand who does what
- **Flexibility**: A node can be provider, subscriber, or both
- **Testing**: Can test each component independently

## 🧪 Testing

```bash
# Run the demo
go run main.go

# Build
go build -o simple-indexer

# Run tests (when added)
go test ./pkg/...

# Benchmarks (when added)
go test -bench=. ./pkg/...
```

## 📝 Common Patterns

### Announce Multiple CIDs at Once
```go
for _, c := range myCIDs {
    provider.Announce(ctx, c, myProtocols)
}
```

### Subscribe to Content Updates
```go
ch, _ := subscriber.Subscribe(ctx, targetCID)
for record := range ch {
    fmt.Printf("New provider: %s\n", record.ProviderID)
}
```

### Periodic Re-announcements
```go
ticker := time.NewTicker(12 * time.Hour)
for range ticker.C {
    for _, c := range provider.ListContent() {
        provider.Announce(ctx, c, myProtocols)
    }
}
```

## 🔗 Related Modules

- **17-ipni-old/**: Full-featured IPNI implementation (production-ready, previous version)
- **02-network**: libp2p networking basics
- **04-bitswap**: Bitswap protocol for content exchange
- **18-multifetcher**: Protocol fallback and racing

## 📚 Further Reading

- [IPNI Specification](https://github.com/ipni/specs)
- [libp2p Documentation](https://docs.libp2p.io/)
- [IPFS Content Routing](https://docs.ipfs.tech/concepts/content-routing/)

## 🤝 Contributing

This is an educational module - simplicity is the goal!

Before adding features, ask:
1. Does this make it **easier** to understand?
2. Does this keep it under **600 lines**?
3. Does this teach a **core concept**?

If yes to all three, submit a PR!

---

**Made with ❤️ for IPFS learners everywhere**

*Part of the boxo-starter-kit educational series*
