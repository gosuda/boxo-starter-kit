# 12-ipld-prime: Advanced IPLD with Go-IPLD-Prime

## 🎯 Learning Objectives

By the end of this module, you will understand:
- How to use the go-ipld-prime library for advanced IPLD operations
- Working with IPLD datamodel.Node types for type-safe data manipulation
- Link systems and storage adapters for flexible data persistence
- Path resolution and traversal in complex linked data structures
- Converting between Go native types and IPLD datamodel nodes
- Building linked data graphs with automatic CID linking

## 📋 Prerequisites

- Completion of [00-block-cid](../00-block-cid) - Block and CID fundamentals
- Completion of [01-persistent](../01-persistent) - Storage backend concepts
- Completion of [05-dag-ipld](../05-dag-ipld) - Basic IPLD understanding
- Familiarity with Go's reflection and type system
- Understanding of content addressing and Merkle DAGs

## 🔑 Core Concepts

### Go-IPLD-Prime vs Basic IPLD

**Go-IPLD-Prime** is the next-generation IPLD library that provides:
- **Type Safety**: Strong typing through datamodel.Node interface
- **Performance**: Optimized for speed and memory efficiency
- **Flexibility**: Pluggable codecs and storage systems
- **Standards Compliance**: Full IPLD specification implementation
- **Advanced Features**: Selectors, traversal, and schema validation

### Key Components

#### 1. DataModel Nodes
The core abstraction representing IPLD data:
```go
type Node interface {
    Kind() Kind
    LookupByString(key string) (Node, error)
    LookupByIndex(idx int64) (Node, error)
    AsString() (string, error)
    AsInt() (int64, error)
    // ... other type accessors
}
```

#### 2. Link System
Manages how data is stored and retrieved:
```go
type LinkSystem struct {
    EncoderChooser    EncoderChooser
    DecoderChooser    DecoderChooser
    StorageReadOpener StorageReadOpener
    StorageWriteOpener StorageWriteOpener
}
```

#### 3. Storage Adapters
Bridge between IPLD and storage backends:
- **bsadapter.Adapter**: Adapts blockstore interface to IPLD
- **In-memory**: For testing and temporary storage
- **File-based**: For persistent local storage

## 💻 Code Architecture

### Module Structure
```
12-ipld-prime/
├── pkg/
│   ├── ipldprime.go    # Main wrapper and IPLD operations
│   ├── utils.go        # Type conversion utilities
│   └── codec.go        # Codec imports (DAG-CBOR, DAG-JSON, Raw)
└── ipldprime_test.go   # Comprehensive tests
```

### Core API

#### IpldWrapper
The main wrapper providing simplified IPLD-prime operations:

```go
type IpldWrapper struct {
    Prefix     *cid.Prefix      // CID generation settings
    LinkSystem linking.LinkSystem // Storage and encoding system
}
```

**Key Methods:**
- `PutIPLD(ctx, node) -> CID`: Store IPLD datamodel node
- `PutIPLDAny(ctx, data) -> CID`: Store Go native types
- `GetIPLD(ctx, cid) -> Node`: Retrieve as datamodel node
- `GetIPLDAny(ctx, cid) -> interface{}`: Retrieve as Go types
- `ResolvePath(ctx, root, path) -> (Node, CID)`: Navigate linked structures

#### Type Conversion System
Utilities for converting between Go types and IPLD nodes:

```go
// Convert Go types to IPLD nodes
func AnyToNode(v any) (datamodel.Node, error)

// Convert IPLD nodes to Go types
func NodeToAny(n datamodel.Node) (any, error)

// Extract all CIDs from a node structure
func NodeToCids(n datamodel.Node) []cid.Cid
```

## 🏃‍♂️ Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "fmt"

    ipld "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
)

