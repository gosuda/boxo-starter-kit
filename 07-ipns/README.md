# 07-ipns: InterPlanetary Name System 구현

## 🎯 학습 목표
- IPNS(InterPlanetary Name System)의 동작 원리와 목적 이해
- 변경 가능한 콘텐츠를 IPFS에서 관리하는 방법 학습
- 디지털 서명을 통한 콘텐츠 인증 구현
- DNS와 유사한 네이밍 시스템 구축
- 실시간 업데이트가 필요한 IPFS 애플리케이션 개발

## 📋 사전 요구사항
- **이전 챕터**: 00-block-cid, 01-persistent, 02-dag-ipld, 03-unixfs 완료
- **기술 지식**: 암호학 기초(공개키/개인키), DNS 개념, 디지털 서명
- **Go 지식**: 암호화 패키지, 시간 처리, 구조체 직렬화

## 🔑 핵심 개념

### IPNS란?
IPNS(InterPlanetary Name System)는 IPFS의 불변성 문제를 해결하기 위한 변경 가능한 네이밍 시스템입니다.

#### IPFS vs IPNS 비교
```
IPFS (불변):
/ipfs/QmHash123... → 항상 같은 콘텐츠

IPNS (변경 가능):
/ipns/Qm12D3Ko... → 시간에 따라 다른 IPFS 해시를 가리킴
```

#### IPNS의 핵심 구성요소
- **IPNS Name**: 공개키에서 파생된 고유 식별자
- **IPNS Record**: 가리키는 IPFS 경로와 메타데이터
- **Digital Signature**: 개인키로 서명된 레코드 인증
- **TTL (Time To Live)**: 레코드 유효 기간

### IPNS 동작 과정
1. **키 쌍 생성**: 개인키/공개키 쌍 생성
2. **레코드 생성**: IPFS 경로를 가리키는 IPNS 레코드 작성
3. **서명**: 개인키로 레코드에 디지털 서명
4. **발행**: DHT에 IPNS 레코드 저장
5. **해석**: IPNS 이름을 통해 최신 IPFS 경로 조회

## 💻 코드 분석

### 1. IPNS 매니저 구조체
```go
type IPNSManager struct {
    privateKey   crypto.PrivKey
    publicKey    crypto.PubKey
    dagWrapper   *dag.DAGWrapper
    recordStore  map[string]*ipns.Record
    recordMutex  sync.RWMutex
}
```

**설계 결정**:
- `privateKey/publicKey`: 암호화 키 쌍으로 레코드 서명/검증
- `dagWrapper`: IPFS 콘텐츠와의 통합
- `recordStore`: 메모리 기반 레코드 저장소 (실제로는 DHT 사용)

### 2. 키 쌍 생성
```go
func generateKeyPair() (crypto.PrivKey, crypto.PubKey, error) {
    privateKey, publicKey, err := crypto.GenerateKeyPair(crypto.Ed25519, 2048)
    if err != nil {
        return nil, nil, fmt.Errorf("키 쌍 생성 실패: %w", err)
    }
    return privateKey, publicKey, nil
}
```

**Ed25519 선택 이유**:
- 빠른 서명/검증 속도
- 작은 키 크기 (32바이트)
- 높은 보안성

### 3. IPNS 레코드 생성 및 서명
```go
func (im *IPNSManager) PublishRecord(ipfsPath string, ttl time.Duration) (string, error) {
    // Path 객체 생성
    p, err := path.FromString(ipfsPath)
    if err != nil {
        return "", fmt.Errorf("경로 파싱 실패: %w", err)
    }

    // IPNS 레코드 생성
    record, err := ipns.NewRecord(im.privateKey, p, 1, time.Now().Add(ttl), ttl)
    if err != nil {
        return "", fmt.Errorf("레코드 생성 실패: %w", err)
    }

    // 레코드 저장
    peerID, err := peer.IDFromPrivateKey(im.privateKey)
    if err != nil {
        return "", fmt.Errorf("Peer ID 생성 실패: %w", err)
    }

    ipnsName := ipns.NameFromPeer(peerID)
    im.recordStore[ipnsName.String()] = record

    return ipnsName.String(), nil
}
```

## 🏃‍♂️ 실습 가이드

### 단계 1: IPNS 매니저 생성
```bash
cd 07-ipns
go run main.go
```

### 단계 2: 콘텐츠 발행 테스트
```go
// 1. 파일을 IPFS에 추가
content := "Hello, IPNS World!"
ipfsHash := addToIPFS(content)

// 2. IPNS 레코드 발행
ipnsName := publishIPNS(ipfsHash)

// 3. IPNS 해석
resolvedPath := resolveIPNS(ipnsName)
```

### 단계 3: 콘텐츠 업데이트
```go
// 새로운 콘텐츠로 업데이트
newContent := "Updated content via IPNS!"
newHash := addToIPFS(newContent)

// 같은 IPNS 이름으로 새 레코드 발행
updateIPNS(ipnsName, newHash)
```

### 예상 결과
- **초기 발행**: IPNS 이름이 첫 번째 콘텐츠를 가리킴
- **업데이트**: 같은 IPNS 이름이 새로운 콘텐츠를 가리킴
- **일관된 접근**: 외부에서는 변경되지 않는 IPNS 이름 사용

## 🚀 고급 활용 사례

### 1. 웹사이트 동적 업데이트
```go
type Website struct {
    ipnsManager *IPNSManager
    ipnsName    string
}

func (w *Website) UpdateContent(htmlContent string) error {
    // HTML을 IPFS에 추가
    ipfsHash, err := w.addHTML(htmlContent)
    if err != nil {
        return err
    }

    // IPNS 레코드 업데이트
    _, err = w.ipnsManager.PublishRecord(
        fmt.Sprintf("/ipfs/%s", ipfsHash),
        24*time.Hour,
    )
    return err
}
```

### 2. 설정 파일 배포
```go
type ConfigDistributor struct {
    ipnsManager *IPNSManager
    configName  string
}

func (cd *ConfigDistributor) UpdateConfig(config map[string]interface{}) error {
    // JSON 직렬화
    jsonData, _ := json.Marshal(config)

    // IPFS 추가 및 IPNS 발행
    ipfsHash := cd.addToIPFS(jsonData)
    return cd.updateIPNS(ipfsHash)
}
```

### 3. 버전 관리 시스템
```go
type VersionedContent struct {
    ipnsManager *IPNSManager
    versions    []string
    current     int
}

func (vc *VersionedContent) Rollback(steps int) error {
    targetVersion := vc.current - steps
    if targetVersion < 0 {
        return errors.New("버전이 없습니다")
    }

    return vc.ipnsManager.PublishRecord(
        vc.versions[targetVersion],
        time.Hour*24,
    )
}
```

## 🔧 성능 최적화

### TTL 전략
```go
const (
    ShortTTL  = 5 * time.Minute   // 자주 변경되는 콘텐츠
    MediumTTL = 1 * time.Hour     // 일반적인 사용
    LongTTL   = 24 * time.Hour    // 안정적인 콘텐츠
)
```

