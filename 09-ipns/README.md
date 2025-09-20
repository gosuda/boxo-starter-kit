# 09-ipns: InterPlanetary Name System Implementation

## 🎯 Learning Objectives

Through this module, you will learn:
- The operating principles and purpose of **IPNS (InterPlanetary Name System)**
- Methods for managing **mutable content** in IPFS
- Implementation of **content authentication** through digital signatures
- Building **naming systems** similar to DNS
- Developing **IPFS applications** that require real-time updates

## 📋 Prerequisites

- **Previous Chapters**: 00-block-cid, 01-persistent, 02-dag-ipld, 03-unixfs completed
- **Technical Knowledge**: Cryptography basics (public/private keys), DNS concepts, digital signatures
- **Go Knowledge**: Cryptographic packages, time handling, struct serialization

## 🔑 Core Concepts

### What is IPNS?

IPNS (InterPlanetary Name System) is a **mutable naming system** designed to solve IPFS's immutability problem.

#### IPFS vs IPNS Comparison
```
IPFS (Immutable):
/ipfs/QmHash123... → Always the same content

IPNS (Mutable):
/ipns/Qm12D3Ko... → Points to different IPFS hashes over time
```

#### Core Components of IPNS
- **IPNS Name**: Unique identifier derived from public key
- **IPNS Record**: Contains the IPFS path being pointed to and metadata
- **Digital Signature**: Record authentication signed with private key
- **TTL (Time To Live)**: Record validity period

### IPNS Operation Process
1. **Key Pair Generation**: Generate private/public key pair
2. **Record Creation**: Create IPNS record pointing to IPFS path
3. **Signing**: Digitally sign record with private key
4. **Publishing**: Store IPNS record in DHT
5. **Resolution**: Query latest IPFS path through IPNS name

## 💻 Code Analysis

### 1. IPNS Manager Structure

```go
type IPNSManager struct {
    privateKey   crypto.PrivKey
    publicKey    crypto.PubKey
    dagWrapper   *dag.DagWrapper
    recordStore  map[string]*ipns.Record
    recordMutex  sync.RWMutex
}
```

**Design Decisions**:
- `privateKey/publicKey`: Cryptographic key pair for record signing/verification
- `dagWrapper`: Integration with IPFS content
- `recordStore`: Memory-based record storage (DHT used in practice)

### 2. Key Pair Generation

```go
func generateKeyPair() (crypto.PrivKey, crypto.PubKey, error) {
    privateKey, publicKey, err := crypto.GenerateKeyPair(crypto.Ed25519, 2048)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
    }
    return privateKey, publicKey, nil
}
```

**Why Ed25519**:
- Fast signing/verification speed
- Small key size (32 bytes)
- High security

### 3. IPNS Record Creation and Signing

```go
func (im *IPNSManager) PublishRecord(ipfsPath string, ttl time.Duration) (string, error) {
    // Create Path object
    p, err := path.FromString(ipfsPath)
    if err != nil {
        return "", fmt.Errorf("failed to parse path: %w", err)
    }

    // Create IPNS record
    record, err := ipns.NewRecord(im.privateKey, p, 1, time.Now().Add(ttl), ttl)
    if err != nil {
        return "", fmt.Errorf("failed to create record: %w", err)
    }

    // Store record
    peerID, err := peer.IDFromPrivateKey(im.privateKey)
    if err != nil {
        return "", fmt.Errorf("failed to create peer ID: %w", err)
    }

    ipnsName := ipns.NameFromPeer(peerID)
    im.recordStore[ipnsName.String()] = record

    return ipnsName.String(), nil
}
```

## 🏃‍♂️ Hands-on Guide

### Step 1: Create IPNS Manager
```bash
cd 09-ipns
go run main.go
```

### Step 2: Test Content Publishing
```go
// 1. Add file to IPFS
content := "Hello, IPNS World!"
ipfsHash := addToIPFS(content)

// 2. Publish IPNS record
ipnsName := publishIPNS(ipfsHash)

// 3. Resolve IPNS
resolvedPath := resolveIPNS(ipnsName)
```

