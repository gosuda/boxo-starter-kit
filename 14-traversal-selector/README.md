# 14-traversal-selector: Advanced IPLD Traversal & Selectors

## 🎯 Learning Objectives

By the end of this module, you will understand:
- How to traverse complex IPLD data structures efficiently
- Working with IPLD selectors for targeted data extraction
- Building sophisticated selectors for precise navigation
- Different traversal patterns and their use cases
- Visitor functions and collectors for processing traversal results
- Transform operations during traversal
- Performance optimization for large data graphs

## 📋 Prerequisites

- Completion of [12-ipld-prime](../12-ipld-prime) - IPLD-prime fundamentals
- Understanding of [05-dag-ipld](../05-dag-ipld) - Basic IPLD concepts
- Familiarity with [13-dasl](../13-dasl) - Schema concepts (helpful)
- Knowledge of tree traversal algorithms
- Understanding of functional programming concepts (map, filter, reduce)

## 🔑 Core Concepts

### IPLD Traversal

**Traversal** is the process of systematically visiting nodes in an IPLD data structure:
- **Depth-First**: Explore deeply before breadth
- **Breadth-First**: Explore all nodes at current level first
- **Selective**: Visit only nodes matching specific criteria
- **Transformative**: Modify data during traversal

### IPLD Selectors

**Selectors** are declarative specifications for which parts of IPLD data to traverse:
- **Matcher**: Select the current node
- **Fields**: Navigate into specific map fields
- **Index**: Navigate into specific list elements
- **Recursive**: Traverse with depth limits
- **Union**: Combine multiple selection criteria

### Selector Types

#### Basic Selectors
```go
// Select just the current node
SelectorOne()

// Select specific field
SelectorField("name")

// Select specific array index
SelectorIndex(0)
```

#### Recursive Selectors
```go
// Select everything (unlimited depth)
SelectorAll(true)

// Select with depth limit
SelectorDepth(3, true)
```

#### Path Selectors
```go
// Navigate complex paths
SelectorPath(datamodel.ParsePath("users/0/name"))
```

## 💻 Code Architecture

### Module Structure
```
14-traversal-selector/
├── pkg/
│   ├── traversal.go           # Main traversal operations
│   ├── selector.go            # Selector building utilities
│   └── visitor.go             # Visitor patterns and collectors
└── traversalselector_test.go  # Comprehensive tests
```

### Core Components

#### TraversalSelectorWrapper
Main wrapper providing traversal and selector operations:

```go
type TraversalSelectorWrapper struct {
    *ipldprime.IpldWrapper  // Embedded IPLD operations
}
```

**Key Methods:**
- `WalkOneNode(ctx, cid, visitFn)`: Visit single node
- `WalkMatching(ctx, cid, selector, visitFn)`: Selective traversal
- `WalkAdv(ctx, cid, selector, advVisitFn)`: Advanced traversal with visit reasons
- `WalkTransforming(ctx, cid, selector, transformFn)`: Transform during traversal

#### Selector Builders
Factory functions for common selector patterns:

```go
// Basic selectors
func SelectorOne() ipld.Node                    // Current node only
func SelectorField(key string) ipld.Node       // Specific field
func SelectorIndex(i int64) ipld.Node          // Specific index

// Recursive selectors
func SelectorAll(match bool) ipld.Node         // All nodes
func SelectorDepth(limit int64, match bool) ipld.Node  // Depth-limited

// Path selectors
func SelectorPath(path datamodel.Path) ipld.Node       // Follow path
```

#### Visitor Patterns
Pre-built visitors for common collection patterns:

```go
// Collect single result
func NewVisitOne(root cid.Cid) (VisitFn, *VisitOne)

// Collect all results
func NewVisitAll(root cid.Cid) (VisitFn, *VisitCollector)

// Stream results
func NewVisitStream(root cid.Cid, buf int) (VisitFn, *VisitStream)

// Transform operations
func NewTransformAll(root cid.Cid, replacer) (TransformFn, *TransformCollector)
```

## 🏃‍♂️ Usage Examples

### Basic Node Traversal

```go
package main

import (
    "context"
    "fmt"

    ts "github.com/gosuda/boxo-starter-kit/14-traversal-selector/pkg"
)

func main() {
    ctx := context.Background()

    // Create wrapper
    wrapper, err := ts.New(nil)
    if err != nil {
        panic(err)
    }

    // Create some data
    rootCID, err := wrapper.PutIPLDAny(ctx, map[string]any{
        "name": "John",
        "age":  30,
        "tags": []string{"developer", "gopher"},
    })
    if err != nil {
        panic(err)
    }

    // Visit just the root node
    visit, state := ts.NewVisitOne(rootCID)
    err = wrapper.WalkOneNode(ctx, rootCID, visit)
    if err != nil {
        panic(err)
    }

    if state.Found {
        fmt.Printf("Visited node: %s\\n", state.Rec.Cid)
    }
}
```