### 캐싱 전략
```go
type CachedIPNSResolver struct {
    cache     map[string]CacheEntry
    cacheTTL  time.Duration
    mutex     sync.RWMutex
}

type CacheEntry struct {
    Path      string
    ExpiresAt time.Time
}
```

### 배치 업데이트
```go
func (im *IPNSManager) BatchUpdate(updates map[string]string) error {
    // 여러 IPNS 레코드를 한 번에 업데이트
    for ipnsName, ipfsPath := range updates {
        go im.PublishRecord(ipfsPath, time.Hour)
    }
    return nil
}
```

## 🔒 보안 고려사항

### 키 관리
```go
func (im *IPNSManager) ExportKey() ([]byte, error) {
    // 개인키를 안전하게 내보내기
    return crypto.MarshalPrivateKey(im.privateKey)
}

func LoadKeyFromFile(filename string) (crypto.PrivKey, error) {
    // 파일에서 개인키 로드
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    return crypto.UnmarshalPrivateKey(data)
}
```

### 레코드 검증
```go
func (im *IPNSManager) ValidateRecord(name string, record *ipns.Record) error {
    // 레코드 서명 검증
    pubKey, err := im.getPublicKey(name)
    if err != nil {
        return err
    }

    return ipns.Validate(pubKey, record)
}
```

## 🐛 트러블슈팅

### 문제 1: 레코드 발행 실패
**증상**: `PublishRecord` 호출 시 에러
**원인**: 잘못된 IPFS 경로 또는 키 문제
**해결책**:
```go
// IPFS 경로 유효성 검사
if !strings.HasPrefix(ipfsPath, "/ipfs/") {
    return "", errors.New("올바른 IPFS 경로가 아닙니다")
}
```

### 문제 2: 해석 시간 초과
**증상**: `ResolveRecord` 호출이 오래 걸림
**원인**: DHT 네트워크 연결 문제
**해결책**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

### 문제 3: 캐시 무효화
**증상**: 오래된 콘텐츠가 계속 표시됨
**원인**: TTL 설정 문제
**해결책**:
```go
// 강제 새로고침
record.TTL = 0
im.PublishRecord(newPath, time.Minute)
```

## 🔗 연계 학습
- **다음 단계**: 99-kubo-api-demo (실제 네트워크 연동)
- **고급 주제**:
  - DNS-Link 통합
  - IPNS over PubSub
  - 분산 웹사이트 배포

