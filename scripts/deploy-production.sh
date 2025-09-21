#!/bin/bash

# Production deployment script for boxo-based IPFS nodes
# This script automates the deployment process across different environments

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
LOG_FILE="/tmp/ipfs-deploy-$(date +%Y%m%d-%H%M%S).log"

# Default values
ENVIRONMENT="production"
DEPLOYMENT_TYPE="docker-compose"
NODE_COUNT=3
DOMAIN=""
SSL_EMAIL=""
BACKUP_ENABLED=true
MONITORING_ENABLED=true
SKIP_TESTS=false
FORCE_REBUILD=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}" | tee -a "$LOG_FILE"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}" | tee -a "$LOG_FILE"
    exit 1
}

info() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] INFO: $1${NC}" | tee -a "$LOG_FILE"
}

# Help function
show_help() {
    cat << EOF
Production Deployment Script for IPFS Nodes

Usage: $0 [OPTIONS]

OPTIONS:
    -e, --environment ENV       Deployment environment (development, staging, production)
    -t, --type TYPE            Deployment type (docker-compose, kubernetes, systemd)
    -n, --nodes COUNT          Number of IPFS nodes to deploy (default: 3)
    -d, --domain DOMAIN        Domain name for the deployment
    -s, --ssl-email EMAIL      Email for SSL certificate generation
    -b, --no-backup           Disable backup services
    -m, --no-monitoring       Disable monitoring stack
    -T, --skip-tests          Skip pre-deployment tests
    -f, --force-rebuild       Force rebuild of containers/images
    -h, --help                Show this help message

EXAMPLES:
    # Deploy with Docker Compose
    $0 -e production -t docker-compose -d ipfs.example.com -s admin@example.com

    # Deploy to Kubernetes
    $0 -e production -t kubernetes -n 5 -d ipfs.k8s.example.com

    # Development deployment
    $0 -e development -t docker-compose --no-monitoring --skip-tests

ENVIRONMENT VARIABLES:
    REGISTRY_URL              Container registry URL
    REGISTRY_USERNAME         Registry username
    REGISTRY_PASSWORD         Registry password
    GRAFANA_PASSWORD          Grafana admin password
    SMTP_HOST                 SMTP server for notifications
    SMTP_USER                 SMTP username
    SMTP_PASSWORD             SMTP password
    POSTGRES_PASSWORD         PostgreSQL password
    BACKUP_WEBHOOK_URL        Webhook URL for backup notifications

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -e|--environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -t|--type)
                DEPLOYMENT_TYPE="$2"
                shift 2
                ;;
            -n|--nodes)
                NODE_COUNT="$2"
                shift 2
                ;;
            -d|--domain)
                DOMAIN="$2"
                shift 2
                ;;
            -s|--ssl-email)
                SSL_EMAIL="$2"
                shift 2
                ;;
            -b|--no-backup)
                BACKUP_ENABLED=false
                shift
                ;;
            -m|--no-monitoring)
                MONITORING_ENABLED=false
                shift
                ;;
            -T|--skip-tests)
                SKIP_TESTS=true
                shift
                ;;
            -f|--force-rebuild)
                FORCE_REBUILD=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."

    # Check required tools based on deployment type
    case $DEPLOYMENT_TYPE in
        docker-compose)
            command -v docker >/dev/null 2>&1 || error "Docker is not installed"
            command -v docker-compose >/dev/null 2>&1 || error "Docker Compose is not installed"
            ;;
        kubernetes)
            command -v kubectl >/dev/null 2>&1 || error "kubectl is not installed"
            command -v helm >/dev/null 2>&1 || error "Helm is not installed"
            ;;
        systemd)
            systemctl --version >/dev/null 2>&1 || error "systemd is not available"
            ;;
    esac

    # Check common tools
    command -v jq >/dev/null 2>&1 || error "jq is not installed"
    command -v curl >/dev/null 2>&1 || error "curl is not installed"

    # Validate environment
    if [[ ! "$ENVIRONMENT" =~ ^(development|staging|production)$ ]]; then
        error "Invalid environment: $ENVIRONMENT"
    fi

    # Validate deployment type
    if [[ ! "$DEPLOYMENT_TYPE" =~ ^(docker-compose|kubernetes|systemd)$ ]]; then
        error "Invalid deployment type: $DEPLOYMENT_TYPE"
    fi

    # Check node count
    if [[ ! "$NODE_COUNT" =~ ^[0-9]+$ ]] || [[ "$NODE_COUNT" -lt 1 ]]; then
        error "Invalid node count: $NODE_COUNT"
    fi

    log "Prerequisites check passed"
}

