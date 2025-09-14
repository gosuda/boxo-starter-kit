# 03-unixfs: 파일시스템 추상화와 대용량 파일 처리

## 🎯 학습 목표

이 모듈을 통해 다음을 학습할 수 있습니다:
- **UnixFS**의 개념과 IPFS에서 파일을 표현하는 방법
- **청킹(Chunking)** 전략과 대용량 파일 처리
- **파일과 디렉터리** 구조의 IPFS 저장 방식
- **Merkle DAG**를 통한 효율적인 파일 검증
- **스트리밍** 기반 파일 입출력과 성능 최적화
- **파일 메타데이터** 관리와 MIME 타입 처리

## 📋 사전 요구사항

- **00-block-cid** 모듈 완료 (Block과 CID 이해)
- **01-persistent** 모듈 완료 (데이터 영속성 이해)
- **02-dag-ipld** 모듈 완료 (DAG와 IPLD 이해)
- 파일시스템의 기본 개념 (파일, 디렉터리, 경로)
- 스트림 처리와 I/O 개념

## 🔑 핵심 개념

### UnixFS란?

**UnixFS**는 IPFS에서 파일과 디렉터리를 표현하기 위한 데이터 형식입니다:

```
일반 파일시스템: /home/user/document.txt
UnixFS in IPFS: QmHash... (파일 내용) + 메타데이터
```

### 청킹(Chunking) 전략

대용량 파일은 작은 청크로 분할되어 저장됩니다:

```
큰 파일 (10MB)
    ↓ 청킹
Chunk1 (256KB) → Chunk2 (256KB) → ... → Chunk40 (256KB)
    ↓ Merkle DAG
      Root
    ↙  ↓  ↘
  C1   C2   C3...
```

### 파일 구조 계층

```
UnixFS Node
├─ Type: File | Directory | Symlink
├─ Data: 실제 내용 또는 청크 참조
├─ Links: 하위 청크/파일에 대한 CID 링크
└─ Metadata: 크기, 권한, 타임스탬프 등
```

### Merkle DAG의 장점

1. **무결성 검증**: 단일 해시로 전체 파일 검증
2. **효율적 동기화**: 변경된 청크만 전송
3. **중복 제거**: 동일한 청크는 한 번만 저장
4. **병렬 처리**: 청크 단위 독립적 처리

## 💻 코드 분석

