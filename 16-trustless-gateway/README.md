# 16-trustless-gateway: HTTP Gateway for Trustless IPFS Access

## 🎯 Learning Objectives

By the end of this module, you will understand:
- How trustless gateways provide HTTP access to IPFS content
- Building HTTP servers that proxy IPFS content without trust requirements
- Working with CAR (Content Addressable aRchive) fetchers for data retrieval
- Implementing retry mechanisms and fallback strategies
- Creating responsive web interfaces for IPFS data access
- Understanding the security model of trustless content delivery
- Performance optimization for gateway operations

## 📋 Prerequisites

- Completion of [06-unixfs-car](../06-unixfs-car) - CAR file operations
- Understanding of [00-block-cid](../00-block-cid) - Content addressing
- Familiarity with HTTP servers and web development
- Knowledge of IPFS gateway concepts and architecture
- Understanding of content verification and trustless systems
- Basic knowledge of HTML/CSS for web interfaces

## 🔑 Core Concepts

### Trustless Gateway Architecture

**Trustless Gateways** provide HTTP access to IPFS content without requiring trust in the gateway operator:
- **Content Verification**: All data is cryptographically verified using CIDs
- **Proxy Function**: Gateway fetches content from multiple upstream sources
- **HTTP Interface**: Standard HTTP/HTTPS access to IPFS content
- **No Trust Required**: Content integrity verified client-side
- **Fallback Support**: Multiple upstream gateways for reliability

### Key Components

#### 1. CAR Fetching
```go
// Remote CAR fetcher retrieves content from upstream gateways
fetcher := gateway.NewRemoteCarFetcher(urls, nil)

// Retry wrapper provides fault tolerance
retryFetcher := gateway.NewRetryCarFetcher(fetcher, 3)

// Backend processes CAR data for HTTP responses
backend := gateway.NewCarBackend(retryFetcher)
```

#### 2. HTTP Server
```go
// Gateway handler processes IPFS requests
handler := gateway.NewHandler(gateway.Config{}, backend)

// HTTP multiplexer routes requests
mux := http.NewServeMux()
mux.Handle("/ipfs/", handler)
mux.Handle("/ipns/", handler)
```

#### 3. Content Verification
- **CID Validation**: Verify content matches requested CID
- **Block Integrity**: Cryptographic hash verification
- **Path Resolution**: Navigate through IPLD structures
- **Content Type Detection**: Proper MIME type handling

### Trustless Model Benefits

#### Security
- **No Trusted Third Party**: Content verification prevents tampering
- **End-to-End Integrity**: Cryptographic guarantees from source to client
- **Censorship Resistance**: Multiple upstream sources prevent blocking
- **Audit Trail**: All content addressable and verifiable

#### Performance
- **Parallel Fetching**: Request from multiple upstreams simultaneously
- **Caching**: HTTP caching headers for efficient browser caching
- **Streaming**: Progressive content delivery for large files
- **Retry Logic**: Automatic fallback for failed requests

## 💻 Code Architecture

### Module Structure
```
16-trustless-gateway/
├── pkg/
│   └── trustless.go        # Gateway wrapper implementation
└── main.go                 # CLI server with configuration
```

### Core Components

#### GatewayWrapper
Main wrapper providing HTTP gateway functionality:

```go
type GatewayWrapper struct {
    port   int            // HTTP server port
    Server *http.Server   // HTTP server instance
}
```

**Key Methods:**
- `NewGatewayWrapper(port, urls)`: Create gateway with upstream URLs
- `Start()`: Start HTTP server
- `Close()`: Graceful shutdown

#### HTTP Request Flow
1. **Request Reception**: HTTP request for `/ipfs/{cid}` or `/ipns/{name}`
2. **CID Extraction**: Parse CID from URL path
3. **Content Fetching**: Retrieve CAR data from upstream gateways
4. **Verification**: Validate content against CID
5. **Response Generation**: Serve content with appropriate headers

## 🏃‍♂️ Usage Examples

### Basic Gateway Setup

```go
package main

import (
    "log"
    "time"

    trustless "github.com/gosuda/boxo-starter-kit/16-trustless-gateway/pkg"
)

func main() {
    // Configure upstream gateways
    upstreams := []string{
        "https://ipfs.io",
        "https://dweb.link",
        "https://gateway.pinata.cloud",
    }

    // Create gateway instance
    gateway, err := trustless.NewGatewayWrapper(8080, upstreams)
    if err != nil {
        log.Fatalf("Failed to create gateway: %v", err)
    }

    // Start server
    log.Println("🚀 Trustless gateway starting on :8080")
    log.Printf("   Upstreams: %v", upstreams)

    if err := gateway.Start(); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

### Custom Configuration

```go
import (
    "net/http"
    "time"
)

