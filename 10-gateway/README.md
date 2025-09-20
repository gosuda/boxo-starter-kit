# 06-gateway: HTTP Gateway and Web Integration

## 🎯 Learning Objectives

Through this module, you will learn:
- The concept of **IPFS HTTP Gateway** and principles of web integration
- **Path resolution** mechanisms (/ipfs/ and /ipns/ paths)
- **HTTP request handling** and RESTful API design patterns
- **Content-Type detection** and appropriate response header configuration
- **Static website hosting** and web application deployment
- **Performance optimization** and caching strategies for gateways

## 📋 Prerequisites

- **00-block-cid** module completed (understanding Blocks and CIDs)
- **01-persistent** module completed (understanding data persistence)
- **02-dag-ipld** module completed (understanding DAG and IPLD)
- **03-unixfs** module completed (understanding file systems and chunking)
- **04-pin-gc** module completed (understanding pin management)
- Basic concepts of HTTP protocols and web servers
- Understanding of web development and REST APIs

## 🔑 Core Concepts

### What is an IPFS Gateway?

An **IPFS Gateway** provides HTTP access to IPFS content, bridging the IPFS network and the traditional web:

```
Web Browser  →  HTTP Request   →  IPFS Gateway  →  IPFS Network
             ←  HTTP Response  ←                ←
```

### Gateway Types

| Type | Description | Usage |
|------|-------------|--------|
| **Public Gateway** | Open access gateway | ipfs.io, dweb.link |
| **Private Gateway** | Access-controlled gateway | Internal services, paid services |
| **Local Gateway** | Individual node gateway | localhost:8080 |
| **Subdomain Gateway** | Subdomain-based routing | {cid}.ipfs.dweb.link |

### Path Resolution

```
/ipfs/QmHash.../path/to/file   → Content-addressed static content
/ipns/domain.com/path          → DNS-linked dynamic content
/ipns/QmPeerID.../path         → IPNS record-based content
```

### HTTP Integration Advantages

1. **Universal Access**: Accessible from any web browser
2. **SEO Compatibility**: Searchable and indexable content
3. **CDN Integration**: Cacheable static content
4. **Standard Protocols**: Leverages existing web infrastructure

## 💻 Code Analysis

### 1. Gateway Server Structure

```go
// pkg/gateway.go:25-45
type Gateway struct {
    dagWrapper    *dag.DagWrapper
    unixfsWrapper *unixfs.UnixFsWrapper
    pinManager    *pin.PinManager
    server        *http.Server
    config        GatewayConfig
}

type GatewayConfig struct {
    Port            string        `json:"port"`
    Host            string        `json:"host"`
    ReadTimeout     time.Duration `json:"read_timeout"`
    WriteTimeout    time.Duration `json:"write_timeout"`
    MaxRequestSize  int64         `json:"max_request_size"`
    EnableCORS      bool          `json:"enable_cors"`
    AllowedOrigins  []string      `json:"allowed_origins"`
}

func NewGateway(dagWrapper *dag.DagWrapper, config GatewayConfig) (*Gateway, error) {
    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    return &Gateway{
        dagWrapper:    dagWrapper,
        unixfsWrapper: unixfsWrapper,
        config:        config,
    }, nil
}
```

**Design Features**:
- **Layered Architecture**: Reuse existing DAG and UnixFS functionality
- **Configurable Server**: Flexible timeout and CORS settings
- **Modular Design**: Clear separation of concerns between components

### 2. HTTP Route Handler