func main() {
    ctx := context.Background()

    // Create IPLD wrapper with defaults
    wrapper, err := ipld.NewDefault(nil, nil)
    if err != nil {
        panic(err)
    }

    // Store structured data
    data := map[string]any{
        "name": "Alice",
        "age":  30,
        "tags": []string{"developer", "gopher"},
    }

    cid, err := wrapper.PutIPLDAny(ctx, data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Stored data with CID: %s\\n", cid)
}
```

### Working with Links

```go
// Create linked structure
leaf1, _ := wrapper.PutIPLDAny(ctx, map[string]any{
    "type": "person",
    "name": "Bob",
})

leaf2, _ := wrapper.PutIPLDAny(ctx, map[string]any{
    "type": "person",
    "name": "Carol",
})

// Create root linking to leaves
root, _ := wrapper.PutIPLDAny(ctx, map[string]any{
    "friends": []any{leaf1, leaf2},
    "metadata": map[string]any{
        "created": "2024-01-01",
        "version": 1,
    },
})

// Navigate through links
friendNode, friendCID, err := wrapper.ResolvePath(ctx, root, "friends/0")
if err != nil {
    panic(err)
}

// friendNode contains the resolved data from leaf1
// friendCID is the CID of leaf1
```

### Advanced Node Manipulation

```go
// Retrieve as datamodel.Node for type-safe access
node, err := wrapper.GetIPLD(ctx, someCID)
if err != nil {
    panic(err)
}

// Navigate structure safely
nameNode, err := node.LookupByString("name")
if err != nil {
    panic(err)
}

name, err := nameNode.AsString()
if err != nil {
    panic(err)
}

fmt.Printf("Name: %s\\n", name)
```

## 🏃‍♂️ Running the Examples

### Run Tests
```bash
cd 12-ipld-prime
go test -v
```

### Test Individual Components
```bash
# Test basic IPLD operations
go test -v -run TestIPLD

# Test linking and path resolution
go test -v -run TestIpldLink
```

### Expected Output
```
=== RUN   TestIPLD
--- PASS: TestIPLD (0.01s)
=== RUN   TestIpldLink
--- PASS: TestIpldLink (0.02s)
PASS
```

## 🔧 Configuration Options

### Custom CID Prefix
```go
import "github.com/multiformats/go-multicodec"

// Create custom CID prefix
prefix := &cid.Prefix{
    Version:  1,
    Codec:    uint64(multicodec.DagCbor),
    MhType:   uint64(multicodec.Sha2_256),
    MhLength: -1, // Default length
}

wrapper, err := ipld.NewDefault(prefix, nil)
```

### Custom Storage Backend
```go
import (
    "github.com/ipld/go-ipld-prime/storage/bsadapter"
    persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

// Use file-based storage
persistentStore, _ := persistent.New(persistent.File, "./ipld-data")

adapter := &bsadapter.Adapter{Wrapped: persistentStore}
linkSystem := cidlink.DefaultLinkSystem()
linkSystem.SetReadStorage(adapter)
linkSystem.SetWriteStorage(adapter)

wrapper, err := ipld.New(nil, &linkSystem)
```

## 🧪 Testing Guide

The module includes comprehensive tests demonstrating:

1. **Basic Operations**: Store and retrieve data with type conversion
2. **Link Resolution**: Navigate through linked data structures
3. **Path Traversal**: Access nested values using path strings
4. **Type Safety**: Proper handling of IPLD datamodel types

### Key Test Cases

```go
// Test 1: Basic IPLD operations with type conversion
func TestIPLD(t *testing.T) {
    // Verify same data produces same CID
    // Test type conversion Go -> IPLD -> Go
    // Validate datamodel.Node navigation
}

// Test 2: Linked structures and path resolution
func TestIpldLink(t *testing.T) {
    // Create multi-level linked structure
    // Test path resolution across links
    // Verify CID resolution accuracy
}
```

## 🔍 Troubleshooting

### Common Issues

1. **Type Conversion Errors**
   ```
   Error: unsupported type chan int
   Solution: IPLD only supports basic types, slices, maps, and links
   ```

2. **Path Resolution Failures**
   ```
   Error: path "invalid/path" not found
   Solution: Verify the path exists and follows IPLD path syntax
   ```

3. **Link System Configuration**
   ```
   Error: linkSystem is required
   Solution: Provide a configured LinkSystem or use NewDefault()
   ```

### Debugging Tips

- Use `node.Kind()` to inspect datamodel node types
- Check CID equality with `cid1.Equals(cid2)`
- Validate paths before resolution
- Use typed accessors (`AsString()`, `AsInt()`) for safe value extraction

## 📊 Performance Characteristics

### Advantages over Basic IPLD
- **Memory Efficiency**: Streaming and lazy loading support
- **Type Safety**: Compile-time type checking where possible
- **Codec Flexibility**: Pluggable encoding (CBOR, JSON, Raw)
- **Advanced Features**: Selectors, schemas, traversal optimization

### Best Practices
- Use `PutIPLDAny()` for convenience, `PutIPLD()` for performance
- Prefer datamodel.Node for repeated operations
- Configure appropriate storage backends for your use case
- Use path resolution for efficient nested data access

## 📚 Next Steps

### Immediate Next Steps
Having mastered IPLD-prime fundamentals, progress to these advanced capabilities:

1. **[13-dasl](../13-dasl)**: Schema-Based Development
   - Learn DASL (Data Schema Language) for type-safe data structures
   - Implement code generation from schemas to Go structs
   - Build validation systems with schema enforcement

2. **[14-traversal-selector](../14-traversal-selector)**: Advanced Traversal Patterns
   - Master sophisticated data structure navigation
   - Implement complex selector patterns for data extraction
   - Optimize traversal performance for large data graphs

### Related Modules
**Prerequisites (Review if needed):**
- [00-block-cid](../00-block-cid): Content addressing fundamentals
- [01-persistent](../01-persistent): Storage backends and persistence
- [05-dag-ipld](../05-dag-ipld): Basic IPLD operations and concepts

**Advanced Applications:**
- [15-graphsync](../15-graphsync): Network synchronization with IPLD-prime
- [16-trustless-gateway](../16-trustless-gateway): HTTP gateways with verification
- [17-ipni](../17-ipni): Content indexing and discovery
- [18-multifetcher](../18-multifetcher): Multi-source content retrieval

### Alternative Learning Paths

**For Data Modeling Focus:**
12-ipld-prime → 13-dasl → 05-dag-ipld (review) → 14-traversal-selector

**For Performance Optimization Focus:**
12-ipld-prime → 14-traversal-selector → 15-graphsync → 18-multifetcher

**For Web Integration Focus:**
12-ipld-prime → 16-trustless-gateway → 17-ipni → Integration Projects

## 📚 Further Reading

- [IPLD Specification](https://ipld.io/docs/)
- [Go-IPLD-Prime Documentation](https://pkg.go.dev/github.com/ipld/go-ipld-prime)
- [IPLD Data Model](https://ipld.io/docs/data-model/)
- [Content Addressing Guide](https://ipld.io/docs/data-model/link/)

---

This module provides the foundation for advanced IPLD operations using the go-ipld-prime library. Master these concepts to build sophisticated content-addressed data structures with type safety and performance optimization.