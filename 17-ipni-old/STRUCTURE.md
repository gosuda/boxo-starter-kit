# 17-ipni Module Structure

## 📁 File Organization

```
17-ipni/
├── main.go                 # Main entry point with demo
├── run.sh                  # Easy run script (dev/prod/demo modes)
├── Makefile               # Build and test automation
├── test.sh                # Test runner script
├── config.yaml            # Production configuration
├── DEPLOYMENT.md          # Production deployment guide
├── README.md              # User documentation
├── STRUCTURE.md           # This file
│
├── pkg/                   # Core implementation
│   ├── ipni.go           # Main IPNI coordinator
│   ├── provider.go       # Provider management
│   ├── planner.go        # Query planning
│   ├── subscriber.go     # Subscription handling
│   ├── pubsub.go         # PubSub implementation
│   ├── advertisement.go  # Advertisement chain
│   ├── gossip.go         # Gossip protocol
│   ├── query_engine.go   # Advanced queries
│   ├── search.go         # Full-text search
│   ├── cache.go          # Multi-level caching
│   ├── compression.go    # Index compression
│   ├── bloom.go          # Bloom filters
│   ├── security.go       # Security features
│   ├── trust.go          # Trust management
│   ├── spam.go           # Spam filtering
│   ├── verifier.go       # Signature verification
│   ├── monitoring.go     # Metrics and health checks
│   ├── types.go          # Type definitions
│   └── *_test.go         # Test files
│
└── examples/              # Example demos (archived)
    ├── demo_pubsub.go
    ├── demo_advertisement.go
    ├── demo_gossip.go
    ├── demo_query.go
    ├── demo_security.go
    └── demo_performance_optimization.go
```

## 🚀 Main Entry Points

### main.go
- Single executable with all features
- Command-line flags for configuration
- Built-in demo mode (`--demo`)
- Monitoring endpoints (metrics, health)

### run.sh
- Convenient wrapper script
- Modes: `--dev`, `--prod`, `--demo`, `--docker`
- Automatic dependency checking
- Color-coded output

### Makefile
- Standard targets: `build`, `run`, `test`, `clean`
- Special targets: `demo`, `dev`, `prod`
- Testing targets: `test`, `bench`, `coverage`
- Docker targets: `docker-build`, `docker-run`

## 📦 Package Structure

### Core Components (pkg/)
1. **Provider System** - Content provider indexing
2. **Query Engine** - Advanced search and filtering
3. **Synchronization** - PubSub, Advertisement Chain, Gossip
4. **Performance** - Caching, compression, bloom filters
5. **Security** - Signatures, trust, spam protection
6. **Monitoring** - Metrics, health checks, circuit breakers

### Examples (examples/)
- Archived individual demo files
- Can be referenced for specific feature examples
- Not needed for normal operation

## 🎯 Usage Patterns

### Quick Start
```bash
# Simple run
./ipni-node

# Demo mode
./ipni-node --demo

# Production mode
make prod
```

### Development
```bash
# Development mode with verbose output
./run.sh --dev

# Run tests
make test

# Watch mode (auto-rebuild)
make watch
```

### Production
```bash
# Deploy with Docker
docker-compose up -d

# Run with systemd
systemctl start ipni
```

## 🔧 Configuration

- **Command-line flags**: Quick overrides
- **config.yaml**: Full production configuration
- **Environment variables**: Docker/K8s friendly

## 📊 Monitoring

- **Metrics**: http://localhost:9090/metrics (Prometheus)
- **Health**: http://localhost:8080/health (JSON)
- **Ready**: http://localhost:8080/ready (K8s probe)
- **Live**: http://localhost:8080/live (K8s probe)

## 🧪 Testing

- **Unit tests**: `pkg/*_test.go`
- **Integration tests**: `pkg/ipni_comprehensive_test.go`
- **Stress tests**: `pkg/stress_test.go`
- **Benchmarks**: `pkg/*_test.go` (Benchmark* functions)