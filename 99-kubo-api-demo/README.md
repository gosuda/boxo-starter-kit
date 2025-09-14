# 99-kubo-api-demo: Kubo HTTP API 클라이언트 구현

## 🎯 학습 목표
- Kubo(go-ipfs) HTTP API와의 통신 방법 이해
- 실제 IPFS 네트워크에 연결하여 데이터 조작
- RESTful API를 통한 IPFS 핵심 기능 활용
- 분산 애플리케이션과 IPFS 노드 통합
- 실제 프로덕션 환경에서의 IPFS 활용법

## 📋 사전 요구사항
- **이전 챕터**: 전체 boxo-starter-kit 모듈 완료 권장
- **기술 지식**: HTTP 클라이언트, JSON 처리, 멀티파트 업로드
- **Go 지식**: HTTP 클라이언트, 구조체 태그, 에러 처리
- **IPFS 노드**: 로컬 또는 원격 Kubo 노드 실행 중

## 🔑 핵심 개념

### Kubo HTTP API란?
Kubo는 Go로 구현된 IPFS의 메인 구현체로, HTTP API를 통해 IPFS 기능을 외부 애플리케이션에 노출합니다.

#### API의 특징
- **RESTful 설계**: HTTP 메서드와 경로 기반 인터페이스
- **JSON 응답**: 구조화된 데이터 반환
- **스트리밍 지원**: 대용량 파일 처리
- **멀티파트 업로드**: 파일 및 디렉터리 업로드

### 주요 API 엔드포인트
```
POST /api/v0/add              # 파일/디렉터리 추가
GET  /api/v0/cat/{hash}       # 파일 내용 조회
POST /api/v0/get/{hash}       # 파일/디렉터리 다운로드
GET  /api/v0/ls/{hash}        # 디렉터리 리스팅
POST /api/v0/pin/add          # 콘텐츠 Pin
POST /api/v0/pin/rm           # Pin 제거
GET  /api/v0/pin/ls           # Pin 리스트
POST /api/v0/name/publish     # IPNS 발행
GET  /api/v0/name/resolve     # IPNS 해석
```

## 💻 코드 분석

### 1. Kubo 클라이언트 구조체
```go
type KuboClient struct {
    apiURL     string
    httpClient *http.Client
    timeout    time.Duration
}
```

**설계 결정**:
- `apiURL`: Kubo 노드의 API 엔드포인트 URL
- `httpClient`: HTTP 요청을 위한 재사용 가능한 클라이언트
- `timeout`: 요청 타임아웃 설정

### 2. API 응답 구조체
```go
type AddResponse struct {
    Name string `json:"Name"`
    Hash string `json:"Hash"`
    Size string `json:"Size"`
}

type LsResponse struct {
    Objects []LsObject `json:"Objects"`
}

type LsObject struct {
    Hash  string   `json:"Hash"`
    Links []LsLink `json:"Links"`
}
```

**JSON 태그 활용**: Kubo API의 대소문자 구분 필드와 매핑

### 3. 멀티파트 업로드 구현
```go
func (kc *KuboClient) createMultipartRequest(filename string, content []byte) (*http.Request, error) {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)

    part, err := writer.CreateFormFile("file", filename)
    if err != nil {
        return nil, err
    }

    _, err = part.Write(content)
    if err != nil {
        return nil, err
    }

    err = writer.Close()
    if err != nil {
        return nil, err
    }

    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/add", body)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Content-Type", writer.FormDataContentType())
    return req, nil
}
```

## 🏃‍♂️ 실습 가이드

### 단계 1: Kubo 노드 시작
```bash
# IPFS 초기화 (처음 한 번만)
ipfs init

# Kubo 노드 시작
ipfs daemon
```

### 단계 2: API 클라이언트 테스트
```bash
cd 99-kubo-api-demo
go run main.go
```

### 단계 3: 기능별 테스트
```bash
# 파일 추가 테스트
echo "Hello, Kubo!" > test.txt
curl -X POST -F file=@test.txt http://127.0.0.1:5001/api/v0/add

# 파일 조회 테스트
curl "http://127.0.0.1:5001/api/v0/cat?arg={HASH}"

# Pin 상태 확인
curl "http://127.0.0.1:5001/api/v0/pin/ls"
```

### 예상 결과
- **파일 추가**: JSON 응답으로 해시와 크기 반환
- **파일 조회**: 원본 파일 내용 반환
- **Pin 관리**: Pin 추가/제거 성공 확인

## 🚀 고급 활용 사례

### 1. 대용량 파일 스트리밍
```go
func (kc *KuboClient) StreamLargeFile(hash string, writer io.Writer) error {
    url := fmt.Sprintf("%s/api/v0/cat?arg=%s", kc.apiURL, hash)

    resp, err := kc.httpClient.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // 청크 단위로 스트리밍
    _, err = io.Copy(writer, resp.Body)
    return err
}
```

### 2. 배치 Pin 관리
```go
func (kc *KuboClient) BulkPin(hashes []string) error {
    for _, hash := range hashes {
        err := kc.PinAdd(hash)
        if err != nil {
            log.Printf("Pin 실패: %s, 에러: %v", hash, err)
            continue
        }
    }
    return nil
}
```

### 3. 네트워크 상태 모니터링
```go
type NetworkStats struct {
    Peers      int    `json:"peers"`
    Bandwidth  string `json:"bandwidth"`
    RepoSize   string `json:"repo_size"`
}

func (kc *KuboClient) GetNetworkStats() (*NetworkStats, error) {
    // 피어 수 조회
    peersResp, err := kc.httpClient.Get(kc.apiURL + "/api/v0/swarm/peers")
    // ... 구현
}
```

## 🔧 성능 최적화

### 연결 풀링
```go
func NewOptimizedKuboClient(apiURL string) *KuboClient {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
    }

    return &KuboClient{
        apiURL: apiURL,
        httpClient: &http.Client{
            Transport: transport,
            Timeout:   30 * time.Second,
        },
    }
}
```

### 요청 재시도 메커니즘
```go
func (kc *KuboClient) retryRequest(req *http.Request, maxRetries int) (*http.Response, error) {
    for i := 0; i < maxRetries; i++ {
        resp, err := kc.httpClient.Do(req)
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }

        if resp != nil {
            resp.Body.Close()
        }

        if i < maxRetries-1 {
            time.Sleep(time.Duration(i+1) * time.Second)
        }
    }

    return nil, fmt.Errorf("최대 재시도 횟수 초과")
}
```

### 병렬 처리
```go
func (kc *KuboClient) ParallelAdd(files []FileInfo) []AddResult {
    results := make([]AddResult, len(files))
    var wg sync.WaitGroup

    for i, file := range files {
        wg.Add(1)
        go func(index int, f FileInfo) {
            defer wg.Done()
            hash, err := kc.Add(f.Name, f.Content)
            results[index] = AddResult{Hash: hash, Error: err}
        }(i, file)
    }

    wg.Wait()
    return results
}
```

## 🔒 보안 고려사항

### API 키 인증
```go
func (kc *KuboClient) SetAuthToken(token string) {
    kc.httpClient.Transport = &AuthTransport{
        Token:     token,
        Transport: http.DefaultTransport,
    }
}

type AuthTransport struct {
    Token     string
    Transport http.RoundTripper
}

func (at *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    req.Header.Set("Authorization", "Bearer "+at.Token)
    return at.Transport.RoundTrip(req)
}
```

### HTTPS 연결
```go
func NewSecureKuboClient(apiURL string) *KuboClient {
    transport := &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    }

    return &KuboClient{
        apiURL: apiURL,
        httpClient: &http.Client{
            Transport: transport,
        },
    }
}
```

## 🐛 트러블슈팅

### 문제 1: 연결 거부
**증상**: `connection refused` 에러
**원인**: Kubo 노드가 실행되지 않음
**해결책**:
```bash
# Kubo 노드 상태 확인
ipfs id

# 노드 재시작
ipfs daemon
```

### 문제 2: API 응답 파싱 실패
**증상**: JSON 언마샬링 에러
**원인**: API 버전 차이 또는 응답 형식 변경
**해결책**:
```go
// 유연한 파싱 구조체 사용
type FlexibleResponse map[string]interface{}

var response FlexibleResponse
err := json.Unmarshal(data, &response)
```

### 문제 3: 대용량 파일 업로드 실패
**증상**: 업로드 시 타임아웃 또는 메모리 부족
**원인**: 메모리 기반 처리 방식
**해결책**:
```go
// 스트리밍 업로드 구현
func (kc *KuboClient) StreamingAdd(reader io.Reader, filename string) (string, error) {
    // Pipe를 사용한 스트리밍 업로드
    pr, pw := io.Pipe()
    writer := multipart.NewWriter(pw)

    go func() {
        defer pw.Close()
        // 스트리밍으로 멀티파트 작성
    }()

    // HTTP 요청에 파이프 리더 사용
    req, _ := http.NewRequest("POST", kc.apiURL+"/api/v0/add", pr)
    // ...
}
```

## 🔗 연계 학습
- **실제 배포**: Docker를 사용한 IPFS 노드 운영
- **고급 주제**:
  - IPFS 클러스터 구성
  - 게이트웨이 최적화
  - 모니터링 및 로깅

