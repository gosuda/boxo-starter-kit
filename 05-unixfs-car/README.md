# 03-unixfs: File System Abstraction and Large File Processing

## 🎯 Learning Objectives

Through this module, you will learn:
- The concept of **UnixFS** and how files are represented in IPFS
- **Chunking** strategies and large file processing
- How **files and directories** are stored in IPFS structure
- Efficient file verification through **Merkle DAG**
- **Streaming**-based file I/O and performance optimization
- **File metadata** management and MIME type handling

## 📋 Prerequisites

- **00-block-cid** module completed (understanding Blocks and CIDs)
- **01-persistent** module completed (understanding data persistence)
- **02-dag-ipld** module completed (understanding DAG and IPLD)
- Basic file system concepts (files, directories, paths)
- Stream processing and I/O concepts

## 🔑 Core Concepts

### What is UnixFS?

**UnixFS** is a data format for representing files and directories in IPFS:

```
Traditional filesystem: /home/user/document.txt
UnixFS in IPFS: QmHash... (file content) + metadata
```

### Chunking Strategy

Large files are divided into smaller chunks for storage:

```
Large file (10MB)
    ↓ chunking
Chunk1 (256KB) → Chunk2 (256KB) → ... → Chunk40 (256KB)
    ↓ Merkle DAG
      Root
    ↙  ↓  ↘
  C1   C2   C3...
```

### File Structure Hierarchy

```
UnixFS Node
├─ Type: File | Directory | Symlink
├─ Data: actual content or chunk references
├─ Links: CID links to child chunks/files
└─ Metadata: size, permissions, timestamp, etc.
```

### Advantages of Merkle DAG

1. **Integrity verification**: verify entire file with a single hash
2. **Efficient synchronization**: transfer only changed chunks
3. **Deduplication**: identical chunks stored only once
4. **Parallel processing**: independent processing per chunk

## 💻 Code Analysis

### 1. UnixFS Wrapper Design

```go
// pkg/unixfs.go:21-31
type UnixFsWrapper struct {
    dagService   format.DAGService
    dagWrapper   *dag.DagWrapper
    chunker      chunk.Splitter
    maxChunkSize int
}

func New(maxChunkSize int) (*UnixFsWrapper, error) {
    if maxChunkSize <= 0 {
        maxChunkSize = DefaultChunkSize // 256KB
    }
    // Initialize DAG service and chunker
}
```

**Design Features**:
- Reuse **dag.DagWrapper** for IPLD functionality
- Flexible chunking strategy with **chunk.Splitter**
- **Configurable chunk size** for use-case optimization

### 2. File Storage (with Chunking)

```go
// pkg/unixfs.go:60-95
func (ufs *UnixFsWrapper) Put(ctx context.Context, file files.File) (cid.Cid, error) {
    // 1. Split file into chunks
    chunker := chunk.NewSizeSplitter(file, int64(ufs.maxChunkSize))

    // 2. Store chunks individually
    var links []*format.Link
    totalSize := uint64(0)

    for {
        chunk, err := chunker.NextBytes()
        if err != nil {
            if err == io.EOF {
                break
            }
            return cid.Undef, fmt.Errorf("failed to get next chunk: %w", err)
        }

        // 3. Create each chunk as UnixFS node
        chunkNode := &dag.ProtoNode{}
        chunkNode.SetData(unixfs.FilePBData(chunk, uint64(len(chunk))))

        // 4. Store chunk and create link
        err = ufs.dagService.Add(ctx, chunkNode)
        if err != nil {
            return cid.Undef, fmt.Errorf("failed to add chunk: %w", err)
        }

        links = append(links, &format.Link{
            Name: "",
            Size: uint64(len(chunk)),
            Cid:  chunkNode.Cid(),
        })
        totalSize += uint64(len(chunk))
    }

    // 5. Create root node (links all chunks)
    rootNode := &dag.ProtoNode{}
    rootNode.SetLinks(links)
    rootNode.SetData(unixfs.FilePBData(nil, totalSize))

    err = ufs.dagService.Add(ctx, rootNode)
    return rootNode.Cid(), err
}
```

**Core Process**:
1. **Chunking**: Split file into configured size chunks
2. **Chunk Storage**: Store each chunk as UnixFS node
3. **Link Collection**: Collect chunk CIDs
4. **Root Creation**: Root node linking all chunks
5. **Metadata**: Include file size and other information