### Step 3: Update Content
```go
// Update with new content
newContent := "Updated content via IPNS!"
newHash := addToIPFS(newContent)

// Publish new record with same IPNS name
updateIPNS(ipnsName, newHash)
```

### Expected Results
- **Initial Publishing**: IPNS name points to first content
- **Update**: Same IPNS name points to new content
- **Consistent Access**: External access uses unchanging IPNS name

## 🚀 Advanced Use Cases

### 1. Dynamic Website Updates

```go
type Website struct {
    ipnsManager *IPNSManager
    ipnsName    string
}

func (w *Website) UpdateContent(htmlContent string) error {
    // Add HTML to IPFS
    ipfsHash, err := w.addHTML(htmlContent)
    if err != nil {
        return err
    }

    // Update IPNS record
    _, err = w.ipnsManager.PublishRecord(
        fmt.Sprintf("/ipfs/%s", ipfsHash),
        24*time.Hour,
    )
    return err
}
```

### 2. Configuration Distribution

```go
type ConfigDistributor struct {
    ipnsManager *IPNSManager
    configName  string
}

func (cd *ConfigDistributor) UpdateConfig(config map[string]interface{}) error {
    // JSON serialization
    jsonData, _ := json.Marshal(config)

    // Add to IPFS and publish IPNS
    ipfsHash := cd.addToIPFS(jsonData)
    return cd.updateIPNS(ipfsHash)
}
```

### 3. Version Control System

```go
type VersionedContent struct {
    ipnsManager *IPNSManager
    versions    []string
    current     int
}

func (vc *VersionedContent) Rollback(steps int) error {
    targetVersion := vc.current - steps
    if targetVersion < 0 {
        return errors.New("no such version")
    }

    return vc.ipnsManager.PublishRecord(
        vc.versions[targetVersion],
        time.Hour*24,
    )
}
```

## 🔧 Performance Optimization

### TTL Strategy
```go
const (
    ShortTTL  = 5 * time.Minute   // Frequently changing content
    MediumTTL = 1 * time.Hour     // General usage
    LongTTL   = 24 * time.Hour    // Stable content
)
```

### Caching Strategy
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

### Batch Updates
```go
func (im *IPNSManager) BatchUpdate(updates map[string]string) error {
    // Update multiple IPNS records at once
    for ipnsName, ipfsPath := range updates {
        go im.PublishRecord(ipfsPath, time.Hour)
    }
    return nil
}
```

## 🔒 Security Considerations

### Key Management
```go
func (im *IPNSManager) ExportKey() ([]byte, error) {
    // Safely export private key
    return crypto.MarshalPrivateKey(im.privateKey)
}

func LoadKeyFromFile(filename string) (crypto.PrivKey, error) {
    // Load private key from file
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    return crypto.UnmarshalPrivateKey(data)
}
```

### Record Validation
```go
func (im *IPNSManager) ValidateRecord(name string, record *ipns.Record) error {
    // Verify record signature
    pubKey, err := im.getPublicKey(name)
    if err != nil {
        return err
    }

    return ipns.Validate(pubKey, record)
}
```

## 🐛 Troubleshooting

### Issue 1: Record Publishing Failure
**Symptom**: Error when calling `PublishRecord`
**Cause**: Invalid IPFS path or key issues
**Solution**:
```go
// IPFS path validity check
if !strings.HasPrefix(ipfsPath, "/ipfs/") {
    return "", errors.New("not a valid IPFS path")
}
```

### Issue 2: Resolution Timeout
**Symptom**: `ResolveRecord` call takes too long
**Cause**: DHT network connection issues
**Solution**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

### Issue 3: Cache Invalidation
**Symptom**: Old content continues to show
**Cause**: TTL setting issues
**Solution**:
```go
// Force refresh
record.TTL = 0
im.PublishRecord(newPath, time.Minute)
```

## 🔗 Related Learning
- **Next Steps**: 99-kubo-api-demo (actual network integration)
- **Advanced Topics**:
  - DNS-Link integration
  - IPNS over PubSub
  - Distributed website deployment