// Create gateway with custom HTTP server settings
func createCustomGateway() (*trustless.GatewayWrapper, error) {
    upstreams := []string{
        "https://ipfs.io",
        "https://dweb.link",
    }

    gateway, err := trustless.NewGatewayWrapper(8080, upstreams)
    if err != nil {
        return nil, err
    }

    // Customize server timeouts
    gateway.Server.ReadTimeout = 60 * time.Second
    gateway.Server.WriteTimeout = 60 * time.Second
    gateway.Server.IdleTimeout = 120 * time.Second

    // Add custom headers
    originalHandler := gateway.Server.Handler
    gateway.Server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add CORS headers
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

        // Add security headers
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")

        originalHandler.ServeHTTP(w, r)
    })

    return gateway, nil
}
```

### Production Deployment

```go
import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func runProductionGateway() {
    // Create gateway
    gateway, err := trustless.NewGatewayWrapper(8080, []string{
        "https://ipfs.io",
        "https://dweb.link",
    })
    if err != nil {
        log.Fatalf("Gateway creation failed: %v", err)
    }

    // Start server in goroutine
    go func() {
        log.Println("🚀 Production gateway starting...")
        if err := gateway.Start(); err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    // Graceful shutdown handling
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

    <-stop
    log.Println("🛑 Shutting down gracefully...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := gateway.Server.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    } else {
        log.Println("✅ Server stopped gracefully")
    }
}
```

### Health Check Integration

```go
import (
    "encoding/json"
    "net/http"
)

type HealthStatus struct {
    Status    string    `json:"status"`
    Timestamp time.Time `json:"timestamp"`
    Upstreams []string  `json:"upstreams"`
    Version   string    `json:"version"`
}

func addHealthEndpoint(gateway *trustless.GatewayWrapper, upstreams []string) {
    // Add health check endpoint
    mux := gateway.Server.Handler.(*http.ServeMux)

    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        health := HealthStatus{
            Status:    "healthy",
            Timestamp: time.Now(),
            Upstreams: upstreams,
            Version:   "1.0.0",
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(health)
    })

    mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
        // Check if gateway is ready to serve requests
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ready"))
    })
}
```

## 🏃‍♂️ Running the Gateway

### Command Line Usage
```bash
cd 16-trustless-gateway

# Build the gateway
go build -o trustless-gateway main.go

# Run with default settings
./trustless-gateway

# Run with custom port and upstreams
./trustless-gateway --port 9090 --upstream "https://ipfs.io,https://dweb.link"

# Run with custom configuration
./trustless-gateway -p 8080 -u "https://gateway.pinata.cloud"
```

### Docker Deployment
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o trustless-gateway main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/trustless-gateway .
EXPOSE 8080
CMD ["./trustless-gateway", "--port", "8080"]
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trustless-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: trustless-gateway
  template:
    metadata:
      labels:
        app: trustless-gateway
    spec:
      containers:
      - name: gateway
        image: trustless-gateway:latest
        ports:
        - containerPort: 8080
        args:
        - "--port=8080"
        - "--upstream=https://ipfs.io,https://dweb.link"
        resources:
          limits:
            memory: "512Mi"
            cpu: "500m"
          requests:
            memory: "256Mi"
            cpu: "250m"
---
apiVersion: v1
kind: Service
metadata:
  name: trustless-gateway-service
spec:
  selector:
    app: trustless-gateway
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

## 🧪 Testing the Gateway

### Manual Testing
```bash
# Test basic content retrieval
curl -v "http://localhost:8080/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG/readme"

# Test with custom headers
curl -H "Accept: application/json" \
     "http://localhost:8080/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"

# Test IPNS resolution
curl "http://localhost:8080/ipns/ipfs.io"

# Test health endpoint
curl "http://localhost:8080/health"
```

### Load Testing
```bash
# Install apache bench
sudo apt-get install apache2-utils

# Run load test
ab -n 1000 -c 10 "http://localhost:8080/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"

# Test with larger files
ab -n 100 -c 5 "http://localhost:8080/ipfs/QmSomelargeFileCID"
```

### Integration Testing
```go
func TestGatewayIntegration(t *testing.T) {
    // Start test gateway
    gateway, err := trustless.NewGatewayWrapper(0, []string{
        "https://ipfs.io",
    })
    require.NoError(t, err)

    // Start server in background
    go gateway.Start()
    defer gateway.Close()

    // Wait for server to start
    time.Sleep(100 * time.Millisecond)

    // Test content retrieval
    resp, err := http.Get("http://localhost:8080/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG/readme")
    require.NoError(t, err)
    defer resp.Body.Close()

    require.Equal(t, http.StatusOK, resp.StatusCode)

    content, err := io.ReadAll(resp.Body)
    require.NoError(t, err)
    require.Contains(t, string(content), "IPFS")
}
```

## 🔧 Configuration Options

### Environment Variables
```bash
# Server configuration
export GATEWAY_PORT=8080
export GATEWAY_UPSTREAMS="https://ipfs.io,https://dweb.link"

# Performance tuning
export GATEWAY_READ_TIMEOUT=60s
export GATEWAY_WRITE_TIMEOUT=60s
export GATEWAY_IDLE_TIMEOUT=120s