```go
// pkg/gateway.go:75-110
func (gw *Gateway) setupRoutes() {
    mux := http.NewServeMux()

    // 1. IPFS content routes
    mux.HandleFunc("/ipfs/", gw.handleIPFS)
    mux.HandleFunc("/ipns/", gw.handleIPNS)

    // 2. API routes
    mux.HandleFunc("/api/v0/add", gw.handleAdd)
    mux.HandleFunc("/api/v0/cat", gw.handleCat)
    mux.HandleFunc("/api/v0/ls", gw.handleList)

    // 3. Gateway info routes
    mux.HandleFunc("/api/v0/version", gw.handleVersion)
    mux.HandleFunc("/api/v0/id", gw.handleNodeInfo)

    // 4. Static file serving (for web UI)
    mux.Handle("/", http.FileServer(http.Dir("./webui/")))

    // 5. Apply middleware
    handler := gw.corsMiddleware(mux)
    handler = gw.loggingMiddleware(handler)
    handler = gw.authMiddleware(handler)

    gw.server.Handler = handler
}
```

### 3. IPFS Path Resolution

```go
// pkg/gateway.go:140-185
func (gw *Gateway) handleIPFS(w http.ResponseWriter, r *http.Request) {
    // 1. Extract path components
    path := strings.TrimPrefix(r.URL.Path, "/ipfs/")
    pathComponents := strings.Split(path, "/")

    if len(pathComponents) == 0 {
        http.Error(w, "Invalid IPFS path", http.StatusBadRequest)
        return
    }

    // 2. Parse CID
    cidStr := pathComponents[0]
    rootCID, err := cid.Decode(cidStr)
    if err != nil {
        http.Error(w, fmt.Sprintf("Invalid CID: %s", err), http.StatusBadRequest)
        return
    }

    // 3. Resolve sub-path if exists
    var node files.Node
    if len(pathComponents) > 1 {
        subPath := "/" + strings.Join(pathComponents[1:], "/")
        node, err = gw.unixfsWrapper.GetPath(r.Context(), rootCID, subPath)
    } else {
        node, err = gw.unixfsWrapper.Get(r.Context(), rootCID)
    }

    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            http.NotFound(w, r)
        } else {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
        return
    }

    // 4. Serve content based on type
    switch n := node.(type) {
    case files.File:
        gw.serveFile(w, r, n, path)
    case files.Directory:
        gw.serveDirectory(w, r, n, path)
    default:
        http.Error(w, "Unsupported content type", http.StatusInternalServerError)
    }
}
```

### 4. File Serving with Content-Type Detection

```go
// pkg/gateway.go:190-240
func (gw *Gateway) serveFile(w http.ResponseWriter, r *http.Request,
                            file files.File, path string) {
    // 1. Detect content type
    contentType := gw.detectContentType(file, path)
    w.Header().Set("Content-Type", contentType)

    // 2. Set cache headers
    w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
    w.Header().Set("ETag", fmt.Sprintf("W/\"%s\"", path))

    // 3. Handle range requests for large files
    if gw.supportsRangeRequests(r) {
        gw.serveFileRange(w, r, file)
        return
    }

    // 4. Stream file content
    defer file.Close()
    size, err := file.Size()
    if err == nil {
        w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
    }

    // 5. Copy file data to response
    _, err = io.Copy(w, file)
    if err != nil {
        log.Printf("Error serving file: %v", err)
        return
    }

    log.Printf("Served file: %s (%s, %d bytes)", path, contentType, size)
}

func (gw *Gateway) detectContentType(file files.File, path string) string {
    // 1. Try extension-based detection first
    ext := filepath.Ext(path)
    if mimeType := mime.TypeByExtension(ext); mimeType != "" {
        return mimeType
    }

    // 2. Content-based detection for unknown extensions
    buffer := make([]byte, 512)
    n, err := file.Read(buffer)
    if err != nil && err != io.EOF {
        return "application/octet-stream"
    }

    // Reset file position
    if seeker, ok := file.(io.Seeker); ok {
        seeker.Seek(0, io.SeekStart)
    }

    contentType := http.DetectContentType(buffer[:n])

    // 3. Special handling for text files
    if strings.HasPrefix(contentType, "text/plain") {
        if strings.Contains(path, ".md") {
            return "text/markdown; charset=utf-8"
        }
        if strings.Contains(path, ".json") {
            return "application/json; charset=utf-8"
        }
    }

    return contentType
}
```