## 📚 Reference Materials
- [IPNS Specification](https://docs.ipfs.tech/concepts/ipns/)
- [go-ipns Documentation](https://pkg.go.dev/github.com/ipfs/boxo/ipns)
- [Cryptographic Keys in IPFS](https://docs.ipfs.tech/concepts/cryptographic-keys/)

---

# 🍳 Practical Cookbook: Ready-to-use Code

## 1. 📰 News Feed System

A system for managing real-time news feeds with IPNS.

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
    // Initialize IPNS manager
    ipnsManager, err := NewIPNSManager()
    if err != nil {
        return nil, err
    }

    // Initialize UnixFS wrapper
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

    // Initialize feed
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

    // Serialize to JSON
    feedData, err := json.MarshalIndent(feed, "", "  ")
    if err != nil {
        return err
    }

    // Add to IPFS
    ctx := context.Background()
    cid, err := nf.unixfsWrapper.AddFile(ctx, "feed.json", feedData)
    if err != nil {
        return err
    }

    // Publish IPNS record
    feedName, err := nf.ipnsManager.PublishRecord(
        fmt.Sprintf("/ipfs/%s", cid.String()),
        time.Hour*24,
    )
    if err != nil {
        return err
    }

    nf.feedName = feedName
    fmt.Printf("News feed created: /ipns/%s\n", feedName)
    return nil
}

func (nf *NewsFeed) AddArticle(title, content, author string, tags []string) error {
    nf.mutex.Lock()
    defer nf.mutex.Unlock()

    // Create new article
    article := Article{
        ID:          fmt.Sprintf("article_%d", time.Now().Unix()),
        Title:       title,
        Content:     content,
        Author:      author,
        PublishedAt: time.Now(),
        Tags:        tags,
    }

    // Add to article list (sorted by latest first)
    nf.articles = append([]Article{article}, nf.articles...)

    // Keep only 50 articles maximum
    if len(nf.articles) > 50 {
        nf.articles = nf.articles[:50]
    }

    return nf.updateFeed()
}

func (nf *NewsFeed) updateFeed() error {
    feed := Feed{
        Title:       "IPFS News Feed",
        Description: "Real-time news feed managed with IPNS",
        Articles:    nf.articles,
        LastUpdate:  time.Now(),
        Version:     len(nf.articles),
    }

    // Serialize to JSON
    feedData, err := json.MarshalIndent(feed, "", "  ")
    if err != nil {
        return err
    }

    // Add to IPFS
    ctx := context.Background()
    cid, err := nf.unixfsWrapper.AddFile(ctx, "feed.json", feedData)
    if err != nil {
        return err
    }

    // Update IPNS record
    _, err = nf.ipnsManager.PublishRecord(
        fmt.Sprintf("/ipfs/%s", cid.String()),
        time.Hour*6, // 6-hour TTL
    )

    if err == nil {
        fmt.Printf("Feed updated: %d articles\n", len(nf.articles))
    }

    return err
}

func (nf *NewsFeed) GetFeedName() string {
    return nf.feedName
}

func (nf *NewsFeed) GetLatestFeed() (*Feed, error) {
    nf.mutex.RLock()
    defer nf.mutex.RUnlock()

    // Resolve current IPNS record
    ipfsPath, err := nf.ipnsManager.ResolveRecord(nf.feedName)
    if err != nil {
        return nil, err
    }

    // Get feed data from IPFS
    ctx := context.Background()
    data, err := nf.unixfsWrapper.GetFile(ctx, ipfsPath)
    if err != nil {
        return nil, err
    }

    // Parse JSON
    var feed Feed
    err = json.Unmarshal(data, &feed)
    if err != nil {
        return nil, err
    }

    return &feed, nil
}

// Auto feed generator
func (nf *NewsFeed) StartAutoGenerator() {
    go func() {
        sampleArticles := []struct {
            title   string
            content string
            author  string
            tags    []string
        }{
            {"IPFS 2.0 Released", "New features have been added to the distributed file system.", "IPFS Team", []string{"ipfs", "release"}},
            {"Web3 Technology Trends", "Let's explore the latest trends in blockchain and distributed systems.", "Tech Reporter", []string{"web3", "blockchain"}},
            {"IPNS Performance Improvements", "The name system resolution speed has been significantly improved.", "Dev Team", []string{"ipns", "performance"}},
            {"Distributed Web Security", "Security considerations for decentralized web applications", "Security Expert", []string{"security", "dweb"}},
        }

        for i := 0; i < len(sampleArticles); i++ {
            time.Sleep(30 * time.Second) // New article every 30 seconds

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
    // Create news feed
    feed, err := NewNewsFeed("IPFS Tech News", "Latest news about IPFS and distributed web technologies")
    if err != nil {
        panic(err)
    }

    // Add initial article
    feed.AddArticle(
        "IPFS News Feed Launch",
        "A real-time news feed using IPNS has been launched. This feed will update automatically.",
        "System",
        []string{"announcement", "ipfs", "ipns"},
    )

    // Start auto feed generation
    feed.StartAutoGenerator()

    fmt.Printf("News feed IPNS: /ipns/%s\n", feed.GetFeedName())
    fmt.Println("New articles will be added every 30 seconds...")

    // Output current feed status every minute
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            currentFeed, err := feed.GetLatestFeed()
            if err != nil {
                fmt.Printf("Feed query error: %v\n", err)
                continue
            }

            fmt.Printf("\n=== Current Feed Status ===\n")
            fmt.Printf("Title: %s\n", currentFeed.Title)
            fmt.Printf("Article count: %d\n", len(currentFeed.Articles))
            fmt.Printf("Last update: %s\n", currentFeed.LastUpdate.Format("2006-01-02 15:04:05"))

            if len(currentFeed.Articles) > 0 {
                latest := currentFeed.Articles[0]
                fmt.Printf("Latest article: %s (by %s)\n", latest.Title, latest.Author)
            }
        }
    }
}
```

## 2. 🏢 Enterprise Configuration Management System

A centralized configuration distribution and version management system.

```go
package main

import (
    "encoding/json"
    "fmt"
    "time"
    "context"
    "sync"

    "github.com/libp2p/go-libp2p/core/crypto"
    ipns "github.com/sonheesung/boxo-starter-kit/09-ipns/pkg"
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
    // Initialize IPNS manager
    ipnsManager, err := ipns.NewIPNSManager()
    if err != nil {
        return nil, err
    }

    // Initialize UnixFS wrapper
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

    // Save config to IPFS
    err := cm.saveConfigToIPFS(configSet)
    if err != nil {
        return nil, err
    }

    cm.configs[cm.getConfigKey(name, environment)] = configSet
    fmt.Printf("Config set created: %s/%s (IPNS: %s)\n", name, environment, configSet.IPNSName)

    return configSet, nil
}

func (cm *ConfigManager) UpdateConfig(name, environment, newVersion string, updates map[string]interface{}, changes []string) error {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()

    key := cm.getConfigKey(name, environment)
    configSet, exists := cm.configs[key]
    if !exists {
        return fmt.Errorf("config not found: %s/%s", name, environment)
    }

    // Apply updates to existing config
    for k, v := range updates {
        configSet.Config[k] = v
    }

    configSet.Version = newVersion
    configSet.UpdatedAt = time.Now()

    // Save updated config to IPFS
    err := cm.saveConfigToIPFS(configSet)
    if err != nil {
        return err
    }

    fmt.Printf("Config updated: %s/%s v%s\n", name, environment, newVersion)
    return nil
}

func (cm *ConfigManager) saveConfigToIPFS(configSet *ConfigSet) error {
    // Serialize to JSON
    configData, err := json.MarshalIndent(configSet, "", "  ")
    if err != nil {
        return err
    }

    // Add to IPFS
    ctx := context.Background()
    cid, err := cm.unixfsWrapper.AddFile(ctx, "config.json", configData)
    if err != nil {
        return err
    }

    // Publish or update IPNS record
    if configSet.IPNSName == "" {
        // Create new IPNS name
        ipnsName, err := cm.ipnsManager.PublishRecord(
            fmt.Sprintf("/ipfs/%s", cid.String()),
            time.Hour*24,
        )
        if err != nil {
            return err
        }
        configSet.IPNSName = ipnsName
    } else {
        // Update existing IPNS record
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
        return nil, fmt.Errorf("config not found: %s/%s", name, environment)
    }

    return configSet, nil
}

func (cm *ConfigManager) GetConfigFromIPNS(ipnsName string) (*ConfigSet, error) {
    // Resolve IPNS
    ipfsPath, err := cm.ipnsManager.ResolveRecord(ipnsName)
    if err != nil {
        return nil, err
    }

    // Get config data from IPFS
    ctx := context.Background()
    data, err := cm.unixfsWrapper.GetFile(ctx, ipfsPath)
    if err != nil {
        return nil, err
    }

    // Parse JSON
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

// Configuration synchronization client
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
        return fmt.Errorf("already subscribed: %s", ipnsName)
    }

    subscription := &ConfigSubscription{
        IPNSName:     ipnsName,
        PollInterval: pollInterval,
        OnUpdate:     onUpdate,
        stopChan:     make(chan bool),
    }

    cc.subscriptions[ipnsName] = subscription
    go cc.pollConfig(subscription)

    fmt.Printf("Config subscription started: %s (poll interval: %v)\n", ipnsName, pollInterval)
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
                fmt.Printf("Config query failed: %v\n", err)
                continue
            }

            if config.Version != lastVersion {
                lastVersion = config.Version
                fmt.Printf("Config change detected: %s v%s\n", config.Name, config.Version)
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
        fmt.Printf("Config subscription stopped: %s\n", ipnsName)
    }
}

