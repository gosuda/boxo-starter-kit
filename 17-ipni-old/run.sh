#!/bin/bash

# IPNI Node Runner Script
# This script provides easy ways to run IPNI with different configurations

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
MODE="default"
DATA_DIR="./ipni-data"
STORAGE="badger"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dev|--development)
            MODE="dev"
            shift
            ;;
        --prod|--production)
            MODE="prod"
            shift
            ;;
        --demo)
            MODE="demo"
            shift
            ;;
        --docker)
            MODE="docker"
            shift
            ;;
        --help|-h)
            MODE="help"
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            MODE="help"
            shift
            ;;
    esac
done

# Functions
show_banner() {
    echo -e "${BLUE}"
    echo "╔═══════════════════════════════════════╗"
    echo "║          IPNI Node Runner             ║"
    echo "║    InterPlanetary Network Indexer     ║"
    echo "╚═══════════════════════════════════════╝"
    echo -e "${NC}"
}

show_help() {
    echo "Usage: ./run.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --dev, --development    Run in development mode (memory storage)"
    echo "  --prod, --production    Run in production mode (persistent storage)"
    echo "  --demo                  Run demo mode with sample data"
    echo "  --docker                Run using Docker"
    echo "  --help, -h              Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./run.sh                # Run with default settings"
    echo "  ./run.sh --dev          # Development mode"
    echo "  ./run.sh --prod         # Production mode"
    echo "  ./run.sh --demo         # Demo mode"
}

check_dependencies() {
    echo -e "${YELLOW}Checking dependencies...${NC}"

    # Check Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}✗ Go is not installed${NC}"
        echo "  Please install Go 1.20+ from https://go.dev"
        exit 1
    else
        GO_VERSION=$(go version | awk '{print $3}')
        echo -e "${GREEN}✓ Go installed: ${GO_VERSION}${NC}"
    fi

    # Check for build
    if [ ! -f "ipni-node" ]; then
        echo -e "${YELLOW}Building IPNI node...${NC}"
        go build -o ipni-node main.go
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ Build successful${NC}"
        else
            echo -e "${RED}✗ Build failed${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}✓ IPNI binary found${NC}"
    fi
}

run_development() {
    echo -e "${BLUE}Starting IPNI in DEVELOPMENT mode...${NC}"
    echo ""

    ./ipni-node \
        --data="./dev-data" \
        --storage="memory" \
        --listen="/ip4/127.0.0.1/tcp/4001" \
        --metrics=9090 \
        --health=8080 \
        --verbose \
        --demo
}

run_production() {
    echo -e "${BLUE}Starting IPNI in PRODUCTION mode...${NC}"
    echo ""

    # Create data directory if not exists
    mkdir -p /var/lib/ipni

    ./ipni-node \
        --data="/var/lib/ipni" \
        --storage="badger" \
        --listen="/ip4/0.0.0.0/tcp/4001" \
        --metrics=9090 \
        --health=8080 \
        --pubsub \
        --cache
}

run_demo() {
    echo -e "${BLUE}Starting IPNI in DEMO mode...${NC}"
    echo ""

    ./ipni-node \
        --data="./demo-data" \
        --storage="memory" \
        --listen="/ip4/127.0.0.1/tcp/4001" \
        --metrics=9090 \
        --health=8080 \
        --verbose \
        --demo \
        --pubsub \
        --cache
}

run_docker() {
    echo -e "${BLUE}Starting IPNI with Docker...${NC}"
    echo ""

    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}✗ Docker is not installed${NC}"
        echo "  Please install Docker from https://docker.com"
        exit 1
    fi

    # Check if docker-compose exists
    if [ -f "docker-compose.yml" ]; then
        echo -e "${YELLOW}Using docker-compose...${NC}"
        docker-compose up -d
        echo -e "${GREEN}✓ IPNI started with docker-compose${NC}"
        echo ""
        echo "View logs: docker-compose logs -f"
        echo "Stop: docker-compose down"
    else
        echo -e "${YELLOW}Building Docker image...${NC}"
        docker build -t ipni:latest .

        echo -e "${YELLOW}Running Docker container...${NC}"
        docker run -d \
            --name ipni-node \
            -p 4001:4001 \
            -p 8080:8080 \
            -p 9090:9090 \
            -v ipni-data:/var/lib/ipni \
            ipni:latest

        echo -e "${GREEN}✓ IPNI started in Docker${NC}"
        echo ""
        echo "View logs: docker logs -f ipni-node"
        echo "Stop: docker stop ipni-node && docker rm ipni-node"
    fi
}

run_default() {
    echo -e "${BLUE}Starting IPNI with default settings...${NC}"
    echo ""

    ./ipni-node \
        --data="$DATA_DIR" \
        --storage="$STORAGE" \
        --metrics=9090 \
        --health=8080 \
        --pubsub \
        --cache
}

show_post_start() {
    echo ""
    echo -e "${GREEN}════════════════════════════════════════${NC}"
    echo -e "${GREEN}IPNI node is running!${NC}"
    echo ""
    echo "📊 Metrics:     http://localhost:9090/metrics"
    echo "🏥 Health:      http://localhost:8080/health"
    echo "🔍 Readiness:   http://localhost:8080/ready"
    echo "💉 Liveness:    http://localhost:8080/live"
    echo ""
    echo "Press Ctrl+C to stop the node"
    echo -e "${GREEN}════════════════════════════════════════${NC}"
}

# Main execution
show_banner

case $MODE in
    help)
        show_help
        exit 0
        ;;
    dev)
        check_dependencies
        run_development
        ;;
    prod)
        check_dependencies
        run_production
        ;;
    demo)
        check_dependencies
        run_demo
        ;;
    docker)
        run_docker
        ;;
    default)
        check_dependencies
        run_default
        ;;
esac

# Show post-start information if not Docker mode
if [ "$MODE" != "docker" ] && [ "$MODE" != "help" ]; then
    show_post_start
fi