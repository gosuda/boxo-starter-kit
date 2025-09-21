# Multi-stage build for production IPFS node
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags '-extldflags "-static" -X main.version='"$(git describe --tags --always)"' -X main.buildTime='"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'' \
    -o ipfs-node ./cmd/node

# Build backup tool
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags '-extldflags "-static"' \
    -o backup-tool ./cmd/backup-tool

# Production stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    jq \
    openssl \
    && rm -rf /var/cache/apk/*

# Create ipfs user and group
RUN addgroup -g 1000 -S ipfs && \
    adduser -u 1000 -S ipfs -G ipfs -h /opt/ipfs -s /bin/sh

# Set working directory
WORKDIR /opt/ipfs

# Copy binaries from builder
COPY --from=builder /app/ipfs-node /usr/local/bin/ipfs-node
COPY --from=builder /app/backup-tool /usr/local/bin/backup-tool

# Copy configuration templates
COPY --chown=ipfs:ipfs configs/ configs/
COPY --chown=ipfs:ipfs scripts/ scripts/

# Create necessary directories
RUN mkdir -p data logs backups temp && \
    chown -R ipfs:ipfs /opt/ipfs

# Create health check script
COPY --chown=ipfs:ipfs <<'EOF' /usr/local/bin/health-check.sh
#!/bin/sh
set -e

# Check if the main process is running
if ! pgrep -f ipfs-node > /dev/null; then
    echo "IPFS node process not running"
    exit 1
fi

# Check HTTP endpoints
if ! curl -f -s http://localhost:8080/health > /dev/null; then
    echo "Gateway health check failed"
    exit 1
fi

if ! curl -f -s http://localhost:5001/api/v0/version > /dev/null; then
    echo "API health check failed"
    exit 1
fi

# Check metrics endpoint
if ! curl -f -s http://localhost:9090/metrics > /dev/null; then
    echo "Metrics endpoint check failed"
    exit 1
fi

echo "All health checks passed"
exit 0
EOF

RUN chmod +x /usr/local/bin/health-check.sh

# Create startup script
COPY --chown=ipfs:ipfs <<'EOF' /usr/local/bin/start-ipfs.sh
#!/bin/sh
set -e

# Set default values for environment variables
IPFS_NODE_ID=${IPFS_NODE_ID:-$(hostname)}
IPFS_LOG_LEVEL=${IPFS_LOG_LEVEL:-info}
IPFS_CONFIG_FILE=${IPFS_CONFIG_FILE:-/opt/ipfs/configs/production.yml}

echo "Starting IPFS node..."
echo "Node ID: $IPFS_NODE_ID"
echo "Log Level: $IPFS_LOG_LEVEL"
echo "Config File: $IPFS_CONFIG_FILE"

# Ensure data directory exists and has correct permissions
mkdir -p /opt/ipfs/data /opt/ipfs/logs /opt/ipfs/backups
chown -R ipfs:ipfs /opt/ipfs/data /opt/ipfs/logs /opt/ipfs/backups

# Initialize datastore if it doesn't exist
if [ ! -f "/opt/ipfs/data/.initialized" ]; then
    echo "Initializing datastore..."
    su-exec ipfs touch /opt/ipfs/data/.initialized
fi

# Start the IPFS node
exec su-exec ipfs ipfs-node \
    -config="$IPFS_CONFIG_FILE" \
    -node-id="$IPFS_NODE_ID" \
    -log-level="$IPFS_LOG_LEVEL"
EOF

RUN chmod +x /usr/local/bin/start-ipfs.sh

# Install su-exec for proper user switching
RUN apk add --no-cache su-exec

# Switch to ipfs user
USER ipfs

# Expose ports
EXPOSE 4001 5001 8080 9090

# Create volumes
VOLUME ["/opt/ipfs/data", "/opt/ipfs/logs", "/opt/ipfs/backups"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD /usr/local/bin/health-check.sh

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/start-ipfs.sh"]

# Labels for metadata
LABEL maintainer="IPFS Team <team@ipfs.io>"
LABEL version="1.0.0"
LABEL description="Production-ready IPFS node built with boxo"
LABEL org.opencontainers.image.source="https://github.com/your-org/boxo-starter-kit"
LABEL org.opencontainers.image.documentation="https://docs.ipfs.io/"
LABEL org.opencontainers.image.licenses="MIT"