func main() {
    // Create config manager
    configManager, err := NewConfigManager()
    if err != nil {
        panic(err)
    }

    // Create development environment config
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

    // Create production environment config
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

    // Create config client and subscribe
    client, err := NewConfigClient()
    if err != nil {
        panic(err)
    }

    // Subscribe to development environment config
    err = client.Subscribe(configSet.IPNSName, 30*time.Second, func(config *ConfigSet) {
        fmt.Printf("\n🔄 [%s] Config update detected!\n", config.Environment)
        fmt.Printf("Version: %s\n", config.Version)
        fmt.Printf("Log level: %v\n", config.Config["log_level"])
        fmt.Printf("API limit: %v\n", config.Config["api_rate_limit"])

        // Apply configuration to actual application here
        applyConfig(config)
    })

    if err != nil {
        panic(err)
    }

    // Test config update after 5 minutes
    go func() {
        time.Sleep(5 * time.Minute)

        updates := map[string]interface{}{
            "log_level":      "warn",
            "api_rate_limit": 500,
        }

        changes := []string{
            "Changed log level to warn",
            "Increased API limit to 500",
        }

        err := configManager.UpdateConfig("myapp", "development", "1.1.0", updates, changes)
        if err != nil {
            fmt.Printf("Config update failed: %v\n", err)
        }
    }()

    fmt.Printf("\n=== Configuration Management System Started ===\n")
    fmt.Printf("Development IPNS: /ipns/%s\n", configSet.IPNSName)
    fmt.Printf("Production IPNS: /ipns/%s\n", prodConfigSet.IPNSName)
    fmt.Println("Configuration will update automatically after 5 minutes...")

    // Periodically output config list
    ticker := time.NewTicker(2 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            fmt.Println("\n=== Current Config List ===")
            configs := configManager.ListConfigs()
            for _, config := range configs {
                fmt.Printf("- %s/%s v%s (updated: %s)\n",
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
    // Apply configuration in actual application here
    fmt.Printf("✅ Configuration applied: %s/%s v%s\n", config.Name, config.Environment, config.Version)
}
```

---

Through these examples, you can learn practical ways to use IPNS:

1. **News Feed**: Real-time content distribution system
2. **Configuration Management**: Centralized configuration distribution and synchronization

Each example is a complete solution that uses IPNS's core functionality to solve real business requirements.