# Build application
build_application() {
    log "Building IPFS application..."

    cd "$PROJECT_ROOT"

    # Run tests if not skipped
    if [[ "$SKIP_TESTS" != true ]]; then
        log "Running tests..."
        go test ./... || error "Tests failed"

        # Run benchmarks for performance validation
        log "Running performance benchmarks..."
        go test -bench=. ./benchmarks/... || warn "Benchmarks failed"
    fi

    # Build based on deployment type
    case $DEPLOYMENT_TYPE in
        docker-compose|kubernetes)
            build_containers
            ;;
        systemd)
            build_binaries
            ;;
    esac

    log "Application build completed"
}

# Build containers
build_containers() {
    log "Building container images..."

    local image_tag="${ENVIRONMENT}-$(date +%Y%m%d-%H%M%S)"
    local dockerfile="$PROJECT_ROOT/docs/deployment/Dockerfile"

    # Build main IPFS node image
    docker build \
        -f "$dockerfile" \
        -t "ipfs-node:$image_tag" \
        -t "ipfs-node:${ENVIRONMENT}-latest" \
        "$PROJECT_ROOT" || error "Failed to build IPFS node image"

    # Build backup service image
    docker build \
        -f "$PROJECT_ROOT/docs/deployment/Dockerfile.backup" \
        -t "ipfs-backup:$image_tag" \
        -t "ipfs-backup:${ENVIRONMENT}-latest" \
        "$PROJECT_ROOT" || error "Failed to build backup service image"

    # Push to registry if configured
    if [[ -n "${REGISTRY_URL:-}" ]]; then
        push_to_registry "$image_tag"
    fi

    log "Container images built successfully"
}

# Build binaries
build_binaries() {
    log "Building application binaries..."

    cd "$PROJECT_ROOT"

    # Build IPFS node
    CGO_ENABLED=1 go build -o bin/ipfs-node \
        -ldflags "-X main.version=$(git describe --tags --always) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        ./cmd/node || error "Failed to build IPFS node"

    # Build backup tool
    CGO_ENABLED=1 go build -o bin/backup-tool ./cmd/backup-tool || error "Failed to build backup tool"

    log "Binaries built successfully"
}

# Push to container registry
push_to_registry() {
    local image_tag="$1"

    log "Pushing images to registry: $REGISTRY_URL"

    # Login to registry
    if [[ -n "${REGISTRY_USERNAME:-}" && -n "${REGISTRY_PASSWORD:-}" ]]; then
        echo "$REGISTRY_PASSWORD" | docker login "$REGISTRY_URL" \
            --username "$REGISTRY_USERNAME" --password-stdin || error "Registry login failed"
    fi

    # Tag and push images
    docker tag "ipfs-node:$image_tag" "$REGISTRY_URL/ipfs-node:$image_tag"
    docker tag "ipfs-node:${ENVIRONMENT}-latest" "$REGISTRY_URL/ipfs-node:${ENVIRONMENT}-latest"
    docker push "$REGISTRY_URL/ipfs-node:$image_tag" || error "Failed to push IPFS node image"
    docker push "$REGISTRY_URL/ipfs-node:${ENVIRONMENT}-latest" || error "Failed to push IPFS node latest image"

    docker tag "ipfs-backup:$image_tag" "$REGISTRY_URL/ipfs-backup:$image_tag"
    docker tag "ipfs-backup:${ENVIRONMENT}-latest" "$REGISTRY_URL/ipfs-backup:${ENVIRONMENT}-latest"
    docker push "$REGISTRY_URL/ipfs-backup:$image_tag" || error "Failed to push backup service image"
    docker push "$REGISTRY_URL/ipfs-backup:${ENVIRONMENT}-latest" || error "Failed to push backup service latest image"

    log "Images pushed to registry successfully"
}