## 📚 참고 자료
- [Kubo HTTP API Documentation](https://docs.ipfs.tech/reference/kubo/rpc/)
- [go-ipfs-api Library](https://github.com/ipfs/go-ipfs-api)
- [IPFS Best Practices](https://docs.ipfs.tech/how-to/)

---

# 🍳 실전 쿡북: 바로 쓸 수 있는 코드

## 1. 📁 파일 백업 및 동기화 시스템

로컬 파일을 IPFS에 자동으로 백업하고 동기화하는 시스템입니다.

```go
package main

import (
    "crypto/md5"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io/fs"
    "log"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/fsnotify/fsnotify"
    kubo "github.com/sonheesung/boxo-starter-kit/99-kubo-api-demo/pkg"
)

type BackupSystem struct {
    client      *kubo.KuboClient
    backupIndex *BackupIndex
    watcher     *fsnotify.Watcher
    watchDirs   map[string]bool
}

type BackupIndex struct {
    Files       map[string]*FileRecord `json:"files"`
    LastBackup  time.Time              `json:"last_backup"`
    IndexHash   string                 `json:"index_hash"`
    TotalFiles  int                    `json:"total_files"`
    TotalSize   int64                  `json:"total_size"`
}

type FileRecord struct {
    Path         string    `json:"path"`
    IPFSHash     string    `json:"ipfs_hash"`
    LocalHash    string    `json:"local_hash"`
    Size         int64     `json:"size"`
    ModTime      time.Time `json:"mod_time"`
    BackupTime   time.Time `json:"backup_time"`
    IsDirectory  bool      `json:"is_directory"`
}

func NewBackupSystem(kuboAPIURL string) (*BackupSystem, error) {
    client := kubo.NewKuboClient(kuboAPIURL)

    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    return &BackupSystem{
        client:      client,
        backupIndex: &BackupIndex{Files: make(map[string]*FileRecord)},
        watcher:     watcher,
        watchDirs:   make(map[string]bool),
    }, nil
}

func (bs *BackupSystem) AddWatchDirectory(dirPath string) error {
    absPath, err := filepath.Abs(dirPath)
    if err != nil {
        return err
    }

    if bs.watchDirs[absPath] {
        return fmt.Errorf("이미 감시 중인 디렉터리입니다: %s", absPath)
    }

    // 디렉터리와 하위 디렉터리를 재귀적으로 감시 추가
    err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }

        if d.IsDir() {
            return bs.watcher.Add(path)
        }
        return nil
    })

    if err != nil {
        return err
    }

    bs.watchDirs[absPath] = true
    fmt.Printf("감시 디렉터리 추가됨: %s\n", absPath)

    return nil
}

func (bs *BackupSystem) StartWatching() {
    go func() {
        for {
            select {
            case event, ok := <-bs.watcher.Events:
                if !ok {
                    return
                }

                if event.Op&fsnotify.Write == fsnotify.Write ||
                   event.Op&fsnotify.Create == fsnotify.Create {

                    fmt.Printf("파일 변경 감지: %s\n", event.Name)

                    // 새 디렉터리가 생성된 경우 감시 추가
                    if event.Op&fsnotify.Create == fsnotify.Create {
                        if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
                            bs.watcher.Add(event.Name)
                        }
                    }

                    // 1초 대기 후 백업 (여러 변경사항을 배치로 처리)
                    time.Sleep(1 * time.Second)
                    bs.backupFile(event.Name)
                }

            case err, ok := <-bs.watcher.Errors:
                if !ok {
                    return
                }
                log.Printf("감시 에러: %v", err)
            }
        }
    }()
}

func (bs *BackupSystem) backupFile(filePath string) error {
    info, err := os.Stat(filePath)
    if err != nil {
        // 파일이 삭제된 경우
        if os.IsNotExist(err) {
            delete(bs.backupIndex.Files, filePath)
            fmt.Printf("파일 삭제됨: %s\n", filePath)
            return bs.saveBackupIndex()
        }
        return err
    }

    if info.IsDir() {
        return nil // 디렉터리는 별도 처리
    }

    // 파일 해시 계산
    localHash, err := bs.calculateFileHash(filePath)
    if err != nil {
        return err
    }

    // 기존 레코드 확인
    existingRecord, exists := bs.backupIndex.Files[filePath]
    if exists && existingRecord.LocalHash == localHash {
        // 변경되지 않은 파일
        return nil
    }

    // 파일 읽기
    content, err := os.ReadFile(filePath)
    if err != nil {
        return err
    }

    // IPFS에 추가
    ipfsHash, err := bs.client.Add(filepath.Base(filePath), content)
    if err != nil {
        return fmt.Errorf("IPFS 추가 실패: %w", err)
    }

    // 백업 인덱스 업데이트
    bs.backupIndex.Files[filePath] = &FileRecord{
        Path:        filePath,
        IPFSHash:    ipfsHash,
        LocalHash:   localHash,
        Size:        info.Size(),
        ModTime:     info.ModTime(),
        BackupTime:  time.Now(),
        IsDirectory: false,
    }

    // Pin 추가 (중요한 파일 보호)
    err = bs.client.PinAdd(ipfsHash)
    if err != nil {
        log.Printf("Pin 추가 실패: %v", err)
    }

    fmt.Printf("파일 백업됨: %s -> %s\n", filePath, ipfsHash)
    return bs.saveBackupIndex()
}

func (bs *BackupSystem) FullBackup(rootDir string) error {
    fmt.Printf("전체 백업 시작: %s\n", rootDir)

    err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }

        if !d.IsDir() {
            return bs.backupFile(path)
        }
        return nil
    })

    if err != nil {
        return err
    }

    bs.backupIndex.LastBackup = time.Now()
    bs.backupIndex.TotalFiles = len(bs.backupIndex.Files)

    var totalSize int64
    for _, record := range bs.backupIndex.Files {
        totalSize += record.Size
    }
    bs.backupIndex.TotalSize = totalSize

    return bs.saveBackupIndex()
}

func (bs *BackupSystem) RestoreFile(filePath, targetDir string) error {
    record, exists := bs.backupIndex.Files[filePath]
    if !exists {
        return fmt.Errorf("백업 파일을 찾을 수 없습니다: %s", filePath)
    }

    // IPFS에서 파일 내용 가져오기
    content, err := bs.client.Cat(record.IPFSHash)
    if err != nil {
        return fmt.Errorf("IPFS에서 파일 조회 실패: %w", err)
    }

    // 복원 경로 계산
    relPath, err := filepath.Rel(filepath.Dir(filePath), filePath)
    if err != nil {
        relPath = filepath.Base(filePath)
    }

    restorePath := filepath.Join(targetDir, relPath)

    // 디렉터리 생성
    err = os.MkdirAll(filepath.Dir(restorePath), 0755)
    if err != nil {
        return err
    }

    // 파일 복원
    err = os.WriteFile(restorePath, content, 0644)
    if err != nil {
        return err
    }

    fmt.Printf("파일 복원됨: %s -> %s\n", record.IPFSHash, restorePath)
    return nil
}

func (bs *BackupSystem) RestoreAll(targetDir string) error {
    fmt.Printf("전체 복원 시작: %s\n", targetDir)

    for _, record := range bs.backupIndex.Files {
        err := bs.RestoreFile(record.Path, targetDir)
        if err != nil {
            log.Printf("파일 복원 실패: %s, 에러: %v", record.Path, err)
            continue
        }
    }

    fmt.Printf("전체 복원 완료: %d개 파일\n", len(bs.backupIndex.Files))
    return nil
}

func (bs *BackupSystem) GetBackupStats() map[string]interface{} {
    stats := map[string]interface{}{
        "total_files":  bs.backupIndex.TotalFiles,
        "total_size":   bs.formatSize(bs.backupIndex.TotalSize),
        "last_backup":  bs.backupIndex.LastBackup.Format("2006-01-02 15:04:05"),
        "index_hash":   bs.backupIndex.IndexHash,
    }

    // 최근 백업된 파일들
    var recentFiles []*FileRecord
    for _, record := range bs.backupIndex.Files {
        if time.Since(record.BackupTime) < 24*time.Hour {
            recentFiles = append(recentFiles, record)
        }
    }
    stats["recent_files"] = len(recentFiles)

    return stats
}

func (bs *BackupSystem) calculateFileHash(filePath string) (string, error) {
    content, err := os.ReadFile(filePath)
    if err != nil {
        return "", err
    }

    hash := md5.Sum(content)
    return hex.EncodeToString(hash[:]), nil
}

func (bs *BackupSystem) saveBackupIndex() error {
    indexData, err := json.MarshalIndent(bs.backupIndex, "", "  ")
    if err != nil {
        return err
    }

    // 백업 인덱스를 IPFS에 저장
    indexHash, err := bs.client.Add("backup-index.json", indexData)
    if err != nil {
        return err
    }

    bs.backupIndex.IndexHash = indexHash

    // 로컬에도 저장
    err = os.WriteFile("backup-index.json", indexData, 0644)
    if err != nil {
        log.Printf("로컬 인덱스 저장 실패: %v", err)
    }

    return nil
}

func (bs *BackupSystem) LoadBackupIndex(indexHash string) error {
    content, err := bs.client.Cat(indexHash)
    if err != nil {
        return err
    }

    return json.Unmarshal(content, &bs.backupIndex)
}

func (bs *BackupSystem) formatSize(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (bs *BackupSystem) Close() error {
    return bs.watcher.Close()
}

func main() {
    // 백업 시스템 초기화
    backupSystem, err := NewBackupSystem("http://127.0.0.1:5001")
    if err != nil {
        panic(err)
    }
    defer backupSystem.Close()

    // 감시할 디렉터리 추가
    err = backupSystem.AddWatchDirectory("./documents")
    if err != nil {
        log.Printf("감시 디렉터리 추가 실패: %v", err)
    }

    // 실시간 감시 시작
    backupSystem.StartWatching()

    // 초기 전체 백업
    if len(os.Args) > 1 && os.Args[1] == "fullbackup" {
        err = backupSystem.FullBackup("./documents")
        if err != nil {
            log.Printf("전체 백업 실패: %v", err)
        }
    }

    // 복원 모드
    if len(os.Args) > 2 && os.Args[1] == "restore" {
        targetDir := os.Args[2]
        err = backupSystem.RestoreAll(targetDir)
        if err != nil {
            log.Printf("복원 실패: %v", err)
        }
        return
    }

    fmt.Println("📁 IPFS 파일 백업 시스템")
    fmt.Println("실시간 파일 감시가 시작되었습니다...")
    fmt.Println("Ctrl+C로 종료")

    // 주기적 상태 출력
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            stats := backupSystem.GetBackupStats()
            fmt.Printf("\n=== 백업 상태 ===\n")
            fmt.Printf("전체 파일: %v\n", stats["total_files"])
            fmt.Printf("총 크기: %v\n", stats["total_size"])
            fmt.Printf("최근 백업: %v개 파일\n", stats["recent_files"])
            fmt.Printf("인덱스 해시: %v\n", stats["index_hash"])
        }
    }
}
```

## 2. 🌐 분산 CDN 시스템

전 세계에 분산된 IPFS 노드를 활용한 콘텐츠 배포 네트워크입니다.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "strings"
    "sync"
    "time"

    kubo "github.com/sonheesung/boxo-starter-kit/99-kubo-api-demo/pkg"
)

type DistributedCDN struct {
    nodes       []*CDNNode
    loadBalancer *LoadBalancer
    cache       *ContentCache
    stats       *CDNStats
    mutex       sync.RWMutex
}

type CDNNode struct {
    ID          string        `json:"id"`
    APIEndpoint string        `json:"api_endpoint"`
    Region      string        `json:"region"`
    Client      *kubo.KuboClient
    Latency     time.Duration `json:"latency"`
    IsHealthy   bool          `json:"is_healthy"`
    LastCheck   time.Time     `json:"last_check"`
}

type LoadBalancer struct {
    strategy string // "round_robin", "latency", "random"
    counter  int
    mutex    sync.Mutex
}

type ContentCache struct {
    entries map[string]*CacheEntry
    mutex   sync.RWMutex
    maxSize int
    ttl     time.Duration
}

type CacheEntry struct {
    Content   []byte
    Hash      string
    ExpiresAt time.Time
    HitCount  int
    Size      int64
}

type CDNStats struct {
    TotalRequests   int64 `json:"total_requests"`
    CacheHits       int64 `json:"cache_hits"`
    CacheMisses     int64 `json:"cache_misses"`
    AverageLatency  int64 `json:"average_latency"`
    BytesServed     int64 `json:"bytes_served"`
    NodesOnline     int   `json:"nodes_online"`
    RequestsPerNode map[string]int64 `json:"requests_per_node"`
    mutex           sync.RWMutex
}

func NewDistributedCDN() *DistributedCDN {
    return &DistributedCDN{
        nodes: make([]*CDNNode, 0),
        loadBalancer: &LoadBalancer{
            strategy: "latency",
        },
        cache: &ContentCache{
            entries: make(map[string]*CacheEntry),
            maxSize: 1000, // 최대 1000개 항목
            ttl:     time.Hour,
        },
        stats: &CDNStats{
            RequestsPerNode: make(map[string]int64),
        },
    }
}

func (cdn *DistributedCDN) AddNode(id, apiEndpoint, region string) error {
    client := kubo.NewKuboClient(apiEndpoint)

    node := &CDNNode{
        ID:          id,
        APIEndpoint: apiEndpoint,
        Region:      region,
        Client:      client,
        IsHealthy:   false,
        LastCheck:   time.Now(),
    }

    // 노드 헬스 체크
    err := cdn.checkNodeHealth(node)
    if err != nil {
        log.Printf("노드 추가 실패 (헬스 체크): %s, 에러: %v", id, err)
    }

    cdn.mutex.Lock()
    cdn.nodes = append(cdn.nodes, node)
    cdn.stats.RequestsPerNode[id] = 0
    cdn.mutex.Unlock()

    fmt.Printf("CDN 노드 추가됨: %s (%s) - %s\n", id, region, apiEndpoint)
    return nil
}

func (cdn *DistributedCDN) checkNodeHealth(node *CDNNode) error {
    start := time.Now()

    // 간단한 헬스 체크 (ID 조회)
    _, err := node.Client.ID()

    node.LastCheck = time.Now()
    node.Latency = time.Since(start)

    if err != nil {
        node.IsHealthy = false
        return err
    }

    node.IsHealthy = true
    return nil
}

func (cdn *DistributedCDN) StartHealthMonitoring() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                cdn.mutex.RLock()
                nodes := make([]*CDNNode, len(cdn.nodes))
                copy(nodes, cdn.nodes)
                cdn.mutex.RUnlock()

                for _, node := range nodes {
                    go func(n *CDNNode) {
                        err := cdn.checkNodeHealth(n)
                        if err != nil {
                            log.Printf("노드 헬스 체크 실패: %s, 에러: %v", n.ID, err)
                        }
                    }(node)
                }

                // 통계 업데이트
                cdn.updateHealthStats()
            }
        }
    }()
}

func (cdn *DistributedCDN) updateHealthStats() {
    cdn.stats.mutex.Lock()
    defer cdn.stats.mutex.Unlock()

    healthyNodes := 0
    for _, node := range cdn.nodes {
        if node.IsHealthy {
            healthyNodes++
        }
    }
    cdn.stats.NodesOnline = healthyNodes
}

func (cdn *DistributedCDN) selectBestNode() *CDNNode {
    cdn.mutex.RLock()
    defer cdn.mutex.RUnlock()

    var healthyNodes []*CDNNode
    for _, node := range cdn.nodes {
        if node.IsHealthy {
            healthyNodes = append(healthyNodes, node)
        }
    }

    if len(healthyNodes) == 0 {
        return nil
    }

    cdn.loadBalancer.mutex.Lock()
    defer cdn.loadBalancer.mutex.Unlock()

    switch cdn.loadBalancer.strategy {
    case "round_robin":
        node := healthyNodes[cdn.loadBalancer.counter%len(healthyNodes)]
        cdn.loadBalancer.counter++
        return node

    case "latency":
        var bestNode *CDNNode
        var bestLatency time.Duration = time.Hour

        for _, node := range healthyNodes {
            if node.Latency < bestLatency {
                bestLatency = node.Latency
                bestNode = node
            }
        }
        return bestNode

    case "random":
        return healthyNodes[time.Now().UnixNano()%int64(len(healthyNodes))]

    default:
        return healthyNodes[0]
    }
}

func (cdn *DistributedCDN) GetContent(hash string) ([]byte, error) {
    start := time.Now()

    // 캐시 확인
    content := cdn.cache.get(hash)
    if content != nil {
        cdn.stats.mutex.Lock()
        cdn.stats.CacheHits++
        cdn.stats.TotalRequests++
        cdn.stats.mutex.Unlock()

        return content, nil
    }

    // 캐시 미스 - 최적 노드에서 조회
    node := cdn.selectBestNode()
    if node == nil {
        return nil, fmt.Errorf("사용 가능한 노드가 없습니다")
    }

    content, err := node.Client.Cat(hash)
    if err != nil {
        // 실패 시 다른 노드 시도
        for _, fallbackNode := range cdn.nodes {
            if fallbackNode.ID != node.ID && fallbackNode.IsHealthy {
                content, err = fallbackNode.Client.Cat(hash)
                if err == nil {
                    node = fallbackNode
                    break
                }
            }
        }

        if err != nil {
            return nil, err
        }
    }

    // 캐시에 저장
    cdn.cache.set(hash, content)

    // 통계 업데이트
    cdn.stats.mutex.Lock()
    cdn.stats.CacheMisses++
    cdn.stats.TotalRequests++
    cdn.stats.RequestsPerNode[node.ID]++
    cdn.stats.BytesServed += int64(len(content))
    cdn.stats.AverageLatency = int64(time.Since(start).Milliseconds())
    cdn.stats.mutex.Unlock()

    return content, nil
}

func (cdn *DistributedCDN) PrefetchContent(hashes []string) {
    go func() {
        for _, hash := range hashes {
            // 백그라운드에서 미리 캐시
            _, err := cdn.GetContent(hash)
            if err != nil {
                log.Printf("프리페치 실패: %s, 에러: %v", hash, err)
            }
            time.Sleep(100 * time.Millisecond) // 부하 조절
        }
    }()
}

func (cache *ContentCache) get(hash string) []byte {
    cache.mutex.RLock()
    defer cache.mutex.RUnlock()

    entry, exists := cache.entries[hash]
    if !exists || time.Now().After(entry.ExpiresAt) {
        return nil
    }

    entry.HitCount++
    return entry.Content
}

func (cache *ContentCache) set(hash string, content []byte) {
    cache.mutex.Lock()
    defer cache.mutex.Unlock()

    // 캐시 크기 제한 확인
    if len(cache.entries) >= cache.maxSize {
        cache.evictLRU()
    }

    cache.entries[hash] = &CacheEntry{
        Content:   content,
        Hash:      hash,
        ExpiresAt: time.Now().Add(cache.ttl),
        HitCount:  1,
        Size:      int64(len(content)),
    }
}

func (cache *ContentCache) evictLRU() {
    var oldestHash string
    var oldestTime time.Time = time.Now()

    for hash, entry := range cache.entries {
        if entry.ExpiresAt.Before(oldestTime) {
            oldestTime = entry.ExpiresAt
            oldestHash = hash
        }
    }

    if oldestHash != "" {
        delete(cache.entries, oldestHash)
    }
}