### 5. Directory Listing with HTML Generation

```go
// pkg/gateway.go:280-340
func (gw *Gateway) serveDirectory(w http.ResponseWriter, r *http.Request,
                                 dir files.Directory, path string) {
    // 1. Set HTML content type
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Header().Set("Cache-Control", "public, max-age=3600")

    // 2. Generate directory listing HTML
    html := gw.generateDirectoryHTML(dir, path)

    // 3. Write HTML response
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(html))

    log.Printf("Served directory listing: %s", path)
}

func (gw *Gateway) generateDirectoryHTML(dir files.Directory, path string) string {
    var html strings.Builder

    // HTML document header
    html.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Directory: ` + html.EscapeString(path) + `</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { border-bottom: 1px solid #ddd; padding-bottom: 10px; }
        .entry { padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
        .entry:hover { background-color: #f8f8f8; }
        .dir { font-weight: bold; }
        .file { color: #666; }
        .size { float: right; color: #999; font-size: 0.9em; }
        .parent { color: #0066cc; }
    </style>
</head>
<body>`)

    // Directory title
    html.WriteString(fmt.Sprintf(`<h1>📁 Directory: %s</h1>`,
                     html.EscapeString(path)))

    // Parent directory link
    if path != "/" {
        parentPath := filepath.Dir(path)
        html.WriteString(fmt.Sprintf(`<div class="entry parent">
            <a href="%s">📁 ../</a> (parent directory)
        </div>`, parentPath))
    }

    // Directory entries
    entries := dir.Entries()
    for entries.Next() {
        name := entries.Name()
        node := entries.Node()

        // Determine entry type and icon
        var icon, class, sizeInfo string
        switch node.(type) {
        case files.Directory:
            icon = "📁"
            class = "dir"
            sizeInfo = "(directory)"
        case files.File:
            icon = "📄"
            class = "file"
            if file, ok := node.(files.File); ok {
                if size, err := file.Size(); err == nil {
                    sizeInfo = gw.formatSize(size)
                }
            }
        default:
            icon = "❓"
            class = "file"
            sizeInfo = "(unknown)"
        }

        // Generate entry HTML
        entryPath := filepath.Join(path, name)
        html.WriteString(fmt.Sprintf(`<div class="entry %s">
            <a href="%s">%s %s</a>
            <span class="size">%s</span>
        </div>`, class, entryPath, icon, html.EscapeString(name), sizeInfo))
    }

    // Footer
    html.WriteString(`
    <hr>
    <footer style="margin-top: 20px; font-size: 0.9em; color: #666;">
        <p>Powered by IPFS Gateway</p>
    </footer>
</body>
</html>`)

    return html.String()
}