## 📚 참고 자료
- [IPNS Specification](https://docs.ipfs.tech/concepts/ipns/)
- [go-ipns Documentation](https://pkg.go.dev/github.com/ipfs/boxo/ipns)
- [Cryptographic Keys in IPFS](https://docs.ipfs.tech/concepts/cryptographic-keys/)

---

# 🍳 실전 쿡북: 바로 쓸 수 있는 코드

## 1. 📰 뉴스 피드 시스템

실시간으로 업데이트되는 뉴스 피드를 IPNS로 관리하는 시스템입니다.

```go
package main

import (
    "encoding/json"
    "fmt"
    "time"
    "context"
    "sync"

    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/peer"
    "github.com/ipfs/boxo/ipns"
    "github.com/ipfs/boxo/path"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type NewsFeed struct {
    ipnsManager   *IPNSManager
    unixfsWrapper *unixfs.UnixFsWrapper
    feedName      string
    articles      []Article
    mutex         sync.RWMutex
}

type Article struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Content     string    `json:"content"`
    Author      string    `json:"author"`
    PublishedAt time.Time `json:"published_at"`
    Tags        []string  `json:"tags"`
}

type Feed struct {
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Articles    []Article `json:"articles"`
    LastUpdate  time.Time `json:"last_update"`
    Version     int       `json:"version"`
}

func NewNewsFeed(title, description string) (*NewsFeed, error) {
    // IPNS 매니저 초기화
    ipnsManager, err := NewIPNSManager()
    if err != nil {
        return nil, err
    }

    // UnixFS 래퍼 초기화
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    nf := &NewsFeed{
        ipnsManager:   ipnsManager,
        unixfsWrapper: unixfsWrapper,
        articles:      make([]Article, 0),
    }

    // 초기 피드 생성
    err = nf.initializeFeed(title, description)
    if err != nil {
        return nil, err
    }

    return nf, nil
}

func (nf *NewsFeed) initializeFeed(title, description string) error {
    feed := Feed{
        Title:       title,
        Description: description,
        Articles:    nf.articles,
        LastUpdate:  time.Now(),
        Version:     1,
    }

    // JSON으로 직렬화
    feedData, err := json.MarshalIndent(feed, "", "  ")
    if err != nil {
        return err
    }

    // IPFS에 추가
    ctx := context.Background()
    cid, err := nf.unixfsWrapper.AddFile(ctx, "feed.json", feedData)
    if err != nil {
        return err
    }

    // IPNS 레코드 발행
    feedName, err := nf.ipnsManager.PublishRecord(
        fmt.Sprintf("/ipfs/%s", cid.String()),
        time.Hour*24,
    )
    if err != nil {
        return err
    }

    nf.feedName = feedName
    fmt.Printf("뉴스 피드 생성됨: /ipns/%s\n", feedName)
    return nil
}

func (nf *NewsFeed) AddArticle(title, content, author string, tags []string) error {
    nf.mutex.Lock()
    defer nf.mutex.Unlock()

    // 새 기사 생성
    article := Article{
        ID:          fmt.Sprintf("article_%d", time.Now().Unix()),
        Title:       title,
        Content:     content,
        Author:      author,
        PublishedAt: time.Now(),
        Tags:        tags,
    }

    // 기사 목록에 추가 (최신순으로 정렬)
    nf.articles = append([]Article{article}, nf.articles...)

    // 최대 50개 기사만 유지
    if len(nf.articles) > 50 {
        nf.articles = nf.articles[:50]
    }

    return nf.updateFeed()
}

func (nf *NewsFeed) updateFeed() error {
    feed := Feed{
        Title:       "IPFS 뉴스 피드",
        Description: "IPNS로 관리되는 실시간 뉴스 피드",
        Articles:    nf.articles,
        LastUpdate:  time.Now(),
        Version:     len(nf.articles),
    }

    // JSON으로 직렬화
    feedData, err := json.MarshalIndent(feed, "", "  ")
    if err != nil {
        return err
    }

    // IPFS에 추가
    ctx := context.Background()
    cid, err := nf.unixfsWrapper.AddFile(ctx, "feed.json", feedData)
    if err != nil {
        return err
    }

    // IPNS 레코드 업데이트
    _, err = nf.ipnsManager.PublishRecord(
        fmt.Sprintf("/ipfs/%s", cid.String()),
        time.Hour*6, // 6시간 TTL
    )

    if err == nil {
        fmt.Printf("피드 업데이트됨: %d개 기사\n", len(nf.articles))
    }

    return err
}

func (nf *NewsFeed) GetFeedName() string {
    return nf.feedName
}

func (nf *NewsFeed) GetLatestFeed() (*Feed, error) {
    nf.mutex.RLock()
    defer nf.mutex.RUnlock()

    // 현재 IPNS 레코드 해석
    ipfsPath, err := nf.ipnsManager.ResolveRecord(nf.feedName)
    if err != nil {
        return nil, err
    }

    // IPFS에서 피드 데이터 가져오기
    ctx := context.Background()
    data, err := nf.unixfsWrapper.GetFile(ctx, ipfsPath)
    if err != nil {
        return nil, err
    }

    // JSON 파싱
    var feed Feed
    err = json.Unmarshal(data, &feed)
    if err != nil {
        return nil, err
    }

    return &feed, nil
}

// 자동 피드 생성기
func (nf *NewsFeed) StartAutoGenerator() {
    go func() {
        sampleArticles := []struct {
            title   string
            content string
            author  string
            tags    []string
        }{
            {"IPFS 2.0 출시", "분산 파일 시스템의 새로운 기능들이 추가되었습니다.", "IPFS Team", []string{"ipfs", "release"}},
            {"웹3 기술 동향", "블록체인과 분산 시스템의 최신 트렌드를 살펴봅니다.", "Tech Reporter", []string{"web3", "blockchain"}},
            {"IPNS 성능 개선", "네임 시스템의 해석 속도가 크게 향상되었습니다.", "Dev Team", []string{"ipns", "performance"}},
            {"분산 웹 보안", "탈중앙화 웹 애플리케이션의 보안 고려사항", "Security Expert", []string{"security", "dweb"}},
        }

        for i := 0; i < len(sampleArticles); i++ {
            time.Sleep(30 * time.Second) // 30초마다 새 기사

            article := sampleArticles[i%len(sampleArticles)]
            nf.AddArticle(
                fmt.Sprintf("%s #%d", article.title, i+1),
                article.content,
                article.author,
                article.tags,
            )
        }
    }()
}

func main() {
    // 뉴스 피드 생성
    feed, err := NewNewsFeed("IPFS 기술 뉴스", "IPFS와 분산 웹 기술 관련 최신 소식")
    if err != nil {
        panic(err)
    }

    // 초기 기사 추가
    feed.AddArticle(
        "IPFS 뉴스 피드 시작",
        "IPNS를 사용한 실시간 뉴스 피드가 시작되었습니다. 이 피드는 자동으로 업데이트됩니다.",
        "System",
        []string{"announcement", "ipfs", "ipns"},
    )

    // 자동 피드 생성 시작
    feed.StartAutoGenerator()

    fmt.Printf("뉴스 피드 IPNS: /ipns/%s\n", feed.GetFeedName())
    fmt.Println("30초마다 새로운 기사가 추가됩니다...")

    // 1분마다 현재 피드 상태 출력
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            currentFeed, err := feed.GetLatestFeed()
            if err != nil {
                fmt.Printf("피드 조회 오류: %v\n", err)
                continue
            }

            fmt.Printf("\n=== 현재 피드 상태 ===\n")
            fmt.Printf("제목: %s\n", currentFeed.Title)
            fmt.Printf("기사 수: %d\n", len(currentFeed.Articles))
            fmt.Printf("마지막 업데이트: %s\n", currentFeed.LastUpdate.Format("2006-01-02 15:04:05"))

            if len(currentFeed.Articles) > 0 {
                latest := currentFeed.Articles[0]
                fmt.Printf("최신 기사: %s (by %s)\n", latest.Title, latest.Author)
            }
        }
    }
}
```

## 2. 🏢 기업 설정 관리 시스템

중앙 집중식 설정 배포 및 버전 관리 시스템입니다.

```go
package main

import (
    "encoding/json"
    "fmt"
    "time"
    "context"
    "sync"

    "github.com/libp2p/go-libp2p/core/crypto"
    ipns "github.com/sonheesung/boxo-starter-kit/07-ipns/pkg"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type ConfigManager struct {
    ipnsManager   *ipns.IPNSManager
    unixfsWrapper *unixfs.UnixFsWrapper
    configs       map[string]*ConfigSet
    mutex         sync.RWMutex
}

type ConfigSet struct {
    Name        string                 `json:"name"`
    Environment string                 `json:"environment"`
    Version     string                 `json:"version"`
    Config      map[string]interface{} `json:"config"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
    IPNSName    string                 `json:"ipns_name"`
}

type ConfigHistory struct {
    Versions []ConfigVersion `json:"versions"`
}

type ConfigVersion struct {
    Version   string    `json:"version"`
    IPFSHash  string    `json:"ipfs_hash"`
    CreatedAt time.Time `json:"created_at"`
    Changes   []string  `json:"changes"`
}

func NewConfigManager() (*ConfigManager, error) {
    // IPNS 매니저 초기화
    ipnsManager, err := ipns.NewIPNSManager()
    if err != nil {
        return nil, err
    }

    // UnixFS 래퍼 초기화
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    return &ConfigManager{
        ipnsManager:   ipnsManager,
        unixfsWrapper: unixfsWrapper,
        configs:       make(map[string]*ConfigSet),
    }, nil
}

func (cm *ConfigManager) CreateConfigSet(name, environment string, initialConfig map[string]interface{}) (*ConfigSet, error) {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()

    configSet := &ConfigSet{
        Name:        name,
        Environment: environment,
        Version:     "1.0.0",
        Config:      initialConfig,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    // IPFS에 설정 저장
    err := cm.saveConfigToIPFS(configSet)
    if err != nil {
        return nil, err
    }

    cm.configs[cm.getConfigKey(name, environment)] = configSet
    fmt.Printf("설정 세트 생성됨: %s/%s (IPNS: %s)\n", name, environment, configSet.IPNSName)

    return configSet, nil
}

func (cm *ConfigManager) UpdateConfig(name, environment, newVersion string, updates map[string]interface{}, changes []string) error {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()

    key := cm.getConfigKey(name, environment)
    configSet, exists := cm.configs[key]
    if !exists {
        return fmt.Errorf("설정을 찾을 수 없습니다: %s/%s", name, environment)
    }

    // 기존 설정에 업데이트 적용
    for k, v := range updates {
        configSet.Config[k] = v
    }

    configSet.Version = newVersion
    configSet.UpdatedAt = time.Now()

    // IPFS에 업데이트된 설정 저장
    err := cm.saveConfigToIPFS(configSet)
    if err != nil {
        return err
    }

    fmt.Printf("설정 업데이트됨: %s/%s v%s\n", name, environment, newVersion)
    return nil
}

func (cm *ConfigManager) saveConfigToIPFS(configSet *ConfigSet) error {
    // JSON으로 직렬화
    configData, err := json.MarshalIndent(configSet, "", "  ")
    if err != nil {
        return err
    }

    // IPFS에 추가
    ctx := context.Background()
    cid, err := cm.unixfsWrapper.AddFile(ctx, "config.json", configData)
    if err != nil {
        return err
    }

    // IPNS 레코드 발행 또는 업데이트
    if configSet.IPNSName == "" {
        // 새로운 IPNS 이름 생성
        ipnsName, err := cm.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*24,
        )
        if err != nil {
            return err
        }
        configSet.IPNSName = ipnsName
    } else {
        // 기존 IPNS 레코드 업데이트
        _, err := cm.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*24,
        )
        if err != nil {
            return err
        }
    }

    return nil
}

func (cm *ConfigManager) GetConfig(name, environment string) (*ConfigSet, error) {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()

    key := cm.getConfigKey(name, environment)
    configSet, exists := cm.configs[key]
    if !exists {
        return nil, fmt.Errorf("설정을 찾을 수 없습니다: %s/%s", name, environment)
    }

    return configSet, nil
}

func (cm *ConfigManager) GetConfigFromIPNS(ipnsName string) (*ConfigSet, error) {
    // IPNS 해석
    ipfsPath, err := cm.ipnsManager.ResolveRecord(ipnsName)
    if err != nil {
        return nil, err
    }

    // IPFS에서 설정 데이터 가져오기
    ctx := context.Background()
    data, err := cm.unixfsWrapper.GetFile(ctx, ipfsPath)
    if err != nil {
        return nil, err
    }

    // JSON 파싱
    var configSet ConfigSet
    err = json.Unmarshal(data, &configSet)
    if err != nil {
        return nil, err
    }

    return &configSet, nil
}

func (cm *ConfigManager) ListConfigs() []*ConfigSet {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()

    configs := make([]*ConfigSet, 0, len(cm.configs))
    for _, config := range cm.configs {
        configs = append(configs, config)
    }

    return configs
}

func (cm *ConfigManager) getConfigKey(name, environment string) string {
    return fmt.Sprintf("%s/%s", name, environment)
}

// 설정 동기화 클라이언트
type ConfigClient struct {
    configManager *ConfigManager
    subscriptions map[string]*ConfigSubscription
    mutex         sync.RWMutex
}

type ConfigSubscription struct {
    IPNSName     string
    PollInterval time.Duration
    OnUpdate     func(*ConfigSet)
    stopChan     chan bool
}

func NewConfigClient() (*ConfigClient, error) {
    configManager, err := NewConfigManager()
    if err != nil {
        return nil, err
    }

    return &ConfigClient{
        configManager: configManager,
        subscriptions: make(map[string]*ConfigSubscription),
    }, nil
}

func (cc *ConfigClient) Subscribe(ipnsName string, pollInterval time.Duration, onUpdate func(*ConfigSet)) error {
    cc.mutex.Lock()
    defer cc.mutex.Unlock()

    if _, exists := cc.subscriptions[ipnsName]; exists {
        return fmt.Errorf("이미 구독 중입니다: %s", ipnsName)
    }

    subscription := &ConfigSubscription{
        IPNSName:     ipnsName,
        PollInterval: pollInterval,
        OnUpdate:     onUpdate,
        stopChan:     make(chan bool),
    }

    cc.subscriptions[ipnsName] = subscription
    go cc.pollConfig(subscription)

    fmt.Printf("설정 구독 시작: %s (폴링 간격: %v)\n", ipnsName, pollInterval)
    return nil
}

func (cc *ConfigClient) pollConfig(subscription *ConfigSubscription) {
    ticker := time.NewTicker(subscription.PollInterval)
    defer ticker.Stop()

    var lastVersion string

    for {
        select {
        case <-ticker.C:
            config, err := cc.configManager.GetConfigFromIPNS(subscription.IPNSName)
            if err != nil {
                fmt.Printf("설정 조회 실패: %v\n", err)
                continue
            }

            if config.Version != lastVersion {
                lastVersion = config.Version
                fmt.Printf("설정 변경 감지: %s v%s\n", config.Name, config.Version)
                subscription.OnUpdate(config)
            }

        case <-subscription.stopChan:
            return
        }
    }
}

func (cc *ConfigClient) Unsubscribe(ipnsName string) {
    cc.mutex.Lock()
    defer cc.mutex.Unlock()

    if subscription, exists := cc.subscriptions[ipnsName]; exists {
        close(subscription.stopChan)
        delete(cc.subscriptions, ipnsName)
        fmt.Printf("설정 구독 해제: %s\n", ipnsName)
    }
}

func main() {
    // 설정 관리자 생성
    configManager, err := NewConfigManager()
    if err != nil {
        panic(err)
    }

    // 개발 환경 설정 생성
    devConfig := map[string]interface{}{
        "database_url":    "postgres://localhost:5432/myapp_dev",
        "redis_url":       "redis://localhost:6379",
        "log_level":       "debug",
        "api_rate_limit":  1000,
        "feature_flags": map[string]bool{
            "new_ui":       true,
            "beta_feature": false,
        },
    }

    configSet, err := configManager.CreateConfigSet("myapp", "development", devConfig)
    if err != nil {
        panic(err)
    }

    // 프로덕션 환경 설정 생성
    prodConfig := map[string]interface{}{
        "database_url":    "postgres://prod-db:5432/myapp",
        "redis_url":       "redis://prod-redis:6379",
        "log_level":       "info",
        "api_rate_limit":  100,
        "feature_flags": map[string]bool{
            "new_ui":       false,
            "beta_feature": false,
        },
    }

    prodConfigSet, err := configManager.CreateConfigSet("myapp", "production", prodConfig)
    if err != nil {
        panic(err)
    }

    // 설정 클라이언트 생성 및 구독
    client, err := NewConfigClient()
    if err != nil {
        panic(err)
    }

    // 개발 환경 설정 구독
    err = client.Subscribe(configSet.IPNSName, 30*time.Second, func(config *ConfigSet) {
        fmt.Printf("\n🔄 [%s] 설정 업데이트 감지!\n", config.Environment)
        fmt.Printf("버전: %s\n", config.Version)
        fmt.Printf("로그 레벨: %v\n", config.Config["log_level"])
        fmt.Printf("API 제한: %v\n", config.Config["api_rate_limit"])

        // 여기서 애플리케이션 설정을 실제로 업데이트
        applyConfig(config)
    })

    if err != nil {
        panic(err)
    }

    // 5분 후 설정 업데이트 테스트
    go func() {
        time.Sleep(5 * time.Minute)

        updates := map[string]interface{}{
            "log_level":      "warn",
            "api_rate_limit": 500,
        }

        changes := []string{
            "로그 레벨을 warn으로 변경",
            "API 제한을 500으로 증가",
        }

        err := configManager.UpdateConfig("myapp", "development", "1.1.0", updates, changes)
        if err != nil {
            fmt.Printf("설정 업데이트 실패: %v\n", err)
        }
    }()

    fmt.Printf("\n=== 설정 관리 시스템 시작 ===\n")
    fmt.Printf("개발 환경 IPNS: /ipns/%s\n", configSet.IPNSName)
    fmt.Printf("프로덕션 환경 IPNS: /ipns/%s\n", prodConfigSet.IPNSName)
    fmt.Println("5분 후 자동으로 설정이 업데이트됩니다...")

    // 설정 목록 주기적 출력
    ticker := time.NewTicker(2 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            fmt.Println("\n=== 현재 설정 목록 ===")
            configs := configManager.ListConfigs()
            for _, config := range configs {
                fmt.Printf("- %s/%s v%s (업데이트: %s)\n",
                    config.Name,
                    config.Environment,
                    config.Version,
                    config.UpdatedAt.Format("15:04:05"),
                )
            }
        }
    }
}

