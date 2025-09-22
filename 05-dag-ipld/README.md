# 05-dag-ipld: Distributed Data Structures with IPLD

## 🎯 Learning Objectives

Through this module, you will learn:
- **DAG (Directed Acyclic Graph)** concepts and their application in IPFS
- **IPLD (InterPlanetary Linked Data)** data model understanding
- How to store and explore **complex data structures** in IPFS
- **Path Resolution** for accessing linked data
- **DAGService** interface implementation and utilization
- Support for various data formats through **Codecs**

## 📋 Prerequisites

- **00-block-cid** module completion (Understanding Blocks and CIDs)
- **01-persistent** module completion (Understanding data persistence)
- Understanding of JSON data structures
- Basic concepts of graph theory (nodes, edges, cycles)

## 🔑 Key Concepts

### What is a DAG (Directed Acyclic Graph)?

A **DAG** is a graph with directed edges and no cycles:

```
     A
   ↙   ↘
  B  →  C
  ↓     ↓
  D  →  E
```

**Characteristics**:
- **Directional**: Each connection has a direction
- **Acyclic**: No path leads back to the starting point
- **Immutable**: Once created, nodes never change

### What is IPLD (InterPlanetary Linked Data)?

**IPLD** is a data model for linking and traversing data across various distributed systems:

```json
{
  "name": "Alice",
  "age": 30,
  "friends": [
    {"/": "bafkreiabcd..."}, // CID link
    {"/": "bafkreiefgh..."}  // CID link
  ],
  "profile": {
    "bio": "Software Engineer",
    "avatar": {"/": "bafkreixyz..."}
  }
}
```

### Path Resolution

IPLD supports **path-based access**:

```
/profile/bio           → "Software Engineer"
/friends/0             → {other user object}
/profile/avatar        → {image data}
```

## 💻 Code Analysis

### 1. DAG Wrapper Design

```go
// pkg/dag.go:24-32
type DagWrapper struct {
    dagService     format.DAGService
    nodeGetter     format.NodeGetter
    persistentWrapper *persistent.PersistentWrapper
}

func New(pw *persistent.PersistentWrapper, datastorePath string) (*DagWrapper, error) {
    // DAGService initialization and IPLD support setup
}
```

**Design Features**:
- **Storage abstraction** through persistent.PersistentWrapper reuse
- **IPLD standard compliance** via format.DAGService interface
- **Flexible architecture** supporting multiple codecs

### 2. Storing Complex Data Structures

```go
// pkg/dag.go:88-110
func (dw *DagWrapper) PutAny(ctx context.Context, data any) (cid.Cid, error) {
    // 1. JSON serialization
    jsonData, err := json.Marshal(data)
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to marshal data: %w", err)
    }

    // 2. Create node with DAG-JSON codec
    node, err := dagJSON.Decode(dagJSON.DecodeOptions{}, bytes.NewReader(jsonData))
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to decode as DAG-JSON: %w", err)
    }

    // 3. Add to DAG
    err = dw.dagService.Add(ctx, node)
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to add node to DAG: %w", err)
    }

    return node.Cid(), nil
}
```

**Core Process**:
1. **Serialization**: Go struct → JSON
2. **IPLD Conversion**: JSON → IPLD node
3. **DAG Storage**: Add node to DAG

### 3. Path Resolution Implementation

```go
// pkg/dag.go:113-142
func (dw *DagWrapper) GetPath(ctx context.Context, rootCID cid.Cid, path string) (any, error) {
    if path == "" || path == "/" {
        return dw.GetAny(ctx, rootCID)
    }

    // 1. Retrieve root node
    rootNode, err := dw.dagService.Get(ctx, rootCID)
    if err != nil {
        return nil, fmt.Errorf("failed to get root node: %w", err)
    }

    // 2. Parse path and traverse
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")
    currentNode := rootNode

    for _, segment := range pathSegments {
        // 3. Traverse each path segment
        nextNode, _, err := currentNode.Resolve([]string{segment})
        if err != nil {
            return nil, fmt.Errorf("failed to resolve path segment '%s': %w", segment, err)
        }

        // 4. Load actual node if it's a CID link
        if nextCID, ok := nextNode.(cid.Cid); ok {
            currentNode, err = dw.dagService.Get(ctx, nextCID)
            if err != nil {
                return nil, fmt.Errorf("failed to get linked node: %w", err)
            }
        }
    }

    return dw.convertIPLDtoGo(currentNode)
}
```

### 4. Creating Linked Data Structures

```go
// main.go:95-115
func createLinkedData(ctx context.Context, dw *dag.DagWrapper) map[string]cid.Cid {
    cids := make(map[string]cid.Cid)

    // 1. Create individual objects
    profileCID, _ := dw.PutAny(ctx, map[string]any{
        "bio":      "IPFS Developer",
        "location": "Distributed Web",
        "skills":   []string{"Go", "IPFS", "Blockchain"},
    })

    addressCID, _ := dw.PutAny(ctx, map[string]any{
        "street": "123 Blockchain Ave",
        "city":   "Decentralized City",
        "country": "Internet",
    })

    // 2. Create linked user object
    userCID, _ := dw.PutAny(ctx, map[string]any{
        "name":    "Alice",
        "age":     30,
        "profile": map[string]string{"/": profileCID.String()}, // CID link
        "address": map[string]string{"/": addressCID.String()}, // CID link
        "metadata": map[string]any{
            "created": time.Now().Format(time.RFC3339),
            "version": "1.0",
        },
    })

    return map[string]cid.Cid{
        "user":    userCID,
        "profile": profileCID,
        "address": addressCID,
    }
}
```

**Data Link Structure**:
```
User
├─ name: "Alice"
├─ age: 30
├─ profile → (CID link) → Profile
│   ├─ bio: "IPFS Developer"
│   ├─ location: "Distributed Web"
│   └─ skills: ["Go", "IPFS", "Blockchain"]
└─ address → (CID link) → Address
    ├─ street: "123 Blockchain Ave"
    ├─ city: "Decentralized City"
    └─ country: "Internet"
```

## 🏃‍♂️ Hands-on Guide

### 1. Basic Execution

```bash
cd 05-dag-ipld
go run main.go
```

**Expected Output**:
```
=== DAG and IPLD Demo ===

1. Setting up DAG service with Badger backend:
   ✅ DAG service ready with persistent storage

2. Storing simple data structures:
   ✅ Stored person → bafyreigq4zsipbdvx...
   ✅ Stored company → bafyreih7x4jzrm2q...

3. Creating linked data structures:
   📦 Creating interconnected objects:
   ✅ profile → bafyreick25vk37ls...
   ✅ address → bafyreifp2eq7nbzd...
   ✅ user → bafyreigxm4b8xqrs...

4. Path resolution examples:
   🔍 Resolving paths in linked data:
   ✅ /name → Alice
   ✅ /age → 30
   ✅ /profile/bio → IPFS Developer
   ✅ /profile/skills/0 → Go
   ✅ /address/city → Decentralized City

5. Complex data operations:
   🔗 Following links across multiple objects:
   ✅ User's skills count: 3
   ✅ Profile creation time: 2024-01-15T10:30:00Z
```

### 2. Path Resolution Experiments

Modify the code to test various paths:

```go
// Paths to test
paths := []string{
    "/name",                    // Direct field
    "/profile/bio",             // Field in linked object
    "/profile/skills/1",        // Array index
    "/address/street",          // Nested link
    "/metadata/created",        // Metadata
}

for _, path := range paths {
    result, err := dw.GetPath(ctx, userCID, path)
    if err != nil {
        fmt.Printf("   ❌ Failed to resolve %s: %v\n", path, err)
    } else {
        fmt.Printf("   ✅ %s → %v\n", path, result)
    }
}
```

### 3. Data Structure Visualization

Print connection relationships to understand DAG structure:

```go
func printDAGStructure(ctx context.Context, dw *dag.DagWrapper, rootCID cid.Cid, depth int) {
    indent := strings.Repeat("  ", depth)

    data, err := dw.GetAny(ctx, rootCID)
    if err != nil {
        fmt.Printf("%s❌ Error: %v\n", indent, err)
        return
    }

    fmt.Printf("%s📦 %s\n", indent, rootCID.String()[:12]+"...")

    if dataMap, ok := data.(map[string]any); ok {
        for key, value := range dataMap {
            fmt.Printf("%s├─ %s: %v\n", indent, key, value)

            // Recursively explore if it's a CID link
            if linkMap, ok := value.(map[string]any); ok {
                if cidStr, exists := linkMap["/"]; exists {
                    if linkedCID, err := cid.Parse(cidStr.(string)); err == nil {
                        printDAGStructure(ctx, dw, linkedCID, depth+1)
                    }
                }
            }
        }
    }
}
```

### 4. Running Tests

```bash
go test -v ./...
```

**Key Test Cases**:
- ✅ Simple object storage/retrieval
- ✅ Complex nested structures
- ✅ Path resolution functionality
- ✅ CID link resolution
- ✅ Error handling (invalid paths)

## 🔍 Advanced Use Cases

### 1. Version Management System

```go
type Document struct {
    Title     string            `json:"title"`
    Content   string            `json:"content"`
    Author    string            `json:"author"`
    Version   int               `json:"version"`
    Parent    map[string]string `json:"parent,omitempty"` // Link to previous version
    Created   string            `json:"created"`
}

func createVersionedDocument(ctx context.Context, dw *dag.DagWrapper,
                           title, content, author string, parentCID *cid.Cid) (cid.Cid, error) {
    doc := Document{
        Title:   title,
        Content: content,
        Author:  author,
        Version: 1,
        Created: time.Now().Format(time.RFC3339),
    }

    if parentCID != nil {
        // Link to previous version
        doc.Parent = map[string]string{"/": parentCID.String()}

        // Increment version number
        if parentDoc, err := dw.GetAny(ctx, *parentCID); err == nil {
            if parent, ok := parentDoc.(map[string]any); ok {
                if v, exists := parent["version"]; exists {
                    doc.Version = int(v.(float64)) + 1
                }
            }
        }
    }

    return dw.PutAny(ctx, doc)
}
```

### 2. Social Graph Implementation

```go
type User struct {
    Name      string              `json:"name"`
    Bio       string              `json:"bio"`
    Following []map[string]string `json:"following"` // CID link array
    Followers []map[string]string `json:"followers"` // CID link array
    Posts     []map[string]string `json:"posts"`     // Post CID links
}

func followUser(ctx context.Context, dw *dag.DagWrapper,
               followerCID, targetCID cid.Cid) error {
    // 1. Retrieve follower user info
    followerData, err := dw.GetAny(ctx, followerCID)
    if err != nil {
        return err
    }

    // 2. Add to following list
    follower := followerData.(map[string]any)
    following := follower["following"].([]any)
    following = append(following, map[string]string{"/": targetCID.String()})
    follower["following"] = following

    // 3. Store updated user info
    _, err = dw.PutAny(ctx, follower)
    return err
}
```

### 3. File System Tree

```go
type FileNode struct {
    Name     string              `json:"name"`
    Type     string              `json:"type"`     // "file" or "directory"
    Size     int64               `json:"size,omitempty"`
    Children []map[string]string `json:"children,omitempty"` // For directories
    Content  map[string]string   `json:"content,omitempty"`  // For files
}

func createFileTree(ctx context.Context, dw *dag.DagWrapper) (cid.Cid, error) {
    // Create files
    file1CID, _ := dw.PutAny(ctx, FileNode{
        Name: "README.md",
        Type: "file",
        Size: 1024,
        Content: map[string]string{"/": "bafkreifile1content..."},
    })

    file2CID, _ := dw.PutAny(ctx, FileNode{
        Name: "main.go",
        Type: "file",
        Size: 2048,
        Content: map[string]string{"/": "bafkreifile2content..."},
    })

    // Create directory
    return dw.PutAny(ctx, FileNode{
        Name: "project",
        Type: "directory",
        Children: []map[string]string{
            {"/": file1CID.String()},
            {"/": file2CID.String()},
        },
    })
}
```

## ⚠️ Cautions and Best Practices

### 1. CID Link Creation

```go
// ✅ Correct CID link format
link := map[string]string{"/": targetCID.String()}

// ❌ Incorrect formats
link := map[string]string{"cid": targetCID.String()}
link := targetCID.String() // Only storing as string
```

### 2. Path Resolution Error Handling

```go
// ✅ Detailed error handling per path
result, err := dw.GetPath(ctx, rootCID, "/profile/skills/10")
if err != nil {
    if strings.Contains(err.Error(), "index out of range") {
        return handleArrayIndexError(err)
    }
    if strings.Contains(err.Error(), "path not found") {
        return handlePathNotFoundError(err)
    }
    return handleGenericError(err)
}
```

### 3. Memory-Efficient Large Data Processing

```go
// ✅ Process large data with streaming approach
func processLargeDataset(ctx context.Context, dw *dag.DagWrapper,
                        dataStream <-chan map[string]any) error {
    for data := range dataStream {
        cid, err := dw.PutAny(ctx, data)
        if err != nil {
            return err
        }

        // Release processed data from memory
        log.Printf("Processed: %s", cid)
    }
    return nil
}
```

### 4. Preventing Circular References

```go
// ✅ Prevent circular references with depth limits
func traverseDAG(ctx context.Context, dw *dag.DagWrapper,
                rootCID cid.Cid, maxDepth int) error {
    visited := make(map[string]bool)
    return traverseDAGRecursive(ctx, dw, rootCID, visited, 0, maxDepth)
}

func traverseDAGRecursive(ctx context.Context, dw *dag.DagWrapper,
                         currentCID cid.Cid, visited map[string]bool,
                         depth, maxDepth int) error {
    if depth > maxDepth {
        return fmt.Errorf("maximum depth exceeded")
    }

    cidStr := currentCID.String()
    if visited[cidStr] {
        return nil // Already visited node
    }
    visited[cidStr] = true

    // Node processing logic...
    return nil
}
```

## 🔧 Troubleshooting

### Issue 1: "path not found" Error

**Cause**: Invalid path or non-existent field
```go
// Solution: Path validation
func validatePath(ctx context.Context, dw *dag.DagWrapper,
                  rootCID cid.Cid, path string) error {
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")

    for i, segment := range pathSegments {
        partialPath := "/" + strings.Join(pathSegments[:i+1], "/")
        _, err := dw.GetPath(ctx, rootCID, partialPath)
        if err != nil {
            return fmt.Errorf("invalid path at segment '%s': %w", segment, err)
        }
    }
    return nil
}
```

### Issue 2: "node not found" Error

**Cause**: Node referenced by CID link does not exist
```go
// Solution: Link integrity validation
func validateLinks(ctx context.Context, dw *dag.DagWrapper,
                  data map[string]any) error {
    for key, value := range data {
        if linkMap, ok := value.(map[string]any); ok {
            if cidStr, exists := linkMap["/"]; exists {
                targetCID, err := cid.Parse(cidStr.(string))
                if err != nil {
                    return fmt.Errorf("invalid CID in field '%s': %w", key, err)
                }

                exists, err := dw.Has(ctx, targetCID)
                if err != nil || !exists {
                    return fmt.Errorf("linked node not found for field '%s': %s",
                                     key, targetCID)
                }
            }
        }
    }
    return nil
}
```

### Issue 3: JSON Serialization Error

**Cause**: Contains non-serializable data types
```go
// Solution: Convert to serializable data
func sanitizeForJSON(data any) any {
    switch v := data.(type) {
    case time.Time:
        return v.Format(time.RFC3339)
    case func():
        return nil // Remove functions
    case chan interface{}:
        return nil // Remove channels
    case map[string]any:
        result := make(map[string]any)
        for k, val := range v {
            if sanitized := sanitizeForJSON(val); sanitized != nil {
                result[k] = sanitized
            }
        }
        return result
    default:
        return v
    }
}
```

## 📊 Performance Optimization

### 1. Batch Processing

```go
// ✅ Process multiple nodes in batches
func putBatch(ctx context.Context, dw *dag.DagWrapper,
             items []any) ([]cid.Cid, error) {
    var cids []cid.Cid

    // Worker pool for parallel processing
    const workers = 4
    jobs := make(chan any, len(items))
    results := make(chan struct{cid cid.Cid; err error}, len(items))

    // Start workers
    for i := 0; i < workers; i++ {
        go func() {
            for item := range jobs {
                cid, err := dw.PutAny(ctx, item)
                results <- struct{cid cid.Cid; err error}{cid, err}
            }
        }()
    }

    // Send jobs
    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    // Collect results
    for i := 0; i < len(items); i++ {
        result := <-results
        if result.err != nil {
            return nil, result.err
        }
        cids = append(cids, result.cid)
    }

    return cids, nil
}
```

### 2. Caching Strategy

```go
// ✅ Improve performance with LRU cache
type CachedDagWrapper struct {
    *DagWrapper
    cache *lru.Cache
}

func (cdw *CachedDagWrapper) GetAny(ctx context.Context, c cid.Cid) (any, error) {
    // Check cache
    if cached, ok := cdw.cache.Get(c.String()); ok {
        return cached, nil
    }

    // Cache miss - fetch from storage
    result, err := cdw.DagWrapper.GetAny(ctx, c)
    if err != nil {
        return nil, err
    }

    // Store in cache
    cdw.cache.Add(c.String(), result)
    return result, nil
}
```

## 📚 Additional Learning Resources

### Related Documentation
- [IPLD Specification](https://ipld.io/docs/)
- [DAG-JSON Codec](https://ipld.io/docs/codecs/dag-json/)
- [IPFS DAG API](https://docs.ipfs.io/reference/kubo/rpc/#api-v0-dag)
- [Graph Theory Basics](https://en.wikipedia.org/wiki/Directed_acyclic_graph)

## 📚 Next Steps

### Immediate Next Steps
1. **[06-unixfs-car](../06-unixfs-car)**: Learn IPLD representation of file and directory structures
   - **Connection**: Build practical file systems using DAG and IPLD concepts
   - **Why Next**: Move from abstract data structures to concrete file system applications
   - **Learning Focus**: UnixFS data model and CAR (Content Addressable aRchive) format

### Related Modules
2. **[07-mfs](../07-mfs)**: Mutable File System built on IPLD foundations
   - **Connection**: Uses DAG structures to create mutable file system views
   - **When to Learn**: After understanding file system basics from UnixFS

3. **[08-pin-gc](../08-pin-gc)**: Data persistence and garbage collection strategies
   - **Connection**: Manage lifecycle of DAG nodes and linked data structures
   - **Relevance**: Ensure important DAG structures remain available

4. **[12-ipld-prime](../12-ipld-prime)**: Advanced IPLD implementation and schemas
   - **Connection**: Next-generation IPLD with enhanced schema support
   - **Advanced Use**: When building complex, schema-validated data structures

### Alternative Learning Paths
- **For Network Distribution**: Go to **[02-network](../02-network)** → **[04-bitswap](../04-bitswap)** to learn how DAG nodes are distributed
- **For Specialized Processing**: Jump to **[14-traversal-selector](../14-traversal-selector)** for advanced DAG query capabilities
- **For File Focus**: Skip directly to **[06-unixfs-car](../06-unixfs-car)** if primarily interested in file systems

## 🎓 Practice Problems

### Basic Exercises
1. Store a simple user profile in IPLD and access it via paths
2. Create mutual references between two objects and follow the links
3. Test path resolution using array indices

### Advanced Exercises
1. Design a blog system with linked post structures
2. Implement a file system tree and directory traversal
3. Represent Git-like commit history as a DAG

### Real-world Projects
1. Model social network follow relationships as a DAG and implement a recommendation algorithm
2. Create a document version management system to track change history
3. Design distributed database schemas with IPLD and implement a query system

Now you understand how to handle complex data structures in IPFS. In the next module, you'll learn how to work with actual files and directories! 🚀