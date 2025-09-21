# Performance Benchmarks: Boxo vs go-ipfs

This directory contains comprehensive performance benchmarks comparing the boxo-starter-kit implementations with the standard go-ipfs implementation.

## 🎯 Benchmark Goals

1. **Block Operations**: CID creation, block storage/retrieval performance
2. **Data Store Performance**: Compare different backends (memory, badger, pebble) vs go-ipfs defaults
3. **Network Performance**: Bitswap protocol efficiency, peer discovery times
4. **Gateway Performance**: HTTP request handling, content delivery speed
5. **Memory Usage**: Memory consumption patterns under load
6. **Concurrent Operations**: Performance under high concurrency

## 📊 Benchmark Categories

### Core Components
- **block_bench_test.go**: Block creation, validation, and CID operations
- **datastore_bench_test.go**: Storage backend performance comparison
- **bitswap_bench_test.go**: Block exchange protocol efficiency
- **gateway_bench_test.go**: HTTP gateway response times

### Integration Tests
- **memory_bench_test.go**: Memory usage profiling
- **concurrent_bench_test.go**: High concurrency scenarios
- **real_world_bench_test.go**: Real-world usage patterns

## 🚀 Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./benchmarks/...

# Run specific benchmark category
go test -bench=BenchmarkBlock -benchmem ./benchmarks/

# Generate CPU/memory profiles
go test -bench=BenchmarkGateway -cpuprofile=cpu.prof -memprofile=mem.prof

# Compare with baseline (requires benchstat)
go test -bench=. -count=5 > new.txt
benchstat baseline.txt new.txt
```

## 📈 Benchmark Results

Results are automatically generated and updated in the `results/` directory:
- `block_results.md`: Block operation comparisons
- `datastore_results.md`: Storage backend performance
- `gateway_results.md`: HTTP gateway benchmarks
- `memory_results.md`: Memory usage analysis

## 🔧 Configuration

Benchmark parameters can be configured in `config.go`:
- Test data sizes
- Concurrency levels
- Benchmark duration
- Memory limits

## 📝 Contributing

When adding new benchmarks:
1. Follow the naming convention: `BenchmarkComponent_Operation`
2. Include both timing and memory measurements
3. Test with realistic data sizes
4. Document expected performance characteristics
5. Add baseline comparisons where applicable