func applyConfig(config *ConfigSet) {
    // 실제 애플리케이션에서는 여기서 설정을 적용
    fmt.Printf("✅ 설정 적용 완료: %s/%s v%s\n", config.Name, config.Environment, config.Version)
}
```

## 3. 📚 분산 위키 시스템

협업 편집이 가능한 분산 위키 시스템입니다.

```go
package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "net/http"
    "strings"
    "time"
    "context"
    "sync"

    ipns "github.com/sonheesung/boxo-starter-kit/07-ipns/pkg"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type DistributedWiki struct {
    ipnsManager   *ipns.IPNSManager
    unixfsWrapper *unixfs.UnixFsWrapper
    pages         map[string]*WikiPage
    wikiName      string
    mutex         sync.RWMutex
}

type WikiPage struct {
    Title       string    `json:"title"`
    Content     string    `json:"content"`
    Author      string    `json:"author"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Version     int       `json:"version"`
    Tags        []string  `json:"tags"`
    Links       []string  `json:"links"`
}

type WikiIndex struct {
    Name        string               `json:"name"`
    Description string               `json:"description"`
    Pages       map[string]*WikiPage `json:"pages"`
    CreatedAt   time.Time            `json:"created_at"`
    UpdatedAt   time.Time            `json:"updated_at"`
    Version     int                  `json:"version"`
}

func NewDistributedWiki(name, description string) (*DistributedWiki, error) {
    // IPNS 매니저 초기화
    ipnsManager, err := ipns.NewIPNSManager()
    if err != nil {
        return nil, err
    }

    // UnixFS 래퍼 초기화
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    wiki := &DistributedWiki{
        ipnsManager:   ipnsManager,
        unixfsWrapper: unixfsWrapper,
        pages:         make(map[string]*WikiPage),
    }

    // 위키 초기화
    err = wiki.initializeWiki(name, description)
    if err != nil {
        return nil, err
    }

    return wiki, nil
}

func (dw *DistributedWiki) initializeWiki(name, description string) error {
    // 홈페이지 생성
    homePage := &WikiPage{
        Title:     "Home",
        Content:   fmt.Sprintf("# %s\n\n%s\n\n분산 위키에 오신 것을 환영합니다!", name, description),
        Author:    "System",
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        Version:   1,
        Tags:      []string{"home", "welcome"},
        Links:     []string{},
    }

    dw.pages["home"] = homePage

    // 위키 인덱스 저장 및 IPNS 발행
    return dw.saveWikiIndex(name, description)
}

func (dw *DistributedWiki) saveWikiIndex(name, description string) error {
    index := WikiIndex{
        Name:        name,
        Description: description,
        Pages:       dw.pages,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
        Version:     len(dw.pages),
    }

    // JSON으로 직렬화
    indexData, err := json.MarshalIndent(index, "", "  ")
    if err != nil {
        return err
    }

    // IPFS에 추가
    ctx := context.Background()
    cid, err := dw.unixfsWrapper.AddFile(ctx, "wiki-index.json", indexData)
    if err != nil {
        return err
    }

    // IPNS 레코드 발행 또는 업데이트
    if dw.wikiName == "" {
        wikiName, err := dw.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*12,
        )
        if err != nil {
            return err
        }
        dw.wikiName = wikiName
        fmt.Printf("분산 위키 생성됨: /ipns/%s\n", wikiName)
    } else {
        _, err := dw.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*12,
        )
        if err != nil {
            return err
        }
        fmt.Printf("위키 인덱스 업데이트됨: %d 페이지\n", len(dw.pages))
    }

    return nil
}

func (dw *DistributedWiki) CreatePage(title, content, author string, tags []string) error {
    dw.mutex.Lock()
    defer dw.mutex.Unlock()

    pageKey := strings.ToLower(strings.ReplaceAll(title, " ", "-"))

    // 링크 추출
    links := dw.extractLinks(content)

    page := &WikiPage{
        Title:     title,
        Content:   content,
        Author:    author,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        Version:   1,
        Tags:      tags,
        Links:     links,
    }

    dw.pages[pageKey] = page
    return dw.saveWikiIndex("IPFS 분산 위키", "IPNS로 동기화되는 협업 위키")
}

func (dw *DistributedWiki) UpdatePage(title, content, author string, tags []string) error {
    dw.mutex.Lock()
    defer dw.mutex.Unlock()

    pageKey := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
    page, exists := dw.pages[pageKey]
    if !exists {
        return fmt.Errorf("페이지를 찾을 수 없습니다: %s", title)
    }

    // 링크 추출
    links := dw.extractLinks(content)

    page.Content = content
    page.Author = author
    page.UpdatedAt = time.Now()
    page.Version++
    page.Tags = tags
    page.Links = links

    return dw.saveWikiIndex("IPFS 분산 위키", "IPNS로 동기화되는 협업 위키")
}

func (dw *DistributedWiki) extractLinks(content string) []string {
    var links []string

    // [[페이지명]] 형식의 위키 링크 추출
    lines := strings.Split(content, "\n")
    for _, line := range lines {
        for strings.Contains(line, "[[") && strings.Contains(line, "]]") {
            start := strings.Index(line, "[[")
            end := strings.Index(line[start:], "]]")
            if end != -1 {
                link := line[start+2 : start+end]
                links = append(links, link)
                line = line[start+end+2:]
            } else {
                break
            }
        }
    }

    return links
}

func (dw *DistributedWiki) GetPage(title string) (*WikiPage, error) {
    dw.mutex.RLock()
    defer dw.mutex.RUnlock()

    pageKey := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
    page, exists := dw.pages[pageKey]
    if !exists {
        return nil, fmt.Errorf("페이지를 찾을 수 없습니다: %s", title)
    }

    return page, nil
}

func (dw *DistributedWiki) ListPages() []*WikiPage {
    dw.mutex.RLock()
    defer dw.mutex.RUnlock()

    pages := make([]*WikiPage, 0, len(dw.pages))
    for _, page := range dw.pages {
        pages = append(pages, page)
    }

    return pages
}

func (dw *DistributedWiki) GetWikiName() string {
    return dw.wikiName
}

// HTTP 서버
func (dw *DistributedWiki) StartHTTPServer(port string) {
    http.HandleFunc("/", dw.homeHandler)
    http.HandleFunc("/page/", dw.pageHandler)
    http.HandleFunc("/edit/", dw.editHandler)
    http.HandleFunc("/create", dw.createHandler)
    http.HandleFunc("/api/pages", dw.apiPagesHandler)

    fmt.Printf("위키 서버 시작됨: http://localhost%s\n", port)
    http.ListenAndServe(port, nil)
}

func (dw *DistributedWiki) homeHandler(w http.ResponseWriter, r *http.Request) {
    pages := dw.ListPages()

    tmpl := template.Must(template.New("home").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>분산 위키</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { border-bottom: 1px solid #ddd; margin-bottom: 20px; padding-bottom: 10px; }
        .page-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 15px; }
        .page-card { border: 1px solid #ddd; border-radius: 5px; padding: 15px; }
        .page-title { font-weight: bold; margin-bottom: 5px; }
        .page-meta { font-size: 0.9em; color: #666; }
        .create-btn { background: #007cba; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; }
        .ipns-info { background: #f8f9fa; padding: 10px; border-radius: 5px; margin-bottom: 20px; font-family: monospace; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🌐 분산 위키</h1>
        <div class="ipns-info">IPNS: /ipns/{{.WikiName}}</div>
        <a href="/create" class="create-btn">새 페이지 만들기</a>
    </div>

    <div class="page-list">
        {{range .Pages}}
        <div class="page-card">
            <div class="page-title">
                <a href="/page/{{.Title}}">{{.Title}}</a>
            </div>
            <div class="page-meta">
                작성자: {{.Author}}<br>
                버전: {{.Version}}<br>
                업데이트: {{.UpdatedAt.Format "2006-01-02 15:04"}}
            </div>
        </div>
        {{end}}
    </div>
</body>
</html>`))

    data := struct {
        WikiName string
        Pages    []*WikiPage
    }{
        WikiName: dw.wikiName,
        Pages:    pages,
    }

    tmpl.Execute(w, data)
}

