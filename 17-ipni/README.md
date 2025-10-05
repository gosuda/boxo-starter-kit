# 17-ipni: Simple IPNI (InterPlanetary Network Indexer) Demo

## 🎯 Overview

This is a simplified IPNI (InterPlanetary Network Indexer) demonstration that shows the basic concepts of content indexing and discovery without complex dependencies.

## 🚀 Quick Start

### Build and Run

```bash
# Build the binary
go build -o ipni-node main.go

# Run with default settings
./ipni-node

# Run with demo mode
./ipni-node --demo

# Run with custom settings
./ipni-node --data ./my-data --topic my-topic --storage file --demo
```

### Using the run script

```bash
# Development mode
./run.sh --dev

# Demo mode
./run.sh --demo

# Production mode (requires permissions)
./run.sh --prod
```

### Using Makefile

```bash
# Build
make build

# Run demo
make demo

# Run in development mode
make dev
```

## 📝 Command Line Options

- `--data`: Data directory for storage (default: `./ipni-data`)
- `--topic`: PubSub topic name (default: `ipni-demo`)
- `--storage`: Storage type - `memory` or `file` (default: `memory`)
- `--demo`: Run demo mode showing basic functionality

## 🎭 Demo Mode

When you run with `--demo`, the application will:

1. **Create sample content**: Generate sample data
2. **Compute CID**: Calculate Content Identifier
3. **Store in index**: Add to provider index
4. **Lookup providers**: Search for content providers
5. **Show statistics**: Display index stats

Example output:
```
=== Simple IPNI Demo ===
📁 Data directory: ./ipni-data
📢 PubSub topic: ipni-demo
💾 Storage type: memory
🚀 IPNI components initialized

=== Demo Mode ===
📝 Creating sample content...
📄 Sample data: Hello IPNI World!
🔍 Computing CID...
✅ Generated CID: bafybeigdyrzt5sfp7ud...
📊 Storing in provider index...
✅ Stored content with CID: bafybeigdyrzt5sfp7ud...
🔍 Looking up providers...
✅ Found 1 provider(s)
📊 Stats: 1 providers, 1 entries
✨ Demo complete!
✅ IPNI running. Press Ctrl+C to stop.
```

## 🏗️ Architecture

This modular implementation demonstrates:

- **Provider Management**: Content provider indexing with persistent storage
- **Content Discovery**: Real CID-based provider lookups
- **Subscriber System**: Content update subscription mechanism
- **Cryptographic Security**: Ed25519 signatures and trust management
- **Anti-Spam Protection**: Rate limiting and provider trust scoring
- **IPNI Coordinator**: Central management of all components
- **Modular Design**: Clean separation of concerns in pkg/ directory

## 🧪 Core Concepts

### IPNI (InterPlanetary Network Indexer)
- Distributed system for finding content providers
- Maps content identifiers (CIDs) to provider information
- Enables efficient content discovery across IPFS networks

### Provider Index
- Local database of content providers
- Maps multihashes to provider metadata
- Supports TTL-based expiration

### Content Identification
- Uses CIDs (Content Identifiers) for content addressing
- Based on cryptographic hashes of content
- Enables content-based lookup and verification

### Security Features
- **Ed25519 Signatures**: Cryptographic signing of provider announcements
- **Trust Scoring**: Provider reputation and reliability assessment
- **Rate Limiting**: Anti-spam protection with configurable limits
- **Signature Verification**: Ensuring announcement authenticity

## 📊 Monitoring

The application provides basic monitoring through:
- Console output with emoji indicators
- Configuration display on startup
- Statistics during demo mode
- Graceful shutdown on Ctrl+C

## 🔧 Development

### Project Structure
```
17-ipni/
├── main.go           # Main application entry point
├── go.mod            # Go module definition
├── pkg/              # IPNI module components
│   ├── ipni.go      # Main IPNI coordinator
│   ├── provider.go  # Provider management
│   ├── subscriber.go # Content subscription
│   ├── security.go  # Cryptographic security
│   └── types.go     # Type definitions
├── run.sh            # Convenience run script
├── Makefile          # Build automation
└── README.md         # This documentation
```

### Building
```bash
go build -o ipni-node main.go
```

### Running Tests
```bash
go test ./...
```

## 📚 Learning Path

This module teaches:

1. **Basic IPNI concepts**: Provider indexing and content discovery
2. **Command-line applications**: Flag parsing and signal handling
3. **Go development**: Project structure and build process
4. **Content addressing**: CID-based content identification

## 🔗 Next Steps

After understanding this basic implementation, explore:
- Real IPNI implementations in production systems
- Integration with IPFS networks
- Advanced provider discovery mechanisms
- Distributed indexing strategies

## 🎯 Key Takeaways

- IPNI enables efficient content discovery in distributed systems
- Provider indexes map content to availability information
- Content addressing provides cryptographic content verification
- Modular design allows for incremental complexity addition