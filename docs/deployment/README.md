# Production Deployment Guide

This guide provides comprehensive instructions for deploying boxo-based IPFS applications in production environments.

## 📋 Table of Contents

1. [Infrastructure Requirements](#infrastructure-requirements)
2. [Security Configuration](#security-configuration)
3. [Performance Optimization](#performance-optimization)
4. [Monitoring and Observability](#monitoring-and-observability)
5. [High Availability Setup](#high-availability-setup)
6. [Backup and Disaster Recovery](#backup-and-disaster-recovery)
7. [Container Deployment](#container-deployment)
8. [Cloud Platform Guides](#cloud-platform-guides)
9. [Troubleshooting](#troubleshooting)

## Infrastructure Requirements

### Minimum Hardware Requirements

#### Single Node Deployment
- **CPU**: 4 cores (2.4GHz+)
- **RAM**: 8GB minimum, 16GB recommended
- **Storage**: 100GB SSD minimum
- **Network**: 1Gbps connection
- **OS**: Linux (Ubuntu 20.04+, CentOS 8+, or RHEL 8+)

#### High Availability Cluster
- **Nodes**: 3+ nodes minimum
- **CPU**: 8 cores per node (2.4GHz+)
- **RAM**: 32GB per node
- **Storage**: 500GB SSD per node
- **Network**: 10Gbps connection with redundancy

### Storage Considerations

```yaml
# Storage Layout Example
/opt/ipfs/
├── data/          # Primary datastore (SSD required)
├── blocks/        # Block storage (SSD recommended)
├── logs/          # Application logs
├── backups/       # Local backup storage
└── temp/          # Temporary files
```

### Network Requirements

- **Ports**: 4001 (libp2p), 5001 (API), 8080 (Gateway)
- **Firewall**: Configure appropriate rules
- **Load Balancer**: For multi-node deployments
- **DNS**: Configure for service discovery

## Security Configuration

### 1. API Security

```go
// Example secure API configuration
config := &gateway.GatewayConfig{
    APIConfig: gateway.APIConfig{
        EnableAuth:     true,
        AllowedOrigins: []string{"https://your-domain.com"},
        RateLimit: gateway.RateLimitConfig{
            RequestsPerSecond: 100,
            BurstSize:        200,
        },
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
}
```

### 2. Access Control

```yaml
# access-control.yml
api_access:
  read_only:
    - "192.168.1.0/24"  # Internal network
  admin:
    - "10.0.0.1"        # Admin host only

gateway_access:
  rate_limits:
    default: 1000/hour
    premium: 10000/hour

blocked_content:
  - "content_hash_1"
  - "content_hash_2"
```

### 3. TLS/SSL Configuration

```bash
# Generate certificates
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Or use Let's Encrypt
certbot certonly --standalone -d your-ipfs-node.com
```

### 4. Firewall Rules

```bash
# UFW (Ubuntu)
sudo ufw allow 22/tcp     # SSH
sudo ufw allow 4001/tcp   # libp2p
sudo ufw allow 5001/tcp   # API (restrict to admin network)
sudo ufw allow 8080/tcp   # Gateway
sudo ufw enable

# iptables
iptables -A INPUT -p tcp --dport 4001 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
iptables -A INPUT -p tcp --dport 5001 -s 10.0.0.0/8 -j ACCEPT
```

## Performance Optimization

### 1. System Tuning

```bash
# /etc/sysctl.conf optimizations
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 65536 134217728
net.ipv4.tcp_wmem = 4096 65536 134217728
net.core.netdev_max_backlog = 5000
fs.file-max = 2097152

# Apply changes
sysctl -p
```

### 2. Application Configuration

```go
// High-performance configuration
config := &app.Config{
    Datastore: &persistent.DatastoreConfig{
        Type: "badger",
        Options: map[string]interface{}{
            "num_memtables":     4,
            "mem_table_size":    64 << 20, // 64MB
            "num_level_zero":    8,
            "value_threshold":   1024,
            "sync_writes":       false, // Async for performance
        },
    },

    Network: &networking.OptimizationConfig{
        EnableConnectionPool:   true,
        EnableMessageBatching:  true,
        EnableBandwidthManager: true,
        EnableAdaptiveTimeouts: true,

        ConnectionPool: networking.ConnectionPoolConfig{
            MaxConnections: 1000,
            MaxPerPeer:    5,
            IdleTimeout:   30 * time.Second,
        },

        MessageBatching: networking.BatchingConfig{
            MaxBatchSize:     100,
            MaxBatchBytes:    64 * 1024,
            BatchTimeout:     10 * time.Millisecond,
            CompressionLevel: 6,
        },
    },

    Gateway: &gateway.GatewayConfig{
        CacheSize:     1000000, // 1M entries
        CacheTTL:      24 * time.Hour,
        MaxRequestSize: 32 << 20, // 32MB
        Timeout:       30 * time.Second,
    },
}
```

### 3. Resource Limits

```yaml
# systemd service file
[Unit]
Description=IPFS Node
After=network.target

[Service]
Type=exec
User=ipfs
Group=ipfs
ExecStart=/opt/ipfs/bin/ipfs-node
Restart=always
RestartSec=5

# Resource limits
LimitNOFILE=65536
LimitNPROC=32768
MemoryMax=16G
CPUQuota=800%

[Install]
WantedBy=multi-user.target
```

## Monitoring and Observability

### 1. Metrics Collection

```go
// Prometheus metrics integration
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func setupMetrics() {
    // Custom metrics
    nodeRequests := prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ipfs_requests_total",
            Help: "Total number of IPFS requests",
        },
        []string{"method", "status"},
    )

    blockSize := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ipfs_block_size_bytes",
            Help:    "Size of IPFS blocks",
            Buckets: prometheus.ExponentialBuckets(1024, 2, 10),
        },
        []string{"type"},
    )

    prometheus.MustRegister(nodeRequests, blockSize)

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(":9090", nil)
}
```

### 2. Health Checks

```go
// Health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
    health := map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now(),
        "checks": map[string]bool{
            "datastore": checkDatastore(),
            "network":   checkNetwork(),
            "gateway":   checkGateway(),
        },
    }

    json.NewEncoder(w).Encode(health)
}

func checkDatastore() bool {
    // Implement datastore health check
    return true
}
```

### 3. Logging Configuration

```yaml
# logging.yml
logging:
  level: "info"
  format: "json"
  outputs:
    - type: "file"
      path: "/var/log/ipfs/app.log"
      max_size: "100MB"
      max_backups: 10
    - type: "syslog"
      facility: "local0"
    - type: "stdout"

  loggers:
    networking: "debug"
    datastore: "info"
    gateway: "warn"
```

## High Availability Setup

### 1. Load Balancer Configuration

```nginx
# nginx.conf
upstream ipfs_gateway {
    least_conn;
    server 10.0.1.10:8080 max_fails=3 fail_timeout=30s;
    server 10.0.1.11:8080 max_fails=3 fail_timeout=30s;
    server 10.0.1.12:8080 max_fails=3 fail_timeout=30s;
}

upstream ipfs_api {
    ip_hash;  # Sticky sessions for API
    server 10.0.1.10:5001;
    server 10.0.1.11:5001;
    server 10.0.1.12:5001;
}

server {
    listen 80;
    server_name ipfs.example.com;

    location / {
        proxy_pass http://ipfs_gateway;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_cache ipfs_cache;
        proxy_cache_valid 200 1h;
    }

    location /api/v0/ {
        proxy_pass http://ipfs_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 2. Database Clustering

```go
// Distributed datastore configuration
config := &cluster.Config{
    Nodes: []string{
        "node1.ipfs.cluster:4001",
        "node2.ipfs.cluster:4001",
        "node3.ipfs.cluster:4001",
    },
    ReplicationFactor: 3,
    ConsistencyLevel:  cluster.ConsistencyQuorum,

    Datastore: &distributed.Config{
        ShardCount:       256,
        RebalanceEnabled: true,
        BackupEnabled:    true,
    },
}
```

### 3. Service Discovery

```yaml
# consul-service.yml
service:
  name: "ipfs-node"
  tags: ["ipfs", "gateway", "api"]
  port: 8080

  checks:
    - name: "Gateway Health"
      http: "http://localhost:8080/health"
      interval: "30s"
      timeout: "5s"

    - name: "API Health"
      http: "http://localhost:5001/api/v0/version"
      interval: "60s"
      timeout: "10s"
```

## Backup and Disaster Recovery

### 1. Automated Backup Strategy

```go
// Production backup configuration
backupConfig := &backup.SchedulerConfig{
    DefaultBackupDir: "/opt/ipfs/backups",
    RetentionPolicy: backup.RetentionPolicy{
        KeepDaily:   30,
        KeepWeekly:  12,
        KeepMonthly: 24,
        KeepYearly:  7,
        MaxAge:      2 * 365 * 24 * time.Hour, // 2 years
    },
    ConcurrentBackups: 2,
    HealthCheckInterval: 1 * time.Hour,
    NotificationConfig: backup.NotificationConfig{
        EmailOnFailure: true,
        Recipients:     []string{"ops@company.com"},
        WebhookURL:     "https://hooks.slack.com/...",
    },
}

scheduler := backup.NewBackupScheduler(backupConfig)

// Critical data backup (every 6 hours)
criticalBackup := &backup.ScheduledBackup{
    ID:       "critical-data",
    Schedule: "0 */6 * * *",
    Config: backup.BackupConfig{
        CompressionLevel: 9,
        VerifyIntegrity:  true,
        ExcludePatterns:  []string{"/temp/*", "/cache/*"},
    },
}

scheduler.AddSchedule(criticalBackup)
```

### 2. Disaster Recovery Plan

```bash
#!/bin/bash
# disaster-recovery.sh

set -e

echo "Starting disaster recovery procedure..."

# 1. Stop all services
systemctl stop ipfs-node
systemctl stop nginx

# 2. Restore from latest backup
LATEST_BACKUP=$(ls -t /opt/ipfs/backups/*.tar.gz | head -n1)
echo "Restoring from: $LATEST_BACKUP"

# 3. Clear existing data
rm -rf /opt/ipfs/data/*

# 4. Restore backup
backup-tool -cmd=restore -backup="$LATEST_BACKUP" -datastore=/opt/ipfs/data

# 5. Verify restoration
backup-tool -cmd=verify -backup="$LATEST_BACKUP"

# 6. Start services
systemctl start ipfs-node
systemctl start nginx

# 7. Health check
sleep 30
curl -f http://localhost:8080/health || exit 1

echo "Disaster recovery completed successfully"
```

## Container Deployment

### 1. Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ipfs-node ./cmd/node

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -S ipfs && adduser -S ipfs -G ipfs

WORKDIR /opt/ipfs

COPY --from=builder /app/ipfs-node .
COPY --chown=ipfs:ipfs configs/ configs/

USER ipfs

EXPOSE 4001 5001 8080 9090

VOLUME ["/opt/ipfs/data", "/opt/ipfs/logs"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

CMD ["./ipfs-node", "-config", "configs/production.yml"]
```

### 2. Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  ipfs-node-1:
    build: .
    container_name: ipfs-node-1
    hostname: node1
    restart: unless-stopped
    ports:
      - "4001:4001"
      - "5001:5001"
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ipfs_data_1:/opt/ipfs/data
      - ipfs_logs_1:/opt/ipfs/logs
      - ./configs:/opt/ipfs/configs:ro
    environment:
      - IPFS_NODE_ID=node1
      - IPFS_CLUSTER_PEERS=node2:4001,node3:4001
    networks:
      - ipfs_network
    deploy:
      resources:
        limits:
          memory: 8G
          cpus: '4'
        reservations:
          memory: 4G
          cpus: '2'

  ipfs-node-2:
    build: .
    container_name: ipfs-node-2
    hostname: node2
    restart: unless-stopped
    ports:
      - "4002:4001"
      - "5002:5001"
      - "8081:8080"
      - "9091:9090"
    volumes:
      - ipfs_data_2:/opt/ipfs/data
      - ipfs_logs_2:/opt/ipfs/logs
      - ./configs:/opt/ipfs/configs:ro
    environment:
      - IPFS_NODE_ID=node2
      - IPFS_CLUSTER_PEERS=node1:4001,node3:4001
    networks:
      - ipfs_network

  ipfs-node-3:
    build: .
    container_name: ipfs-node-3
    hostname: node3
    restart: unless-stopped
    ports:
      - "4003:4001"
      - "5003:5001"
      - "8082:8080"
      - "9092:9090"
    volumes:
      - ipfs_data_3:/opt/ipfs/data
      - ipfs_logs_3:/opt/ipfs/logs
      - ./configs:/opt/ipfs/configs:ro
    environment:
      - IPFS_NODE_ID=node3
      - IPFS_CLUSTER_PEERS=node1:4001,node2:4001
    networks:
      - ipfs_network

  nginx:
    image: nginx:alpine
    container_name: ipfs-loadbalancer
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - ipfs-node-1
      - ipfs-node-2
      - ipfs-node-3
    networks:
      - ipfs_network

  prometheus:
    image: prom/prometheus
    container_name: ipfs-prometheus
    restart: unless-stopped
    ports:
      - "9093:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    networks:
      - ipfs_network

  grafana:
    image: grafana/grafana
    container_name: ipfs-grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=your_secure_password
    networks:
      - ipfs_network

volumes:
  ipfs_data_1:
  ipfs_data_2:
  ipfs_data_3:
  ipfs_logs_1:
  ipfs_logs_2:
  ipfs_logs_3:
  prometheus_data:
  grafana_data:

networks:
  ipfs_network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

### 3. Kubernetes Deployment

```yaml
# k8s-deployment.yml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ipfs-cluster
  namespace: ipfs
spec:
  serviceName: ipfs-headless
  replicas: 3
  selector:
    matchLabels:
      app: ipfs-node
  template:
    metadata:
      labels:
        app: ipfs-node
    spec:
      containers:
      - name: ipfs-node
        image: your-registry/ipfs-node:latest
        ports:
        - containerPort: 4001
          name: p2p
        - containerPort: 5001
          name: api
        - containerPort: 8080
          name: gateway
        - containerPort: 9090
          name: metrics
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        volumeMounts:
        - name: ipfs-data
          mountPath: /opt/ipfs/data
        - name: ipfs-config
          mountPath: /opt/ipfs/configs
        env:
        - name: IPFS_NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
      volumes:
      - name: ipfs-config
        configMap:
          name: ipfs-config
  volumeClaimTemplates:
  - metadata:
      name: ipfs-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 500Gi
      storageClassName: fast-ssd

---
apiVersion: v1
kind: Service
metadata:
  name: ipfs-gateway
  namespace: ipfs
spec:
  selector:
    app: ipfs-node
  ports:
  - port: 80
    targetPort: 8080
    name: gateway
  type: LoadBalancer

---
apiVersion: v1
kind: Service
metadata:
  name: ipfs-api
  namespace: ipfs
spec:
  selector:
    app: ipfs-node
  ports:
  - port: 5001
    targetPort: 5001
    name: api
  type: ClusterIP

---
apiVersion: v1
kind: Service
metadata:
  name: ipfs-headless
  namespace: ipfs
spec:
  clusterIP: None
  selector:
    app: ipfs-node
  ports:
  - port: 4001
    name: p2p
```

## Cloud Platform Guides

### AWS Deployment

```yaml
# aws-infrastructure.yml (CloudFormation)
AWSTemplateFormatVersion: '2010-09-09'
Description: 'IPFS Cluster Infrastructure'

Parameters:
  InstanceType:
    Type: String
    Default: m5.2xlarge
    Description: EC2 instance type for IPFS nodes

Resources:
  IPFSSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for IPFS nodes
      SecurityGroupIngress:
        - IpProtocol: tcp
          FromPort: 22
          ToPort: 22
          CidrIp: 0.0.0.0/0
        - IpProtocol: tcp
          FromPort: 4001
          ToPort: 4001
          CidrIp: 0.0.0.0/0
        - IpProtocol: tcp
          FromPort: 8080
          ToPort: 8080
          CidrIp: 0.0.0.0/0

  IPFSLaunchTemplate:
    Type: AWS::EC2::LaunchTemplate
    Properties:
      LaunchTemplateName: ipfs-cluster-template
      LaunchTemplateData:
        ImageId: ami-0abcdef1234567890  # Ubuntu 20.04 LTS
        InstanceType: !Ref InstanceType
        SecurityGroupIds:
          - !Ref IPFSSecurityGroup
        UserData:
          Fn::Base64: !Sub |
            #!/bin/bash
            apt-get update
            apt-get install -y docker.io docker-compose
            systemctl start docker
            systemctl enable docker

            # Download and setup IPFS node
            wget https://github.com/your-org/ipfs-node/releases/latest/download/ipfs-node
            chmod +x ipfs-node
            mv ipfs-node /usr/local/bin/

            # Create systemd service
            cat > /etc/systemd/system/ipfs-node.service << EOF
            [Unit]
            Description=IPFS Node
            After=network.target

            [Service]
            Type=exec
            User=ubuntu
            ExecStart=/usr/local/bin/ipfs-node
            Restart=always

            [Install]
            WantedBy=multi-user.target
            EOF

            systemctl enable ipfs-node
            systemctl start ipfs-node

  IPFSAutoScalingGroup:
    Type: AWS::AutoScaling::AutoScalingGroup
    Properties:
      MinSize: 3
      MaxSize: 10
      DesiredCapacity: 3
      LaunchTemplate:
        LaunchTemplateId: !Ref IPFSLaunchTemplate
        Version: !GetAtt IPFSLaunchTemplate.LatestVersionNumber
      AvailabilityZones:
        - us-west-2a
        - us-west-2b
        - us-west-2c

  IPFSApplicationLoadBalancer:
    Type: AWS::ElasticLoadBalancingV2::LoadBalancer
    Properties:
      Type: application
      Scheme: internet-facing
      SecurityGroups:
        - !Ref IPFSSecurityGroup
      Subnets:
        - subnet-12345678
        - subnet-87654321
```

### Google Cloud Platform

```yaml
# gcp-deployment.yml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ipfs-config
data:
  config.yml: |
    api:
      address: "0.0.0.0:5001"
    gateway:
      address: "0.0.0.0:8080"
    cluster:
      enabled: true
      discovery: "gke"

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ipfs-cluster
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ipfs
  template:
    metadata:
      labels:
        app: ipfs
    spec:
      containers:
      - name: ipfs
        image: gcr.io/your-project/ipfs-node:latest
        ports:
        - containerPort: 4001
        - containerPort: 5001
        - containerPort: 8080
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        volumeMounts:
        - name: ipfs-storage
          mountPath: /opt/ipfs/data
        - name: config
          mountPath: /opt/ipfs/configs
      volumes:
      - name: config
        configMap:
          name: ipfs-config
      - name: ipfs-storage
        persistentVolumeClaim:
          claimName: ipfs-pvc

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ipfs-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Gi
  storageClassName: fast-ssd
```

### Azure Deployment

```yaml
# azure-container-instance.yml
apiVersion: '2019-12-01'
location: eastus
name: ipfs-cluster
properties:
  containers:
  - name: ipfs-node-1
    properties:
      image: your-registry.azurecr.io/ipfs-node:latest
      resources:
        requests:
          cpu: 2
          memoryInGb: 8
      ports:
      - port: 4001
        protocol: TCP
      - port: 5001
        protocol: TCP
      - port: 8080
        protocol: TCP
      volumeMounts:
      - name: ipfs-storage
        mountPath: /opt/ipfs/data
      environmentVariables:
      - name: IPFS_NODE_ID
        value: node1
  volumes:
  - name: ipfs-storage
    azureFile:
      shareName: ipfs-data
      storageAccountName: your-storage-account
      storageAccountKey: your-storage-key
  osType: Linux
  restartPolicy: Always
  ipAddress:
    type: Public
    ports:
    - protocol: TCP
      port: 8080
    dnsNameLabel: ipfs-cluster
type: Microsoft.ContainerInstance/containerGroups
```

## Troubleshooting

### Common Issues

#### 1. High Memory Usage

```bash
# Check memory usage
free -h
ps aux --sort=-%mem | head

# Optimize datastore settings
echo "Reduce cache sizes in datastore configuration"
echo "Consider using different datastore backend"
```

#### 2. Network Connectivity Issues

```bash
# Check peer connections
curl http://localhost:5001/api/v0/swarm/peers | jq '.Peers | length'

# Test connectivity
telnet peer-address 4001

# Check firewall
ufw status
iptables -L
```

#### 3. Performance Issues

```bash
# Monitor I/O
iostat -x 1

# Check disk usage
df -h
du -sh /opt/ipfs/data/*

# Monitor network
iftop
netstat -i
```

#### 4. Service Failures

```bash
# Check service status
systemctl status ipfs-node

# View logs
journalctl -u ipfs-node -f

# Check configuration
ipfs-node -config /opt/ipfs/configs/production.yml -validate
```

### Log Analysis

```bash
# Common log patterns to watch for
grep -i error /var/log/ipfs/app.log
grep -i "connection refused" /var/log/ipfs/app.log
grep -i "timeout" /var/log/ipfs/app.log

# Performance metrics
grep "request_duration" /var/log/ipfs/app.log | tail -100
```

### Recovery Procedures

```bash
#!/bin/bash
# recovery-checklist.sh

echo "=== IPFS Node Recovery Checklist ==="

echo "1. Check service status"
systemctl is-active ipfs-node

echo "2. Check disk space"
df -h /opt/ipfs

echo "3. Check memory usage"
free -h

echo "4. Check network connectivity"
curl -f http://localhost:8080/health

echo "5. Check recent logs"
journalctl -u ipfs-node --since "1 hour ago" | tail -20

echo "6. Validate configuration"
ipfs-node -validate-config

echo "Recovery checklist completed"
```

## Support and Maintenance

### Regular Maintenance Tasks

1. **Daily**: Check service health, monitor disk usage
2. **Weekly**: Review logs, update security patches
3. **Monthly**: Performance optimization, backup verification
4. **Quarterly**: Capacity planning, disaster recovery testing

### Upgrade Procedures

```bash
#!/bin/bash
# upgrade-procedure.sh

set -e

echo "Starting IPFS node upgrade..."

# 1. Create backup
backup-tool -cmd=backup -datastore=/opt/ipfs/data

# 2. Stop service
systemctl stop ipfs-node

# 3. Backup current binary
cp /usr/local/bin/ipfs-node /usr/local/bin/ipfs-node.backup

# 4. Download new version
wget https://releases.example.com/ipfs-node-v1.2.3
chmod +x ipfs-node-v1.2.3
mv ipfs-node-v1.2.3 /usr/local/bin/ipfs-node

# 5. Validate configuration
ipfs-node -validate-config

# 6. Start service
systemctl start ipfs-node

# 7. Health check
sleep 30
curl -f http://localhost:8080/health

echo "Upgrade completed successfully"
```

This comprehensive deployment guide covers all aspects of running boxo-based IPFS applications in production, from infrastructure setup to ongoing maintenance and troubleshooting.