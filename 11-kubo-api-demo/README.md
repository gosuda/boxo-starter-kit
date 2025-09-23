# 11-kubo-api-demo: Real IPFS Network Integration

## 🎯 Learning Objectives

Through this final module, you will learn:
- How to connect to **real IPFS networks** using Kubo API
- **HTTP API client** implementation and usage methods
- Integration between **local development** and **production networks**
- **Cross-network data sharing** and replication strategies
- **Performance monitoring** and troubleshooting in real network environments
- Practical approaches for **deploying IPFS applications to production**

## 📋 Prerequisites

- **All Previous Modules**: 00-block-cid through 10-gateway completed
- **Technical Knowledge**: HTTP client programming, API integration, network debugging
- **Go Knowledge**: HTTP clients, JSON handling, concurrent programming
- **Infrastructure**: Running Kubo (go-ipfs) node or access to public IPFS gateway

## 🔑 Core Concepts

### What is Kubo?

**Kubo** (formerly go-ipfs) is the reference implementation of IPFS, providing:

```
Local Development Environment:
├─ boxo library (embedded IPFS functionality)
└─ Direct access to data structures and algorithms

Production Environment:
├─ Kubo daemon (full IPFS node)
├─ HTTP API (standard interface)
└─ Network connectivity (DHT, bitswap, etc.)
```

### Development vs Production Architecture

| Aspect | Development (boxo) | Production (Kubo API) |
|--------|-------------------|----------------------|
| **Data Storage** | Memory/local files | Persistent blockstore |
| **Network** | Isolated | Global IPFS network |
| **Performance** | Fast (direct access) | Network-dependent |
| **Scalability** | Limited | Distributed |
| **Deployment** | Embedded | Separate service |

### HTTP API Endpoints

```
Core Operations:
POST /api/v0/add          - Add files
POST /api/v0/cat          - Retrieve content
POST /api/v0/get          - Download files
POST /api/v0/ls           - List directory

Network Operations:
POST /api/v0/pin/add      - Pin content
POST /api/v0/pin/rm       - Unpin content
POST /api/v0/pin/ls       - List pins
POST /api/v0/bitswap/wantlist - Check wanted blocks

Node Management:
POST /api/v0/id           - Node information
POST /api/v0/version      - Version info
POST /api/v0/swarm/peers  - Connected peers
```

## 💻 Code Analysis

### 1. Kubo HTTP Client Design

```go
// pkg/kubo_client.go:25-50
type KuboClient struct {
    httpClient *http.Client
    apiURL     string
    timeout    time.Duration
}

type APIResponse struct {
    Hash  string `json:"Hash"`
    Name  string `json:"Name"`
    Size  string `json:"Size"`
}

type NodeInfo struct {
    ID              string   `json:"ID"`
    PublicKey       string   `json:"PublicKey"`
    Addresses       []string `json:"Addresses"`
    AgentVersion    string   `json:"AgentVersion"`
    ProtocolVersion string   `json:"ProtocolVersion"`
}

func NewKuboClient(apiURL string, timeout time.Duration) *KuboClient {
    return &KuboClient{
        httpClient: &http.Client{
            Timeout: timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
        apiURL:  strings.TrimSuffix(apiURL, "/"),
        timeout: timeout,
    }
}
```

**Design Features**:
- **HTTP connection pooling** for performance optimization
- **Configurable timeouts** to handle network latency
- **Structured response parsing** for type safety
- **Error handling** for network failures

### 2. File Upload Implementation

```go
// pkg/kubo_client.go:85-130
func (kc *KuboClient) AddFile(filename string, content []byte) (*APIResponse, error) {
    // 1. Create multipart form
    var buffer bytes.Buffer
    writer := multipart.NewWriter(&buffer)

    // Add file field
    fileWriter, err := writer.CreateFormFile("file", filename)
    if err != nil {
        return nil, fmt.Errorf("failed to create form file: %w", err)
    }

    _, err = fileWriter.Write(content)
    if err != nil {
        return nil, fmt.Errorf("failed to write file content: %w", err)
    }

    // Add optional parameters
    writer.WriteField("pin", "true")
    writer.WriteField("quieter", "true")
    writer.Close()

    // 2. Create HTTP request
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/add", &buffer)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", writer.FormDataContentType())

    // 3. Execute request
    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to execute request: %w", err)
    }
    defer resp.Body.Close()

    // 4. Handle response
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, body)
    }

    // 5. Parse JSON response
    var apiResp APIResponse
    decoder := json.NewDecoder(resp.Body)
    if err := decoder.Decode(&apiResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &apiResp, nil
}
```