### 1. UnixFS Wrapper 설계

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
    // DAG 서비스와 청커 초기화
}
```

**설계 특징**:
- **dag.DagWrapper** 재사용으로 IPLD 기능 활용
- **chunk.Splitter**로 유연한 청킹 전략
- **설정 가능한 청크 크기**로 용도별 최적화

### 2. 파일 저장 (청킹 포함)

```go
// pkg/unixfs.go:60-95
func (ufs *UnixFsWrapper) Put(ctx context.Context, file files.File) (cid.Cid, error) {
    // 1. 파일을 청크로 분할
    chunker := chunk.NewSizeSplitter(file, int64(ufs.maxChunkSize))

    // 2. 청크들을 개별적으로 저장
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

        // 3. 각 청크를 UnixFS 노드로 생성
        chunkNode := &dag.ProtoNode{}
        chunkNode.SetData(unixfs.FilePBData(chunk, uint64(len(chunk))))

        // 4. 청크 저장 및 링크 생성
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

    // 5. 루트 노드 생성 (모든 청크를 링크)
    rootNode := &dag.ProtoNode{}
    rootNode.SetLinks(links)
    rootNode.SetData(unixfs.FilePBData(nil, totalSize))

    err = ufs.dagService.Add(ctx, rootNode)
    return rootNode.Cid(), err
}
```

**핵심 과정**:
1. **청킹**: 파일을 설정된 크기로 분할
2. **청크 저장**: 각 청크를 UnixFS 노드로 저장
3. **링크 수집**: 청크 CID들을 수집
4. **루트 생성**: 모든 청크를 링크하는 루트 노드
5. **메타데이터**: 파일 크기 등 정보 포함

### 3. 파일 검색 (스트리밍)

```go
// pkg/unixfs.go:98-130
func (ufs *UnixFsWrapper) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
    // 1. 루트 노드 조회
    rootNode, err := ufs.dagService.Get(ctx, c)
    if err != nil {
        return nil, fmt.Errorf("failed to get root node: %w", err)
    }

    // 2. UnixFS 메타데이터 파싱
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
    // 3. 파일 리더 생성 (스트리밍)
    dagReader, err := uio.NewDagReader(ctx, rootNode, ufs.dagService)
    if err != nil {
        return nil, fmt.Errorf("failed to create DAG reader: %w", err)
    }

    // 4. 청크들을 순차적으로 읽는 스트림 반환
    return files.NewReaderFile(dagReader), nil
}
```

### 4. 디렉터리 처리

```go
// pkg/unixfs.go:170-200
func (ufs *UnixFsWrapper) putDirectory(ctx context.Context, dir files.Directory) (cid.Cid, error) {
    // 1. 디렉터리 노드 생성
    dirNode := &dag.ProtoNode{}
    dirNode.SetData(unixfs.FolderPBData())

    // 2. 디렉터리 내 파일/하위 디렉터리 처리
    entries := dir.Entries()
    for entries.Next() {
        entry := entries.Node()
        entryName := entries.Name()

        // 3. 재귀적으로 하위 항목 처리
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

        // 4. 디렉터리 링크 추가
        err = dirNode.AddNodeLink(entryName, &dag.ProtoNode{})
        if err != nil {
            return cid.Undef, fmt.Errorf("failed to add link: %w", err)
        }
    }

    // 5. 디렉터리 노드 저장
    err := ufs.dagService.Add(ctx, dirNode)
    return dirNode.Cid(), err
}
```

### 5. 경로 기반 파일 접근

```go
// pkg/unixfs.go:240-270
func (ufs *UnixFsWrapper) GetPath(ctx context.Context, rootCID cid.Cid, path string) (files.Node, error) {
    if path == "" || path == "/" {
        return ufs.Get(ctx, rootCID)
    }

    // 1. 경로 파싱
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")
    currentCID := rootCID

    // 2. 경로를 따라 순차적으로 탐색
    for _, segment := range pathSegments {
        // 현재 노드 조회
        currentNode, err := ufs.dagService.Get(ctx, currentCID)
        if err != nil {
            return nil, fmt.Errorf("failed to get node: %w", err)
        }

        // UnixFS 메타데이터 파싱
        fsNode, err := unixfs.FSNodeFromBytes(currentNode.RawData())
        if err != nil {
            return nil, fmt.Errorf("failed to parse UnixFS node: %w", err)
        }

        // 디렉터리인지 확인
        if fsNode.Type() != unixfs.TDirectory {
            return nil, fmt.Errorf("path segment '%s' is not a directory", segment)
        }

        // 3. 디렉터리에서 해당 이름의 링크 찾기
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

    // 4. 최종 노드 반환
    return ufs.Get(ctx, currentCID)
}
```

## 🏃‍♂️ 실습 가이드

### 1. 기본 실행

```bash
cd 03-unixfs
go run main.go
```

**예상 출력**:
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

### 2. 청킹 전략 실험

다양한 청크 크기로 성능 비교:

```bash
# 작은 청크 (64KB)
UnixFS_CHUNK_SIZE=65536 go run main.go

# 큰 청크 (1MB)
UnixFS_CHUNK_SIZE=1048576 go run main.go

# 기본값 (256KB)
go run main.go
```

**관찰 포인트**:
- 청크 크기가 작을수록 더 많은 청크 생성
- 청크 크기가 클수록 메모리 사용량 증가
- 네트워크 조건에 따른 최적 청크 크기 변화

### 3. 대용량 파일 처리 테스트

```go
// 10MB 파일 생성 및 처리
func testLargeFile() {
    largeData := make([]byte, 10*1024*1024)
    for i := range largeData {
        largeData[i] = byte(i % 256)
    }

    file := files.NewBytesFile(largeData)
    cid, err := unixfsWrapper.Put(ctx, file)
    // 청킹 및 메모리 사용량 관찰
}
```

### 4. 테스트 실행

```bash
go test -v ./...
```

**주요 테스트 케이스**:
- ✅ 작은 파일 저장/검색
- ✅ 대용량 파일 청킹
- ✅ 디렉터리 구조 생성
- ✅ 경로 기반 접근
- ✅ 스트리밍 I/O 성능

## 🔍 고급 활용 사례

### 1. 웹사이트 호스팅

```go
type WebsiteBuilder struct {
    unixfs *UnixFsWrapper
}

func (wb *WebsiteBuilder) BuildSite(sitePath string) (cid.Cid, error) {
    // 1. HTML, CSS, JS 파일들을 수집
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

    // 2. 웹사이트 디렉터리 구조 생성
    websiteDir := files.NewMapDirectory(map[string]files.Node{
        "index.html": htmlFiles[0],
        "assets":     files.NewMapDirectory(assetsMap),
    })

    // 3. 전체 사이트를 IPFS에 추가
    return wb.unixfs.Put(ctx, websiteDir)
}
```

### 2. 버전 관리 시스템

```go
type VersionedFile struct {
    Content     []byte            `json:"content"`
    Version     int               `json:"version"`
    PreviousRef map[string]string `json:"previous,omitempty"` // CID 링크
    Timestamp   string            `json:"timestamp"`
    Author      string            `json:"author"`
    Message     string            `json:"message"`
}

func (vcs *VersionControlSystem) CommitFile(content []byte, message, author string,
                                          previousCID *cid.Cid) (cid.Cid, error) {
    version := 1
    var previousRef map[string]string

    if previousCID != nil {
        // 이전 버전 정보 조회하여 버전 번호 증가
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

    // JSON으로 직렬화하여 저장
    data, _ := json.Marshal(versionedFile)
    file := files.NewBytesFile(data)
    return vcs.unixfs.Put(ctx, file)
}
```

### 3. 분산 파일 백업

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

        // 1. 파일 해시 계산 (중복 제거)
        fileHash := calculateFileHash(path)

        // 2. 중복이 아닌 경우만 업로드
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

            // 3. 매니페스트에 파일 정보 기록
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

    // 4. 매니페스트 자체도 IPFS에 저장
    manifestData, _ := json.Marshal(manifest)
    manifestFile := files.NewBytesFile(manifestData)
    manifestCID, err := bs.unixfs.Put(ctx, manifestFile)

    manifest.ManifestCID = manifestCID.String()
    return manifest, err
}
```

### 4. 미디어 스트리밍

```go
type MediaStreamer struct {
    unixfs *UnixFsWrapper
}

func (ms *MediaStreamer) StreamVideo(videoCID cid.Cid,
                                   startByte, endByte int64) (io.Reader, error) {
    // 1. 비디오 파일 노드 조회
    videoNode, err := ms.unixfs.Get(ctx, videoCID)
    if err != nil {
        return nil, err
    }

    videoFile, ok := videoNode.(files.File)
    if !ok {
        return nil, fmt.Errorf("not a file")
    }

    // 2. 범위 기반 읽기 (HTTP Range Request 지원)
    if startByte > 0 {
        _, err = videoFile.Seek(startByte, io.SeekStart)
        if err != nil {
            return nil, err
        }
    }

    // 3. 제한된 크기만 읽는 리더 반환
    if endByte > startByte {
        return io.LimitReader(videoFile, endByte-startByte+1), nil
    }

    return videoFile, nil
}

func (ms *MediaStreamer) CreateVideoManifest(videoCID cid.Cid) (*HLSManifest, error) {
    // HLS (HTTP Live Streaming) 매니페스트 생성
    manifest := &HLSManifest{
        Version:    3,
        TargetDuration: 10,
        Segments:   []HLSSegment{},
    }

    // 비디오를 10초 세그먼트로 분할하여 각각 CID 생성
    // 실제 구현에서는 FFmpeg 등을 사용하여 세그먼트 분할

    return manifest, nil
}
```

## ⚠️ 주의사항 및 모범 사례

### 1. 청크 크기 선택

```go
// ✅ 용도별 최적 청크 크기
func selectChunkSize(fileSize int64, networkCondition string) int {
    switch {
    case fileSize < 1*1024*1024: // 1MB 미만
        return 64 * 1024 // 64KB
    case networkCondition == "slow":
        return 128 * 1024 // 128KB
    case networkCondition == "fast":
        return 1024 * 1024 // 1MB
    default:
        return 256 * 1024 // 256KB (기본값)
    }
}
```

### 2. 메모리 효율적인 대용량 파일 처리

```go
// ✅ 스트리밍 기반 처리
func processLargeFile(filePath string, ufs *UnixFsWrapper) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()

    // 파일 전체를 메모리에 로드하지 않고 스트림으로 처리
    readerFile := files.NewReaderFile(file)
    _, err = ufs.Put(ctx, readerFile)
    return err
}

// ❌ 잘못된 방법: 메모리 부족 위험
func processLargeFileWrong(filePath string, ufs *UnixFsWrapper) error {
    data, err := ioutil.ReadFile(filePath) // 전체 파일을 메모리에 로드
    if err != nil {
        return err
    }

    file := files.NewBytesFile(data)
    _, err = ufs.Put(ctx, file)
    return err
}
```

### 3. 경로 정규화

```go
// ✅ 안전한 경로 처리
func normalizePath(path string) string {
    // 상대 경로 공격 방지
    path = filepath.Clean(path)

    // 절대 경로를 상대 경로로 변환
    if filepath.IsAbs(path) {
        path = path[1:]
    }

    // 빈 경로 처리
    if path == "." {
        return ""
    }

    return path
}
```

### 4. MIME 타입 처리

```go
// ✅ 자동 MIME 타입 감지
func detectMimeType(filename string, content []byte) string {
    // 1. 확장자 기반 감지
    ext := filepath.Ext(filename)
    if mimeType := mime.TypeByExtension(ext); mimeType != "" {
        return mimeType
    }

    // 2. 내용 기반 감지
    return http.DetectContentType(content)
}

func addFileWithMetadata(ufs *UnixFsWrapper, filename string,
                        content []byte) (cid.Cid, error) {
    mimeType := detectMimeType(filename, content)

    // 메타데이터와 함께 저장
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

## 🔧 트러블슈팅

### 문제 1: "chunk too large" 에러

**원인**: 청크 크기가 시스템 한계를 초과
```go
// 해결: 청크 크기 제한
const MaxChunkSize = 1024 * 1024 // 1MB

func validateChunkSize(chunkSize int) int {
    if chunkSize > MaxChunkSize {
        log.Printf("Chunk size %d exceeds maximum, using %d", chunkSize, MaxChunkSize)
        return MaxChunkSize
    }
    return chunkSize
}
```

### 문제 2: "out of memory" 에러

**원인**: 대용량 파일을 한 번에 메모리에 로드
```go
// 해결: 스트리밍 처리
func processFileStream(reader io.Reader, ufs *UnixFsWrapper) (cid.Cid, error) {
    // io.Reader를 직접 사용하여 메모리 사용량 최소화
    file := files.NewReaderFile(reader)
    return ufs.Put(ctx, file)
}
```

### 문제 3: "path not found" 에러

**원인**: 잘못된 경로 또는 디렉터리 구조
```go
// 해결: 경로 유효성 검사
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

### 문제 4: 청킹 불일치

**원인**: 다른 청크 크기로 저장된 파일 접근
```go
// 해결: 청크 정보 메타데이터 저장
type FileMetadata struct {
    ChunkSize    int    `json:"chunk_size"`
    TotalSize    int64  `json:"total_size"`
    ChunkCount   int    `json:"chunk_count"`
    Algorithm    string `json:"algorithm"`
}

func putFileWithMetadata(ufs *UnixFsWrapper, file files.File,
                        chunkSize int) (cid.Cid, error) {
    // 파일과 메타데이터를 함께 저장
    metadata := FileMetadata{
        ChunkSize:  chunkSize,
        Algorithm:  "size-splitter",
    }

    // 메타데이터를 포함한 wrapper 구조 생성
    wrapper := struct {
        Metadata FileMetadata `json:"metadata"`
        Content  string       `json:"content"` // 실제 파일 CID
    }{
        Metadata: metadata,
    }

    // 실제 파일 저장
    fileCID, err := ufs.Put(ctx, file)
    if err != nil {
        return cid.Undef, err
    }

    wrapper.Content = fileCID.String()

    // 래퍼 저장
    wrapperData, _ := json.Marshal(wrapper)
    wrapperFile := files.NewBytesFile(wrapperData)
    return ufs.Put(ctx, wrapperFile)
}
```

## 📊 성능 최적화

### 1. 병렬 청킹

```go
// ✅ 병렬 청크 처리
func putFileParallel(ufs *UnixFsWrapper, file files.File) (cid.Cid, error) {
    const workers = 4
    chunkQueue := make(chan []byte, workers*2)
    resultQueue := make(chan chunkResult, workers*2)

    // 워커 시작
    for i := 0; i < workers; i++ {
        go func() {
            for chunk := range chunkQueue {
                chunkFile := files.NewBytesFile(chunk)
                cid, err := ufs.Put(ctx, chunkFile)
                resultQueue <- chunkResult{cid: cid, err: err}
            }
        }()
    }

    // 청킹 및 큐에 전송
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

    // 결과 수집
    var links []*format.Link
    for i := 0; i < chunkCount; i++ {
        result := <-resultQueue
        if result.err != nil {
            return cid.Undef, result.err
        }
        links = append(links, &format.Link{Cid: result.cid})
    }

    // 루트 노드 생성
    return createRootNode(links)
}
```

### 2. 지능형 캐싱

```go
// ✅ LRU 캐시로 자주 접근하는 파일 캐싱
type CachedUnixFS struct {
    *UnixFsWrapper
    cache *lru.Cache
}

func (cufs *CachedUnixFS) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
    // 캐시 확인
    if cached, ok := cufs.cache.Get(c.String()); ok {
        return cached.(files.Node), nil
    }

    // 캐시 미스 시 실제 조회
    node, err := cufs.UnixFsWrapper.Get(ctx, c)
    if err != nil {
        return nil, err
    }

    // 캐시에 저장 (크기 제한)
    if estimatedSize(node) < MaxCacheNodeSize {
        cufs.cache.Add(c.String(), node)
    }

    return node, nil
}
```

### 3. 압축 지원

```go
// ✅ 자동 압축으로 저장 공간 절약
func putFileWithCompression(ufs *UnixFsWrapper, file files.File,
                          compress bool) (cid.Cid, error) {
    if !compress {
        return ufs.Put(ctx, file)
    }

    // gzip 압축 적용
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

    // 압축된 데이터를 메타데이터와 함께 저장
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

## 📚 추가 학습 자료

### 관련 문서
- [UnixFS Specification](https://github.com/ipfs/specs/blob/master/UNIXFS.md)
- [IPFS File API](https://docs.ipfs.io/reference/kubo/rpc/#api-v0-add)
- [Merkle DAG Structure](https://docs.ipfs.io/concepts/merkle-dag/)
- [Chunking Strategies](https://docs.ipfs.io/concepts/file-systems/#chunking)

### 다음 단계
1. **04-network-bitswap**: P2P 네트워크에서 파일 블록 교환
2. **05-pin-gc**: 파일 생명주기 관리와 가비지 컬렉션
3. **06-gateway**: HTTP를 통한 파일 웹 접근

## 🎓 연습 문제

### 기초 연습
1. 다양한 크기의 파일을 저장하고 청킹 결과를 비교해보세요
2. 디렉터리 구조를 만들고 경로를 통해 파일에 접근해보세요
3. 같은 파일을 다른 청크 크기로 저장했을 때 CID 차이를 확인해보세요

### 심화 연습
1. 이미지 파일을 저장하고 썸네일 생성 시스템을 만들어보세요
2. 텍스트 파일의 버전 관리 시스템을 구현해보세요
3. 대용량 로그 파일을 효율적으로 저장하고 검색하는 시스템을 설계해보세요

### 실전 과제
1. 정적 웹사이트 호스팅 시스템을 만들어보세요
2. 파일 백업 및 복원 도구를 구현해보세요
3. 미디어 스트리밍 서비스의 기본 구조를 설계해보세요

이제 IPFS에서 파일과 디렉터리를 어떻게 효율적으로 다루는지 이해하셨을 것입니다. 다음 모듈에서는 P2P 네트워크에서 이러한 데이터를 어떻게 공유하는지 학습하겠습니다! 🚀