### 3. File Retrieval (Streaming)

```go
// pkg/unixfs.go:98-130
func (ufs *UnixFsWrapper) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
    // 1. Retrieve root node
    rootNode, err := ufs.dagService.Get(ctx, c)
    if err != nil {
        return nil, fmt.Errorf("failed to get root node: %w", err)
    }

    // 2. Parse UnixFS metadata
    fsNode, err := unixfs.FSNodeFromBytes(rootNode.RawData())
    if err != nil {
        return nil, fmt.Errorf("failed to parse UnixFS node: %w", err)
    }

    switch fsNode.Type() {
    case unixfs.TFile:
        return ufs.getFile(ctx, rootNode, fsNode)
    case unixfs.TDirectory:
        return ufs.getDirectory(ctx, rootNode, fsNode)
    default:
        return nil, fmt.Errorf("unsupported UnixFS type: %v", fsNode.Type())
    }
}

func (ufs *UnixFsWrapper) getFile(ctx context.Context, rootNode format.Node,
                                 fsNode *unixfs.FSNode) (files.File, error) {
    // 3. Create file reader (streaming)
    dagReader, err := uio.NewDagReader(ctx, rootNode, ufs.dagService)
    if err != nil {
        return nil, fmt.Errorf("failed to create DAG reader: %w", err)
    }

    // 4. Return stream that reads chunks sequentially
    return files.NewReaderFile(dagReader), nil
}
```

### 4. Directory Processing

```go
// pkg/unixfs.go:170-200
func (ufs *UnixFsWrapper) putDirectory(ctx context.Context, dir files.Directory) (cid.Cid, error) {
    // 1. Create directory node
    dirNode := &dag.ProtoNode{}
    dirNode.SetData(unixfs.FolderPBData())

    // 2. Process files/subdirectories in directory
    entries := dir.Entries()
    for entries.Next() {
        entry := entries.Node()
        entryName := entries.Name()

        // 3. Recursively process child entries
        var entryCID cid.Cid
        var err error

        switch entry := entry.(type) {
        case files.File:
            entryCID, err = ufs.Put(ctx, entry)
        case files.Directory:
            entryCID, err = ufs.putDirectory(ctx, entry)
        default:
            return cid.Undef, fmt.Errorf("unsupported file type")
        }

        if err != nil {
            return cid.Undef, fmt.Errorf("failed to add entry %s: %w", entryName, err)
        }

        // 4. Add directory link
        err = dirNode.AddNodeLink(entryName, &dag.ProtoNode{})
        if err != nil {
            return cid.Undef, fmt.Errorf("failed to add link: %w", err)
        }
    }

    // 5. Store directory node
    err := ufs.dagService.Add(ctx, dirNode)
    return dirNode.Cid(), err
}
```

### 5. Path-based File Access

```go
// pkg/unixfs.go:240-270
func (ufs *UnixFsWrapper) GetPath(ctx context.Context, rootCID cid.Cid, path string) (files.Node, error) {
    if path == "" || path == "/" {
        return ufs.Get(ctx, rootCID)
    }

    // 1. Parse path
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")
    currentCID := rootCID

    // 2. Navigate through path sequentially
    for _, segment := range pathSegments {
        // Get current node
        currentNode, err := ufs.dagService.Get(ctx, currentCID)
        if err != nil {
            return nil, fmt.Errorf("failed to get node: %w", err)
        }

        // Parse UnixFS metadata
        fsNode, err := unixfs.FSNodeFromBytes(currentNode.RawData())
        if err != nil {
            return nil, fmt.Errorf("failed to parse UnixFS node: %w", err)
        }

        // Check if it's a directory
        if fsNode.Type() != unixfs.TDirectory {
            return nil, fmt.Errorf("path segment '%s' is not a directory", segment)
        }

        // 3. Find link with matching name in directory
        found := false
        for _, link := range currentNode.Links() {
            if link.Name == segment {
                currentCID = link.Cid
                found = true
                break
            }
        }

        if !found {
            return nil, fmt.Errorf("path segment '%s' not found", segment)
        }
    }

    // 4. Return final node
    return ufs.Get(ctx, currentCID)
}
```

## 🏃‍♂️ Hands-on Guide