func (dw *DistributedWiki) pageHandler(w http.ResponseWriter, r *http.Request) {
    title := strings.TrimPrefix(r.URL.Path, "/page/")
    page, err := dw.GetPage(title)
    if err != nil {
        http.NotFound(w, r)
        return
    }

    // 마크다운 스타일 렌더링 (간단한 변환)
    content := strings.ReplaceAll(page.Content, "\n", "<br>")
    content = strings.ReplaceAll(content, "[[", "<a href=\"/page/")
    content = strings.ReplaceAll(content, "]]", "\">")

    tmpl := template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}} - 분산 위키</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { border-bottom: 1px solid #ddd; margin-bottom: 20px; padding-bottom: 10px; }
        .meta { background: #f8f9fa; padding: 10px; border-radius: 5px; margin-bottom: 20px; }
        .content { line-height: 1.6; }
        .edit-btn { background: #28a745; color: white; padding: 8px 16px; text-decoration: none; border-radius: 3px; }
        .tags { margin-top: 20px; }
        .tag { background: #e9ecef; padding: 2px 8px; border-radius: 12px; font-size: 0.9em; margin-right: 5px; }
    </style>
</head>
<body>
    <div class="header">
        <a href="/">← 홈으로</a>
        <h1>{{.Title}}</h1>
        <a href="/edit/{{.Title}}" class="edit-btn">편집</a>
    </div>

    <div class="meta">
        작성자: {{.Author}} | 버전: {{.Version}} | 업데이트: {{.UpdatedAt.Format "2006-01-02 15:04:05"}}
    </div>

    <div class="content">{{.Content}}</div>

    {{if .Tags}}
    <div class="tags">
        <strong>태그:</strong>
        {{range .Tags}}<span class="tag">{{.}}</span>{{end}}
    </div>
    {{end}}
</body>
</html>`))

    data := struct {
        *WikiPage
        Content template.HTML
    }{
        WikiPage: page,
        Content:  template.HTML(content),
    }

    tmpl.Execute(w, data)
}

func (dw *DistributedWiki) editHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        title := r.FormValue("title")
        content := r.FormValue("content")
        author := r.FormValue("author")
        tags := strings.Split(r.FormValue("tags"), ",")

        // 태그 정리
        for i, tag := range tags {
            tags[i] = strings.TrimSpace(tag)
        }

        err := dw.UpdatePage(title, content, author, tags)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        http.Redirect(w, r, "/page/"+title, http.StatusSeeOther)
        return
    }

    // GET 요청 - 편집 폼 표시
    title := strings.TrimPrefix(r.URL.Path, "/edit/")
    page, err := dw.GetPage(title)
    if err != nil {
        http.NotFound(w, r)
        return
    }

    tmpl := template.Must(template.New("edit").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}} 편집 - 분산 위키</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 3px; }
        textarea { height: 300px; font-family: monospace; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; border-radius: 3px; cursor: pointer; }
        .btn:hover { background: #0056b3; }
    </style>
</head>
<body>
    <h1>{{.Title}} 편집</h1>

    <form method="post">
        <div class="form-group">
            <label>제목:</label>
            <input type="text" name="title" value="{{.Title}}" readonly>
        </div>

        <div class="form-group">
            <label>작성자:</label>
            <input type="text" name="author" value="{{.Author}}" required>
        </div>

        <div class="form-group">
            <label>내용:</label>
            <textarea name="content" required>{{.Content}}</textarea>
        </div>

        <div class="form-group">
            <label>태그 (쉼표로 구분):</label>
            <input type="text" name="tags" value="{{range $i, $tag := .Tags}}{{if $i}}, {{end}}{{$tag}}{{end}}">
        </div>

        <button type="submit" class="btn">저장</button>
        <a href="/page/{{.Title}}">취소</a>
    </form>
</body>
</html>`))

    tmpl.Execute(w, page)
}

func (dw *DistributedWiki) createHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        title := r.FormValue("title")
        content := r.FormValue("content")
        author := r.FormValue("author")
        tags := strings.Split(r.FormValue("tags"), ",")

        // 태그 정리
        for i, tag := range tags {
            tags[i] = strings.TrimSpace(tag)
        }

        err := dw.CreatePage(title, content, author, tags)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        http.Redirect(w, r, "/page/"+title, http.StatusSeeOther)
        return
    }

    // GET 요청 - 생성 폼 표시
    tmpl := template.Must(template.New("create").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>새 페이지 만들기 - 분산 위키</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input, textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 3px; }
        textarea { height: 300px; font-family: monospace; }
        .btn { background: #007cba; color: white; padding: 10px 20px; border: none; border-radius: 3px; cursor: pointer; }
        .btn:hover { background: #0056b3; }
        .help { font-size: 0.9em; color: #666; margin-top: 5px; }
    </style>
</head>
<body>
    <h1>새 페이지 만들기</h1>

    <form method="post">
        <div class="form-group">
            <label>제목:</label>
            <input type="text" name="title" required>
        </div>

        <div class="form-group">
            <label>작성자:</label>
            <input type="text" name="author" required>
        </div>

        <div class="form-group">
            <label>내용:</label>
            <textarea name="content" required placeholder="페이지 내용을 입력하세요..."></textarea>
            <div class="help">팁: [[페이지명]]으로 다른 페이지를 링크할 수 있습니다.</div>
        </div>

        <div class="form-group">
            <label>태그 (쉼표로 구분):</label>
            <input type="text" name="tags" placeholder="예: ipfs, wiki, tutorial">
        </div>

        <button type="submit" class="btn">페이지 생성</button>
        <a href="/">취소</a>
    </form>
</body>
</html>`))

    tmpl.Execute(w, nil)
}

func (dw *DistributedWiki) apiPagesHandler(w http.ResponseWriter, r *http.Request) {
    pages := dw.ListPages()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(pages)
}

func main() {
    // 분산 위키 생성
    wiki, err := NewDistributedWiki("IPFS 기술 위키", "IPFS와 분산 웹 기술에 대한 협업 위키")
    if err != nil {
        panic(err)
    }

    // 샘플 페이지 생성
    wiki.CreatePage("IPFS 소개", `# IPFS 소개

IPFS(InterPlanetary File System)는 분산 버전 관리 파일 시스템입니다.

## 주요 특징
- 콘텐츠 주소 지정
- 중복 제거
- P2P 네트워킹

관련 페이지: [[IPNS]], [[Bitswap]]`, "System", []string{"ipfs", "introduction"})

    wiki.CreatePage("IPNS", `# IPNS (InterPlanetary Name System)

IPNS는 IPFS의 변경 가능한 네이밍 시스템입니다.

## 사용 사례
- 동적 웹사이트
- 설정 배포
- [[위키]] 시스템

자세한 내용은 [[IPFS 소개]]를 참조하세요.`, "System", []string{"ipns", "naming"})

    fmt.Printf("분산 위키 IPNS: /ipns/%s\n", wiki.GetWikiName())

    // HTTP 서버 시작
    wiki.StartHTTPServer(":8080")
}
```

## 4. 🔄 버전 관리 시스템

Git과 유사한 분산 버전 관리 시스템입니다.

```go
package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "time"
    "context"
    "sync"

    ipns "github.com/sonheesung/boxo-starter-kit/07-ipns/pkg"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type VersionControl struct {
    ipnsManager   *ipns.IPNSManager
    unixfsWrapper *unixfs.UnixFsWrapper
    repository    *Repository
    mutex         sync.RWMutex
}

type Repository struct {
    Name        string             `json:"name"`
    Description string             `json:"description"`
    Branches    map[string]*Branch `json:"branches"`
    Commits     map[string]*Commit `json:"commits"`
    HEAD        string             `json:"head"`
    IPNSName    string             `json:"ipns_name"`
    CreatedAt   time.Time          `json:"created_at"`
    UpdatedAt   time.Time          `json:"updated_at"`
}

type Branch struct {
    Name      string `json:"name"`
    CommitID  string `json:"commit_id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Commit struct {
    ID        string                `json:"id"`
    Message   string                `json:"message"`
    Author    string                `json:"author"`
    Parent    string                `json:"parent"`
    Files     map[string]*FileEntry `json:"files"`
    Timestamp time.Time             `json:"timestamp"`
    TreeHash  string                `json:"tree_hash"`
}

type FileEntry struct {
    Path     string `json:"path"`
    Hash     string `json:"hash"`
    Size     int64  `json:"size"`
    Mode     string `json:"mode"`
}

func NewVersionControl(repoName, description string) (*VersionControl, error) {
    // IPNS 매니저 초기화
    ipnsManager, err := ipns.NewIPNSManager()
    if err != nil {
        return nil, err
    }

    // UnixFS 래퍼 초기화
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    vc := &VersionControl{
        ipnsManager:   ipnsManager,
        unixfsWrapper: unixfsWrapper,
    }

    // 저장소 초기화
    err = vc.initRepository(repoName, description)
    if err != nil {
        return nil, err
    }

    return vc, nil
}

func (vc *VersionControl) initRepository(name, description string) error {
    // 초기 커밋 생성
    initialCommit := &Commit{
        ID:        vc.generateCommitID("initial commit", "System", "", time.Now()),
        Message:   "Initial commit",
        Author:    "System",
        Parent:    "",
        Files:     make(map[string]*FileEntry),
        Timestamp: time.Now(),
        TreeHash:  "",
    }

    // main 브랜치 생성
    mainBranch := &Branch{
        Name:      "main",
        CommitID:  initialCommit.ID,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    vc.repository = &Repository{
        Name:        name,
        Description: description,
        Branches:    map[string]*Branch{"main": mainBranch},
        Commits:     map[string]*Commit{initialCommit.ID: initialCommit},
        HEAD:        "main",
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    return vc.saveRepository()
}

func (vc *VersionControl) saveRepository() error {
    // JSON으로 직렬화
    repoData, err := json.MarshalIndent(vc.repository, "", "  ")
    if err != nil {
        return err
    }

    // IPFS에 추가
    ctx := context.Background()
    cid, err := vc.unixfsWrapper.AddFile(ctx, "repository.json", repoData)
    if err != nil {
        return err
    }

    // IPNS 레코드 발행 또는 업데이트
    if vc.repository.IPNSName == "" {
        ipnsName, err := vc.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*24,
        )
        if err != nil {
            return err
        }
        vc.repository.IPNSName = ipnsName
        fmt.Printf("저장소 생성됨: /ipns/%s\n", ipnsName)
    } else {
        _, err := vc.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*24,
        )
        if err != nil {
            return err
        }
    }

    vc.repository.UpdatedAt = time.Now()
    return nil
}

func (vc *VersionControl) AddFile(path string, content []byte) error {
    vc.mutex.Lock()
    defer vc.mutex.Unlock()

    // 파일을 IPFS에 추가
    ctx := context.Background()
    cid, err := vc.unixfsWrapper.AddFile(ctx, path, content)
    if err != nil {
        return err
    }

    // 파일 해시 계산
    hash := vc.calculateFileHash(content)

    // 현재 브랜치의 최신 커밋 가져오기
    currentBranch := vc.repository.Branches[vc.repository.HEAD]
    currentCommit := vc.repository.Commits[currentBranch.CommitID]

    // 파일 엔트리 생성
    fileEntry := &FileEntry{
        Path: path,
        Hash: cid.String(),
        Size: int64(len(content)),
        Mode: "100644", // 일반 파일
    }

    // 기존 파일 목록 복사 후 새 파일 추가
    newFiles := make(map[string]*FileEntry)
    for k, v := range currentCommit.Files {
        newFiles[k] = v
    }
    newFiles[path] = fileEntry

    fmt.Printf("파일 추가됨: %s (해시: %s)\n", path, hash)
    return nil
}

func (vc *VersionControl) Commit(message, author string) (string, error) {
    vc.mutex.Lock()
    defer vc.mutex.Unlock()

    // 현재 브랜치 가져오기
    currentBranch := vc.repository.Branches[vc.repository.HEAD]
    parentCommitID := currentBranch.CommitID

    // 새 커밋 ID 생성
    commitID := vc.generateCommitID(message, author, parentCommitID, time.Now())

    // 트리 해시 계산
    treeHash := vc.calculateTreeHash(vc.repository.Commits[parentCommitID].Files)

    // 새 커밋 생성
    newCommit := &Commit{
        ID:        commitID,
        Message:   message,
        Author:    author,
        Parent:    parentCommitID,
        Files:     vc.repository.Commits[parentCommitID].Files,
        Timestamp: time.Now(),
        TreeHash:  treeHash,
    }

    // 커밋 저장
    vc.repository.Commits[commitID] = newCommit

    // 브랜치 업데이트
    currentBranch.CommitID = commitID
    currentBranch.UpdatedAt = time.Now()

    // 저장소 저장
    err := vc.saveRepository()
    if err != nil {
        return "", err
    }

    fmt.Printf("커밋 생성됨: %s (%s)\n", commitID[:8], message)
    return commitID, nil
}

func (vc *VersionControl) CreateBranch(branchName string) error {
    vc.mutex.Lock()
    defer vc.mutex.Unlock()

    if _, exists := vc.repository.Branches[branchName]; exists {
        return fmt.Errorf("브랜치가 이미 존재합니다: %s", branchName)
    }

    // 현재 브랜치의 최신 커밋에서 새 브랜치 생성
    currentBranch := vc.repository.Branches[vc.repository.HEAD]

    newBranch := &Branch{
        Name:      branchName,
        CommitID:  currentBranch.CommitID,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    vc.repository.Branches[branchName] = newBranch

    err := vc.saveRepository()
    if err != nil {
        return err
    }

    fmt.Printf("브랜치 생성됨: %s\n", branchName)
    return nil
}

func (vc *VersionControl) SwitchBranch(branchName string) error {
    vc.mutex.Lock()
    defer vc.mutex.Unlock()

    if _, exists := vc.repository.Branches[branchName]; !exists {
        return fmt.Errorf("브랜치를 찾을 수 없습니다: %s", branchName)
    }

    vc.repository.HEAD = branchName

    err := vc.saveRepository()
    if err != nil {
        return err
    }

    fmt.Printf("브랜치 전환됨: %s\n", branchName)
    return nil
}

func (vc *VersionControl) MergeBranch(sourceBranch, targetBranch string) error {
    vc.mutex.Lock()
    defer vc.mutex.Unlock()

    source, exists := vc.repository.Branches[sourceBranch]
    if !exists {
        return fmt.Errorf("소스 브랜치를 찾을 수 없습니다: %s", sourceBranch)
    }

    target, exists := vc.repository.Branches[targetBranch]
    if !exists {
        return fmt.Errorf("타겟 브랜치를 찾을 수 없습니다: %s", targetBranch)
    }

    // 간단한 머지 (실제로는 복잡한 3-way 머지 필요)
    sourceCommit := vc.repository.Commits[source.CommitID]

    // 머지 커밋 생성
    mergeCommitID := vc.generateCommitID(
        fmt.Sprintf("Merge branch '%s' into '%s'", sourceBranch, targetBranch),
        "System",
        target.CommitID,
        time.Now(),
    )

    mergeCommit := &Commit{
        ID:        mergeCommitID,
        Message:   fmt.Sprintf("Merge branch '%s' into '%s'", sourceBranch, targetBranch),
        Author:    "System",
        Parent:    target.CommitID,
        Files:     sourceCommit.Files, // 단순 머지
        Timestamp: time.Now(),
        TreeHash:  sourceCommit.TreeHash,
    }

    vc.repository.Commits[mergeCommitID] = mergeCommit

    // 타겟 브랜치 업데이트
    target.CommitID = mergeCommitID
    target.UpdatedAt = time.Now()

    err := vc.saveRepository()
    if err != nil {
        return err
    }

    fmt.Printf("브랜치 머지됨: %s -> %s\n", sourceBranch, targetBranch)
    return nil
}

func (vc *VersionControl) GetCommitHistory(limit int) []*Commit {
    vc.mutex.RLock()
    defer vc.mutex.RUnlock()

    var history []*Commit
    currentBranch := vc.repository.Branches[vc.repository.HEAD]
    commitID := currentBranch.CommitID

    for len(history) < limit && commitID != "" {
        commit := vc.repository.Commits[commitID]
        history = append(history, commit)
        commitID = commit.Parent
    }

    return history
}

func (vc *VersionControl) GetRepositoryInfo() *Repository {
    vc.mutex.RLock()
    defer vc.mutex.RUnlock()

    return vc.repository
}

func (vc *VersionControl) generateCommitID(message, author, parent string, timestamp time.Time) string {
    data := fmt.Sprintf("%s%s%s%d", message, author, parent, timestamp.Unix())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

func (vc *VersionControl) calculateFileHash(content []byte) string {
    hash := sha256.Sum256(content)
    return hex.EncodeToString(hash[:])
}

func (vc *VersionControl) calculateTreeHash(files map[string]*FileEntry) string {
    var hashData string
    for path, file := range files {
        hashData += fmt.Sprintf("%s:%s", path, file.Hash)
    }
    hash := sha256.Sum256([]byte(hashData))
    return hex.EncodeToString(hash[:])
}

func main() {
    // 버전 관리 시스템 생성
    vc, err := NewVersionControl("my-project", "IPFS 기반 분산 버전 관리 시스템")
    if err != nil {
        panic(err)
    }

    // 파일 추가 및 커밋
    vc.AddFile("README.md", []byte("# My Project\n\nIPFS 기반 프로젝트입니다."))
    vc.Commit("Add README.md", "Developer")

    vc.AddFile("main.go", []byte("package main\n\nfunc main() {\n    println(\"Hello, IPFS!\")\n}"))
    vc.Commit("Add main.go", "Developer")

    // 새 브랜치 생성 및 전환
    vc.CreateBranch("feature/new-feature")
    vc.SwitchBranch("feature/new-feature")

    vc.AddFile("feature.go", []byte("package main\n\nfunc NewFeature() {\n    println(\"New feature!\")\n}"))
    vc.Commit("Add new feature", "Developer")

    // main 브랜치로 돌아가서 머지
    vc.SwitchBranch("main")
    vc.MergeBranch("feature/new-feature", "main")

    // 커밋 히스토리 출력
    fmt.Printf("\n=== 저장소 정보 ===\n")
    repo := vc.GetRepositoryInfo()
    fmt.Printf("저장소: %s\n", repo.Name)
    fmt.Printf("IPNS: /ipns/%s\n", repo.IPNSName)
    fmt.Printf("현재 브랜치: %s\n", repo.HEAD)

    fmt.Printf("\n=== 브랜치 목록 ===\n")
    for name, branch := range repo.Branches {
        fmt.Printf("- %s (커밋: %s)\n", name, branch.CommitID[:8])
    }

    fmt.Printf("\n=== 커밋 히스토리 ===\n")
    history := vc.GetCommitHistory(10)
    for _, commit := range history {
        fmt.Printf("%s - %s (%s)\n",
            commit.ID[:8],
            commit.Message,
            commit.Timestamp.Format("2006-01-02 15:04:05"))
    }

    fmt.Printf("\n버전 관리 시스템이 IPFS/IPNS에 저장되었습니다!\n")
    fmt.Printf("다른 노드에서 /ipns/%s로 접근 가능합니다.\n", repo.IPNSName)
}
```

---

이 쿡북의 예제들을 통해 IPNS의 실용적인 활용 방법을 학습할 수 있습니다:

1. **뉴스 피드**: 실시간 콘텐츠 배포 시스템
2. **설정 관리**: 중앙 집중식 설정 배포 및 동기화
3. **분산 위키**: 협업 기반 지식 관리 시스템
4. **버전 관리**: Git과 유사한 분산 버전 컨트롤

각 예제는 IPNS의 핵심 기능을 활용하여 실제 비즈니스 요구사항을 해결하는 완전한 솔루션입니다.