### Selective Traversal with Selectors

```go
// Create complex nested structure
data := map[string]any{
    "users": []any{
        map[string]any{
            "id":   "user1",
            "name": "Alice",
            "profile": map[string]any{
                "bio": "Software Engineer",
                "location": "San Francisco",
            },
        },
        map[string]any{
            "id":   "user2",
            "name": "Bob",
            "profile": map[string]any{
                "bio": "Product Manager",
                "location": "New York",
            },
        },
    },
}

rootCID, _ := wrapper.PutIPLDAny(ctx, data)

// Select all user names: users/*/name
pathSelector := ts.SelectorPath(datamodel.ParsePath("users"))
selector, err := ts.CompileSelector(pathSelector)
if err != nil {
    panic(err)
}

// Collect all matching nodes
visit, collector := ts.NewVisitAll(rootCID)
err = wrapper.WalkMatching(ctx, rootCID, selector, visit)
if err != nil {
    panic(err)
}

fmt.Printf("Found %d nodes\\n", len(collector.Records))
for _, record := range collector.Records {
    // Process each visited node
    fmt.Printf("Node CID: %s\\n", record.Cid)
}
```

### Recursive Traversal with Depth Limits

```go
// Build a tree structure
treeData := map[string]any{
    "level": 0,
    "children": []any{
        map[string]any{
            "level": 1,
            "children": []any{
                map[string]any{"level": 2, "leaf": true},
                map[string]any{"level": 2, "leaf": true},
            },
        },
        map[string]any{
            "level": 1,
            "children": []any{
                map[string]any{"level": 2, "leaf": true},
            },
        },
    },
}

treeCID, _ := wrapper.PutIPLDAny(ctx, treeData)

// Traverse with depth limit of 2
depthSelector := ts.SelectorDepth(2, true)
selector, _ := ts.CompileSelector(depthSelector)

visit, collector := ts.NewVisitAll(treeCID)
err := wrapper.WalkMatching(ctx, treeCID, selector, visit)
if err != nil {
    panic(err)
}

fmt.Printf("Visited %d nodes within depth 2\\n", len(collector.Records))
```

### Transform Operations

```go
// Transform data during traversal
transformFn, collector := ts.NewTransformAll(rootCID,
    func(p traversal.Progress, n datamodel.Node) (datamodel.Node, error) {
        // Example: add timestamp to all map nodes
        if n.Kind() == datamodel.Kind_Map {
            // Create new node with additional field
            // (This is a simplified example)
            return n, nil
        }
        return n, nil
    },
)

selector, _ := ts.CompileSelector(ts.SelectorAll(true))
transformedNode, err := wrapper.WalkTransforming(ctx, rootCID, selector, transformFn)
if err != nil {
    panic(err)
}

fmt.Printf("Transformed root node: %T\\n", transformedNode)
```

### Streaming Large Datasets

```go
// For large datasets, use streaming to avoid memory issues
visit, stream := ts.NewVisitStream(rootCID, 100) // Buffer 100 items

// Start traversal in goroutine
go func() {
    defer stream.Close()

    selector, _ := ts.CompileSelector(ts.SelectorAll(true))
    err := wrapper.WalkMatching(ctx, rootCID, selector, visit)
    if err != nil {
        fmt.Printf("Traversal error: %v\\n", err)
    }
}()

// Process results as they arrive
for record := range stream.C {
    fmt.Printf("Processing node: %s\\n", record.Cid)
    // Process record.Node...
}
```

## 🏃‍♂️ Running the Examples

### Run Tests
```bash
cd 14-traversal-selector
go test -v
```

### Expected Output
```
=== RUN   TestWalkOneNode
--- PASS: TestWalkOneNode (0.01s)
=== RUN   TestWalkMatchingAll
--- PASS: TestWalkMatchingAll (0.02s)
PASS
```

### Test Explanations

The tests build binary tree structures and demonstrate:
1. **Single Node Access**: Visit just the root node
2. **Complete Traversal**: Visit all nodes in a 3-level binary tree
3. **Node Counting**: Verify correct number of visited nodes (21 total)

## 🔧 Advanced Selector Patterns

### Complex Path Selection
```go
// Navigate: data/users/0/profile/location
path := datamodel.ParsePath("data/users/0/profile/location")
selector := ts.SelectorPath(path)
```