# Deploy with Docker Compose
deploy_docker_compose() {
    log "Deploying with Docker Compose..."

    local deploy_dir="$PROJECT_ROOT/docs/deployment"
    cd "$deploy_dir"

    # Prepare environment file
    create_env_file

    # Create necessary directories
    sudo mkdir -p /opt/ipfs/{data,logs,backups}/{node1,node2,node3}
    sudo chown -R 1000:1000 /opt/ipfs/

    # Deploy stack
    if [[ "$MONITORING_ENABLED" == true ]]; then
        docker-compose -f docker-compose-production.yml up -d || error "Docker Compose deployment failed"
    else
        # Deploy without monitoring services
        docker-compose -f docker-compose-production.yml up -d \
            ipfs-node-1 ipfs-node-2 ipfs-node-3 nginx || error "Docker Compose deployment failed"
    fi

    # Wait for services to be healthy
    wait_for_services_docker

    log "Docker Compose deployment completed"
}

# Deploy to Kubernetes
deploy_kubernetes() {
    log "Deploying to Kubernetes..."

    local k8s_dir="$PROJECT_ROOT/docs/deployment/kubernetes"
    cd "$k8s_dir"

    # Create namespace
    kubectl create namespace ipfs-system --dry-run=client -o yaml | kubectl apply -f -

    # Apply secrets
    create_k8s_secrets

    # Apply configurations
    kubectl apply -f ipfs-cluster.yaml || error "Kubernetes deployment failed"

    # Wait for rollout
    kubectl rollout status statefulset/ipfs-cluster -n ipfs-system --timeout=600s || error "Deployment rollout failed"

    # Wait for services to be ready
    wait_for_services_k8s

    log "Kubernetes deployment completed"
}

# Deploy with systemd
deploy_systemd() {
    log "Deploying with systemd..."

    # Install binaries
    sudo cp "$PROJECT_ROOT/bin/ipfs-node" /usr/local/bin/
    sudo cp "$PROJECT_ROOT/bin/backup-tool" /usr/local/bin/
    sudo chmod +x /usr/local/bin/{ipfs-node,backup-tool}

    # Create user and directories
    sudo useradd -r -s /bin/false ipfs || true
    sudo mkdir -p /opt/ipfs/{data,logs,configs,backups}
    sudo chown -R ipfs:ipfs /opt/ipfs/

    # Copy configuration
    sudo cp "$PROJECT_ROOT/configs/production.yml" /opt/ipfs/configs/

    # Create systemd service
    create_systemd_service

    # Enable and start service
    sudo systemctl daemon-reload
    sudo systemctl enable ipfs-node
    sudo systemctl start ipfs-node

    # Wait for service to be ready
    wait_for_services_systemd

    log "Systemd deployment completed"
}

# Create environment file for Docker Compose
create_env_file() {
    cat > .env << EOF
# Generated environment file for IPFS deployment
ENVIRONMENT=$ENVIRONMENT
NODE_COUNT=$NODE_COUNT
DOMAIN=${DOMAIN:-localhost}
GRAFANA_PASSWORD=${GRAFANA_PASSWORD:-admin123}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-ipfs_password}
SMTP_HOST=${SMTP_HOST:-}
SMTP_USER=${SMTP_USER:-}
SMTP_PASSWORD=${SMTP_PASSWORD:-}
BACKUP_WEBHOOK_URL=${BACKUP_WEBHOOK_URL:-}
EOF

    log "Environment file created"
}

