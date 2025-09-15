# 00-block-cid: IPFS Fundamentals - Blocks and CIDs

## 🎯 Learning Objectives

Through this module, you will learn:
- Understanding **Content Addressing**, the core concept of IPFS
- The role and structure of **Blocks** and **CIDs (Content Identifiers)**
- Differences between **CID v0** and **CID v1** and their usage scenarios
- Characteristics of various **hash algorithms** (SHA2-256, BLAKE3, etc.)
- Data storage and retrieval methods through **Blockstore**

## 📋 Prerequisites

- Basic knowledge of Go programming
- Basic concepts of cryptographic hash functions
- Understanding of JSON data structures

## 🔑 Key Concepts

### What is Content Addressing?

Traditional file systems use **location-based addressing** (e.g., `/home/user/document.txt`). In contrast, IPFS uses **content-based addressing**.

```
Traditional approach: "Where is it?" → /path/to/file.txt
IPFS approach: "What is it?" → QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
```

### What is a Block?

**Block** is the fundamental unit for storing data in IPFS:

```go
type Block interface {
    RawData() []byte    // Actual data
    Cid() cid.Cid      // Unique identifier for this block
}
```

### What is a CID (Content Identifier)?

**CID** is a unique address for identifying content in IPFS:

```
CID structure: <version><codec><multihash>
Example: QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
```

#### CID v0 vs v1

| Feature | CID v0 | CID v1 |
|---------|--------|---------|
| Format | Base58 | Multibase (base32, base64, etc.) |
| Example | `QmYwAPJ...` | `bafybeig...` |
| Codec | DAG-PB fixed | Various codec support |
| Usage | Legacy compatibility | Modern applications |

## 💻 Code Analysis

### 1. Block Wrapper Implementation

```go
// pkg/block.go:19-30
type BlockWrapper struct {
    blockstore blockstore.Blockstore
}

func New(bs blockstore.Blockstore) *BlockWrapper {
    if bs == nil {
        bs = blockstore.NewBlockstore(datastore.NewMapDatastore())
    }
    return &BlockWrapper{blockstore: bs}
}
```

**Design Decisions**:
- Automatically creates memory-based storage if `blockstore` is nil
- Flexible storage selection through dependency injection

### 2. Data Storage and CID Generation

```go
// pkg/block.go:33-46
func (bw *BlockWrapper) Put(ctx context.Context, data []byte) (cid.Cid, error) {
    // 1. Hash calculation
    hash := sha256.Sum256(data)
    mhash, err := multihash.Encode(hash[:], multihash.SHA2_256)

    // 2. CID generation (v1, raw codec)
    c := cid.NewCidV1(cid.Raw, mhash)

    // 3. Block creation and storage
    block, err := blocks.NewBlockWithCid(data, c)
    return c, bw.blockstore.Put(ctx, block)
}
```

**Core Process**:
1. **Hash Calculation**: Generate data fingerprint with SHA2-256
2. **Multihash Encoding**: Include hash algorithm information
3. **CID Generation**: v1 + Raw codec combination
4. **Block Storage**: Permanent storage in Blockstore

### 3. Various Hash Algorithm Support

```go
// pkg/block.go:85-102
func (bw *BlockWrapper) PutWithHash(ctx context.Context, data []byte, hashType uint64) (cid.Cid, error) {
    switch hashType {
    case multihash.SHA2_256:
        hash := sha256.Sum256(data)
        mhash, err := multihash.Encode(hash[:], hashType)
        // ...
    case multihash.BLAKE3:
        hasher := blake3.New(32, nil)
        hasher.Write(data)
        hash := hasher.Sum(nil)
        mhash, err := multihash.Encode(hash, hashType)
        // ...
    }
}
```

**Hash Algorithm Comparison**:

| Algorithm | Speed | Security | Use Case |
|-----------|-------|----------|----------|
| SHA2-256 | Medium | High | Default recommendation |
| BLAKE3 | Fast | High | When performance matters |

### 4. CID v0 Compatibility

```go
// pkg/block.go:149-163
func (bw *BlockWrapper) PutCIDv0(ctx context.Context, data []byte) (cid.Cid, error) {
    hash := sha256.Sum256(data)
    mhash, err := multihash.Encode(hash[:], multihash.SHA2_256)

    // CID v0: Uses DAG-PB codec
    c := cid.NewCidV0(mhash)

    block, err := blocks.NewBlockWithCid(data, c)
    return c, bw.blockstore.Put(ctx, block)
}
```

## 🏃‍♂️ Practice Guide

### 1. Basic Execution

```bash
cd 00-block-cid
go run main.go
```

**Expected Output**:
```
=== Block and CID Demo ===

1. Basic Block Operations:
   ✅ Stored data → bafkreibvjvcv2i...
   ✅ Retrieved data matches original
   ✅ Block exists in blockstore

2. CID Version Comparison:
   📋 Same data, different CIDs:
      CID v0: QmYwAPJzv5CZsnA625s3Xf2ne...
      CID v1: bafkreibvjvcv2i5ijlrkflt...
   🔍 Both CIDs point to same data: true
```

### 2. Hash Algorithm Comparison Experiment

Observe this part in the code:

```go
// main.go:111-125
// Same data, different hash algorithms
data := []byte("Hash algorithm comparison")

sha256CID, _ := blockWrapper.PutWithHash(ctx, data, multihash.SHA2_256)
blake3CID, _ := blockWrapper.PutWithHash(ctx, data, multihash.BLAKE3)

fmt.Printf("   SHA2-256: %s\n", sha256CID.String()[:25]+"...")
fmt.Printf("   BLAKE3:   %s\n", blake3CID.String()[:25]+"...")
```

### 3. Codec Impact Experiment

```go
// main.go:135-149
// Same data, different codecs
rawCID, _ := blockWrapper.PutWithCodec(ctx, data, cid.Raw)
dagPBCID, _ := blockWrapper.PutWithCodec(ctx, data, cid.DagProtobuf)

// Result: Different CIDs are generated
```

**Learning Point**: The same data generates different CIDs when using different codecs.

### 4. Running Tests

```bash
go test -v ./...
```

**Key Test Cases**:
- ✅ Basic store/retrieve functionality
- ✅ CID v0/v1 compatibility
- ✅ Various hash algorithms
- ✅ Error handling (non-existent CID)

## 🔗 Real-World Use Cases

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

## ⚠️ Cautions and Best Practices

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

### Problem 2: Memory Usage Increase

**Cause**: Storing large data in memory blockstore
```go
// Solution: Use persistent storage (learned in next module)
// Refer to 01-persistent module
```

### Problem 3: CID Format Error

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

### Next Steps
1. **01-persistent**: Learn various storage backends
2. **02-dag-ipld**: Learn complex data structures and DAG
3. **03-unixfs**: Learn file system abstraction

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