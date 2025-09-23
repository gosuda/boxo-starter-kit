# 01-persistent: Data Persistence and Storage Backend Selection

## 🎯 Learning Objectives

Through this module, you will learn:
- The importance of **data persistence** in IPFS and implementation methods
- Characteristics of **4 storage backends** and their suitable use cases
- Benefits of abstraction through **Datastore interface**
- **Performance vs Persistence** trade-off analysis
- **Storage selection criteria** for production environments

## 📋 Prerequisites

- **00-block-cid** module completion (Block and CID understanding)
- Basic understanding of database systems
- Knowledge of file I/O operations
- Basic concepts of key-value storage

## 🔑 Key Concepts

### Why Data Persistence?

The **00-block-cid** module used in-memory storage, which has limitations:

```
Memory Storage Issues:
❌ Data lost on program termination
❌ Limited by available RAM
❌ No data sharing between processes
❌ No fault tolerance
```

**Persistent storage** solves these problems:

```
Persistent Storage Benefits:
✅ Data survives program restarts
✅ Scalable storage capacity
✅ Process-independent data access
✅ Backup and recovery support
```

### Storage Backend Types

| Backend | Type | Use Case | Performance | Durability |
|---------|------|----------|-------------|------------|
| **Memory** | In-memory | Testing, cache | Fastest | Volatile |
| **File** | File-based | Simple deployment | Medium | Persistent |
| **Badger** | LSM-Tree | High write load | Fast | Persistent |
| **Pebble** | LSM-Tree | Large datasets | Very fast | Persistent |

## 💻 Code Analysis

### 1. Persistent Wrapper Implementation

```go
// pkg/persistent.go:19-30
type PersistentWrapper struct {
    blockWrapper *block.BlockWrapper
    persistentType string
    dataPath     string
}

func New(blockWrapper *block.BlockWrapper, persistentType, dataPath string) (*PersistentWrapper, error) {
    if persistentType == "" {
        persistentType = "memory"
    }

    return &PersistentWrapper{
        blockWrapper:   blockWrapper,
        persistentType: persistentType,
        dataPath:      dataPath,
    }, nil
}
```

**Design Features**:
- Wraps block functionality with persistence layer
- Default to memory storage if type not specified
- Configurable data path for persistent backends

### 2. Backend Factory Pattern

```go
// pkg/persistent.go:45-70
func (pw *PersistentWrapper) createDatastore() (datastore.Datastore, error) {
    switch pw.persistentType {
    case "memory":
        return datastore.NewMapDatastore(), nil

    case "file":
        if pw.dataPath == "" {
            pw.dataPath = "./data/leveldb"
        }
        return leveldb.NewDatastore(pw.dataPath, nil)

    case "badger":
        if pw.dataPath == "" {
            pw.dataPath = "./data/badger"
        }
        opts := badger.DefaultOptions(pw.dataPath)
        opts.Logger = nil  // Disable verbose logging
        db, err := badger.Open(opts)
        if err != nil {
            return nil, err
        }
        return badgerdatastore.Wrap(db), nil

    case "pebble":
        if pw.dataPath == "" {
            pw.dataPath = "./data/pebble"
        }
        db, err := pebble.Open(pw.dataPath, &pebble.Options{})
        if err != nil {
            return nil, err
        }
        return pebbledatastore.Wrap(db), nil

    default:
        return nil, fmt.Errorf("unsupported persistent type: %s", pw.persistentType)
    }
}
```

**Implementation Strategy**:
- Factory pattern for clean backend creation
- Default paths for each backend type
- Error handling for unsupported backends
- Configuration flexibility

## 🏃‍♂️ Practice Guide

### 1. Basic Execution

```bash
cd 01-persistent
go run main.go
```

**Expected Output**:
```
=== Persistent Storage Demo ===

1. Testing Memory Backend:
   ✅ Memory datastore initialized
   ✅ Stored 3 blocks successfully
   ✅ All blocks retrieved correctly
   📊 Performance: 1.2ms average

2. Testing File Backend (LevelDB):
   ✅ LevelDB datastore created at ./data/leveldb
   ✅ Stored 3 blocks successfully
   ✅ All blocks retrieved correctly
   📊 Performance: 15.3ms average
   💾 Persistent: data survives restart

3. Testing Badger Backend:
   ✅ Badger datastore created at ./data/badger
   ✅ Stored 3 blocks successfully
   ✅ All blocks retrieved correctly
   📊 Performance: 8.7ms average
   🗜️ Built-in compression enabled

4. Testing Pebble Backend:
   ✅ Pebble datastore created at ./data/pebble
   ✅ Stored 3 blocks successfully
   ✅ All blocks retrieved correctly
   📊 Performance: 6.2ms average
   ⚡ Optimized for high throughput
```

### 2. Backend Comparison Test

```bash
# Test with specific backend
BACKEND=badger go run main.go

# Test with custom data path
BACKEND=pebble DATA_PATH=./custom/path go run main.go
```

### 3. Persistence Verification

```bash
# First run - creates persistent data
BACKEND=file go run main.go

# Check data was created
ls -la ./data/leveldb/

# Second run - should load existing data
BACKEND=file go run main.go
```

### 4. Running Tests

```bash
go test -v ./...
```

