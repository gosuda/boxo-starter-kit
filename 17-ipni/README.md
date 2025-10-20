# 17-ipni: IPFS Content Indexer

**Learn the Provider/Subscriber pattern in IPFS - understand in 10 minutes!**

## 🎯 What is IPNI?

IPNI (InterPlanetary Network Indexer) is a system that helps you find who has specific content in the IPFS network. Think of it as a **phonebook for IPFS content** - instead of mapping names to phone numbers, it maps Content IDs (CIDs) to providers.

### The Problem

In IPFS, content is addressed by its CID (Content Identifier), not by location. But how do you find **who** has that content?

```
You: "I need bafybeiabc123..."
Network: "Provider A has it via Bitswap and HTTP"
         "Provider B has it via GraphSync"
```

### The Solution: Provider/Subscriber Pattern

- **Providers**: Nodes that **HAVE** content and announce it
- **Index**: Central registry that **STORES** the mappings
- **Subscribers**: Nodes that **NEED** content and query for it

## 🚀 Quick Start

```bash
cd 17-ipni
go run main.go
```

You'll see an 8-step demo showing providers announcing content, subscribers finding it, protocol filtering, and TTL expiration.

## 📖 Core API

### Provider: Announce Content

```go
provider := indexer.NewSimpleProvider(host, index)
provider.Announce(ctx, cid, []indexer.Protocol{
    indexer.ProtocolBitswap,
    indexer.ProtocolHTTP,
})
```

### Subscriber: Find Providers

```go
subscriber := indexer.NewSimpleSubscriber(host, index)
providers, _ := subscriber.FindProviders(ctx, cid)
best, _ := subscriber.GetBestProvider(ctx, cid)
```

### Index: Query & Manage

```go
index := indexer.NewSimpleIndex()
providers, _ := index.Query(cid)
stats := index.Stats()
```

## 📚 Documentation

See full API documentation, use cases, and examples in the sections below.

---

