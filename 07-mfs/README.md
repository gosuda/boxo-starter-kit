# 07-mfs: Mutable File System

A wrapper around the boxo MFS (Mutable File System) that enables treating IPFS content as a traditional file system with read/write operations.

## 🎯 Learning Objectives

- Understand the concept of MFS (Mutable File System) in IPFS
- Learn how to perform file system operations on IPFS content
- Master creating, reading, updating, and deleting files and directories
- Explore the relationship between MFS and UnixFS
- Understand how MFS maintains mutability on top of immutable IPFS blocks

## 📋 Prerequisites

- **Previous Chapters**: 05-dag-ipld, 06-unixfs-car (understanding of UnixFS structure)
- **Technical Knowledge**: File system concepts, IPFS content addressing, DAG structures
- **Go Experience**: Context handling, error management, file I/O operations

## 🔑 Core Concepts

### What is MFS?

MFS (Mutable File System) provides a familiar file system interface over IPFS's content-addressed storage. While IPFS content is immutable by nature, MFS creates the illusion of mutability by:

1. **Root Management**: Maintaining a mutable root that points to the current state
2. **Copy-on-Write**: Creating new versions when files are modified
3. **Path Resolution**: Translating file paths to IPFS content addresses
4. **Directory Updates**: Efficiently updating directory structures

### Key Features

- **File System Interface**: Standard file operations (create, read, update, delete)
- **Directory Operations**: mkdir, rmdir, listing contents
- **Path-based Access**: Use familiar file paths like `/documents/readme.txt`
- **Atomic Updates**: Changes are atomic at the root level
- **Version History**: Previous versions remain accessible via their CIDs

### MFS vs UnixFS

| Aspect | UnixFS | MFS |
|--------|--------|-----|
| **Mutability** | Immutable | Mutable interface |
| **Access Pattern** | Content-addressed (CID) | Path-based |
| **Use Case** | Content storage/sharing | File system operations |
| **State** | Static once created | Dynamic, updatable |

## 💻 Code Analysis

### Core Structure

```go
type MFSWrapper struct {
    *unixfs.UnixFsWrapper  // Underlying UnixFS functionality
    root *mfs.Root         // MFS root for mutable operations
    cur  cid.Cid          // Current root CID
}
```

The wrapper builds on UnixFS and adds:
- **Mutable Root**: Tracks the current state of the file system
- **Path Operations**: Enables file system-style operations
- **State Management**: Maintains consistency across operations

### Key Methods

#### 1. File Operations

```go
// Write data to a file path
func (m *MFSWrapper) WriteBytes(ctx context.Context, path string, data []byte, create bool) error

// Read data from a file path
func (m *MFSWrapper) ReadBytes(ctx context.Context, path string) ([]byte, error)

// Remove a file or directory
func (m *MFSWrapper) Remove(ctx context.Context, path string) error
```

#### 2. Directory Operations

```go
// Create a directory
func (m *MFSWrapper) Mkdir(ctx context.Context, path string) error

// List directory contents
func (m *MFSWrapper) List(ctx context.Context, path string) ([]os.FileInfo, error)
```

#### 3. State Management

```go
// Get current root CID
func (m *MFSWrapper) GetCid() cid.Cid

// Flush changes to get new root CID
func (m *MFSWrapper) Flush(ctx context.Context) (cid.Cid, error)
```

### Implementation Details

#### Root Management
```go
func New(ctx context.Context, ufs *unixfs.UnixFsWrapper, c cid.Cid) (*MFSWrapper, error) {
    if c == cid.Undef {
        // Create empty root for new file system
        root, err = mfs.NewEmptyRoot(ctx, ufs.DAGService, dummypf, nil, mfs.MkdirOpts{})
    } else {
        // Load existing file system from CID
        nd, err := ufs.IpldWrapper.Get(ctx, c)
        root, err = mfs.NewRoot(ctx, ufs.DAGService, nd, dummypf)
    }
}
```