func (cdn *DistributedCDN) StartHTTPServer(port string) {
    http.HandleFunc("/", cdn.dashboardHandler)
    http.HandleFunc("/content/", cdn.contentHandler)
    http.HandleFunc("/api/stats", cdn.statsHandler)
    http.HandleFunc("/api/nodes", cdn.nodesHandler)
    http.HandleFunc("/api/cache", cdn.cacheHandler)

    fmt.Printf("분산 CDN 서버 시작됨: http://localhost%s\n", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

func (cdn *DistributedCDN) contentHandler(w http.ResponseWriter, r *http.Request) {
    hash := strings.TrimPrefix(r.URL.Path, "/content/")
    if hash == "" {
        http.Error(w, "해시가 필요합니다", http.StatusBadRequest)
        return
    }

    content, err := cdn.GetContent(hash)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    // 콘텐츠 타입 감지
    contentType := http.DetectContentType(content)
    w.Header().Set("Content-Type", contentType)
    w.Header().Set("Cache-Control", "public, max-age=3600")
    w.Header().Set("X-IPFS-Hash", hash)

    w.Write(content)
}

func (cdn *DistributedCDN) statsHandler(w http.ResponseWriter, r *http.Request) {
    cdn.stats.mutex.RLock()
    defer cdn.stats.mutex.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(cdn.stats)
}

func (cdn *DistributedCDN) nodesHandler(w http.ResponseWriter, r *http.Request) {
    cdn.mutex.RLock()
    defer cdn.mutex.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(cdn.nodes)
}

func (cdn *DistributedCDN) cacheHandler(w http.ResponseWriter, r *http.Request) {
    cdn.cache.mutex.RLock()
    defer cdn.cache.mutex.RUnlock()

    cacheInfo := map[string]interface{}{
        "total_entries": len(cdn.cache.entries),
        "max_size":      cdn.cache.maxSize,
        "ttl_seconds":   int(cdn.cache.ttl.Seconds()),
    }

    var totalSize int64
    for _, entry := range cdn.cache.entries {
        totalSize += entry.Size
    }
    cacheInfo["total_size_bytes"] = totalSize

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(cacheInfo)
}

func (cdn *DistributedCDN) dashboardHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>분산 CDN 대시보드</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .dashboard-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: white; border-radius: 8px; padding: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric { text-align: center; margin: 10px 0; }
        .metric-value { font-size: 2em; font-weight: bold; color: #007cba; }
        .metric-label { color: #666; }
        .node-list { margin-top: 10px; }
        .node-item { display: flex; justify-content: space-between; padding: 5px 0; border-bottom: 1px solid #eee; }
        .status-healthy { color: #28a745; }
        .status-unhealthy { color: #dc3545; }
        h1 { text-align: center; color: #333; }
        .refresh-btn { background: #007cba; color: white; padding: 10px 20px; border: none; border-radius: 5px; cursor: pointer; }
    </style>
    <script>
        function refreshData() {
            location.reload();
        }

        setInterval(refreshData, 30000); // 30초마다 자동 새로고침
    </script>
</head>
<body>
    <div class="container">
        <h1>🌐 분산 CDN 대시보드</h1>
        <button class="refresh-btn" onclick="refreshData()">새로고침</button>

        <div class="dashboard-grid">
            <div class="card">
                <h3>📊 성능 통계</h3>
                <div class="metric">
                    <div class="metric-value" id="total-requests">{{.TotalRequests}}</div>
                    <div class="metric-label">총 요청 수</div>
                </div>
                <div class="metric">
                    <div class="metric-value" id="cache-hit-rate">{{.CacheHitRate}}%</div>
                    <div class="metric-label">캐시 히트율</div>
                </div>
                <div class="metric">
                    <div class="metric-value" id="avg-latency">{{.AverageLatency}}ms</div>
                    <div class="metric-label">평균 지연시간</div>
                </div>
            </div>

            <div class="card">
                <h3>🌍 노드 상태</h3>
                <div class="metric">
                    <div class="metric-value" id="nodes-online">{{.NodesOnline}}/{{.TotalNodes}}</div>
                    <div class="metric-label">온라인 노드</div>
                </div>
                <div class="node-list" id="node-list">
                    <!-- 동적으로 업데이트됨 -->
                </div>
            </div>

            <div class="card">
                <h3>💾 캐시 상태</h3>
                <div class="metric">
                    <div class="metric-value" id="cache-entries">{{.CacheEntries}}</div>
                    <div class="metric-label">캐시된 항목</div>
                </div>
                <div class="metric">
                    <div class="metric-value" id="bytes-served">{{.BytesServed}}</div>
                    <div class="metric-label">전송된 데이터</div>
                </div>
            </div>
        </div>
    </div>
</body>
</html>`))

    // 템플릿 데이터 준비
    cdn.stats.mutex.RLock()
    cacheHitRate := float64(0)
    if cdn.stats.TotalRequests > 0 {
        cacheHitRate = float64(cdn.stats.CacheHits) / float64(cdn.stats.TotalRequests) * 100
    }

    data := struct {
        TotalRequests int64
        CacheHitRate  float64
        AverageLatency int64
        NodesOnline   int
        TotalNodes    int
        CacheEntries  int
        BytesServed   string
    }{
        TotalRequests:  cdn.stats.TotalRequests,
        CacheHitRate:   cacheHitRate,
        AverageLatency: cdn.stats.AverageLatency,
        NodesOnline:    cdn.stats.NodesOnline,
        TotalNodes:     len(cdn.nodes),
        CacheEntries:   len(cdn.cache.entries),
        BytesServed:    formatBytes(cdn.stats.BytesServed),
    }
    cdn.stats.mutex.RUnlock()

    tmpl.Execute(w, data)
}

func formatBytes(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
    // 분산 CDN 초기화
    cdn := NewDistributedCDN()

    // 여러 지역의 IPFS 노드 추가
    cdn.AddNode("seoul", "http://127.0.0.1:5001", "Seoul, KR")
    cdn.AddNode("tokyo", "http://127.0.0.1:5002", "Tokyo, JP")    // 가상 노드
    cdn.AddNode("singapore", "http://127.0.0.1:5003", "Singapore") // 가상 노드

    // 헬스 모니터링 시작
    cdn.StartHealthMonitoring()

    // 인기 콘텐츠 프리페치 (예시)
    popularContent := []string{
        "QmYjtig7VJQ6XsnUjqqJvj7QaMcCAwtrgNdahSiFofrE7o", // 예시 해시
        "QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V", // 예시 해시
    }
    cdn.PrefetchContent(popularContent)

    fmt.Println("🌐 분산 CDN 시스템")
    fmt.Printf("노드 수: %d\n", len(cdn.nodes))
    fmt.Println("헬스 모니터링이 시작되었습니다")

    // HTTP 서버 시작
    cdn.StartHTTPServer(":8080")
}
```

## 3. 📊 IPFS 네트워크 모니터링 시스템

IPFS 네트워크의 상태를 실시간으로 모니터링하고 분석하는 시스템입니다.

```go
package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "sort"
    "sync"
    "time"

    kubo "github.com/sonheesung/boxo-starter-kit/99-kubo-api-demo/pkg"
)

type NetworkMonitor struct {
    client      *kubo.KuboClient
    metrics     *NetworkMetrics
    alerts      []*Alert
    collectors  []*MetricCollector
    mutex       sync.RWMutex
}

type NetworkMetrics struct {
    Timestamp       time.Time              `json:"timestamp"`
    NodeInfo        *NodeInfo              `json:"node_info"`
    PeerCount       int                    `json:"peer_count"`
    RepoStats       *RepoStats             `json:"repo_stats"`
    BandwidthStats  *BandwidthStats        `json:"bandwidth_stats"`
    PinnedObjects   int                    `json:"pinned_objects"`
    DHTPeers        int                    `json:"dht_peers"`
    ConnectedPeers  []*PeerInfo            `json:"connected_peers"`
    NetworkHealth   float64                `json:"network_health"`
    ErrorRate       float64                `json:"error_rate"`
    ResponseTime    time.Duration          `json:"response_time"`
}

type NodeInfo struct {
    ID           string   `json:"id"`
    PublicKey    string   `json:"public_key"`
    Addresses    []string `json:"addresses"`
    AgentVersion string   `json:"agent_version"`
    ProtocolVersion string `json:"protocol_version"`
}

type RepoStats struct {
    NumObjects   int64 `json:"num_objects"`
    RepoSize     int64 `json:"repo_size"`
    StorageMax   int64 `json:"storage_max"`
    RepoPath     string `json:"repo_path"`
    Version      string `json:"version"`
}

type BandwidthStats struct {
    TotalIn  int64   `json:"total_in"`
    TotalOut int64   `json:"total_out"`
    RateIn   float64 `json:"rate_in"`
    RateOut  float64 `json:"rate_out"`
}

type PeerInfo struct {
    ID        string        `json:"id"`
    Address   string        `json:"address"`
    Latency   time.Duration `json:"latency"`
    Direction string        `json:"direction"`
    Mux       string        `json:"mux"`
}

type Alert struct {
    ID          string    `json:"id"`
    Type        string    `json:"type"`
    Severity    string    `json:"severity"`
    Message     string    `json:"message"`
    Timestamp   time.Time `json:"timestamp"`
    Resolved    bool      `json:"resolved"`
    ResolvedAt  time.Time `json:"resolved_at,omitempty"`
}

type MetricCollector struct {
    Name        string
    Interval    time.Duration
    CollectFunc func(*NetworkMonitor) error
    LastRun     time.Time
    ErrorCount  int
}

func NewNetworkMonitor(kuboAPIURL string) *NetworkMonitor {
    client := kubo.NewKuboClient(kuboAPIURL)

    nm := &NetworkMonitor{
        client:  client,
        metrics: &NetworkMetrics{},
        alerts:  make([]*Alert, 0),
    }

    // 메트릭 컬렉터 설정
    nm.setupCollectors()

    return nm
}

func (nm *NetworkMonitor) setupCollectors() {
    nm.collectors = []*MetricCollector{
        {
            Name:        "node_info",
            Interval:    5 * time.Minute,
            CollectFunc: (*NetworkMonitor).collectNodeInfo,
        },
        {
            Name:        "peer_stats",
            Interval:    30 * time.Second,
            CollectFunc: (*NetworkMonitor).collectPeerStats,
        },
        {
            Name:        "repo_stats",
            Interval:    1 * time.Minute,
            CollectFunc: (*NetworkMonitor).collectRepoStats,
        },
        {
            Name:        "bandwidth_stats",
            Interval:    10 * time.Second,
            CollectFunc: (*NetworkMonitor).collectBandwidthStats,
        },
        {
            Name:        "health_check",
            Interval:    15 * time.Second,
            CollectFunc: (*NetworkMonitor).collectHealthMetrics,
        },
    }
}

func (nm *NetworkMonitor) StartMonitoring() {
    fmt.Println("📊 IPFS 네트워크 모니터링 시작")

    for _, collector := range nm.collectors {
        go nm.runCollector(collector)
    }

    // 알림 처리
    go nm.processAlerts()
}

func (nm *NetworkMonitor) runCollector(collector *MetricCollector) {
    ticker := time.NewTicker(collector.Interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            start := time.Now()
            err := collector.CollectFunc(nm)

            collector.LastRun = time.Now()

            if err != nil {
                collector.ErrorCount++
                log.Printf("컬렉터 에러 [%s]: %v", collector.Name, err)

                // 연속 에러 시 알림
                if collector.ErrorCount >= 3 {
                    nm.addAlert("collector_error", "warning",
                        fmt.Sprintf("컬렉터 '%s'에서 연속 에러 발생", collector.Name))
                }
            } else {
                collector.ErrorCount = 0
            }

            duration := time.Since(start)
            if duration > collector.Interval/2 {
                nm.addAlert("slow_collector", "warning",
                    fmt.Sprintf("컬렉터 '%s' 실행 시간이 길어짐: %v", collector.Name, duration))
            }
        }
    }
}

func (nm *NetworkMonitor) collectNodeInfo(nm2 *NetworkMonitor) error {
    nodeInfo, err := nm.client.ID()
    if err != nil {
        return err
    }

    nm.mutex.Lock()
    nm.metrics.NodeInfo = &NodeInfo{
        ID:              nodeInfo.ID,
        PublicKey:       nodeInfo.PublicKey,
        Addresses:       nodeInfo.Addresses,
        AgentVersion:    nodeInfo.AgentVersion,
        ProtocolVersion: nodeInfo.ProtocolVersion,
    }
    nm.mutex.Unlock()

    return nil
}

func (nm *NetworkMonitor) collectPeerStats(nm2 *NetworkMonitor) error {
    peers, err := nm.client.SwarmPeers()
    if err != nil {
        return err
    }

    var connectedPeers []*PeerInfo
    for _, peer := range peers {
        peerInfo := &PeerInfo{
            ID:        peer.ID,
            Address:   peer.Address,
            Latency:   peer.Latency,
            Direction: peer.Direction,
            Mux:       peer.Mux,
        }
        connectedPeers = append(connectedPeers, peerInfo)
    }

    nm.mutex.Lock()
    nm.metrics.PeerCount = len(connectedPeers)
    nm.metrics.ConnectedPeers = connectedPeers
    nm.metrics.Timestamp = time.Now()
    nm.mutex.Unlock()

    // 피어 수 알림
    if len(connectedPeers) < 5 {
        nm.addAlert("low_peer_count", "warning",
            fmt.Sprintf("연결된 피어 수가 적습니다: %d개", len(connectedPeers)))
    } else if len(connectedPeers) > 100 {
        nm.addAlert("high_peer_count", "info",
            fmt.Sprintf("많은 피어에 연결되어 있습니다: %d개", len(connectedPeers)))
    }

    return nil
}

func (nm *NetworkMonitor) collectRepoStats(nm2 *NetworkMonitor) error {
    repoStats, err := nm.client.RepoStat()
    if err != nil {
        return err
    }

    nm.mutex.Lock()
    nm.metrics.RepoStats = &RepoStats{
        NumObjects: repoStats.NumObjects,
        RepoSize:   repoStats.RepoSize,
        StorageMax: repoStats.StorageMax,
        RepoPath:   repoStats.RepoPath,
        Version:    repoStats.Version,
    }
    nm.mutex.Unlock()

    // 저장소 사용량 알림
    if repoStats.StorageMax > 0 {
        usagePercent := float64(repoStats.RepoSize) / float64(repoStats.StorageMax) * 100
        if usagePercent > 90 {
            nm.addAlert("storage_full", "critical",
                fmt.Sprintf("저장소 사용량이 %.1f%%입니다", usagePercent))
        } else if usagePercent > 80 {
            nm.addAlert("storage_warning", "warning",
                fmt.Sprintf("저장소 사용량이 %.1f%%입니다", usagePercent))
        }
    }

    return nil
}

func (nm *NetworkMonitor) collectBandwidthStats(nm2 *NetworkMonitor) error {
    bwStats, err := nm.client.StatsBW()
    if err != nil {
        return err
    }

    nm.mutex.Lock()
    nm.metrics.BandwidthStats = &BandwidthStats{
        TotalIn:  bwStats.TotalIn,
        TotalOut: bwStats.TotalOut,
        RateIn:   bwStats.RateIn,
        RateOut:  bwStats.RateOut,
    }
    nm.mutex.Unlock()

    return nil
}

func (nm *NetworkMonitor) collectHealthMetrics(nm2 *NetworkMonitor) error {
    start := time.Now()

    // 응답 시간 측정
    _, err := nm.client.ID()
    responseTime := time.Since(start)

    nm.mutex.Lock()
    nm.metrics.ResponseTime = responseTime
    nm.mutex.Unlock()

    // 네트워크 건강도 계산
    health := nm.calculateNetworkHealth()

    nm.mutex.Lock()
    nm.metrics.NetworkHealth = health
    nm.mutex.Unlock()

    // 응답 시간 알림
    if responseTime > 5*time.Second {
        nm.addAlert("slow_response", "warning",
            fmt.Sprintf("API 응답 시간이 느립니다: %v", responseTime))
    }

    // 네트워크 건강도 알림
    if health < 0.5 {
        nm.addAlert("poor_health", "critical",
            fmt.Sprintf("네트워크 건강도가 낮습니다: %.1f%%", health*100))
    }

    return err
}

func (nm *NetworkMonitor) calculateNetworkHealth() float64 {
    nm.mutex.RLock()
    defer nm.mutex.RUnlock()

    var healthScore float64 = 1.0

    // 피어 수 기반 점수
    if nm.metrics.PeerCount < 5 {
        healthScore -= 0.3
    } else if nm.metrics.PeerCount > 50 {
        healthScore += 0.1
    }

    // 응답 시간 기반 점수
    if nm.metrics.ResponseTime > 2*time.Second {
        healthScore -= 0.2
    } else if nm.metrics.ResponseTime < 500*time.Millisecond {
        healthScore += 0.1
    }

    // 저장소 사용량 기반 점수
    if nm.metrics.RepoStats != nil && nm.metrics.RepoStats.StorageMax > 0 {
        usagePercent := float64(nm.metrics.RepoStats.RepoSize) / float64(nm.metrics.RepoStats.StorageMax)
        if usagePercent > 0.9 {
            healthScore -= 0.3
        } else if usagePercent > 0.8 {
            healthScore -= 0.1
        }
    }

    if healthScore < 0 {
        healthScore = 0
    }
    if healthScore > 1 {
        healthScore = 1
    }

    return healthScore
}

func (nm *NetworkMonitor) addAlert(alertType, severity, message string) {
    nm.mutex.Lock()
    defer nm.mutex.Unlock()

    // 중복 알림 확인
    for _, alert := range nm.alerts {
        if !alert.Resolved && alert.Type == alertType {
            return // 이미 동일한 알림이 존재
        }
    }

    alert := &Alert{
        ID:        fmt.Sprintf("%s_%d", alertType, time.Now().Unix()),
        Type:      alertType,
        Severity:  severity,
        Message:   message,
        Timestamp: time.Now(),
        Resolved:  false,
    }

    nm.alerts = append(nm.alerts, alert)

    // 최대 100개 알림만 유지
    if len(nm.alerts) > 100 {
        nm.alerts = nm.alerts[len(nm.alerts)-100:]
    }

    fmt.Printf("🚨 알림 [%s]: %s\n", severity, message)
}

func (nm *NetworkMonitor) processAlerts() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            nm.mutex.Lock()
            for _, alert := range nm.alerts {
                if !alert.Resolved && time.Since(alert.Timestamp) > 5*time.Minute {
                    // 5분 후 자동 해결
                    if alert.Severity != "critical" {
                        alert.Resolved = true
                        alert.ResolvedAt = time.Now()
                    }
                }
            }
            nm.mutex.Unlock()
        }
    }
}

func (nm *NetworkMonitor) GetMetrics() *NetworkMetrics {
    nm.mutex.RLock()
    defer nm.mutex.RUnlock()

    // 깊은 복사
    metrics := *nm.metrics
    return &metrics
}

func (nm *NetworkMonitor) GetActiveAlerts() []*Alert {
    nm.mutex.RLock()
    defer nm.mutex.RUnlock()

    var activeAlerts []*Alert
    for _, alert := range nm.alerts {
        if !alert.Resolved {
            activeAlerts = append(activeAlerts, alert)
        }
    }

    // 시간순 정렬 (최신순)
    sort.Slice(activeAlerts, func(i, j int) bool {
        return activeAlerts[i].Timestamp.After(activeAlerts[j].Timestamp)
    })

    return activeAlerts
}

func (nm *NetworkMonitor) StartHTTPServer(port string) {
    http.HandleFunc("/", nm.dashboardHandler)
    http.HandleFunc("/api/metrics", nm.metricsHandler)
    http.HandleFunc("/api/alerts", nm.alertsHandler)
    http.HandleFunc("/api/peers", nm.peersHandler)

    fmt.Printf("모니터링 대시보드: http://localhost%s\n", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

func (nm *NetworkMonitor) metricsHandler(w http.ResponseWriter, r *http.Request) {
    metrics := nm.GetMetrics()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}

func (nm *NetworkMonitor) alertsHandler(w http.ResponseWriter, r *http.Request) {
    alerts := nm.GetActiveAlerts()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(alerts)
}

func (nm *NetworkMonitor) peersHandler(w http.ResponseWriter, r *http.Request) {
    nm.mutex.RLock()
    peers := nm.metrics.ConnectedPeers
    nm.mutex.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(peers)
}

func (nm *NetworkMonitor) dashboardHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>IPFS 네트워크 모니터</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; background: #1a1a1a; color: #ffffff; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .dashboard-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .card { background: #2d2d2d; border-radius: 10px; padding: 20px; box-shadow: 0 4px 6px rgba(0,0,0,0.3); }
        .metric { text-align: center; margin: 15px 0; }
        .metric-value { font-size: 2.5em; font-weight: bold; margin-bottom: 5px; }
        .metric-label { color: #aaa; font-size: 0.9em; }
        .health-excellent { color: #00ff88; }
        .health-good { color: #88ff00; }
        .health-warning { color: #ffaa00; }
        .health-critical { color: #ff4444; }
        .alert-item { background: #3d2d2d; margin: 10px 0; padding: 10px; border-radius: 5px; border-left: 4px solid; }
        .alert-critical { border-left-color: #ff4444; }
        .alert-warning { border-left-color: #ffaa00; }
        .alert-info { border-left-color: #0088ff; }
        .peer-list { max-height: 300px; overflow-y: auto; }
        .peer-item { display: flex; justify-content: space-between; padding: 5px 0; border-bottom: 1px solid #444; }
        .node-id { font-family: monospace; font-size: 0.8em; color: #888; }
        h1 { color: #00ff88; }
        h3 { color: #88ff00; margin-top: 0; }
    </style>
    <script>
        function refreshData() {
            fetch('/api/metrics')
                .then(response => response.json())
                .then(data => {
                    updateMetrics(data);
                });

            fetch('/api/alerts')
                .then(response => response.json())
                .then(data => {
                    updateAlerts(data);
                });
        }

        function updateMetrics(metrics) {
            document.getElementById('peer-count').textContent = metrics.peer_count || 0;
            document.getElementById('repo-size').textContent = formatBytes(metrics.repo_stats?.repo_size || 0);
            document.getElementById('network-health').textContent = ((metrics.network_health || 0) * 100).toFixed(1) + '%';
            document.getElementById('response-time').textContent = (metrics.response_time / 1000000).toFixed(0) + 'ms';

            // 건강도 색상 업데이트
            const healthElement = document.getElementById('network-health');
            const health = metrics.network_health || 0;
            healthElement.className = 'metric-value ' + getHealthClass(health);
        }

        function updateAlerts(alerts) {
            const container = document.getElementById('alerts-container');
            container.innerHTML = '';

            alerts.forEach(alert => {
                const alertDiv = document.createElement('div');
                alertDiv.className = 'alert-item alert-' + alert.severity;
                alertDiv.innerHTML = '<strong>' + alert.type + '</strong><br>' + alert.message +
                    '<br><small>' + new Date(alert.timestamp).toLocaleString() + '</small>';
                container.appendChild(alertDiv);
            });

            if (alerts.length === 0) {
                container.innerHTML = '<p style="color: #888; text-align: center;">활성 알림이 없습니다</p>';
            }
        }

        function getHealthClass(health) {
            if (health >= 0.8) return 'health-excellent';
            if (health >= 0.6) return 'health-good';
            if (health >= 0.4) return 'health-warning';
            return 'health-critical';
        }

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
        }

        setInterval(refreshData, 10000); // 10초마다 업데이트
        window.onload = refreshData;
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📊 IPFS 네트워크 모니터</h1>
            <div class="node-id">노드 ID: {{.NodeID}}</div>
        </div>

        <div class="dashboard-grid">
            <div class="card">
                <h3>🌐 네트워크 상태</h3>
                <div class="metric">
                    <div class="metric-value" id="peer-count">{{.PeerCount}}</div>
                    <div class="metric-label">연결된 피어</div>
                </div>
                <div class="metric">
                    <div class="metric-value {{.HealthClass}}" id="network-health">{{.NetworkHealth}}%</div>
                    <div class="metric-label">네트워크 건강도</div>
                </div>
                <div class="metric">
                    <div class="metric-value" id="response-time">{{.ResponseTime}}</div>
                    <div class="metric-label">응답 시간</div>
                </div>
            </div>

            <div class="card">
                <h3>💾 저장소 상태</h3>
                <div class="metric">
                    <div class="metric-value" id="repo-size">{{.RepoSize}}</div>
                    <div class="metric-label">저장소 크기</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{{.NumObjects}}</div>
                    <div class="metric-label">저장된 객체</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{{.PinnedObjects}}</div>
                    <div class="metric-label">Pin된 객체</div>
                </div>
            </div>

            <div class="card">
                <h3>🚨 활성 알림</h3>
                <div id="alerts-container">
                    {{range .Alerts}}
                    <div class="alert-item alert-{{.Severity}}">
                        <strong>{{.Type}}</strong><br>
                        {{.Message}}<br>
                        <small>{{.Timestamp.Format "2006-01-02 15:04:05"}}</small>
                    </div>
                    {{else}}
                    <p style="color: #888; text-align: center;">활성 알림이 없습니다</p>
                    {{end}}
                </div>
            </div>

            <div class="card">
                <h3>👥 연결된 피어</h3>
                <div class="peer-list">
                    {{range .Peers}}
                    <div class="peer-item">
                        <span>{{.ID}}</span>
                        <span>{{.Latency}}</span>
                    </div>
                    {{end}}
                </div>
            </div>
        </div>
    </div>
</body>
</html>`))

    // 템플릿 데이터 준비
    metrics := nm.GetMetrics()
    alerts := nm.GetActiveAlerts()

    healthClass := "health-critical"
    if metrics.NetworkHealth >= 0.8 {
        healthClass = "health-excellent"
    } else if metrics.NetworkHealth >= 0.6 {
        healthClass = "health-good"
    } else if metrics.NetworkHealth >= 0.4 {
        healthClass = "health-warning"
    }

    data := struct {
        NodeID         string
        PeerCount      int
        NetworkHealth  string
        HealthClass    string
        ResponseTime   string
        RepoSize       string
        NumObjects     int64
        PinnedObjects  int
        Alerts         []*Alert
        Peers          []*PeerInfo
    }{
        NodeID:         func() string { if metrics.NodeInfo != nil { return metrics.NodeInfo.ID } else { return "Unknown" } }(),
        PeerCount:      metrics.PeerCount,
        NetworkHealth:  fmt.Sprintf("%.1f", metrics.NetworkHealth*100),
        HealthClass:    healthClass,
        ResponseTime:   fmt.Sprintf("%.0fms", float64(metrics.ResponseTime.Nanoseconds())/1000000),
        RepoSize:       formatBytes(func() int64 { if metrics.RepoStats != nil { return metrics.RepoStats.RepoSize } else { return 0 } }()),
        NumObjects:     func() int64 { if metrics.RepoStats != nil { return metrics.RepoStats.NumObjects } else { return 0 } }(),
        PinnedObjects:  metrics.PinnedObjects,
        Alerts:         alerts,
        Peers:          metrics.ConnectedPeers,
    }

    tmpl.Execute(w, data)
}

func main() {
    // 네트워크 모니터 초기화
    monitor := NewNetworkMonitor("http://127.0.0.1:5001")

    // 모니터링 시작
    monitor.StartMonitoring()

    fmt.Println("📊 IPFS 네트워크 모니터링 시스템")
    fmt.Println("실시간 메트릭 수집이 시작되었습니다")

    // HTTP 대시보드 시작
    monitor.StartHTTPServer(":8080")
}
```

## 4. 🔗 IPFS 기반 링크 단축 서비스

URL을 IPFS에 저장하고 짧은 링크로 접근할 수 있는 서비스입니다.

```go
package main

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "time"

    kubo "github.com/sonheesung/boxo-starter-kit/99-kubo-api-demo/pkg"
)

type LinkShortener struct {
    client    *kubo.KuboClient
    linkDB    map[string]*ShortLink
    analytics map[string]*LinkAnalytics
    mutex     sync.RWMutex
    domain    string
}

type ShortLink struct {
    ID          string    `json:"id"`
    OriginalURL string    `json:"original_url"`
    ShortCode   string    `json:"short_code"`
    IPFSHash    string    `json:"ipfs_hash"`
    CreatedAt   time.Time `json:"created_at"`
    ExpiresAt   time.Time `json:"expires_at,omitempty"`
    CreatorIP   string    `json:"creator_ip"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    IsActive    bool      `json:"is_active"`
    Password    string    `json:"password,omitempty"`
}

type LinkAnalytics struct {
    LinkID      string              `json:"link_id"`
    ClickCount  int64               `json:"click_count"`
    LastClicked time.Time           `json:"last_clicked"`
    Referrers   map[string]int64    `json:"referrers"`
    Countries   map[string]int64    `json:"countries"`
    Devices     map[string]int64    `json:"devices"`
    DailyClicks map[string]int64    `json:"daily_clicks"`
    mutex       sync.RWMutex
}

type ClickEvent struct {
    LinkID    string    `json:"link_id"`
    IP        string    `json:"ip"`
    UserAgent string    `json:"user_agent"`
    Referrer  string    `json:"referrer"`
    Timestamp time.Time `json:"timestamp"`
}

func NewLinkShortener(kuboAPIURL, domain string) *LinkShortener {
    client := kubo.NewKuboClient(kuboAPIURL)

    return &LinkShortener{
        client:    client,
        linkDB:    make(map[string]*ShortLink),
        analytics: make(map[string]*LinkAnalytics),
        domain:    domain,
    }
}

func (ls *LinkShortener) ShortenURL(originalURL, title, description, creatorIP string, expiresAt *time.Time, password string) (*ShortLink, error) {
    // URL 유효성 검사
    parsedURL, err := url.Parse(originalURL)
    if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
        return nil, fmt.Errorf("유효하지 않은 URL입니다")
    }

    // 짧은 코드 생성
    shortCode := ls.generateShortCode()

    // 링크 메타데이터 생성
    linkData := map[string]interface{}{
        "original_url": originalURL,
        "title":        title,
        "description":  description,
        "created_at":   time.Now(),
        "creator_ip":   creatorIP,
    }

    if expiresAt != nil {
        linkData["expires_at"] = *expiresAt
    }

    if password != "" {
        linkData["password"] = ls.hashPassword(password)
    }

    // JSON으로 직렬화
    jsonData, err := json.MarshalIndent(linkData, "", "  ")
    if err != nil {
        return nil, err
    }

    // IPFS에 저장
    ipfsHash, err := ls.client.Add(fmt.Sprintf("link_%s.json", shortCode), jsonData)
    if err != nil {
        return nil, fmt.Errorf("IPFS 저장 실패: %w", err)
    }

    // Pin 추가 (중요한 데이터 보호)
    err = ls.client.PinAdd(ipfsHash)
    if err != nil {
        log.Printf("Pin 추가 실패: %v", err)
    }

    // 짧은 링크 객체 생성
    shortLink := &ShortLink{
        ID:          ls.generateID(),
        OriginalURL: originalURL,
        ShortCode:   shortCode,
        IPFSHash:    ipfsHash,
        CreatedAt:   time.Now(),
        CreatorIP:   creatorIP,
        Title:       title,
        Description: description,
        IsActive:    true,
        Password:    password,
    }

    if expiresAt != nil {
        shortLink.ExpiresAt = *expiresAt
    }

    // 메모리에 저장
    ls.mutex.Lock()
    ls.linkDB[shortCode] = shortLink
    ls.analytics[shortLink.ID] = &LinkAnalytics{
        LinkID:      shortLink.ID,
        Referrers:   make(map[string]int64),
        Countries:   make(map[string]int64),
        Devices:     make(map[string]int64),
        DailyClicks: make(map[string]int64),
    }
    ls.mutex.Unlock()

    fmt.Printf("링크 단축됨: %s -> %s\n", originalURL, shortCode)
    return shortLink, nil
}

func (ls *LinkShortener) ResolveShortLink(shortCode, password, userIP, userAgent, referrer string) (*ShortLink, error) {
    ls.mutex.RLock()
    shortLink, exists := ls.linkDB[shortCode]
    ls.mutex.RUnlock()

    if !exists {
        return nil, fmt.Errorf("링크를 찾을 수 없습니다")
    }

    // 만료 확인
    if !shortLink.ExpiresAt.IsZero() && time.Now().After(shortLink.ExpiresAt) {
        return nil, fmt.Errorf("링크가 만료되었습니다")
    }

    // 활성 상태 확인
    if !shortLink.IsActive {
        return nil, fmt.Errorf("비활성화된 링크입니다")
    }

    // 패스워드 확인
    if shortLink.Password != "" {
        if password == "" {
            return nil, fmt.Errorf("패스워드가 필요합니다")
        }
        if ls.hashPassword(password) != ls.hashPassword(shortLink.Password) {
            return nil, fmt.Errorf("패스워드가 틀렸습니다")
        }
    }

    // 클릭 이벤트 기록
    clickEvent := &ClickEvent{
        LinkID:    shortLink.ID,
        IP:        userIP,
        UserAgent: userAgent,
        Referrer:  referrer,
        Timestamp: time.Now(),
    }

    go ls.recordClickEvent(clickEvent)

    return shortLink, nil
}

func (ls *LinkShortener) recordClickEvent(event *ClickEvent) {
    ls.mutex.Lock()
    defer ls.mutex.Unlock()

    analytics, exists := ls.analytics[event.LinkID]
    if !exists {
        return
    }

    analytics.mutex.Lock()
    defer analytics.mutex.Unlock()

    // 클릭 수 증가
    analytics.ClickCount++
    analytics.LastClicked = event.Timestamp

    // 리퍼러 통계
    if event.Referrer != "" {
        referrerDomain := ls.extractDomain(event.Referrer)
        analytics.Referrers[referrerDomain]++
    } else {
        analytics.Referrers["direct"]++
    }

    // 디바이스 통계 (간단한 User-Agent 분석)
    device := ls.detectDevice(event.UserAgent)
    analytics.Devices[device]++

    // 일일 클릭 통계
    dateKey := event.Timestamp.Format("2006-01-02")
    analytics.DailyClicks[dateKey]++

    // 지역 통계 (실제로는 GeoIP 서비스 사용)
    country := ls.detectCountry(event.IP)
    analytics.Countries[country]++
}

func (ls *LinkShortener) GetLinkAnalytics(linkID string) (*LinkAnalytics, error) {
    ls.mutex.RLock()
    defer ls.mutex.RUnlock()

    analytics, exists := ls.analytics[linkID]
    if !exists {
        return nil, fmt.Errorf("분석 데이터를 찾을 수 없습니다")
    }

    analytics.mutex.RLock()
    defer analytics.mutex.RUnlock()

    // 깊은 복사
    result := &LinkAnalytics{
        LinkID:      analytics.LinkID,
        ClickCount:  analytics.ClickCount,
        LastClicked: analytics.LastClicked,
        Referrers:   make(map[string]int64),
        Countries:   make(map[string]int64),
        Devices:     make(map[string]int64),
        DailyClicks: make(map[string]int64),
    }

    for k, v := range analytics.Referrers {
        result.Referrers[k] = v
    }
    for k, v := range analytics.Countries {
        result.Countries[k] = v
    }
    for k, v := range analytics.Devices {
        result.Devices[k] = v
    }
    for k, v := range analytics.DailyClicks {
        result.DailyClicks[k] = v
    }

    return result, nil
}

func (ls *LinkShortener) generateShortCode() string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    const length = 7

    bytes := make([]byte, length)
    rand.Read(bytes)

    for i, b := range bytes {
        bytes[i] = charset[b%byte(len(charset))]
    }

    return string(bytes)
}

func (ls *LinkShortener) generateID() string {
    data := fmt.Sprintf("%d%d", time.Now().UnixNano(), rand.Int63())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])[:16]
}

func (ls *LinkShortener) hashPassword(password string) string {
    hash := sha256.Sum256([]byte(password + "salt"))
    return hex.EncodeToString(hash[:])
}

func (ls *LinkShortener) extractDomain(urlStr string) string {
    parsedURL, err := url.Parse(urlStr)
    if err != nil {
        return "unknown"
    }
    return parsedURL.Host
}

func (ls *LinkShortener) detectDevice(userAgent string) string {
    ua := strings.ToLower(userAgent)
    if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
        return "mobile"
    } else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
        return "tablet"
    }
    return "desktop"
}

func (ls *LinkShortener) detectCountry(ip string) string {
    // 실제로는 GeoIP 데이터베이스나 API 사용
    // 여기서는 간단한 예시
    if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "127.") {
        return "Local"
    }
    return "Unknown"
}

func (ls *LinkShortener) StartHTTPServer(port string) {
    http.HandleFunc("/", ls.homeHandler)
    http.HandleFunc("/shorten", ls.shortenHandler)
    http.HandleFunc("/s/", ls.redirectHandler)
    http.HandleFunc("/analytics/", ls.analyticsHandler)
    http.HandleFunc("/api/shorten", ls.apiShortenHandler)
    http.HandleFunc("/api/analytics/", ls.apiAnalyticsHandler)

    fmt.Printf("링크 단축 서비스: http://localhost%s\n", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

func (ls *LinkShortener) homeHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.New("home").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>IPFS 링크 단축 서비스</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; background: #f8f9fa; }
        .container { background: white; padding: 40px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .form-group { margin-bottom: 20px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, textarea { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 5px; font-size: 16px; }
        .btn { background: #007cba; color: white; padding: 12px 30px; border: none; border-radius: 5px; cursor: pointer; font-size: 16px; }
        .btn:hover { background: #0056b3; }
        .result { background: #d4edda; border: 1px solid #c3e6cb; border-radius: 5px; padding: 15px; margin-top: 20px; }
        .short-url { font-family: monospace; font-size: 18px; font-weight: bold; color: #007cba; }
        h1 { text-align: center; color: #333; margin-bottom: 30px; }
        .features { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-top: 30px; }
        .feature { text-align: center; padding: 20px; background: #f8f9fa; border-radius: 5px; }
        .feature h3 { color: #007cba; margin-bottom: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔗 IPFS 링크 단축 서비스</h1>

        <form id="shortenForm">
            <div class="form-group">
                <label>원본 URL:</label>
                <input type="url" id="originalUrl" placeholder="https://example.com/very/long/url" required>
            </div>

            <div class="form-group">
                <label>제목 (선택사항):</label>
                <input type="text" id="title" placeholder="링크 제목">
            </div>

            <div class="form-group">
                <label>설명 (선택사항):</label>
                <textarea id="description" rows="3" placeholder="링크 설명"></textarea>
            </div>

            <div class="form-group">
                <label>만료일 (선택사항):</label>
                <input type="datetime-local" id="expiresAt">
            </div>

            <div class="form-group">
                <label>비밀번호 (선택사항):</label>
                <input type="password" id="password" placeholder="접근 제한을 위한 비밀번호">
            </div>

            <button type="submit" class="btn">링크 단축하기</button>
        </form>

        <div id="result" class="result" style="display: none;">
            <h3>단축된 링크:</h3>
            <div class="short-url" id="shortUrl"></div>
            <p>이 링크는 IPFS에 안전하게 저장되었습니다.</p>
            <p>분석 페이지: <a id="analyticsUrl" href="#" target="_blank">통계 보기</a></p>
        </div>

        <div class="features">
            <div class="feature">
                <h3>🌐 분산 저장</h3>
                <p>링크 데이터가 IPFS에 저장되어 중앙 서버 없이도 접근 가능</p>
            </div>
            <div class="feature">
                <h3>📊 상세 분석</h3>
                <p>클릭 수, 리퍼러, 지역별 통계 등 상세한 분석 제공</p>
            </div>
            <div class="feature">
                <h3>🔒 보안 기능</h3>
                <p>패스워드 보호, 만료일 설정 등 보안 기능</p>
            </div>
            <div class="feature">
                <h3>⚡ 빠른 접근</h3>
                <p>IPFS 네트워크를 통한 빠른 전 세계 접근</p>
            </div>
        </div>
    </div>

    <script>
        document.getElementById('shortenForm').addEventListener('submit', async function(e) {
            e.preventDefault();

            const formData = {
                original_url: document.getElementById('originalUrl').value,
                title: document.getElementById('title').value,
                description: document.getElementById('description').value,
                expires_at: document.getElementById('expiresAt').value,
                password: document.getElementById('password').value
            };

            try {
                const response = await fetch('/api/shorten', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(formData)
                });

                const result = await response.json();

                if (response.ok) {
                    const shortUrl = window.location.origin + '/s/' + result.short_code;
                    document.getElementById('shortUrl').textContent = shortUrl;
                    document.getElementById('analyticsUrl').href = '/analytics/' + result.id;
                    document.getElementById('result').style.display = 'block';
                } else {
                    alert('에러: ' + result.error);
                }
            } catch (error) {
                alert('요청 실패: ' + error.message);
            }
        });
    </script>
</body>
</html>`))

    tmpl.Execute(w, nil)
}

func (ls *LinkShortener) redirectHandler(w http.ResponseWriter, r *http.Request) {
    shortCode := strings.TrimPrefix(r.URL.Path, "/s/")
    password := r.URL.Query().Get("password")

    userIP := ls.getClientIP(r)
    userAgent := r.Header.Get("User-Agent")
    referrer := r.Header.Get("Referer")

    shortLink, err := ls.ResolveShortLink(shortCode, password, userIP, userAgent, referrer)
    if err != nil {
        if strings.Contains(err.Error(), "패스워드") {
            // 패스워드 입력 페이지 표시
            ls.showPasswordPage(w, shortCode, err.Error())
            return
        }
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    // 리다이렉트
    http.Redirect(w, r, shortLink.OriginalURL, http.StatusTemporaryRedirect)
}

func (ls *LinkShortener) showPasswordPage(w http.ResponseWriter, shortCode, errorMsg string) {
    tmpl := template.Must(template.New("password").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>비밀번호 입력</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
        .container { background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        input { width: 100%; padding: 10px; margin: 10px 0; border: 1px solid #ddd; border-radius: 5px; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; border-radius: 5px; cursor: pointer; width: 100%; }
        .error { color: #dc3545; margin-bottom: 15px; }
        h2 { text-align: center; color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <h2>🔒 보호된 링크</h2>
        {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
        <p>이 링크에 접근하려면 비밀번호가 필요합니다.</p>

        <form method="get">
            <input type="password" name="password" placeholder="비밀번호 입력" required>
            <button type="submit" class="btn">접근하기</button>
        </form>
    </div>
</body>
</html>`))

    data := struct {
        Error string
    }{
        Error: errorMsg,
    }

    tmpl.Execute(w, data)
}

func (ls *LinkShortener) apiShortenHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "POST 메서드만 허용", http.StatusMethodNotAllowed)
        return
    }

    var req struct {
        OriginalURL string `json:"original_url"`
        Title       string `json:"title"`
        Description string `json:"description"`
        ExpiresAt   string `json:"expires_at"`
        Password    string `json:"password"`
    }

    err := json.NewDecoder(r.Body).Decode(&req)
    if err != nil {
        http.Error(w, "잘못된 JSON", http.StatusBadRequest)
        return
    }

    var expiresAt *time.Time
    if req.ExpiresAt != "" {
        parsed, err := time.Parse("2006-01-02T15:04", req.ExpiresAt)
        if err == nil {
            expiresAt = &parsed
        }
    }

    creatorIP := ls.getClientIP(r)
    shortLink, err := ls.ShortenURL(req.OriginalURL, req.Title, req.Description, creatorIP, expiresAt, req.Password)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(shortLink)
}

func (ls *LinkShortener) analyticsHandler(w http.ResponseWriter, r *http.Request) {
    linkID := strings.TrimPrefix(r.URL.Path, "/analytics/")

    analytics, err := ls.GetLinkAnalytics(linkID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    // 링크 정보 찾기
    var shortLink *ShortLink
    ls.mutex.RLock()
    for _, link := range ls.linkDB {
        if link.ID == linkID {
            shortLink = link
            break
        }
    }
    ls.mutex.RUnlock()

    if shortLink == nil {
        http.Error(w, "링크를 찾을 수 없습니다", http.StatusNotFound)
        return
    }

    tmpl := template.Must(template.New("analytics").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>링크 분석 - {{.Link.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f8f9fa; }
        .container { max-width: 1200px; margin: 0 auto; }
        .card { background: white; border-radius: 10px; padding: 20px; margin: 20px 0; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .metric { text-align: center; margin: 15px; }
        .metric-value { font-size: 2em; font-weight: bold; color: #007cba; }
        .metric-label { color: #666; }
        .chart-container { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .chart { background: #f8f9fa; padding: 15px; border-radius: 5px; }
        .chart h4 { margin-top: 0; color: #333; }
        .bar { background: #007cba; height: 20px; margin: 5px 0; border-radius: 3px; }
        .bar-label { display: flex; justify-content: space-between; margin-bottom: 3px; }
        h1 { text-align: center; color: #333; }
        .link-info { background: #e9ecef; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 링크 분석</h1>

        <div class="link-info">
            <h3>{{.Link.Title}}</h3>
            <p><strong>원본 URL:</strong> <a href="{{.Link.OriginalURL}}" target="_blank">{{.Link.OriginalURL}}</a></p>
            <p><strong>단축 링크:</strong> {{.Domain}}/s/{{.Link.ShortCode}}</p>
            <p><strong>생성일:</strong> {{.Link.CreatedAt.Format "2006-01-02 15:04:05"}}</p>
            <p><strong>IPFS 해시:</strong> {{.Link.IPFSHash}}</p>
        </div>

        <div class="card">
            <div class="metric">
                <div class="metric-value">{{.Analytics.ClickCount}}</div>
                <div class="metric-label">총 클릭 수</div>
            </div>
            {{if not .Analytics.LastClicked.IsZero}}
            <div class="metric">
                <div class="metric-label">마지막 클릭: {{.Analytics.LastClicked.Format "2006-01-02 15:04:05"}}</div>
            </div>
            {{end}}
        </div>

        <div class="chart-container">
            <div class="chart">
                <h4>🌐 리퍼러</h4>
                {{range $referrer, $count := .Analytics.Referrers}}
                <div class="bar-label">
                    <span>{{$referrer}}</span>
                    <span>{{$count}}</span>
                </div>
                <div class="bar" style="width: {{percentage $count $.Analytics.ClickCount}}%;"></div>
                {{end}}
            </div>

            <div class="chart">
                <h4>📱 디바이스</h4>
                {{range $device, $count := .Analytics.Devices}}
                <div class="bar-label">
                    <span>{{$device}}</span>
                    <span>{{$count}}</span>
                </div>
                <div class="bar" style="width: {{percentage $count $.Analytics.ClickCount}}%;"></div>
                {{end}}
            </div>

            <div class="chart">
                <h4>🌍 지역</h4>
                {{range $country, $count := .Analytics.Countries}}
                <div class="bar-label">
                    <span>{{$country}}</span>
                    <span>{{$count}}</span>
                </div>
                <div class="bar" style="width: {{percentage $count $.Analytics.ClickCount}}%;"></div>
                {{end}}
            </div>
        </div>
    </div>
</body>
</html>`))

    funcMap := template.FuncMap{
        "percentage": func(part, total int64) int {
            if total == 0 {
                return 0
            }
            return int(float64(part) / float64(total) * 100)
        },
    }

    tmpl = tmpl.Funcs(funcMap)

    data := struct {
        Link      *ShortLink
        Analytics *LinkAnalytics
        Domain    string
    }{
        Link:      shortLink,
        Analytics: analytics,
        Domain:    ls.domain,
    }

    tmpl.Execute(w, data)
}

func (ls *LinkShortener) getClientIP(r *http.Request) string {
    // X-Forwarded-For 헤더 확인
    xff := r.Header.Get("X-Forwarded-For")
    if xff != "" {
        ips := strings.Split(xff, ",")
        return strings.TrimSpace(ips[0])
    }

    // X-Real-IP 헤더 확인
    xri := r.Header.Get("X-Real-IP")
    if xri != "" {
        return xri
    }

    // RemoteAddr 사용
    ip := r.RemoteAddr
    if strings.Contains(ip, ":") {
        ip = strings.Split(ip, ":")[0]
    }

    return ip
}

func main() {
    // 링크 단축 서비스 초기화
    shortener := NewLinkShortener("http://127.0.0.1:5001", "http://localhost:8080")

    fmt.Println("🔗 IPFS 링크 단축 서비스")
    fmt.Println("모든 링크 데이터는 IPFS에 안전하게 저장됩니다")

    // HTTP 서버 시작
    shortener.StartHTTPServer(":8080")
}
```

---

이 쿡북의 예제들을 통해 Kubo HTTP API의 실전 활용법을 익힐 수 있습니다:

1. **파일 백업 시스템**: 실시간 파일 감시와 자동 IPFS 백업
2. **분산 CDN**: 전 세계 IPFS 노드를 활용한 콘텐츠 배포
3. **네트워크 모니터링**: IPFS 노드의 상태를 실시간으로 모니터링
4. **링크 단축 서비스**: IPFS 기반의 안전한 URL 단축 서비스

각 예제는 실제 프로덕션 환경에서 사용할 수 있는 완전한 기능을 제공하며, Kubo API의 다양한 기능을 활용합니다.