### 1. Basic Execution

```bash
cd 03-unixfs
go run main.go
```

**Expected Output**:
```
=== UnixFS Demo ===

1. Setting up UnixFS with 256KB chunks:
   ✅ UnixFS initialized with chunk size: 262144 bytes

2. Adding various file types:
   📄 Adding text file:
   ✅ Text file → bafkreigh2akiscai...
   📄 Adding binary file:
   ✅ Binary file → bafkreibc4uoyerf...
   📄 Adding large file (1MB):
   ✅ Large file → bafybeihdwdcwfw...

3. File retrieval and verification:
   ✅ Text file content matches
   ✅ Binary file content matches
   ✅ Large file content matches

4. Directory operations:
   📁 Creating nested directory structure:
   ✅ Directory → bafybeigqkjhkr3y...

   📂 Directory listing:
   ├─ 📄 readme.txt (245 bytes)
   ├─ 📄 data.json (156 bytes)
   └─ 📁 subdir/
       └─ 📄 nested.txt (89 bytes)

5. Path-based file access:
   ✅ /readme.txt → "This is a README file..."
   ✅ /subdir/nested.txt → "Nested file content"
   ✅ /data.json → {"name": "test", "value": 42}

6. Chunking demonstration:
   📊 Large file chunking analysis:
      File size: 1048576 bytes
      Chunk size: 262144 bytes
      Number of chunks: 4
      Chunk distribution: [262144, 262144, 262144, 262144]
```

### 2. Chunking Strategy Experiments

Compare performance with different chunk sizes:

```bash
# Small chunks (64KB)
UnixFS_CHUNK_SIZE=65536 go run main.go

# Large chunks (1MB)
UnixFS_CHUNK_SIZE=1048576 go run main.go

# Default (256KB)
go run main.go
```

**Observation Points**:
- Smaller chunk sizes create more chunks
- Larger chunk sizes increase memory usage
- Optimal chunk size varies with network conditions

### 3. Large File Processing Test

```go
// Test 10MB file creation and processing
func testLargeFile() {
    largeData := make([]byte, 10*1024*1024)
    for i := range largeData {
        largeData[i] = byte(i % 256)
    }

    file := files.NewBytesFile(largeData)
    cid, err := unixfsWrapper.Put(ctx, file)
    // Observe chunking and memory usage
}
```

### 4. Running Tests

```bash
go test -v ./...
```

**Key Test Cases**:
- ✅ Small file storage/retrieval
- ✅ Large file chunking
- ✅ Directory structure creation
- ✅ Path-based access
- ✅ Streaming I/O performance

## 🔍 Advanced Use Cases

### 1. Website Hosting

```go
type WebsiteBuilder struct {
    unixfs *UnixFsWrapper
}

func (wb *WebsiteBuilder) BuildSite(sitePath string) (cid.Cid, error) {
    // 1. Collect HTML, CSS, JS files
    var htmlFiles []files.File
    var assetFiles []files.File

    err := filepath.Walk(sitePath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if !info.IsDir() {
            file, err := os.Open(path)
            if err != nil {
                return err
            }

            switch filepath.Ext(path) {
            case ".html":
                htmlFiles = append(htmlFiles, files.NewReaderFile(file))
            case ".css", ".js", ".png", ".jpg":
                assetFiles = append(assetFiles, files.NewReaderFile(file))
            }
        }
        return nil
    })

    // 2. Create website directory structure
    websiteDir := files.NewMapDirectory(map[string]files.Node{
        "index.html": htmlFiles[0],
        "assets":     files.NewMapDirectory(assetsMap),
    })

    // 3. Add entire site to IPFS
    return wb.unixfs.Put(ctx, websiteDir)
}
```

### 2. Version Control System

```go
type VersionedFile struct {
    Content     []byte            `json:"content"`
    Version     int               `json:"version"`
    PreviousRef map[string]string `json:"previous,omitempty"` // CID links
    Timestamp   string            `json:"timestamp"`
    Author      string            `json:"author"`
    Message     string            `json:"message"`
}

func (vcs *VersionControlSystem) CommitFile(content []byte, message, author string,
                                          previousCID *cid.Cid) (cid.Cid, error) {
    version := 1
    var previousRef map[string]string

    if previousCID != nil {
        // Get previous version info to increment version number
        version = getPreviousVersion(*previousCID) + 1
        previousRef = map[string]string{"/": previousCID.String()}
    }

    versionedFile := VersionedFile{
        Content:     content,
        Version:     version,
        PreviousRef: previousRef,
        Timestamp:   time.Now().Format(time.RFC3339),
        Author:      author,
        Message:     message,
    }

    // Serialize to JSON and store
    data, _ := json.Marshal(versionedFile)
    file := files.NewBytesFile(data)
    return vcs.unixfs.Put(ctx, file)
}
```