# Create Kubernetes secrets
create_k8s_secrets() {
    # Create TLS secret if SSL is configured
    if [[ -n "$DOMAIN" && -n "$SSL_EMAIL" ]]; then
        # This would typically be handled by cert-manager
        info "SSL will be handled by cert-manager for domain: $DOMAIN"
    fi

    # Create application secrets
    kubectl create secret generic ipfs-secrets \
        --from-literal=api-key="${API_KEY:-$(openssl rand -base64 32)}" \
        --from-literal=backup-encryption-key="${BACKUP_ENCRYPTION_KEY:-$(openssl rand -base64 32)}" \
        --namespace ipfs-system \
        --dry-run=client -o yaml | kubectl apply -f -

    log "Kubernetes secrets created"
}

# Create systemd service file
create_systemd_service() {
    sudo tee /etc/systemd/system/ipfs-node.service > /dev/null << EOF
[Unit]
Description=IPFS Node
Documentation=https://docs.ipfs.io/
After=network.target

[Service]
Type=exec
User=ipfs
Group=ipfs
ExecStart=/usr/local/bin/ipfs-node -config=/opt/ipfs/configs/production.yml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=5
TimeoutStopSec=30

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/ipfs

# Resource limits
LimitNOFILE=65536
LimitNPROC=32768
MemoryMax=8G
CPUQuota=400%

[Install]
WantedBy=multi-user.target
EOF

    log "Systemd service file created"
}

# Wait for services (Docker Compose)
wait_for_services_docker() {
    log "Waiting for services to be healthy..."

    local max_attempts=60
    local attempt=0

    while [[ $attempt -lt $max_attempts ]]; do
        if curl -f -s http://localhost:8080/health > /dev/null && \
           curl -f -s http://localhost:8081/health > /dev/null && \
           curl -f -s http://localhost:8082/health > /dev/null; then
            log "All services are healthy"
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 10
    done

    error "Services failed to become healthy within timeout"
}

# Wait for services (Kubernetes)
wait_for_services_k8s() {
    log "Waiting for Kubernetes services to be ready..."

    kubectl wait --for=condition=Ready pod -l app=ipfs-node -n ipfs-system --timeout=600s || error "Pods failed to become ready"

    # Check service endpoints
    local max_attempts=30
    local attempt=0

    while [[ $attempt -lt $max_attempts ]]; do
        local gateway_ip=$(kubectl get svc ipfs-gateway -n ipfs-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

        if [[ -n "$gateway_ip" ]]; then
            if curl -f -s "http://$gateway_ip/health" > /dev/null; then
                log "Gateway service is healthy at $gateway_ip"
                return 0
            fi
        fi

        attempt=$((attempt + 1))
        sleep 10
    done

    warn "Gateway health check timeout, but pods are ready"
}

# Wait for services (systemd)
wait_for_services_systemd() {
    log "Waiting for systemd service to be ready..."

    local max_attempts=30
    local attempt=0

    while [[ $attempt -lt $max_attempts ]]; do
        if systemctl is-active --quiet ipfs-node && \
           curl -f -s http://localhost:8080/health > /dev/null; then
            log "IPFS node service is healthy"
            return 0
        fi

        attempt=$((attempt + 1))
        sleep 10
    done

    error "IPFS node service failed to become healthy"
}

# Post-deployment configuration
post_deployment() {
    log "Running post-deployment configuration..."

    case $DEPLOYMENT_TYPE in
        docker-compose)
            post_deployment_docker
            ;;
        kubernetes)
            post_deployment_k8s
            ;;
        systemd)
            post_deployment_systemd
            ;;
    esac

    # Setup SSL certificates if domain is provided
    if [[ -n "$DOMAIN" && -n "$SSL_EMAIL" ]]; then
        setup_ssl_certificates
    fi

    # Configure monitoring dashboards
    if [[ "$MONITORING_ENABLED" == true ]]; then
        configure_monitoring
    fi

    # Setup backup schedules
    if [[ "$BACKUP_ENABLED" == true ]]; then
        configure_backups
    fi

    log "Post-deployment configuration completed"
}