**Test Coverage**:
- ✅ All backend creation and initialization
- ✅ Data storage and retrieval across backends
- ✅ Error handling for invalid configurations
- ✅ Resource cleanup and proper closing

## 🚀 Performance Characteristics

### Backend Performance Comparison

| Operation | Memory | File (LevelDB) | Badger | Pebble |
|-----------|---------|----------------|---------|---------|
| **Write Latency** | ~0.001ms | ~15ms | ~8ms | ~6ms |
| **Read Latency** | ~0.001ms | ~12ms | ~5ms | ~4ms |
| **Memory Usage** | High | Low | Medium | Medium |
| **Disk Usage** | None | Medium | Low (compressed) | Low |
| **Startup Time** | Instant | ~100ms | ~200ms | ~150ms |

### Use Case Recommendations

```go
// Development and testing
persistentType := "memory"

// Simple production deployments
persistentType := "file"

// High-write workloads
persistentType := "badger"

// Large-scale, high-performance needs
persistentType := "pebble"
```

## ⚠️ Best Practices and Considerations

### 1. Backend Selection Guidelines

```go
// ✅ Choose based on requirements
func selectBackend(requirements Requirements) string {
    if requirements.TestingOnly {
        return "memory"
    }

    if requirements.WriteHeavy && requirements.DiskSpace.IsLimited() {
        return "badger"  // Built-in compression
    }

    if requirements.HighPerformance && requirements.LargeDataset {
        return "pebble"  // Optimized for scale
    }

    return "file"  // Default for simplicity
}
```

### 2. Resource Management

```go
// ✅ Always clean up resources
func (pw *PersistentWrapper) Close() error {
    if closer, ok := pw.datastore.(io.Closer); ok {
        return closer.Close()
    }
    return nil
}

// Usage with proper cleanup
wrapper, err := persistent.New(blockWrapper, "badger", "./data")
if err != nil {
    return err
}
defer wrapper.Close()
```

### 3. Error Handling

```go
// ✅ Handle backend-specific errors
func handleStorageError(err error, backend string) error {
    switch backend {
    case "badger":
        if strings.Contains(err.Error(), "manifest has unsupported version") {
            return fmt.Errorf("badger database version incompatible, consider migration: %w", err)
        }
    case "pebble":
        if strings.Contains(err.Error(), "pebble: database") {
            return fmt.Errorf("pebble database corrupted, restore from backup: %w", err)
        }
    }
    return err
}
```

## 🔧 Troubleshooting

### Problem 1: Permission Denied

**Cause**: Insufficient file system permissions
```bash
# Solution: Fix directory permissions
mkdir -p ./data
chmod 755 ./data
```

### Problem 2: Database Lock Error

**Cause**: Multiple processes accessing same database
```go
// Solution: Implement process locking
func acquireFileLock(path string) (*os.File, error) {
    lockPath := filepath.Join(path, ".lock")
    lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
    if err != nil {
        return nil, fmt.Errorf("another process is using this database: %w", err)
    }
    return lock, nil
}
```

### Problem 3: Disk Space Issues

**Cause**: Unbounded data growth
```go
// Solution: Monitor disk usage
func checkDiskSpace(path string) error {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return err
    }

    available := stat.Bavail * uint64(stat.Bsize)
    if available < 100*1024*1024 { // Less than 100MB
        return fmt.Errorf("insufficient disk space: %d bytes available", available)
    }

    return nil
}
```

## 📚 Next Steps

### Immediate Next Steps
1. **[02-network](../02-network)**: Learn P2P networking fundamentals with libp2p
   - **Connection**: Build networking layer on top of persistent storage
   - **Why Next**: Essential for distributed data exchange and peer communication
   - **Learning Focus**: Host creation, peer connections, and message protocols

2. **[05-dag-ipld](../05-dag-ipld)**: Understand complex data structures and linking
   - **Connection**: Uses persistent storage to store linked data structures
   - **Why Important**: Move from simple blocks to sophisticated data organization

### Related Modules
3. **[03-dht-router](../03-dht-router)**: DHT-based content and peer discovery
   - **Connection**: Requires persistent storage for routing tables and peer information
   - **When to Learn**: After networking basics are understood

4. **[08-pin-gc](../08-pin-gc)**: Pin management and garbage collection
   - **Connection**: Uses persistent backends to track pinned content and optimize storage
   - **Relevance**: Storage optimization and content lifecycle management

### Alternative Learning Paths
- **For Data Structure Focus**: Go directly to **[05-dag-ipld](../05-dag-ipld)** to understand linked data before networking
- **For Network Focus**: Continue with **[02-network](../02-network)** → **[03-dht-router](../03-dht-router)** → **[04-bitswap](../04-bitswap)** sequence
- **For File System Focus**: Jump to **[06-unixfs-car](../06-unixfs-car)** to see how file systems are built on persistent storage

## 🎓 Practice Exercises

### Basic Exercises
1. Create a utility that migrates data from memory to persistent backend
2. Implement a simple caching layer with TTL using memory backend
3. Compare write performance across all backends with large datasets

### Advanced Exercises
1. Design a hybrid storage system with hot/warm/cold data tiers
2. Implement automatic backup rotation for persistent backends
3. Create a monitoring system that tracks storage metrics and alerts

Now you understand how to persist IPFS data reliably using various storage backends. The next module will teach you how to create complex data structures using DAGs! 🚀