### 3. Distributed File Backup

```go
type BackupSystem struct {
    unixfs *UnixFsWrapper
}

func (bs *BackupSystem) BackupDirectory(sourcePath string) (*BackupManifest, error) {
    manifest := &BackupManifest{
        Timestamp: time.Now(),
        Files:     make(map[string]FileInfo),
    }

    err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        // 1. Calculate file hash (for deduplication)
        fileHash := calculateFileHash(path)

        // 2. Upload only if not duplicate
        if !bs.isDuplicate(fileHash) {
            file, err := os.Open(path)
            if err != nil {
                return err
            }
            defer file.Close()

            cid, err := bs.unixfs.Put(ctx, files.NewReaderFile(file))
            if err != nil {
                return err
            }

            // 3. Record file info in manifest
            relativePath, _ := filepath.Rel(sourcePath, path)
            manifest.Files[relativePath] = FileInfo{
                CID:      cid.String(),
                Size:     info.Size(),
                ModTime:  info.ModTime(),
                Hash:     fileHash,
            }
        }

        return nil
    })

    // 4. Store manifest itself in IPFS
    manifestData, _ := json.Marshal(manifest)
    manifestFile := files.NewBytesFile(manifestData)
    manifestCID, err := bs.unixfs.Put(ctx, manifestFile)

    manifest.ManifestCID = manifestCID.String()
    return manifest, err
}
```

### 4. Media Streaming

```go
type MediaStreamer struct {
    unixfs *UnixFsWrapper
}

func (ms *MediaStreamer) StreamVideo(videoCID cid.Cid,
                                   startByte, endByte int64) (io.Reader, error) {
    // 1. Get video file node
    videoNode, err := ms.unixfs.Get(ctx, videoCID)
    if err != nil {
        return nil, err
    }

    videoFile, ok := videoNode.(files.File)
    if !ok {
        return nil, fmt.Errorf("not a file")
    }

    // 2. Range-based reading (HTTP Range Request support)
    if startByte > 0 {
        _, err = videoFile.Seek(startByte, io.SeekStart)
        if err != nil {
            return nil, err
        }
    }

    // 3. Return reader limited to specified size
    if endByte > startByte {
        return io.LimitReader(videoFile, endByte-startByte+1), nil
    }

    return videoFile, nil
}

func (ms *MediaStreamer) CreateVideoManifest(videoCID cid.Cid) (*HLSManifest, error) {
    // Create HLS (HTTP Live Streaming) manifest
    manifest := &HLSManifest{
        Version:    3,
        TargetDuration: 10,
        Segments:   []HLSSegment{},
    }

    // Split video into 10-second segments, each with its own CID
    // In real implementation, use FFmpeg or similar for segment splitting

    return manifest, nil
}
```

## ⚠️ Best Practices and Considerations

### 1. Chunk Size Selection

```go
// ✅ Optimal chunk sizes by use case
func selectChunkSize(fileSize int64, networkCondition string) int {
    switch {
    case fileSize < 1*1024*1024: // Under 1MB
        return 64 * 1024 // 64KB
    case networkCondition == "slow":
        return 128 * 1024 // 128KB
    case networkCondition == "fast":
        return 1024 * 1024 // 1MB
    default:
        return 256 * 1024 // 256KB (default)
    }
}
```

### 2. Memory-efficient Large File Processing

```go
// ✅ Streaming-based processing
func processLargeFile(filePath string, ufs *UnixFsWrapper) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()

    // Process as stream without loading entire file into memory
    readerFile := files.NewReaderFile(file)
    _, err = ufs.Put(ctx, readerFile)
    return err
}

// ❌ Wrong approach: Risk of memory exhaustion
func processLargeFileWrong(filePath string, ufs *UnixFsWrapper) error {
    data, err := ioutil.ReadFile(filePath) // Load entire file into memory
    if err != nil {
        return err
    }

    file := files.NewBytesFile(data)
    _, err = ufs.Put(ctx, file)
    return err
}
```