### Union Selectors
```go
// Combine multiple criteria (requires manual building)
ssb := sb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
unionSelector := ssb.ExploreUnion(
    ssb.ExploreField("name", ssb.Matcher()),
    ssb.ExploreField("email", ssb.Matcher()),
).Node()
```

### Conditional Selectors
```go
// Select based on interpretation (e.g., different codec)
interpretSelector := ts.SelectorInterpretAs("dag-json",
    ssb.ExploreField("data", ssb.Matcher()))
```

## 🧪 Testing Patterns

### Building Test Data
The module includes utilities for building complex test structures:

```go
func buildBinaryTree(level int, prefix string) cid.Cid {
    // Recursively builds tree structures for testing
    // Returns root CID of constructed tree
}
```

### Verification Helpers
```go
func loadBool(t *testing.T, n datamodel.Node, key string) bool
func loadString(t *testing.T, n datamodel.Node, key string) string
func loadLink(t *testing.T, n datamodel.Node, key string) cid.Cid
```

## 🔍 Troubleshooting

### Common Issues

1. **Selector Compilation Errors**
   ```
   Error: selector compilation failed
   Solution: Validate selector syntax and ensure proper nesting
   ```

2. **Traversal Depth Issues**
   ```
   Error: maximum recursion depth exceeded
   Solution: Use SelectorDepth() to limit traversal depth
   ```

3. **Memory Issues with Large Graphs**
   ```
   Error: out of memory during traversal
   Solution: Use streaming visitors instead of collectors
   ```

4. **CID Resolution Failures**
   ```
   Error: link not found during traversal
   Solution: Ensure all referenced content is available in storage
   ```

### Performance Tips

- Use **depth limits** for large graphs
- Choose **streaming** over collection for large datasets
- Use **specific selectors** instead of traversing everything
- **Cache** frequently accessed nodes
- Consider **parallel traversal** for independent subgraphs

## 📊 Performance Characteristics

### Selector Efficiency
- **Specific selectors**: O(path length) for direct access
- **Recursive selectors**: O(nodes) for complete traversal
- **Depth-limited**: O(nodes at depth ≤ limit)
- **Field selectors**: O(1) for direct field access

### Memory Usage
- **Collectors**: Store all results in memory
- **Streams**: Bounded memory usage with buffering
- **Transform**: Memory proportional to transformation complexity

## 📚 Next Steps

### Immediate Next Steps
With advanced traversal mastery, apply these skills to network protocols and optimization:

1. **[15-graphsync](../15-graphsync)**: Network-Based Selective Synchronization
   - Apply selector expertise to peer-to-peer data synchronization
   - Build efficient network protocols using traversal patterns
   - Master selective sync for distributed data systems

2. **Production Optimization Paths**: Choose your focus:
   - **[18-multifetcher](../18-multifetcher)**: Multi-source traversal optimization
   - **[17-ipni](../17-ipni)**: Large-scale content discovery with traversal

### Related Modules
**Prerequisites (Essential foundation):**
- [12-ipld-prime](../12-ipld-prime): IPLD-prime operations and datamodel types
- [13-dasl](../13-dasl): Schema concepts for type-aware traversal (recommended)
- [05-dag-ipld](../05-dag-ipld): Basic IPLD concepts and DAG structures

**Network Applications:**
- [15-graphsync](../15-graphsync): P2P synchronization with selectors
- [02-network](../02-network): Networking fundamentals for distributed traversal
- [04-bitswap](../04-bitswap): Block exchange with selective requests

**Advanced Systems:**
- [17-ipni](../17-ipni): Content indexing with traversal optimization
- [16-trustless-gateway](../16-trustless-gateway): Gateway optimization with selectors
- [18-multifetcher](../18-multifetcher): Multi-protocol optimization

### Alternative Learning Paths

**For Network Protocol Development:**
14-traversal-selector → 15-graphsync → 02-network (review) → Distributed Systems

**For Performance Engineering:**
14-traversal-selector → 18-multifetcher → 17-ipni → High-Performance Systems

**For Data Architecture:**
14-traversal-selector → 17-ipni → Content Discovery Systems → Enterprise Solutions

**For Gateway Development:**
14-traversal-selector → 16-trustless-gateway → Web Integration Projects

## 📚 Further Reading

- [IPLD Selectors Specification](https://ipld.io/specs/selectors/)
- [Go-IPLD-Prime Traversal](https://pkg.go.dev/github.com/ipld/go-ipld-prime/traversal)
- [Selector Builder Documentation](https://pkg.go.dev/github.com/ipld/go-ipld-prime/traversal/selector/builder)
- [IPLD Path Syntax](https://ipld.io/specs/pathing/)

---

This module provides powerful tools for navigating and processing complex IPLD data structures. Master these traversal patterns to build efficient, selective data processing pipelines.