#### Write Operations
```go
func (m *MFSWrapper) WriteBytes(ctx context.Context, path string, data []byte, create bool) error {
    // Navigate to parent directory
    dirname, filename := path.Split(path)

    // Create file node
    fileNode, err := m.UnixFsWrapper.CreateFile(ctx, bytes.NewReader(data))

    // Add to MFS directory structure
    dir := m.root.GetDirectory()
    err = mfs.PutNode(dir, filename, fileNode)

    return err
}
```

## 🏃‍♂️ Practical Usage

### Example 1: Creating a Simple File System

```bash
cd 07-mfs
go run main.go
```

**Expected Output:**
```
=== MFS (Mutable File System) Demo ===

📁 Creating new MFS instance...
✅ MFS created with empty root

📝 Writing files to MFS...
   Writing /notes/day1.md
   Writing /notes/day2.md
   Writing /config.json

📋 Current MFS structure:
/notes/
  day1.md (25 bytes)
  day2.md (30 bytes)
/config.json (45 bytes)

🔄 Updating existing file...
   Updating /notes/day1.md

💾 Flushing MFS to get root CID...
✅ New root CID: bafkreif2hf4q5j7l8x9k3m...
```

### Example 2: Loading from Existing CID

```go
// Load MFS from existing root CID
existingCID := "bafkreif2hf4q5j7l8x9k3m..."
mfsWrapper, err := mfs.New(ctx, unixfsWrapper, cid.MustParse(existingCID))

// Continue working with existing file system
files, err := mfsWrapper.List(ctx, "/documents")
```

### Example 3: File System Operations

```go
// Create directory structure
err = mfsWrapper.Mkdir(ctx, "/projects/web-app")
err = mfsWrapper.Mkdir(ctx, "/projects/web-app/src")

// Write application files
err = mfsWrapper.WriteBytes(ctx, "/projects/web-app/README.md",
    []byte("# Web Application\n\nA sample web app"), true)

err = mfsWrapper.WriteBytes(ctx, "/projects/web-app/src/main.js",
    []byte("console.log('Hello, World!');"), true)

// Read and modify files
content, err := mfsWrapper.ReadBytes(ctx, "/projects/web-app/README.md")
updatedContent := string(content) + "\n\n## Features\n- Responsive design"
err = mfsWrapper.WriteBytes(ctx, "/projects/web-app/README.md",
    []byte(updatedContent), false)

// Get final state
rootCID, err := mfsWrapper.Flush(ctx)
fmt.Printf("Final root CID: %s\n", rootCID)
```

## 🔍 Key Features Demonstrated

### 1. **Mutable File Operations**
- Create, read, update, delete files using familiar paths
- Atomic operations with consistent state

### 2. **Directory Management**
- Create nested directory structures
- List directory contents with file information
- Remove files and directories

### 3. **State Persistence**
- Flush changes to obtain immutable CID
- Load existing file systems from CIDs
- Maintain version history through CIDs

### 4. **Integration with UnixFS**
- Built on top of UnixFS for compatibility
- Leverage existing IPFS file representations
- Seamless transition between mutable and immutable views

## 🧪 Running Tests

```bash
# Run all MFS tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific test
go test -v -run TestMFSWrapper_WriteAndRead

# Test with race detection
go test -race ./...
```

### Test Coverage

The test suite covers:
- ✅ File creation and reading
- ✅ Directory operations
- ✅ File updates and deletions
- ✅ State management and flushing
- ✅ Error handling for invalid operations
- ✅ Integration with UnixFS

## 🔗 Integration Examples

### With Gateway (09-gateway)

```go
// Serve MFS content through HTTP gateway
mfsWrapper, _ := mfs.New(ctx, unixfsWrapper, cid.Undef)

// Populate with content
mfsWrapper.WriteBytes(ctx, "/index.html", htmlContent, true)
mfsWrapper.WriteBytes(ctx, "/style.css", cssContent, true)

// Get root CID for gateway
rootCID, _ := mfsWrapper.Flush(ctx)

// Access via gateway: http://localhost:8080/ipfs/{rootCID}/index.html
```