### 3. Content Retrieval with Streaming

```go
// pkg/kubo_client.go:160-200
func (kc *KuboClient) GetFile(hash string) ([]byte, error) {
    // 1. Create request with hash parameter
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/cat", nil)
    if err != nil {
        return nil, err
    }

    // Add hash as query parameter
    q := req.URL.Query()
    q.Add("arg", hash)
    req.URL.RawQuery = q.Encode()

    // 2. Execute request
    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to get file: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, body)
    }

    // 3. Read response body (file content)
    content, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    return content, nil
}

// Streaming version for large files
func (kc *KuboClient) GetFileStream(hash string) (io.ReadCloser, error) {
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/cat", nil)
    if err != nil {
        return nil, err
    }

    q := req.URL.Query()
    q.Add("arg", hash)
    req.URL.RawQuery = q.Encode()

    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode != http.StatusOK {
        resp.Body.Close()
        return nil, fmt.Errorf("API error (status %d)", resp.StatusCode)
    }

    return resp.Body, nil // Caller must close
}
```

### 4. Pin Management Integration

```go
// pkg/kubo_client.go:220-280
func (kc *KuboClient) PinAdd(hash string, recursive bool) error {
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/pin/add", nil)
    if err != nil {
        return err
    }

    q := req.URL.Query()
    q.Add("arg", hash)
    if recursive {
        q.Add("recursive", "true")
    }
    req.URL.RawQuery = q.Encode()

    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to pin: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("pin failed (status %d): %s", resp.StatusCode, body)
    }

    return nil
}

func (kc *KuboClient) PinList() ([]PinInfo, error) {
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/pin/ls", nil)
    if err != nil {
        return nil, err
    }

    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to list pins (status %d)", resp.StatusCode)
    }

    var pinResponse struct {
        Keys map[string]struct {
            Type string `json:"Type"`
        } `json:"Keys"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&pinResponse); err != nil {
        return nil, err
    }

    pins := make([]PinInfo, 0, len(pinResponse.Keys))
    for hash, info := range pinResponse.Keys {
        pins = append(pins, PinInfo{
            Hash: hash,
            Type: info.Type,
        })
    }

    return pins, nil
}

type PinInfo struct {
    Hash string `json:"hash"`
    Type string `json:"type"`
}
```

### 5. Network Status Monitoring

```go
// pkg/kubo_client.go:300-360
func (kc *KuboClient) GetNodeInfo() (*NodeInfo, error) {
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/id", nil)
    if err != nil {
        return nil, err
    }

    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to get node info (status %d)", resp.StatusCode)
    }

    var nodeInfo NodeInfo
    if err := json.NewDecoder(resp.Body).Decode(&nodeInfo); err != nil {
        return nil, err
    }

    return &nodeInfo, nil
}