## Table of Contents
- [Architecture](#architecture)
- [Components](#components)
- [API Reference](#api-reference)
- [Common Use Cases](#common-use-cases)
- [Learning Path](#learning-path)
- [Advanced Features](#advanced-features)
- [Related Modules](#related-modules)

---

## 🏗️ Architecture

```
┌─────────────┐         ┌─────────────┐         ┌──────────────┐
│  Provider 1 │────────▶│             │◀────────│ Subscriber A │
│ "I have X"  │ Announce│   Index     │  Query  │ "Who has X?" │
└─────────────┘         │ (Phonebook) │         └──────────────┘
                        │ CID→Provider│
┌─────────────┐         │             │         ┌──────────────┐
│  Provider 2 │────────▶│             │◀────────│ Subscriber B │
│ "I have Y"  │         │             │         │ "Who has Y?" │
└─────────────┘         └─────────────┘         └──────────────┘
```

## 📦 Components

**Provider** - Announces content availability  
**Index** - Stores CID-to-Provider mappings  
**Subscriber** - Queries for content providers

## 📖 API Reference

### SimpleProvider

```go
// Create
provider := indexer.NewSimpleProvider(host, index)

// Announce content
provider.Announce(ctx, cid, []Protocol{ProtocolBitswap, ProtocolHTTP})

// Remove
provider.Remove(ctx, cid)

// List & Check
provider.ListContent()          // []cid.Cid
provider.HasContent(cid)        // bool

// Configure
provider.SetTTL(24 * time.Hour)

// Stats
provider.Stats()  // ProviderStats{ProviderID, ContentCount, Protocols, TTL}
```

### SimpleIndex

```go
// Create
index := indexer.NewSimpleIndex()

// Query
index.Query(cid)                      // []*ProviderRecord
index.GetProviderContent(providerID)  // []cid.Cid

// List
index.ListContent()    // []cid.Cid
index.ListProviders()  // []peer.ID

// Manage
index.RemoveProvider(providerID)
index.CleanupExpired()  // int (removed count)

// Stats
index.Stats()  // IndexStats{TotalContent, TotalProviders, TotalRecords, ExpiredRecords}
```

### SimpleSubscriber

```go
// Create
subscriber := indexer.NewSimpleSubscriber(host, index)

// Find
subscriber.FindProviders(ctx, cid)
subscriber.FindProvidersByProtocol(ctx, cid, ProtocolHTTP)
subscriber.GetBestProvider(ctx, cid)  // Most protocols, most recent

// Subscribe (streaming)
ch, _ := subscriber.Subscribe(ctx, cid)
for record := range ch { ... }

// Cache
subscriber.ClearCache()
subscriber.ClearCacheFor(cid)

// Stats
subscriber.Stats()  // SubscriberStats{SubscriberID, CachedContent, TotalProviders}
```

### ProviderRecord

```go
type ProviderRecord struct {
    ProviderID peer.ID      // Who has it
    ContentID  cid.Cid      // What content
    Protocols  []Protocol   // How to get it
    Addresses  []string     // Where (multiaddrs)
    Timestamp  time.Time    // When announced
    TTL        time.Duration // Validity period
}

record.IsExpired()
record.SupportsProtocol(ProtocolHTTP)
```

## 💡 Common Use Cases

### 1. Content Discovery

```go
subscriber := indexer.NewSimpleSubscriber(host, index)
providers, _ := subscriber.FindProviders(ctx, targetCID)

for _, p := range providers {
    if data, err := fetchFrom(p); err == nil {
        return data
    }
}
```

### 2. Protocol Preference

```go
// Prefer HTTP, fallback to Bitswap
httpProviders, _ := subscriber.FindProvidersByProtocol(ctx, cid, ProtocolHTTP)
if len(httpProviders) > 0 {
    return fetchViaHTTP(httpProviders[0])
}

bitswapProviders, _ := subscriber.FindProvidersByProtocol(ctx, cid, ProtocolBitswap)
return fetchViaBitswap(bitswapProviders[0])
```

### 3. Load Balancing

```go
providers, _ := subscriber.FindProviders(ctx, cid)
selected := providers[requestCount % len(providers)]  // Round-robin
fetchFrom(selected)
```

## 🎓 Learning Path

1. **Run Demo** (5 min): `go run main.go`
2. **Read Code** (10 min): `types.go` → `index.go` → `provider.go` → `subscriber.go`
3. **Experiment** (15 min): Modify `main.go` - add providers, try protocols
4. **Build** (30 min): Create your own provider/subscriber app

## 🔧 Advanced Features

### TTL & Expiration

```go
provider.SetTTL(12 * time.Hour)
removed := index.CleanupExpired()  // Automatic cleanup every 10min
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

### Monitoring

```go
stats := index.Stats()
fmt.Printf("Content: %d, Providers: %d, Records: %d\n",
    stats.TotalContent, stats.TotalProviders, stats.TotalRecords)
```

## 🏃 Performance

- **Caching**: Subscribers cache query results
- **O(1) Lookups**: By CID or Provider ID
- **Thread-Safe**: All operations use proper locking
- **Auto-Cleanup**: Background goroutine removes expired records
- **Max Limit**: 20 providers per content (configurable)

## 🆚 vs Production Version

| Feature | 17-ipni (Current) | 17-ipni-old |
|---------|-------------------|-------------|
| Lines | ~985 | 2,642 |
| Files | 4 | 8 |
| Learning | 10 min | Hours |
| Persistence | In-memory | Configurable |
| PubSub | No | Yes |
| Use Case | Education | Production |

**When to use:** Learning & small projects → this version. Production → see `../17-ipni-old/`

## 🔗 Protocol Support

- `ProtocolBitswap` - Traditional IPFS block exchange
- `ProtocolHTTP` - Browser-compatible HTTP
- `ProtocolGraphSync` - Efficient graph sync

Easy to extend with custom protocols.

## 🔗 Related Modules

- [02-network](../02-network/) - libp2p basics
- [04-bitswap](../04-bitswap/) - Bitswap protocol
- [10-gateway](../10-gateway/) - HTTP Gateway
- [18-multifetcher](../18-multifetcher/) - Protocol fallback
- [17-ipni-old](../17-ipni-old/) - Production version

## 📚 Further Reading

- [IPNI Spec](https://github.com/ipni/specs)
- [libp2p Docs](https://docs.libp2p.io/)
- [IPFS Content Routing](https://docs.ipfs.tech/concepts/content-routing/)

---

**Code Stats**: 985 lines total | types(45) + index(329) + provider(183) + subscriber(172) + demo(226)

**Made with ❤️ for IPFS learners** | *Part of boxo-starter-kit*