### With IPNS (09-ipns)

```go
// Publish MFS state to IPNS for mutable addressing
mfsWrapper, _ := mfs.New(ctx, unixfsWrapper, cid.Undef)

// Update content regularly
go func() {
    for {
        // Update content
        timestamp := time.Now().Format(time.RFC3339)
        content := fmt.Sprintf("Last updated: %s", timestamp)
        mfsWrapper.WriteBytes(ctx, "/status.txt", []byte(content), true)

        // Publish to IPNS
        rootCID, _ := mfsWrapper.Flush(ctx)
        ipnsWrapper.PublishRecord(ctx, rootCID, privateKey)

        time.Sleep(5 * time.Minute)
    }
}()
```

## 🎯 Use Cases

### 1. **Content Management Systems**
- Blog platforms with file-based content
- Documentation systems
- Static site generators

### 2. **Development Workflows**
- Version-controlled file systems
- Build artifacts management
- Configuration management

### 3. **Data Applications**
- Dataset versioning
- Collaborative document editing
- Backup and archival systems

### 4. **Real-time Applications**
- Live content updates
- Dynamic web applications
- IoT data logging

## 🔧 Advanced Configuration

### Custom MFS Options

```go
// Configure MFS with custom options
opts := mfs.MkdirOpts{
    Flush: true,     // Auto-flush on operations
    CidBuilder: nil, // Custom CID builder
}

root, err := mfs.NewEmptyRoot(ctx, dagService, pinFunc, opts)
```

### Performance Tuning

```go
// Batch operations for better performance
mfsWrapper.WriteBytes(ctx, "/batch/file1.txt", data1, true)
mfsWrapper.WriteBytes(ctx, "/batch/file2.txt", data2, true)
mfsWrapper.WriteBytes(ctx, "/batch/file3.txt", data3, true)

// Single flush for all operations
rootCID, err := mfsWrapper.Flush(ctx)
```

## 🔗 Next Steps

After mastering MFS, explore:

1. **08-pin-gc**: Pin important MFS states and manage storage
2. **09-ipns**: Publish MFS content with mutable addresses
3. **10-gateway**: Serve MFS content over HTTP
4. **Advanced Topics**: Conflict resolution, collaborative editing

## 🐛 Troubleshooting

### Common Issues

1. **Path Not Found**
   ```
   Error: no such file or directory
   ```
   - Ensure parent directories exist
   - Use `Mkdir` to create directory structure

2. **Write Permission**
   ```
   Error: operation not permitted
   ```
   - Set `create: true` for new files
   - Check path formatting (use forward slashes)

3. **State Inconsistency**
   ```
   Error: root changed during operation
   ```
   - Call `Flush()` to get consistent state
   - Avoid concurrent modifications

### Debug Tips

```go
// Enable debug logging
import "log"

// Check current state
files, err := mfsWrapper.List(ctx, "/")
log.Printf("Current files: %+v", files)

// Verify file existence
exists, err := mfsWrapper.Exists(ctx, "/path/to/file")
log.Printf("File exists: %v", exists)
```

## 📚 Additional Resources

- [IPFS MFS Documentation](https://docs.ipfs.io/concepts/file-systems/#mutable-file-system-mfs)
- [Boxo MFS Package](https://pkg.go.dev/github.com/ipfs/boxo/mfs)
- [UnixFS Specification](https://github.com/ipfs/specs/blob/master/UNIXFS.md)
- [Content Addressing Guide](https://docs.ipfs.io/concepts/content-addressing/)

---

MFS bridges the gap between IPFS's content-addressed storage and traditional file system interfaces, enabling powerful applications that combine the benefits of both paradigms.