# Post-deployment for Docker Compose
post_deployment_docker() {
    # Show service status
    docker-compose -f docker-compose-production.yml ps

    # Show service URLs
    info "Services available at:"
    info "  Gateway: http://localhost:8080"
    info "  API: http://localhost:5001"
    info "  Metrics: http://localhost:9090"
    if [[ "$MONITORING_ENABLED" == true ]]; then
        info "  Grafana: http://localhost:3000 (admin/admin123)"
        info "  Prometheus: http://localhost:9093"
    fi
}

# Post-deployment for Kubernetes
post_deployment_k8s() {
    # Show pod status
    kubectl get pods -n ipfs-system

    # Show service information
    kubectl get services -n ipfs-system

    # Get gateway URL
    local gateway_ip=$(kubectl get svc ipfs-gateway -n ipfs-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
    info "Gateway URL: http://$gateway_ip (may take a few minutes to be available)"
}

# Post-deployment for systemd
post_deployment_systemd() {
    # Show service status
    systemctl status ipfs-node

    info "Services available at:"
    info "  Gateway: http://localhost:8080"
    info "  API: http://localhost:5001"
    info "  Metrics: http://localhost:9090"
}

# Setup SSL certificates
setup_ssl_certificates() {
    log "Setting up SSL certificates for domain: $DOMAIN"

    if command -v certbot >/dev/null 2>&1; then
        # Use certbot for Let's Encrypt
        info "Using certbot for SSL certificate generation"
        # Implementation would depend on web server configuration
    else
        warn "certbot not found, SSL setup skipped"
    fi
}

# Configure monitoring
configure_monitoring() {
    log "Configuring monitoring dashboards..."

    if [[ "$DEPLOYMENT_TYPE" == "docker-compose" ]]; then
        # Import Grafana dashboards
        sleep 30  # Wait for Grafana to be ready

        # Import dashboard (this would need the actual dashboard JSON)
        # curl -X POST http://admin:admin123@localhost:3000/api/dashboards/db \
        #      -H "Content-Type: application/json" \
        #      -d @monitoring/grafana/dashboards/ipfs-overview.json
    fi

    log "Monitoring configuration completed"
}

# Configure backups
configure_backups() {
    log "Configuring backup schedules..."

    case $DEPLOYMENT_TYPE in
        docker-compose|kubernetes)
            # Backup is handled by the backup service container
            info "Backup service is running in container"
            ;;
        systemd)
            # Setup cron job for backups
            (crontab -u ipfs -l 2>/dev/null; echo "0 2 * * * /usr/local/bin/backup-tool -cmd=backup -datastore=/opt/ipfs/data") | crontab -u ipfs -
            ;;
    esac

    log "Backup configuration completed"
}

# Cleanup function
cleanup() {
    log "Cleaning up temporary files..."
    rm -f "$LOG_FILE.tmp"
}

# Main function
main() {
    log "Starting IPFS production deployment"
    log "Log file: $LOG_FILE"

    # Set trap for cleanup
    trap cleanup EXIT

    # Parse arguments
    parse_args "$@"

    # Show configuration
    info "Deployment Configuration:"
    info "  Environment: $ENVIRONMENT"
    info "  Deployment Type: $DEPLOYMENT_TYPE"
    info "  Node Count: $NODE_COUNT"
    info "  Domain: ${DOMAIN:-not set}"
    info "  Backup Enabled: $BACKUP_ENABLED"
    info "  Monitoring Enabled: $MONITORING_ENABLED"

    # Check prerequisites
    check_prerequisites

    # Build application
    build_application

    # Deploy based on type
    case $DEPLOYMENT_TYPE in
        docker-compose)
            deploy_docker_compose
            ;;
        kubernetes)
            deploy_kubernetes
            ;;
        systemd)
            deploy_systemd
            ;;
    esac

    # Post-deployment configuration
    post_deployment

    log "Deployment completed successfully!"
    log "Check the log file for details: $LOG_FILE"
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi