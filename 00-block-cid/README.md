# 00-block-cid: IPFS Fundamentals - Blocks and CIDs

## 🎯 Learning Objectives

Through this module, you will learn:
- Understanding **Content Addressing**, the core concept of IPFS
- The role and structure of **Blocks** and **CIDs (Content Identifiers)**
- Differences between **CID v0** and **CID v1** and their usage scenarios
- Characteristics of various **hash algorithms** (SHA2-256, BLAKE3, etc.)
- How to use a **Blockstore** wrapper for storing, retrieving, and managing blocks

## 📋 Prerequisites

- Basic knowledge of Go programming
- Basic concepts of cryptographic hash functions
- Understanding of JSON data structures

## 🔑 Key Concepts

### Content Addressing

Traditional file systems use **location-based addressing** (e.g., `/home/user/document.txt`). In contrast, IPFS uses **content-based addressing**.

```
Traditional approach: "Where is it?" → /path/to/file.txt
IPFS approach: "What is it?" → QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
```
### Block

**Block** is the fundamental unit for storing data in IPFS:

```go
type Block interface {
    RawData() []byte    // Actual data
    Cid() cid.Cid      // Unique identifier for this block
}
```
### CID (Content Identifier)

**CID** is a unique address for identifying content in IPFS:

```
CID structure: <version><codec><multihash>
Example: QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
```

#### CID v0 vs v1

| Feature | CID v0 | CID v1 |
|---------|--------|---------|
| Format | Base58 | Multibase (base32, base64, etc.) |
| Example | `Qm...` | `bafy...` |
| Codec | DAG-PB only | Multiple codec (Raw, DAG-PB, Cbor, etc.) |
| Usage | Legacy compatibility | Modern applications (recommended) |

## 💻 Code Overview

### 1. Block Wrapper Implementation

```go
type BlockWrapper struct {
    Batching ds.Batching
    blockstore.Blockstore
}

func New(ds ds.Batching, opts ...blockstore.Option) *BlockWrapper {
    bs := blockstore.NewBlockstore(ds, opts...)
    return &BlockWrapper{
        Batching:   ds,
        Blockstore: bs,
    }
}
```

### 2. Storing Data

```go
func (s *BlockWrapper) Put(ctx context.Context, b blocks.Block) error
func (s *BlockWrapper) PutV0Cid(ctx context.Context, data []byte) (cid.Cid, error)
func (s *BlockWrapper) PutV1Cid(ctx context.Context, data []byte, prefix *cid.Prefix) (cid.Cid, error)
```

### 3. Retrieving Data
```go
func (s *BlockWrapper) Has(ctx context.Context, c cid.Cid) (bool, error)
func (s *BlockWrapper) Get(ctx context.Context, c cid.Cid) (blockformat.Block, error)
func (s *BlockWrapper) GetRaw(ctx context.Context, c cid.Cid) ([]byte, error)
func (s *BlockWrapper) GetSize(ctx context.Context, c cid.Cid) (int, error)
func (s *BlockWrapper) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error)
func (s *BlockWrapper) Delete(ctx context.Context, c cid.Cid) error
func (s *BlockWrapper) Close() error
```

## 🏃‍♂️ Practice Guide

### Basic Execution

```bash
cd 00-block-cid
go test -v ./...
```

**Key Test Cases**:
- ✅ Basic store/retrieve functionality
- ✅ CID v0/v1 compatibility
- ✅ Various hash algorithms
- ✅ Error handling (non-existent CID)

## 🔗 Use Cases

### 1. File Integrity Verification

```go
// Ensuring integrity when uploading files
originalCID, _ := blockWrapper.Put(ctx, fileData)

// Verification when downloading later
retrievedData, _ := blockWrapper.Get(ctx, originalCID)
// retrievedData == fileData is guaranteed
```

### 2. Deduplication

```go
// Files with same content generate same CID
file1CID, _ := blockWrapper.Put(ctx, []byte("Hello"))
file2CID, _ := blockWrapper.Put(ctx, []byte("Hello"))
// file1CID == file2CID (automatic deduplication)
```

### 3. Version Control

```go
// Each version of document has unique CID
v1CID, _ := blockWrapper.Put(ctx, []byte("Document v1"))
v2CID, _ := blockWrapper.Put(ctx, []byte("Document v2"))
// Change tracking possible
```

## ⚠️ Practical Usage & Best Practices

### 1. CID Version Selection Guide

```go
// ✅ Recommended: New applications
cid := cid.NewCidV1(cid.Raw, mhash)

// ⚠️ Caution: Only when legacy compatibility needed
cid := cid.NewCidV0(mhash)
```

### 2. Hash Algorithm Selection

```go
// ✅ General purpose: SHA2-256 (default recommendation)
hashType := multihash.SHA2_256

// ✅ Performance critical: BLAKE3
hashType := multihash.BLAKE3

// ❌ Avoid: MD5, SHA1 (security vulnerabilities)
```

**Hash Algorithm Comparison**:

| Algorithm | Speed | Security | Use Case |
|-----------|-------|----------|----------|
| SHA2-256 | Medium | High | Default recommendation |
| BLAKE3 | Fast | High | When performance matters |

### 3. Error Handling

```go
// ✅ Always check errors
data, err := blockWrapper.Get(ctx, someCID)
if err != nil {
    if err == blockstore.ErrNotFound {
        // Block doesn't exist
    }
    return err
}
```

## 🔧 Troubleshooting

### Problem 1: "block not found" Error

**Cause**: Requesting data with non-existent CID
```go
// Solution: Check existence first
exists, err := blockWrapper.Has(ctx, someCID)
if !exists {
    log.Printf("Block %s does not exist", someCID)
}
```

### Problem 2: CID Format Error

**Cause**: Invalid CID string parsing
```go
// Solution: Validate CID
if !cid.IsValid() {
    return fmt.Errorf("invalid CID format")
}
```

## 📚 Additional Learning Resources

### Related Documentation
- [IPFS Concepts: Content Addressing](https://docs.ipfs.io/concepts/content-addressing/)
- [CID Specification](https://github.com/multiformats/cid)
- [Multihash Specification](https://github.com/multiformats/multihash)

## 📚 Next Steps

### Immediate Next Steps
1. **[01-persistent](../01-persistent)**: Learn data persistence and storage backend selection
   - **Connection**: Build on block storage concepts with persistent storage backends
   - **Why Next**: Essential for real-world applications that need data durability
   - **Learning Focus**: Memory, File, Badger, and Pebble backend comparisons

### Related Modules
2. **[05-dag-ipld](../05-dag-ipld)**: Learn about complex data structures using CIDs
   - **Connection**: Uses CID linking to create Directed Acyclic Graphs
   - **When to Learn**: After understanding persistent storage fundamentals

3. **[06-unixfs-car](../06-unixfs-car)**: Understand file system abstractions
   - **Connection**: File chunking and directory structures built on block foundations

### Alternative Learning Paths
- **For Network Focus**: Jump to **[02-network](../02-network)** to understand P2P communication before storage
- **For Data Structure Focus**: Go directly to **[05-dag-ipld](../05-dag-ipld)** to see how blocks link together

## 🎓 Practice Problems

### Basic Exercises
1. Store the string "Hello IPFS!" and output its CID
2. Check CID differences when storing same data with SHA2-256 and BLAKE3
3. Handle errors when querying with non-existent CID

### Advanced Exercises
1. Write a function that serializes JSON objects for storage and deserializes them back
2. Create a utility that calculates file CID for integrity verification
3. Implement a function that converts CID v0 to v1

Now you should understand the fundamentals of IPFS - Blocks and CIDs. In the next module, we'll learn how to store this data persistently! 🚀