func (gw *Gateway) formatSize(size int64) string {
    const unit = 1024
    if size < unit {
        return fmt.Sprintf("%d B", size)
    }
    div, exp := int64(unit), 0
    for n := size / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
```

## 🏃‍♂️ Hands-on Guide

### 1. Basic Execution

```bash
cd 06-gateway
go run main.go
```

**Expected Output**:
```
=== IPFS HTTP Gateway Demo ===

1. Setting up gateway server:
   ✅ DAG service initialized
   ✅ UnixFS wrapper ready
   ✅ Gateway configured (port: 8080)

2. Starting HTTP server:
   🌐 Gateway server starting on http://localhost:8080
   ✅ Server ready to handle requests

3. Available endpoints:
   📁 /ipfs/{CID}           - IPFS content access
   🔗 /ipns/{name}          - IPNS name resolution
   📄 /api/v0/add          - Upload content
   🔍 /api/v0/cat          - View content
   📋 /api/v0/ls           - List directory
   ℹ️  /api/v0/version      - Gateway version
   🏠 /                    - Gateway web UI

4. Test content added:
   📄 Text file: http://localhost:8080/ipfs/bafkreigh2ak...
   📁 Directory: http://localhost:8080/ipfs/bafybeigdyr...
   🖼️ Image file: http://localhost:8080/ipfs/bafkreihwd...

5. Server status:
   ⏱️  Request timeout: 30s
   📝 CORS enabled for: *
   🔒 Authentication: disabled (demo mode)
   📊 Max request size: 32MB

Access the gateway at: http://localhost:8080
```

### 2. Web Browser Testing

Open your web browser and test the following URLs:

```bash
# View text file
http://localhost:8080/ipfs/bafkreigh2akiscaiaanfkiuokmv4ooqlm5u7r22krvotqm7uhtgvowim2i

# Browse directory
http://localhost:8080/ipfs/bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi

# View image file
http://localhost:8080/ipfs/bafkreihwdcxzvn353r4l5iy67q7vnk2ute6x6l7l5j3eeqrbzt5j7xqb7q

# IPNS resolution (if configured)
http://localhost:8080/ipns/example.com
```

### 3. API Testing with curl

```bash
# Upload file via API
curl -X POST -F "file=@test.txt" http://localhost:8080/api/v0/add

# Retrieve file content
curl "http://localhost:8080/api/v0/cat?arg=QmHashFromAbove"

# List directory contents
curl "http://localhost:8080/api/v0/ls?arg=QmDirectoryHash"

# Check gateway version
curl "http://localhost:8080/api/v0/version"
```

### 4. Running Tests

```bash
go test -v ./...
```

**Key Test Cases**:
- ✅ HTTP route handling
- ✅ IPFS path resolution
- ✅ Content-type detection
- ✅ Directory listing generation
- ✅ Error handling and status codes

## 🔍 Advanced Use Cases

### 1. Static Website Hosting

```go
type WebsiteGateway struct {
    *Gateway
    domains map[string]string // domain -> IPFS hash mapping
}

func (wg *WebsiteGateway) handleCustomDomain(w http.ResponseWriter, r *http.Request) {
    host := r.Host

    // 1. Check if it's a registered domain
    if ipfsHash, exists := wg.domains[host]; exists {
        // Redirect to IPFS path
        targetPath := "/ipfs/" + ipfsHash + r.URL.Path
        r.URL.Path = targetPath
        wg.handleIPFS(w, r)
        return
    }

    // 2. Try subdomain gateway pattern
    if strings.Contains(host, ".ipfs.") {
        parts := strings.Split(host, ".")
        if len(parts) >= 3 && parts[1] == "ipfs" {
            cid := parts[0]
            targetPath := "/ipfs/" + cid + r.URL.Path
            r.URL.Path = targetPath
            wg.handleIPFS(w, r)
            return
        }
    }

    // 3. Default handling
    wg.Gateway.handleIPFS(w, r)
}

func (wg *WebsiteGateway) RegisterDomain(domain, ipfsHash string) {
    if wg.domains == nil {
        wg.domains = make(map[string]string)
    }
    wg.domains[domain] = ipfsHash
    log.Printf("Registered domain: %s -> %s", domain, ipfsHash)
}
```

### 2. API Gateway with Rate Limiting

```go
type APIGateway struct {
    *Gateway
    rateLimiter map[string]*rate.Limiter
    mutex       sync.RWMutex
}

func (ag *APIGateway) rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Get client IP
        clientIP := ag.getClientIP(r)

        // 2. Get or create rate limiter for this IP
        ag.mutex.RLock()
        limiter, exists := ag.rateLimiter[clientIP]
        ag.mutex.RUnlock()

        if !exists {
            // Create new rate limiter: 10 requests per second, burst of 20
            limiter = rate.NewLimiter(10, 20)
            ag.mutex.Lock()
            ag.rateLimiter[clientIP] = limiter
            ag.mutex.Unlock()
        }

        // 3. Check rate limit
        if !limiter.Allow() {
            w.Header().Set("Retry-After", "1")
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func (ag *APIGateway) getClientIP(r *http.Request) string {
    // Check X-Forwarded-For header first
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        parts := strings.Split(xff, ",")
        return strings.TrimSpace(parts[0])
    }

    // Check X-Real-IP header
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }

    // Fall back to remote address
    host, _, _ := net.SplitHostPort(r.RemoteAddr)
    return host
}
```

### 3. Content Delivery Network (CDN)

```go
type CDNGateway struct {
    *Gateway
    cache        *lru.Cache
    cacheTTL     time.Duration
    edgeNodes    []string
    loadBalancer *LoadBalancer
}