func (kc *KuboClient) GetConnectedPeers() ([]PeerInfo, error) {
    req, err := http.NewRequest("POST", kc.apiURL+"/api/v0/swarm/peers", nil)
    if err != nil {
        return nil, err
    }

    resp, err := kc.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var peersResponse struct {
        Peers []PeerInfo `json:"Peers"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&peersResponse); err != nil {
        return nil, err
    }

    return peersResponse.Peers, nil
}

type PeerInfo struct {
    Peer    string `json:"Peer"`
    Address string `json:"Addr"`
    Latency string `json:"Latency,omitempty"`
}
```

## 🏃‍♂️ Hands-on Guide

### 1. Kubo Node Setup

```bash
# Install Kubo
curl -sSL https://dist.ipfs.io/go-ipfs/v0.24.0/go-ipfs_v0.24.0_linux-amd64.tar.gz | tar -xzv
cd go-ipfs && sudo bash install.sh

# Initialize IPFS node
ipfs init

# Start daemon (in separate terminal)
ipfs daemon
```

### 2. Demo Application Execution

```bash
cd 11-kubo-api-demo
go run main.go
```

**Expected Output**:
```
=== Kubo API Integration Demo ===

1. Connecting to Kubo node:
   API URL: http://localhost:5001
   ✅ Connection successful

2. Node information:
   ID: QmYourNodeID123...
   Agent: go-ipfs/0.24.0/
   Peers connected: 15

3. File operations:
   📄 Uploading sample file...
   ✅ File uploaded: QmSampleHash123...
   📄 Downloading file...
   ✅ Content verified: matches original

4. Pin management:
   📌 Pinning uploaded content...
   ✅ Pin added successfully
   📋 Current pins: 1 items

5. Network integration:
   🌐 Content available on network
   🔍 Retrieving from different gateway...
   ✅ Network retrieval successful

6. Performance metrics:
   Upload speed: 1.2 MB/s
   Download speed: 2.1 MB/s
   Network latency: 45ms
```

### 3. Cross-Network Testing

Test content sharing across different networks:

```bash
# Upload to local node
curl -X POST -F file=@test.txt http://localhost:5001/api/v0/add

# Retrieve from public gateway
curl https://ipfs.io/ipfs/[YOUR_HASH]

# Check pin status
curl -X POST http://localhost:5001/api/v0/pin/ls
```

### 4. Running Integration Tests

```bash
go test -v ./...
```

**Key Test Cases**:
- ✅ Kubo API connectivity
- ✅ File upload/download cycle
- ✅ Pin operations
- ✅ Network propagation
- ✅ Error handling

## 🔍 Advanced Use Cases

### 1. Distributed Application Backend

```go
type DistributedApp struct {
    kuboClient   *KuboClient
    localStore   *boxo.Store        // Local development cache
    networkMode  bool               // Development vs Production
}

func (da *DistributedApp) StoreData(key string, data []byte) (string, error) {
    if da.networkMode {
        // Production: Use Kubo API
        resp, err := da.kuboClient.AddFile(key, data)
        if err != nil {
            return "", err
        }

        // Ensure persistence with pinning
        err = da.kuboClient.PinAdd(resp.Hash, false)
        if err != nil {
            log.Printf("Warning: Pin failed for %s: %v", resp.Hash, err)
        }

        return resp.Hash, nil
    } else {
        // Development: Use local boxo store
        cid, err := da.localStore.Put(context.Background(), data)
        return cid.String(), err
    }
}

func (da *DistributedApp) RetrieveData(hash string) ([]byte, error) {
    if da.networkMode {
        // Production: Network retrieval with fallbacks
        data, err := da.kuboClient.GetFile(hash)
        if err != nil {
            // Try alternative gateways
            for _, gateway := range da.getBackupGateways() {
                if data, err = da.tryGateway(gateway, hash); err == nil {
                    break
                }
            }
        }
        return data, err
    } else {
        // Development: Local access
        cid, _ := cid.Decode(hash)
        return da.localStore.Get(context.Background(), cid)
    }
}
```

### 2. Content Synchronization System

```go
type ContentSyncer struct {
    sourceClient *KuboClient      // Source network node
    targetClient *KuboClient      // Target network node
    syncQueue    chan SyncJob
    workers      int
}

type SyncJob struct {
    Hash     string
    Priority int
    Retries  int
}

func (cs *ContentSyncer) StartSync() {
    // Start worker pool
    for i := 0; i < cs.workers; i++ {
        go cs.syncWorker()
    }

    // Monitor source for new content
    go cs.monitorSource()
}

func (cs *ContentSyncer) syncWorker() {
    for job := range cs.syncQueue {
        err := cs.syncContent(job.Hash)
        if err != nil && job.Retries < 3 {
            // Retry with exponential backoff
            time.Sleep(time.Duration(job.Retries+1) * time.Second)
            job.Retries++
            cs.syncQueue <- job
        }
    }
}

func (cs *ContentSyncer) syncContent(hash string) error {
    // 1. Check if content already exists in target
    exists, err := cs.checkExists(hash)
    if err != nil {
        return err
    }
    if exists {
        return nil // Already synced
    }

    // 2. Retrieve from source
    content, err := cs.sourceClient.GetFile(hash)
    if err != nil {
        return fmt.Errorf("failed to get from source: %w", err)
    }

    // 3. Upload to target
    resp, err := cs.targetClient.AddFile("sync-"+hash, content)
    if err != nil {
        return fmt.Errorf("failed to add to target: %w", err)
    }

    // 4. Pin in target to ensure persistence
    err = cs.targetClient.PinAdd(resp.Hash, false)
    if err != nil {
        log.Printf("Warning: Pin failed for synced content %s", resp.Hash)
    }

    log.Printf("Synced content: %s -> %s", hash, resp.Hash)
    return nil
}
```

### 3. Load Balancing Gateway

```go
type LoadBalancer struct {
    clients    []*KuboClient
    current    int32
    healthData map[string]*HealthStatus
    mutex      sync.RWMutex
}

type HealthStatus struct {
    Available     bool
    ResponseTime  time.Duration
    LastCheck     time.Time
    ErrorCount    int
}

func (lb *LoadBalancer) GetNextClient() *KuboClient {
    lb.mutex.RLock()
    defer lb.mutex.RUnlock()

    // Round-robin among healthy nodes
    for attempts := 0; attempts < len(lb.clients); attempts++ {
        idx := int(atomic.AddInt32(&lb.current, 1)) % len(lb.clients)
        client := lb.clients[idx]

        if lb.isHealthy(client) {
            return client
        }
    }

    // No healthy clients, return first one (circuit breaker will handle)
    return lb.clients[0]
}

func (lb *LoadBalancer) RetrieveWithFailover(hash string) ([]byte, error) {
    var lastErr error

    // Try each client until success or all fail
    for i := 0; i < len(lb.clients); i++ {
        client := lb.GetNextClient()

        start := time.Now()
        data, err := client.GetFile(hash)
        duration := time.Since(start)

        // Update health metrics
        lb.updateHealth(client, err == nil, duration)

        if err == nil {
            return data, nil
        }

        lastErr = err
        log.Printf("Client failed, trying next: %v", err)
    }

    return nil, fmt.Errorf("all clients failed, last error: %w", lastErr)
}

func (lb *LoadBalancer) StartHealthChecks() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            lb.performHealthChecks()
        }
    }()
}

func (lb *LoadBalancer) performHealthChecks() {
    for _, client := range lb.clients {
        go func(c *KuboClient) {
            start := time.Now()
            _, err := c.GetNodeInfo()
            duration := time.Since(start)

            lb.updateHealth(c, err == nil, duration)
        }(client)
    }
}
```

### 4. Distributed CDN Implementation

```go
type DistributedCDN struct {
    edgeNodes    map[string]*KuboClient  // Region -> Client
    originNode   *KuboClient
    cache        *TTLCache
    geolocation  *GeoLocator
}

func (cdn *DistributedCDN) ServeContent(hash, clientIP string) ([]byte, error) {
    // 1. Check cache first
    if cached := cdn.cache.Get(hash); cached != nil {
        return cached.([]byte), nil
    }

    // 2. Find nearest edge node
    region := cdn.geolocation.GetRegion(clientIP)
    edgeClient := cdn.getEdgeNode(region)

    // 3. Try edge node first
    content, err := edgeClient.GetFile(hash)
    if err == nil {
        cdn.cache.Set(hash, content, time.Hour)
        return content, nil
    }

    // 4. Fallback to origin
    content, err = cdn.originNode.GetFile(hash)
    if err != nil {
        return nil, err
    }

    // 5. Replicate to edge for future requests
    go cdn.replicateToEdge(hash, content, region)

    cdn.cache.Set(hash, content, time.Hour)
    return content, nil
}

func (cdn *DistributedCDN) replicateToEdge(hash string, content []byte, region string) {
    edgeClient := cdn.getEdgeNode(region)

    resp, err := edgeClient.AddFile("cdn-"+hash, content)
    if err != nil {
        log.Printf("Edge replication failed for region %s: %v", region, err)
        return
    }

    // Pin in edge node
    err = edgeClient.PinAdd(resp.Hash, false)
    if err != nil {
        log.Printf("Edge pin failed: %v", err)
    } else {
        log.Printf("Content replicated to edge %s: %s", region, resp.Hash)
    }
}
```

## ⚠️ Production Deployment Considerations

### 1. Connection Management

```go
// ✅ Production-ready HTTP client configuration
func NewProductionKuboClient(apiURL string) *KuboClient {
    return &KuboClient{
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
                TLSHandshakeTimeout: 10 * time.Second,
                ResponseHeaderTimeout: 10 * time.Second,

                // Connection pooling
                DisableKeepAlives: false,

                // Retry configuration
                MaxConnsPerHost: 10,
            },
        },
        apiURL: apiURL,
    }
}
```

### 2. Error Handling and Retry Logic

```go
// ✅ Robust error handling with exponential backoff
func (kc *KuboClient) AddFileWithRetry(filename string, content []byte, maxRetries int) (*APIResponse, error) {
    var lastErr error

    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            // Exponential backoff with jitter
            backoff := time.Duration(1<<uint(attempt-1)) * time.Second
            jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
            time.Sleep(backoff + jitter)
        }

        resp, err := kc.AddFile(filename, content)
        if err == nil {
            return resp, nil
        }

        // Check if error is retryable
        if !isRetryableError(err) {
            return nil, err
        }

        lastErr = err
        log.Printf("Attempt %d failed: %v", attempt+1, err)
    }

    return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

func isRetryableError(err error) bool {
    // Network errors are retryable
    if netErr, ok := err.(net.Error); ok {
        return netErr.Timeout() || netErr.Temporary()
    }

    // HTTP 5xx errors are retryable
    if strings.Contains(err.Error(), "status 5") {
        return true
    }

    return false
}
```

### 3. Monitoring and Metrics

```go
// ✅ Comprehensive monitoring
type KuboClientMetrics struct {
    RequestCount    prometheus.Counter
    RequestDuration prometheus.Histogram
    ErrorRate       prometheus.Counter
}

func (kc *KuboClient) AddFileWithMetrics(filename string, content []byte) (*APIResponse, error) {
    start := time.Now()

    // Increment request counter
    kc.metrics.RequestCount.Inc()

    resp, err := kc.AddFile(filename, content)

    // Record duration
    duration := time.Since(start).Seconds()
    kc.metrics.RequestDuration.Observe(duration)

    // Record errors
    if err != nil {
        kc.metrics.ErrorRate.Inc()
    }

    return resp, err
}
```

### 4. Security and Authentication

```go
// ✅ Secure API access
type SecureKuboClient struct {
    *KuboClient
    apiKey    string
    tlsConfig *tls.Config
}

func (skc *SecureKuboClient) makeAuthenticatedRequest(req *http.Request) (*http.Response, error) {
    // Add API key authentication
    if skc.apiKey != "" {
        req.Header.Set("Authorization", "Bearer "+skc.apiKey)
    }

    // Add security headers
    req.Header.Set("User-Agent", "MyApp/1.0")
    req.Header.Set("X-Requested-With", "XMLHttpRequest")

    return skc.httpClient.Do(req)
}
```

## 🔧 Troubleshooting

### Issue 1: "connection refused" to Kubo API

**Cause**: Kubo daemon not running or wrong API address
```bash
# Solution: Check and restart Kubo daemon
ipfs id                 # Check if daemon is running
ipfs config Addresses.API   # Check API address
ipfs daemon            # Restart daemon
```

### Issue 2: Timeout errors in network operations

**Cause**: Network latency or node connectivity issues
```go
// Solution: Implement timeout configuration
client := &http.Client{
    Timeout: 60 * time.Second, // Increase timeout for network operations
}
```

### Issue 3: Content not found on network

**Cause**: Content not properly announced or DHT propagation delay
```bash
# Solution: Check content propagation
ipfs dht provide [HASH]     # Manually announce content
ipfs dht findprovs [HASH]   # Check who provides the content
```

## 📚 Additional Learning Resources

### Related Documentation
- [Kubo HTTP API Reference](https://docs.ipfs.io/reference/kubo/rpc/)
- [IPFS HTTP Client Libraries](https://docs.ipfs.io/reference/http/api/)
- [Production Deployment Guide](https://docs.ipfs.io/install/server-infrastructure/)

## 📚 Next Steps

### Immediate Next Steps
**Congratulations!** You've completed the core boxo learning path. Here are your next opportunities:

1. **Production Deployment**: Deploy your IPFS applications to production environments
   - **Focus**: Infrastructure setup, monitoring, and scaling
   - **Next Actions**: Server configuration, load balancing, and operational procedures

2. **Advanced Feature Integration**: Explore specialized IPFS features
   - **Options**: PubSub messaging, Circuit Relay, Experimental features
   - **Focus**: Advanced networking and distributed application patterns

### Related Advanced Modules
3. **[12-ipld-prime](../12-ipld-prime)**: Next-generation IPLD with advanced schemas
   - **Connection**: Enhanced data structures and type safety
   - **When to Learn**: For complex data modeling requirements

4. **[16-trustless-gateway](../16-trustless-gateway)**: Verified content serving
   - **Connection**: Production-grade gateway with cryptographic verification
   - **Advanced Use**: High-security content delivery applications

5. **[17-ipni](../17-ipni)**: Large-scale content indexing and discovery
   - **Connection**: Enhanced content discovery for production networks
   - **Enterprise Use**: Large-scale content management systems

### Specialized Learning Paths
- **For Data Processing**: Explore **[13-dasl](../13-dasl)** → **[14-traversal-selector](../14-traversal-selector)** → **[15-graphsync](../15-graphsync)**
- **For Content Distribution**: Focus on **[16-trustless-gateway](../16-trustless-gateway)** → **[17-ipni](../17-ipni)** → **[18-multifetcher](../18-multifetcher)**
- **For Application Development**: Build custom applications using the patterns learned throughout this series

## 🍳 Production Cookbook - Ready-to-Deploy Code

### 🚀 Complete Production Integration System

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    "sync"

    kubo "github.com/your-org/boxo-starter-kit/11-kubo-api-demo/pkg"
)

// Production-ready IPFS application with full integration
type ProductionIPFSApp struct {
    primaryClient   *kubo.KuboClient
    backupClients   []*kubo.KuboClient
    cache          *ProductionCache
    metrics        *ApplicationMetrics
    config         *AppConfig
    healthChecker  *HealthChecker
}

type AppConfig struct {
    PrimaryAPI    string        `json:"primary_api"`
    BackupAPIs    []string      `json:"backup_apis"`
    CacheTTL      time.Duration `json:"cache_ttl"`
    RetryCount    int           `json:"retry_count"`
    HealthCheck   time.Duration `json:"health_check_interval"`
    MetricsPort   int           `json:"metrics_port"`
}

func NewProductionIPFSApp(config *AppConfig) (*ProductionIPFSApp, error) {
    // Initialize primary client
    primaryClient := kubo.NewKuboClient(config.PrimaryAPI, 30*time.Second)

    // Initialize backup clients
    backupClients := make([]*kubo.KuboClient, len(config.BackupAPIs))
    for i, api := range config.BackupAPIs {
        backupClients[i] = kubo.NewKuboClient(api, 30*time.Second)
    }

    app := &ProductionIPFSApp{
        primaryClient: primaryClient,
        backupClients: backupClients,
        cache:        NewProductionCache(config.CacheTTL),
        metrics:      NewApplicationMetrics(),
        config:       config,
        healthChecker: NewHealthChecker(),
    }

    // Start background services
    app.startBackgroundServices()

    return app, nil
}

func (app *ProductionIPFSApp) StoreContent(key string, data []byte) (string, error) {
    start := time.Now()
    defer func() {
        app.metrics.RecordOperation("store", time.Since(start))
    }()

    // Try primary client first
    hash, err := app.storeWithClient(app.primaryClient, key, data)
    if err == nil {
        log.Printf("Content stored via primary client: %s", hash)
        return hash, nil
    }

    // Try backup clients
    for i, client := range app.backupClients {
        hash, err = app.storeWithClient(client, key, data)
        if err == nil {
            log.Printf("Content stored via backup client %d: %s", i, hash)
            app.metrics.RecordFailover("store")
            return hash, nil
        }
    }

    app.metrics.RecordError("store")
    return "", fmt.Errorf("all storage attempts failed: %w", err)
}

func (app *ProductionIPFSApp) storeWithClient(client *kubo.KuboClient, key string, data []byte) (string, error) {
    // Upload file
    resp, err := client.AddFileWithRetry(key, data, app.config.RetryCount)
    if err != nil {
        return "", err
    }

    // Ensure persistence with pinning
    err = client.PinAdd(resp.Hash, false)
    if err != nil {
        log.Printf("Warning: Pin failed for %s: %v", resp.Hash, err)
        // Don't fail the operation for pin failures
    }

    // Cache locally for faster future access
    app.cache.Set(resp.Hash, data)

    return resp.Hash, nil
}

func (app *ProductionIPFSApp) RetrieveContent(hash string) ([]byte, error) {
    start := time.Now()
    defer func() {
        app.metrics.RecordOperation("retrieve", time.Since(start))
    }()

    // Check cache first
    if cached := app.cache.Get(hash); cached != nil {
        app.metrics.RecordCacheHit()
        return cached, nil
    }

    app.metrics.RecordCacheMiss()

    // Try retrieval with failover
    data, err := app.retrieveWithFailover(hash)
    if err != nil {
        app.metrics.RecordError("retrieve")
        return nil, err
    }

    // Cache the result
    app.cache.Set(hash, data)

    return data, nil
}

func (app *ProductionIPFSApp) retrieveWithFailover(hash string) ([]byte, error) {
    // Try primary client
    data, err := app.primaryClient.GetFile(hash)
    if err == nil {
        return data, nil
    }

    // Try backup clients
    for i, client := range app.backupClients {
        data, err = client.GetFile(hash)
        if err == nil {
            log.Printf("Content retrieved via backup client %d", i)
            app.metrics.RecordFailover("retrieve")
            return data, nil
        }
    }

    return nil, fmt.Errorf("content retrieval failed from all sources: %w", err)
}

func (app *ProductionIPFSApp) GetSystemStatus() *SystemStatus {
    status := &SystemStatus{
        Timestamp: time.Now(),
        Nodes:     make([]NodeStatus, 0),
    }

    // Check primary node
    nodeInfo, err := app.primaryClient.GetNodeInfo()
    if err != nil {
        status.Nodes = append(status.Nodes, NodeStatus{
            Type:      "primary",
            Available: false,
            Error:     err.Error(),
        })
    } else {
        peers, _ := app.primaryClient.GetConnectedPeers()
        status.Nodes = append(status.Nodes, NodeStatus{
            Type:      "primary",
            Available: true,
            NodeID:    nodeInfo.ID,
            PeerCount: len(peers),
        })
    }

    // Check backup nodes
    for i, client := range app.backupClients {
        nodeInfo, err := client.GetNodeInfo()
        if err != nil {
            status.Nodes = append(status.Nodes, NodeStatus{
                Type:      fmt.Sprintf("backup-%d", i),
                Available: false,
                Error:     err.Error(),
            })
        } else {
            peers, _ := client.GetConnectedPeers()
            status.Nodes = append(status.Nodes, NodeStatus{
                Type:      fmt.Sprintf("backup-%d", i),
                Available: true,
                NodeID:    nodeInfo.ID,
                PeerCount: len(peers),
            })
        }
    }

    return status
}

func (app *ProductionIPFSApp) startBackgroundServices() {
    // Start health checker
    go app.healthChecker.Start(app)

    // Start metrics server
    go app.metrics.StartServer(app.config.MetricsPort)

    // Start cache cleanup
    go app.cache.StartCleanup()
}

type SystemStatus struct {
    Timestamp time.Time    `json:"timestamp"`
    Nodes     []NodeStatus `json:"nodes"`
}

type NodeStatus struct {
    Type      string `json:"type"`
    Available bool   `json:"available"`
    NodeID    string `json:"node_id,omitempty"`
    PeerCount int    `json:"peer_count,omitempty"`
    Error     string `json:"error,omitempty"`
}

// Production cache implementation
type ProductionCache struct {
    data   map[string]CacheEntry
    ttl    time.Duration
    mutex  sync.RWMutex
}

type CacheEntry struct {
    Data      []byte
    ExpiresAt time.Time
}

func NewProductionCache(ttl time.Duration) *ProductionCache {
    return &ProductionCache{
        data: make(map[string]CacheEntry),
        ttl:  ttl,
    }
}

func (pc *ProductionCache) Get(key string) []byte {
    pc.mutex.RLock()
    defer pc.mutex.RUnlock()

    entry, exists := pc.data[key]
    if !exists || time.Now().After(entry.ExpiresAt) {
        return nil
    }

    return entry.Data
}

func (pc *ProductionCache) Set(key string, data []byte) {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()

    pc.data[key] = CacheEntry{
        Data:      data,
        ExpiresAt: time.Now().Add(pc.ttl),
    }
}

func (pc *ProductionCache) StartCleanup() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        pc.cleanup()
    }
}

func (pc *ProductionCache) cleanup() {
    pc.mutex.Lock()
    defer pc.mutex.Unlock()

    now := time.Now()
    for key, entry := range pc.data {
        if now.After(entry.ExpiresAt) {
            delete(pc.data, key)
        }
    }
}

// Application metrics
type ApplicationMetrics struct {
    operationCount   map[string]int64
    operationTime    map[string]time.Duration
    errorCount       map[string]int64
    cacheHits        int64
    cacheMisses      int64
    failoverCount    map[string]int64
    mutex           sync.RWMutex
}

func NewApplicationMetrics() *ApplicationMetrics {
    return &ApplicationMetrics{
        operationCount: make(map[string]int64),
        operationTime:  make(map[string]time.Duration),
        errorCount:     make(map[string]int64),
        failoverCount:  make(map[string]int64),
    }
}

func (am *ApplicationMetrics) RecordOperation(operation string, duration time.Duration) {
    am.mutex.Lock()
    defer am.mutex.Unlock()

    am.operationCount[operation]++
    am.operationTime[operation] += duration
}

func (am *ApplicationMetrics) RecordError(operation string) {
    am.mutex.Lock()
    defer am.mutex.Unlock()

    am.errorCount[operation]++
}

func (am *ApplicationMetrics) RecordCacheHit() {
    am.mutex.Lock()
    defer am.mutex.Unlock()

    am.cacheHits++
}

func (am *ApplicationMetrics) RecordCacheMiss() {
    am.mutex.Lock()
    defer am.mutex.Unlock()

    am.cacheMisses++
}

func (am *ApplicationMetrics) RecordFailover(operation string) {
    am.mutex.Lock()
    defer am.mutex.Unlock()

    am.failoverCount[operation]++
}

func (am *ApplicationMetrics) StartServer(port int) {
    // Start metrics HTTP server for monitoring integration
    // Implementation would include Prometheus metrics endpoint
    log.Printf("Metrics server started on port %d", port)
}

// Health checker
type HealthChecker struct {
    interval time.Duration
}

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{
        interval: 30 * time.Second,
    }
}

func (hc *HealthChecker) Start(app *ProductionIPFSApp) {
    ticker := time.NewTicker(hc.interval)
    for range ticker.C {
        status := app.GetSystemStatus()
        hc.logSystemHealth(status)
    }
}

func (hc *HealthChecker) logSystemHealth(status *SystemStatus) {
    healthy := 0
    total := len(status.Nodes)

    for _, node := range status.Nodes {
        if node.Available {
            healthy++
        }
    }

    log.Printf("System health: %d/%d nodes healthy", healthy, total)
    if healthy == 0 {
        log.Printf("CRITICAL: All IPFS nodes are unavailable!")
    }
}

func main() {
    config := &AppConfig{
        PrimaryAPI:    "http://localhost:5001",
        BackupAPIs:    []string{"https://ipfs.infura.io:5001"},
        CacheTTL:      time.Hour,
        RetryCount:    3,
        HealthCheck:   30 * time.Second,
        MetricsPort:   9090,
    }

    app, err := NewProductionIPFSApp(config)
    if err != nil {
        log.Fatal("Failed to initialize app:", err)
    }

    // Demo usage
    fmt.Println("=== Production IPFS Application Started ===")

    // Store content
    content := []byte("Hello, Production IPFS World! " + time.Now().String())
    hash, err := app.StoreContent("demo.txt", content)
    if err != nil {
        log.Printf("Store failed: %v", err)
    } else {
        fmt.Printf("Content stored: %s\n", hash)
    }

    // Retrieve content
    if hash != "" {
        retrieved, err := app.RetrieveContent(hash)
        if err != nil {
            log.Printf("Retrieve failed: %v", err)
        } else {
            fmt.Printf("Content retrieved: %s\n", string(retrieved))
        }
    }

    // Show system status
    status := app.GetSystemStatus()
    fmt.Printf("System status: %d nodes\n", len(status.Nodes))
    for _, node := range status.Nodes {
        if node.Available {
            fmt.Printf("✅ %s: %d peers\n", node.Type, node.PeerCount)
        } else {
            fmt.Printf("❌ %s: %s\n", node.Type, node.Error)
        }
    }

    // Keep running
    select {}
}
```

Congratulations! You have completed the entire boxo starter kit and are now ready to build production-grade IPFS applications! 🚀