### 3. Path Normalization

```go
// ✅ Safe path handling
func normalizePath(path string) string {
    // Prevent path traversal attacks
    path = filepath.Clean(path)

    // Convert absolute path to relative path
    if filepath.IsAbs(path) {
        path = path[1:]
    }

    // Handle empty paths
    if path == "." {
        return ""
    }

    return path
}
```

### 4. MIME Type Handling

```go
// ✅ Automatic MIME type detection
func detectMimeType(filename string, content []byte) string {
    // 1. Extension-based detection
    ext := filepath.Ext(filename)
    if mimeType := mime.TypeByExtension(ext); mimeType != "" {
        return mimeType
    }

    // 2. Content-based detection
    return http.DetectContentType(content)
}

func addFileWithMetadata(ufs *UnixFsWrapper, filename string,
                        content []byte) (cid.Cid, error) {
    mimeType := detectMimeType(filename, content)

    // Store with metadata
    fileWithMeta := struct {
        Content  []byte `json:"content"`
        Filename string `json:"filename"`
        MimeType string `json:"mime_type"`
        Size     int64  `json:"size"`
    }{
        Content:  content,
        Filename: filename,
        MimeType: mimeType,
        Size:     int64(len(content)),
    }

    data, _ := json.Marshal(fileWithMeta)
    file := files.NewBytesFile(data)
    return ufs.Put(ctx, file)
}
```

## 🔧 Troubleshooting

### Issue 1: "chunk too large" error

**Cause**: Chunk size exceeds system limits
```go
// Solution: Limit chunk size
const MaxChunkSize = 1024 * 1024 // 1MB

func validateChunkSize(chunkSize int) int {
    if chunkSize > MaxChunkSize {
        log.Printf("Chunk size %d exceeds maximum, using %d", chunkSize, MaxChunkSize)
        return MaxChunkSize
    }
    return chunkSize
}
```

### Issue 2: "out of memory" error

**Cause**: Loading large files entirely into memory
```go
// Solution: Streaming processing
func processFileStream(reader io.Reader, ufs *UnixFsWrapper) (cid.Cid, error) {
    // Use io.Reader directly to minimize memory usage
    file := files.NewReaderFile(reader)
    return ufs.Put(ctx, file)
}
```

### Issue 3: "path not found" error

**Cause**: Invalid path or directory structure
```go
// Solution: Path validation
func validatePath(ufs *UnixFsWrapper, rootCID cid.Cid, path string) error {
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")

    for i := 0; i < len(pathSegments); i++ {
        partialPath := "/" + strings.Join(pathSegments[:i+1], "/")
        _, err := ufs.GetPath(ctx, rootCID, partialPath)
        if err != nil {
            return fmt.Errorf("invalid path at '%s': %w", partialPath, err)
        }
    }
    return nil
}
```

### Issue 4: Chunking inconsistency

**Cause**: Accessing files stored with different chunk sizes
```go
// Solution: Store chunk info in metadata
type FileMetadata struct {
    ChunkSize    int    `json:"chunk_size"`
    TotalSize    int64  `json:"total_size"`
    ChunkCount   int    `json:"chunk_count"`
    Algorithm    string `json:"algorithm"`
}

func putFileWithMetadata(ufs *UnixFsWrapper, file files.File,
                        chunkSize int) (cid.Cid, error) {
    // Store file with metadata
    metadata := FileMetadata{
        ChunkSize:  chunkSize,
        Algorithm:  "size-splitter",
    }

    // Create wrapper structure with metadata
    wrapper := struct {
        Metadata FileMetadata `json:"metadata"`
        Content  string       `json:"content"` // actual file CID
    }{
        Metadata: metadata,
    }

    // Store actual file
    fileCID, err := ufs.Put(ctx, file)
    if err != nil {
        return cid.Undef, err
    }

    wrapper.Content = fileCID.String()

    // Store wrapper
    wrapperData, _ := json.Marshal(wrapper)
    wrapperFile := files.NewBytesFile(wrapperData)
    return ufs.Put(ctx, wrapperFile)
}
```

## 📊 Performance Optimization