func (cg *CDNGateway) handleWithCaching(w http.ResponseWriter, r *http.Request) {
    cacheKey := r.URL.Path

    // 1. Check cache first
    if cached, ok := cg.cache.Get(cacheKey); ok {
        if entry, valid := cached.(*CacheEntry); valid && !entry.IsExpired() {
            // Serve from cache
            cg.serveCachedContent(w, r, entry)
            return
        }
    }

    // 2. If not in cache or expired, fetch from IPFS
    recorder := &ResponseRecorder{ResponseWriter: w}
    cg.Gateway.handleIPFS(recorder, r)

    // 3. Cache successful responses
    if recorder.StatusCode == 200 {
        entry := &CacheEntry{
            Content:     recorder.Body.Bytes(),
            ContentType: recorder.Header().Get("Content-Type"),
            CachedAt:    time.Now(),
            TTL:         cg.cacheTTL,
        }
        cg.cache.Add(cacheKey, entry)
    }
}

func (cg *CDNGateway) serveCachedContent(w http.ResponseWriter, r *http.Request,
                                        entry *CacheEntry) {
    // Set cache headers
    w.Header().Set("Content-Type", entry.ContentType)
    w.Header().Set("X-Cache", "HIT")
    w.Header().Set("Cache-Control", "public, max-age=3600")

    // Serve content
    w.WriteHeader(http.StatusOK)
    w.Write(entry.Content)

    log.Printf("Served from cache: %s", r.URL.Path)
}
```

### 4. Media Streaming Gateway

```go
type StreamingGateway struct {
    *Gateway
    transcoder *VideoTranscoder
}

func (sg *StreamingGateway) handleVideoStream(w http.ResponseWriter, r *http.Request) {
    // 1. Parse streaming parameters
    quality := r.URL.Query().Get("quality")  // 720p, 1080p, etc.
    format := r.URL.Query().Get("format")    // mp4, webm, hls
    startTime := r.URL.Query().Get("t")      // seek time

    // 2. Get video file from IPFS
    videoPath := strings.TrimPrefix(r.URL.Path, "/stream/ipfs/")
    parts := strings.Split(videoPath, "/")
    if len(parts) == 0 {
        http.Error(w, "Invalid video path", http.StatusBadRequest)
        return
    }

    cidStr := parts[0]
    videoCID, err := cid.Decode(cidStr)
    if err != nil {
        http.Error(w, "Invalid CID", http.StatusBadRequest)
        return
    }

    // 3. Get video file
    node, err := sg.unixfsWrapper.Get(r.Context(), videoCID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    videoFile, ok := node.(files.File)
    if !ok {
        http.Error(w, "Not a video file", http.StatusBadRequest)
        return
    }

    // 4. Handle different streaming formats
    switch format {
    case "hls":
        sg.serveHLS(w, r, videoFile, quality)
    case "dash":
        sg.serveDASH(w, r, videoFile, quality)
    default:
        sg.serveProgressiveVideo(w, r, videoFile, startTime)
    }
}

func (sg *StreamingGateway) serveHLS(w http.ResponseWriter, r *http.Request,
                                   video files.File, quality string) {
    // Generate HLS manifest
    manifest := sg.transcoder.GenerateHLSManifest(video, quality)

    w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(manifest))
}

func (sg *StreamingGateway) serveProgressiveVideo(w http.ResponseWriter, r *http.Request,
                                                video files.File, startTime string) {
    // Handle HTTP range requests for video seeking
    size, _ := video.Size()
    rangeHeader := r.Header.Get("Range")

    if rangeHeader != "" {
        // Parse range request
        ranges, err := parseRangeHeader(rangeHeader, size)
        if err != nil {
            http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
            return
        }

        // Serve partial content
        for _, r := range ranges {
            sg.serveVideoRange(w, video, r.start, r.end, size)
        }
    } else {
        // Serve entire video
        w.Header().Set("Content-Type", "video/mp4")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
        io.Copy(w, video)
    }
}
```

## ⚠️ Best Practices and Security Considerations

### 1. Security Headers

```go
// ✅ Essential security headers
func (gw *Gateway) securityMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Prevent clickjacking
        w.Header().Set("X-Frame-Options", "DENY")

        // Prevent MIME type sniffing
        w.Header().Set("X-Content-Type-Options", "nosniff")

        // XSS protection
        w.Header().Set("X-XSS-Protection", "1; mode=block")

        // HTTPS enforcement
        if gw.config.ForceHTTPS {
            w.Header().Set("Strict-Transport-Security",
                          "max-age=31536000; includeSubDomains")
        }

        // Content Security Policy
        w.Header().Set("Content-Security-Policy",
                      "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'")

        next.ServeHTTP(w, r)
    })
}
```

### 2. Input Validation and Sanitization

```go
// ✅ Path validation
func (gw *Gateway) validatePath(path string) error {
    // Prevent path traversal attacks
    if strings.Contains(path, "..") {
        return fmt.Errorf("invalid path: contains path traversal")
    }

    // Limit path length
    if len(path) > 1000 {
        return fmt.Errorf("path too long")
    }

    // Validate CID format
    parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
    if len(parts) == 0 {
        return fmt.Errorf("empty path")
    }

    _, err := cid.Decode(parts[0])
    return err
}
```

### 3. Performance Optimization

```go
// ✅ Connection pooling and keep-alive
func (gw *Gateway) optimizedTransport() *http.Transport {
    return &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
        DisableCompression:  false,

        // Connection pooling for IPFS requests
        DialContext: (&net.Dialer{
            Timeout:   30 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,
    }
}
```

### 4. Error Handling and Monitoring

```go
// ✅ Comprehensive error handling
func (gw *Gateway) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
    // Log error for monitoring
    log.Printf("Gateway error: %s %s - %v", r.Method, r.URL.Path, err)

    // Increment error metrics
    gw.metrics.ErrorCount.Inc()

    // Determine appropriate status code
    var statusCode int
    var message string

    switch {
    case strings.Contains(err.Error(), "not found"):
        statusCode = http.StatusNotFound
        message = "Content not found"
    case strings.Contains(err.Error(), "timeout"):
        statusCode = http.StatusGatewayTimeout
        message = "Request timeout"
    case strings.Contains(err.Error(), "invalid"):
        statusCode = http.StatusBadRequest
        message = "Invalid request"
    default:
        statusCode = http.StatusInternalServerError
        message = "Internal server error"
    }

    // Return structured error response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "error":   true,
        "message": message,
        "code":    statusCode,
    })
}
```

## 🔧 Troubleshooting

### Issue 1: "connection refused" error

**Cause**: Gateway server not running or port conflict
```bash
# Solution: Check port availability and start server
netstat -an | grep 8080
sudo lsof -i :8080

