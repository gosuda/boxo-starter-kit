# IPNI Makefile

.PHONY: all build run clean test demo help

# Variables
BINARY_NAME=ipni-node
DOCKER_IMAGE=ipni:latest
GO=go
GOFLAGS=-v

# Default target
all: build

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'

## build: Build the IPNI binary
build:
	@echo "🔨 Building IPNI..."
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) main.go
	@echo "✅ Build complete: $(BINARY_NAME)"

## run: Run IPNI with default settings
run: build
	@echo "🚀 Starting IPNI..."
	./$(BINARY_NAME)

## demo: Run IPNI in demo mode
demo: build
	@echo "🎭 Starting IPNI in demo mode..."
	./$(BINARY_NAME) --demo --verbose

## dev: Run IPNI in development mode (memory storage, verbose)
dev: build
	@echo "💻 Starting IPNI in development mode..."
	./$(BINARY_NAME) \
		--storage=memory \
		--verbose \
		--demo \
		--metrics=9090 \
		--health=8080

## prod: Run IPNI in production mode
prod: build
	@echo "🏭 Starting IPNI in production mode..."
	./$(BINARY_NAME) \
		--storage=badger \
		--data=/var/lib/ipni \
		--metrics=9090 \
		--health=8080 \
		--pubsub \
		--cache

## test: Run all tests
test:
	@echo "🧪 Running tests..."
	cd pkg && $(GO) test -v -cover ./...

## test-short: Run short tests only
test-short:
	@echo "🧪 Running short tests..."
	cd pkg && $(GO) test -v -short ./...

## test-stress: Run stress tests
test-stress:
	@echo "💪 Running stress tests..."
	cd pkg && $(GO) test -v -run TestStress -timeout 10m ./...

## bench: Run benchmarks
bench:
	@echo "📊 Running benchmarks..."
	cd pkg && $(GO) test -bench=. -benchmem -benchtime=10s ./...

## coverage: Generate test coverage report
coverage:
	@echo "📈 Generating coverage report..."
	cd pkg && $(GO) test -coverprofile=coverage.out ./...
	cd pkg && $(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: pkg/coverage.html"

## docker-build: Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "✅ Docker image built: $(DOCKER_IMAGE)"

## docker-run: Run IPNI in Docker
docker-run: docker-build
	@echo "🐳 Running IPNI in Docker..."
	docker run -it --rm \
		-p 4001:4001 \
		-p 8080:8080 \
		-p 9090:9090 \
		-v ipni-data:/var/lib/ipni \
		$(DOCKER_IMAGE)

## docker-compose: Run with docker-compose
docker-compose:
	@echo "🐳 Starting with docker-compose..."
	docker-compose up -d
	@echo "✅ Services started. Check: docker-compose ps"

## install: Install IPNI to system
install: build
	@echo "📦 Installing IPNI..."
	sudo cp $(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "✅ Installed to /usr/local/bin/$(BINARY_NAME)"

## uninstall: Remove IPNI from system
uninstall:
	@echo "🗑️  Uninstalling IPNI..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✅ Uninstalled"

## clean: Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f pkg/coverage.out pkg/coverage.html
	rm -rf ipni-data
	@echo "✅ Clean complete"

## deps: Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "✅ Dependencies ready"

## fmt: Format code
fmt:
	@echo "🎨 Formatting code..."
	$(GO) fmt ./...
	@echo "✅ Code formatted"

## lint: Run linter
lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...
	@echo "✅ Linting complete"

## vet: Run go vet
vet:
	@echo "🔍 Running go vet..."
	$(GO) vet ./...
	@echo "✅ Vet complete"

## check: Run all checks (fmt, vet, lint, test-short)
check: fmt vet test-short
	@echo "✅ All checks passed!"

# Development helpers
watch:
	@echo "👁️  Watching for changes..."
	@which reflex > /dev/null || (echo "Installing reflex..." && go install github.com/cespare/reflex@latest)
	reflex -r '\.go$$' -s -- sh -c 'make build && ./$(BINARY_NAME) --demo'

logs:
	@echo "📜 Tailing logs..."
	journalctl -u ipni -f

status:
	@echo "📊 Service status:"
	systemctl status ipni || true
	@echo "\n🌐 Network status:"
	ss -tlnp | grep ipni || true
	@echo "\n💾 Memory usage:"
	ps aux | grep ipni | grep -v grep || true