### 1. Parallel Chunking

```go
// ✅ Parallel chunk processing
func putFileParallel(ufs *UnixFsWrapper, file files.File) (cid.Cid, error) {
    const workers = 4
    chunkQueue := make(chan []byte, workers*2)
    resultQueue := make(chan chunkResult, workers*2)

    // Start workers
    for i := 0; i < workers; i++ {
        go func() {
            for chunk := range chunkQueue {
                chunkFile := files.NewBytesFile(chunk)
                cid, err := ufs.Put(ctx, chunkFile)
                resultQueue <- chunkResult{cid: cid, err: err}
            }
        }()
    }

    // Chunking and queue sending
    chunker := chunk.NewSizeSplitter(file, int64(ufs.maxChunkSize))
    var chunkCount int

    go func() {
        defer close(chunkQueue)
        for {
            chunk, err := chunker.NextBytes()
            if err == io.EOF {
                break
            }
            if err != nil {
                log.Printf("Chunking error: %v", err)
                return
            }
            chunkQueue <- chunk
            chunkCount++
        }
    }()

    // Collect results
    var links []*format.Link
    for i := 0; i < chunkCount; i++ {
        result := <-resultQueue
        if result.err != nil {
            return cid.Undef, result.err
        }
        links = append(links, &format.Link{Cid: result.cid})
    }

    // Create root node
    return createRootNode(links)
}
```

### 2. Intelligent Caching

```go
// ✅ Cache frequently accessed files with LRU cache
type CachedUnixFS struct {
    *UnixFsWrapper
    cache *lru.Cache
}

func (cufs *CachedUnixFS) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
    // Check cache
    if cached, ok := cufs.cache.Get(c.String()); ok {
        return cached.(files.Node), nil
    }

    // Cache miss - perform actual lookup
    node, err := cufs.UnixFsWrapper.Get(ctx, c)
    if err != nil {
        return nil, err
    }

    // Store in cache (with size limit)
    if estimatedSize(node) < MaxCacheNodeSize {
        cufs.cache.Add(c.String(), node)
    }

    return node, nil
}
```

### 3. Compression Support

```go
// ✅ Auto-compression to save storage space
func putFileWithCompression(ufs *UnixFsWrapper, file files.File,
                          compress bool) (cid.Cid, error) {
    if !compress {
        return ufs.Put(ctx, file)
    }

    // Apply gzip compression
    var buf bytes.Buffer
    gzipWriter := gzip.NewWriter(&buf)

    _, err := io.Copy(gzipWriter, file)
    if err != nil {
        return cid.Undef, err
    }

    err = gzipWriter.Close()
    if err != nil {
        return cid.Undef, err
    }

    // Store compressed data with metadata
    compressed := CompressedFile{
        Data:             buf.Bytes(),
        OriginalSize:     getFileSize(file),
        CompressionType:  "gzip",
        Compressed:       true,
    }

    compressedData, _ := json.Marshal(compressed)
    compressedFile := files.NewBytesFile(compressedData)
    return ufs.Put(ctx, compressedFile)
}
```

## 📚 Additional Learning Resources

### Related Documentation
- [UnixFS Specification](https://github.com/ipfs/specs/blob/master/UNIXFS.md)
- [IPFS File API](https://docs.ipfs.io/reference/kubo/rpc/#api-v0-add)
- [Merkle DAG Structure](https://docs.ipfs.io/concepts/merkle-dag/)
- [Chunking Strategies](https://docs.ipfs.io/concepts/file-systems/#chunking)

### Next Steps
1. **04-network-bitswap**: File block exchange in P2P networks
2. **05-pin-gc**: File lifecycle management and garbage collection
3. **06-gateway**: HTTP access to files via web

## 🎓 Practice Exercises

### Basic Exercises
1. Store files of various sizes and compare chunking results
2. Create directory structures and access files through paths
3. Store the same file with different chunk sizes and verify CID differences

### Advanced Exercises
1. Store image files and create a thumbnail generation system
2. Implement a version control system for text files
3. Design a system for efficiently storing and searching large log files

### Practical Projects
1. Create a static website hosting system
2. Implement a file backup and restore tool
3. Design the basic structure of a media streaming service

Now you understand how to efficiently handle files and directories in IPFS. In the next module, we'll learn how to share this data in P2P networks! 🚀