# Change port if needed
GATEWAY_PORT=8081 go run main.go
```

### Issue 2: "not found" error for valid content

**Cause**: Content not pinned or network connectivity issues
```go
// Solution: Verify content exists and is accessible
func (gw *Gateway) debugContentAccess(ctx context.Context, c cid.Cid) {
    // Check if content exists locally
    node, err := gw.dagWrapper.Get(ctx, c)
    if err != nil {
        log.Printf("Content not found locally: %v", err)
        // Try to fetch from network
        gw.dagWrapper.FetchFromNetwork(ctx, c)
    } else {
        log.Printf("Content found locally: %s", c)
    }
}
```

### Issue 3: CORS errors in browser

**Cause**: Cross-origin requests blocked
```go
// Solution: Configure CORS properly
func (gw *Gateway) corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

## 📚 Additional Learning Resources

### Related Documentation
- [IPFS HTTP Gateway Specification](https://specs.ipfs.tech/http-gateways/)
- [Gateway Implementation Guide](https://docs.ipfs.io/how-to/configure-gateway/)
- [Web Integration Patterns](https://docs.ipfs.io/how-to/websites-on-ipfs/)

### Next Steps
1. **09-ipns**: Dynamic naming and mutable content
2. **99-kubo-api-demo**: Full IPFS network integration

## 🍳 Cookbook - Ready-to-use Code

### 🌐 Complete Website Gateway

```go
package main

import (
    "fmt"
    "html/template"
    "net/http"
    "path/filepath"
    "strings"
    "time"

    gateway "github.com/your-org/boxo-starter-kit/07-gateway/pkg"
    dag "github.com/your-org/boxo-starter-kit/02-dag-ipld/pkg"
)

// Complete website hosting gateway
type WebsiteGateway struct {
    *gateway.Gateway
    templates map[string]*template.Template
    config    WebsiteConfig
}

type WebsiteConfig struct {
    DefaultIndex   string            // Default index file
    ErrorPages     map[int]string    // Error page mappings
    RedirectRules  map[string]string // Redirect rules
    CustomDomains  map[string]string // Domain to IPFS hash mapping
}

func NewWebsiteGateway(dagWrapper *dag.DagWrapper) (*WebsiteGateway, error) {
    baseGateway, err := gateway.NewGateway(dagWrapper, gateway.GatewayConfig{
        Port:         ":8080",
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
    })
    if err != nil {
        return nil, err
    }

    return &WebsiteGateway{
        Gateway:   baseGateway,
        templates: make(map[string]*template.Template),
        config: WebsiteConfig{
            DefaultIndex: "index.html",
            ErrorPages: map[int]string{
                404: "404.html",
                500: "500.html",
            },
            RedirectRules:  make(map[string]string),
            CustomDomains: make(map[string]string),
        },
    }, nil
}

// Custom routing for websites
func (wg *WebsiteGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Handle custom domains
    if wg.handleCustomDomain(w, r) {
        return
    }

    // 2. Apply redirect rules
    if wg.applyRedirectRules(w, r) {
        return
    }

    // 3. Handle special routes
    switch r.URL.Path {
    case "/health":
        wg.handleHealthCheck(w, r)
        return
    case "/metrics":
        wg.handleMetrics(w, r)
        return
    }

    // 4. Handle regular IPFS content
    wg.Gateway.ServeHTTP(w, r)
}

func (wg *WebsiteGateway) handleCustomDomain(w http.ResponseWriter, r *http.Request) bool {
    host := strings.Split(r.Host, ":")[0] // Remove port

    if ipfsHash, exists := wg.config.CustomDomains[host]; exists {
        // Rewrite request to point to IPFS content
        originalPath := r.URL.Path
        if originalPath == "/" {
            originalPath = "/" + wg.config.DefaultIndex
        }

        r.URL.Path = "/ipfs/" + ipfsHash + originalPath
        wg.Gateway.ServeHTTP(w, r)
        return true
    }

    return false
}

func (wg *WebsiteGateway) applyRedirectRules(w http.ResponseWriter, r *http.Request) bool {
    if target, exists := wg.config.RedirectRules[r.URL.Path]; exists {
        http.Redirect(w, r, target, http.StatusMovedPermanently)
        return true
    }

    return false
}

// Enhanced error page handling
func (wg *WebsiteGateway) handleError(w http.ResponseWriter, r *http.Request,
                                     statusCode int, err error) {
    if errorPage, exists := wg.config.ErrorPages[statusCode]; exists {
        // Try to serve custom error page from IPFS
        if wg.serveCustomErrorPage(w, r, errorPage, statusCode) {
            return
        }
    }

    // Fall back to default error handling
    wg.serveDefaultError(w, r, statusCode, err)
}

func (wg *WebsiteGateway) serveCustomErrorPage(w http.ResponseWriter, r *http.Request,
                                              errorPage string, statusCode int) bool {
    // Try to get error page from the same site
    if referer := r.Header.Get("Referer"); referer != "" {
        // Extract IPFS hash from referer
        if ipfsHash := wg.extractIPFSHash(referer); ipfsHash != "" {
            errorPath := "/ipfs/" + ipfsHash + "/" + errorPage

            // Create new request for error page
            errorReq := &http.Request{
                Method: "GET",
                URL:    &url.URL{Path: errorPath},
                Header: make(http.Header),
            }
            errorReq = errorReq.WithContext(r.Context())

            // Try to serve error page
            recorder := httptest.NewRecorder()
            wg.Gateway.ServeHTTP(recorder, errorReq)

            if recorder.Code == 200 {
                // Copy headers and content
                for k, v := range recorder.Header() {
                    w.Header()[k] = v
                }
                w.WriteHeader(statusCode)
                w.Write(recorder.Body.Bytes())
                return true
            }
        }
    }

    return false
}

// Health check endpoint
func (wg *WebsiteGateway) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now().Unix(),
        "version":   "1.0.0",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

// Metrics endpoint for monitoring
func (wg *WebsiteGateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := wg.collectMetrics()

    w.Header().Set("Content-Type", "text/plain")
    for key, value := range metrics {
        fmt.Fprintf(w, "%s %v\n", key, value)
    }
}

// Register a website with custom domain
func (wg *WebsiteGateway) RegisterWebsite(domain, ipfsHash string) {
    wg.config.CustomDomains[domain] = ipfsHash
    log.Printf("Registered website: %s -> %s", domain, ipfsHash)
}

// Add redirect rule
func (wg *WebsiteGateway) AddRedirectRule(from, to string) {
    wg.config.RedirectRules[from] = to
    log.Printf("Added redirect rule: %s -> %s", from, to)
}

func main() {
    // Initialize DAG wrapper
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        panic(err)
    }

    // Create website gateway
    websiteGateway, err := NewWebsiteGateway(dagWrapper)
    if err != nil {
        panic(err)
    }

    // Register sample websites
    websiteGateway.RegisterWebsite("example.com", "QmExampleSiteHash123")
    websiteGateway.RegisterWebsite("blog.example.com", "QmBlogSiteHash456")

    // Add redirect rules
    websiteGateway.AddRedirectRule("/old-path", "/new-path")
    websiteGateway.AddRedirectRule("/docs", "/ipfs/QmDocsHash789")

    // Start server
    fmt.Println("🌐 Website Gateway starting on :8080")
    fmt.Println("📝 Access websites:")
    fmt.Println("   - http://example.com:8080")
    fmt.Println("   - http://blog.example.com:8080")
    fmt.Println("   - http://localhost:8080/ipfs/{hash}")

    server := &http.Server{
        Addr:    ":8080",
        Handler: websiteGateway,

        // Production-ready timeouts
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    if err := server.ListenAndServe(); err != nil {
        log.Fatal("Server failed to start:", err)
    }
}
```

Now you have a complete understanding of HTTP Gateway implementation and web integration! 🌐