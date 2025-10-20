# IPNI Production Deployment Guide

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Deployment](#deployment)
5. [Monitoring](#monitoring)
6. [Maintenance](#maintenance)
7. [Troubleshooting](#troubleshooting)
8. [Security Checklist](#security-checklist)

## Prerequisites

### System Requirements
- **OS**: Linux (Ubuntu 20.04+ or CentOS 8+)
- **CPU**: 4+ cores (8+ recommended)
- **RAM**: 8GB minimum (16GB+ recommended)
- **Storage**: 100GB+ SSD
- **Network**: 1Gbps+ connection

### Software Dependencies
```bash
# Go 1.20+
wget https://go.dev/dl/go1.20.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Required packages
apt-get update
apt-get install -y build-essential git curl wget

# Docker (optional, for containerized deployment)
curl -fsSL https://get.docker.com | sh
```

## Installation

### 1. Clone and Build
```bash
# Clone repository
git clone https://github.com/gosuda/boxo-starter-kit.git
cd boxo-starter-kit/17-ipni

# Install dependencies
go mod download

# Build binary
CGO_ENABLED=1 go build -o ipni-node ./cmd/ipni

# Verify build
./ipni-node --version
```

### 2. Create System User
```bash
# Create dedicated user
useradd -r -s /bin/false ipni

# Create directories
mkdir -p /var/lib/ipni/{data,cache,logs}
mkdir -p /etc/ipni
chown -R ipni:ipni /var/lib/ipni
```

### 3. Install as Service
```bash
# Copy binary
cp ipni-node /usr/local/bin/
chmod +x /usr/local/bin/ipni-node

# Copy configuration
cp config.yaml /etc/ipni/
chmod 640 /etc/ipni/config.yaml
chown root:ipni /etc/ipni/config.yaml
```

### 4. Create Systemd Service
```ini
# /etc/systemd/system/ipni.service
[Unit]
Description=IPNI Node Service
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=ipni
Group=ipni
WorkingDirectory=/var/lib/ipni
ExecStart=/usr/local/bin/ipni-node --config=/etc/ipni/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
KillSignal=SIGTERM
Restart=always
RestartSec=5
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ipni

[Install]
WantedBy=multi-user.target
```

Enable and start service:
```bash
systemctl daemon-reload
systemctl enable ipni
systemctl start ipni
systemctl status ipni
```

## Configuration

### Environment-Specific Settings

#### Development
```yaml
# config-dev.yaml
ipni:
  enable_health_monitoring: true
  health_check_interval: "1m"

network:
  listen_addresses:
    - "/ip4/127.0.0.1/tcp/4001"

storage:
  backend: "memory"

logging:
  level: "debug"
  format: "text"
```

#### Production
```yaml
# config-prod.yaml
ipni:
  enable_health_monitoring: true
  health_check_interval: "5m"
  max_index_size: 10737418240  # 10GB

network:
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
    - "/ip6/::/tcp/4001"

storage:
  backend: "badger"
  badger:
    sync_writes: true
    compression: "snappy"

security:
  require_signatures: true
  min_trust_level: "medium"

logging:
  level: "info"
  format: "json"
  output: "file"
```

### Environment Variables
```bash
# /etc/ipni/ipni.env
IPNI_CONFIG=/etc/ipni/config.yaml
IPNI_DATA_DIR=/var/lib/ipni/data
IPNI_LOG_LEVEL=info
IPNI_METRICS_PORT=9090
IPNI_HEALTH_PORT=8080
GOMAXPROCS=8
```

## Deployment

### Docker Deployment

#### Dockerfile
```dockerfile
FROM golang:1.20-alpine AS builder

RUN apk add --no-cache build-base git

WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=1 go build -o ipni-node ./cmd/ipni

FROM alpine:latest

RUN apk add --no-cache ca-certificates
RUN adduser -D -u 1000 ipni

COPY --from=builder /build/ipni-node /usr/local/bin/
COPY config.yaml /etc/ipni/

USER ipni
WORKDIR /var/lib/ipni

EXPOSE 4001 8080 9090

ENTRYPOINT ["/usr/local/bin/ipni-node"]
CMD ["--config=/etc/ipni/config.yaml"]
```

#### Docker Compose
```yaml
version: '3.8'

services:
  ipni:
    build: .
    container_name: ipni-node
    ports:
      - "4001:4001"    # P2P
      - "8080:8080"    # Health
      - "9090:9090"    # Metrics
    volumes:
      - ipni-data:/var/lib/ipni/data
      - ipni-cache:/var/lib/ipni/cache
      - ./config.yaml:/etc/ipni/config.yaml:ro
    environment:
      - IPNI_LOG_LEVEL=info
      - GOMAXPROCS=4
    restart: unless-stopped
    networks:
      - ipni-network

volumes:
  ipni-data:
  ipni-cache:

networks:
  ipni-network:
    driver: bridge
```

### Kubernetes Deployment

#### Deployment YAML
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ipni
  namespace: ipni-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ipni
  template:
    metadata:
      labels:
        app: ipni
    spec:
      containers:
      - name: ipni
        image: ipni:latest
        ports:
        - containerPort: 4001
          name: p2p
        - containerPort: 8080
          name: health
        - containerPort: 9090
          name: metrics
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
          limits:
            cpu: "4"
            memory: "8Gi"
        livenessProbe:
          httpGet:
            path: /live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        volumeMounts:
        - name: data
          mountPath: /var/lib/ipni/data
        - name: config
          mountPath: /etc/ipni
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: ipni-data
      - name: config
        configMap:
          name: ipni-config
```

## Monitoring

### Prometheus Configuration
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'ipni'
    static_configs:
      - targets: ['ipni-node:9090']
    scrape_interval: 30s
```

### Grafana Dashboard
Import the provided dashboard JSON from `monitoring/grafana-dashboard.json`

Key metrics to monitor:
- Query latency (p50, p95, p99)
- Cache hit rate
- Provider count
- Index size
- Error rate
- Memory usage
- CPU usage
- Network I/O

### Alerts
```yaml
# alerts.yml
groups:
  - name: ipni
    rules:
    - alert: IPNIHighErrorRate
      expr: rate(ipni_errors_total[5m]) > 0.05
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: High error rate detected

    - alert: IPNILowCacheHitRate
      expr: ipni_cache_hit_rate{cache_level="l1"} < 0.3
      for: 10m
      labels:
        severity: info
      annotations:
        summary: L1 cache hit rate is low

    - alert: IPNIHighMemoryUsage
      expr: ipni_memory_usage_bytes{component="total"} > 8589934592
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: Memory usage exceeds 8GB
```

## Maintenance

### Backup
```bash
#!/bin/bash
# backup.sh
BACKUP_DIR="/backup/ipni"
DATE=$(date +%Y%m%d_%H%M%S)

# Stop service
systemctl stop ipni

# Backup data
tar -czf $BACKUP_DIR/ipni-data-$DATE.tar.gz /var/lib/ipni/data

# Backup config
cp /etc/ipni/config.yaml $BACKUP_DIR/config-$DATE.yaml

# Start service
systemctl start ipni

# Cleanup old backups (keep 30 days)
find $BACKUP_DIR -type f -mtime +30 -delete
```

### Update Procedure
1. Build new version
2. Test in staging environment
3. Create backup
4. Deploy using rolling update
5. Monitor metrics
6. Rollback if needed

### Log Rotation
```bash
# /etc/logrotate.d/ipni
/var/log/ipni/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 640 ipni ipni
    sharedscripts
    postrotate
        systemctl reload ipni
    endscript
}
```

## Troubleshooting

### Common Issues

#### High Memory Usage
```bash
# Check memory usage
ps aux | grep ipni
cat /proc/$(pgrep ipni)/status | grep VmRSS

# Force garbage collection
kill -USR1 $(pgrep ipni)
```

#### Connection Issues
```bash
# Check listening ports
ss -tlnp | grep ipni

# Check firewall
iptables -L -n | grep 4001

# Test connectivity
nc -zv localhost 4001
```

#### Performance Issues
```bash
# Generate CPU profile
curl http://localhost:9090/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Generate heap profile
curl http://localhost:9090/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

### Debug Mode
```bash
# Run with debug logging
IPNI_LOG_LEVEL=debug ipni-node --config=/etc/ipni/config.yaml

# Enable verbose output
ipni-node --verbose --debug --config=/etc/ipni/config.yaml
```

## Security Checklist

### Pre-Deployment
- [ ] Generate strong cryptographic keys
- [ ] Configure firewall rules
- [ ] Set up TLS certificates
- [ ] Enable signature verification
- [ ] Configure rate limiting
- [ ] Set appropriate file permissions

### Network Security
```bash
# Firewall rules (iptables)
iptables -A INPUT -p tcp --dport 4001 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 9090 -s 10.0.0.0/8 -j ACCEPT
```

### File Permissions
```bash
chmod 600 /etc/ipni/config.yaml
chmod 700 /var/lib/ipni/data
chown -R ipni:ipni /var/lib/ipni
```

### Secrets Management
- Use environment variables for sensitive data
- Consider using HashiCorp Vault or similar
- Rotate keys regularly
- Never commit secrets to version control

## Performance Tuning

### System Tuning
```bash
# /etc/sysctl.d/99-ipni.conf
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 134217728
net.ipv4.tcp_wmem = 4096 65536 134217728
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_congestion_control = bbr
fs.file-max = 2097152
```

### Go Runtime
```bash
# Environment variables
export GOGC=100
export GOMEMLIMIT=8GiB
export GOMAXPROCS=8
```

## Production Readiness Checklist

### Infrastructure
- [ ] High availability setup (3+ nodes)
- [ ] Load balancer configured
- [ ] Persistent storage provisioned
- [ ] Backup strategy implemented
- [ ] Disaster recovery plan

### Monitoring
- [ ] Metrics collection enabled
- [ ] Alerting configured
- [ ] Log aggregation setup
- [ ] Distributed tracing (optional)
- [ ] Dashboard created

### Security
- [ ] Security audit completed
- [ ] Penetration testing performed
- [ ] Compliance requirements met
- [ ] Incident response plan
- [ ] Regular security updates

### Performance
- [ ] Load testing completed
- [ ] Benchmarks established
- [ ] Caching optimized
- [ ] Database indexes created
- [ ] Query optimization

### Documentation
- [ ] Runbook created
- [ ] API documentation
- [ ] Architecture diagram
- [ ] Network topology
- [ ] Contact information

## Support

For production support:
- GitHub Issues: https://github.com/gosuda/boxo-starter-kit/issues
- Documentation: https://docs.ipni.io
- Community: https://discuss.ipfs.io

## License

See LICENSE file for details.