# Security settings
export GATEWAY_MAX_HEADER_BYTES=1048576
export GATEWAY_ENABLE_CORS=true
```

### Configuration File
```yaml
# gateway.yaml
server:
  port: 8080
  read_timeout: 60s
  write_timeout: 60s
  idle_timeout: 120s

upstreams:
  - https://ipfs.io
  - https://dweb.link
  - https://gateway.pinata.cloud

security:
  cors_enabled: true
  max_header_bytes: 1048576

logging:
  level: info
  format: json
```

## 🔍 Troubleshooting

### Common Issues

1. **Upstream Connection Failures**
   ```
   Error: failed to fetch from upstream
   Solution: Check upstream gateway availability and network connectivity
   ```

2. **Content Not Found**
   ```
   Error: 404 content not found
   Solution: Verify CID exists and is available on upstream gateways
   ```

3. **Timeout Issues**
   ```
   Error: request timeout
   Solution: Increase server timeouts or check upstream performance
   ```

4. **Memory Issues**
   ```
   Error: out of memory
   Solution: Implement streaming for large files and tune memory limits
   ```

### Performance Optimization

- **Upstream Selection**: Choose geographically close and reliable upstreams
- **Caching**: Implement HTTP caching headers for browser efficiency
- **Connection Pooling**: Reuse HTTP connections to upstreams
- **Compression**: Enable gzip compression for text content
- **Load Balancing**: Use multiple gateway instances behind a load balancer

### Monitoring and Metrics
```go
// Add prometheus metrics
import "github.com/prometheus/client_golang/prometheus"

var (
    requestCount = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gateway_requests_total",
            Help: "Total number of requests",
        },
        []string{"method", "endpoint", "status"},
    )

    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "gateway_request_duration_seconds",
            Help: "Request duration in seconds",
        },
        []string{"method", "endpoint"},
    )
)
```

## 📊 Performance Characteristics

### Gateway Throughput
- **Concurrent Requests**: Handle hundreds of simultaneous requests
- **Bandwidth**: Limited by upstream gateway capacity
- **Latency**: Adds minimal overhead to upstream response times
- **Caching**: HTTP caching reduces repeated upstream requests

### Resource Usage
- **Memory**: Typically 50-200MB depending on concurrent load
- **CPU**: Low CPU usage for proxying operations
- **Network**: Bandwidth scales with content size and request volume
- **Storage**: Minimal local storage (only for temporary operations)

## 📚 Next Steps

### Immediate Next Steps
With trustless gateway expertise, advance to comprehensive content discovery and optimization:

1. **[17-ipni](../17-ipni)**: Content Indexing and Discovery
   - Integrate trustless gateways with IPNI for intelligent content routing
   - Build scalable content discovery for gateway networks
   - Master provider selection for optimal gateway performance

2. **Advanced Integration Options**: Choose your focus:
   - **[18-multifetcher](../18-multifetcher)**: Multi-source optimization with trustless verification
   - **Production Deployment**: Enterprise gateway implementations

### Related Modules
**Prerequisites (Essential foundation):**
- [06-unixfs-car](../06-unixfs-car): CAR file operations and content verification
- [00-block-cid](../00-block-cid): Content addressing and verification fundamentals
- [10-gateway](../10-gateway): Basic HTTP gateway concepts (for comparison)

**Supporting Technologies:**
- [15-graphsync](../15-graphsync): Efficient data transfer protocols
- [14-traversal-selector](../14-traversal-selector): Advanced content selection
- [12-ipld-prime](../12-ipld-prime): High-performance IPLD operations

**Advanced Applications:**
- [17-ipni](../17-ipni): Content discovery and intelligent routing
- [18-multifetcher](../18-multifetcher): Multi-source content retrieval with verification
- Production CDN: Enterprise content delivery networks

### Alternative Learning Paths

**For Security-First Development:**
16-trustless-gateway → Cryptographic Verification Systems → Security Auditing

**For Performance Engineering:**
16-trustless-gateway → 18-multifetcher → 17-ipni → High-Performance CDN

**For Web3 Infrastructure:**
16-trustless-gateway → Decentralized Web Applications → Blockchain Integration

**For Enterprise Solutions:**
16-trustless-gateway → 17-ipni → Large-Scale Gateway Networks → CDN Implementation

## 📚 Further Reading

- [IPFS Gateway Specification](https://specs.ipfs.tech/http-gateways/)
- [Trustless Gateway Specification](https://specs.ipfs.tech/http-gateways/trustless-gateway/)
- [CAR Format Specification](https://ipld.io/specs/transport/car/)
- [IPFS HTTP Gateway Documentation](https://docs.ipfs.tech/concepts/ipfs-gateway/)
- [Go HTTP Server Best Practices](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)

---

This module demonstrates how to build production-ready HTTP gateways for trustless IPFS content access. Master these patterns to provide reliable, verifiable web access to distributed content.