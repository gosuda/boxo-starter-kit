#!/bin/bash
set -e

# Boxo Starter Kit Benchmark Runner
echo "🚀 Running Boxo Starter Kit Performance Benchmarks"
echo "=================================================="

# Check if go is available
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed or not in PATH"
    exit 1
fi

# Set up variables
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BENCHMARK_DIR="$ROOT_DIR/benchmarks"
RESULTS_DIR="$ROOT_DIR/benchmark_results"

# Create results directory
mkdir -p "$RESULTS_DIR"

echo "📁 Benchmark directory: $BENCHMARK_DIR"
echo "📊 Results directory: $RESULTS_DIR"
echo ""

# Change to benchmark directory
cd "$BENCHMARK_DIR"

# Parse command line arguments
CATEGORIES="${1:-block,datastore,memory}"
VERBOSE="${2:-false}"

echo "🎯 Running benchmark categories: $CATEGORIES"
echo ""

# Function to run benchmarks for a specific category
run_category_benchmarks() {
    local category="$1"
    echo "🔍 Running $category benchmarks..."

    case "$category" in
        "block")
            echo "  ⚙️  Testing block operations and CID creation..."
            go test -bench=BenchmarkBlock -benchmem -run=^$ | tee "$RESULTS_DIR/block_results.txt"
            ;;
        "datastore")
            echo "  ⚙️  Testing datastore performance (memory, badger, pebble)..."
            go test -bench=BenchmarkDatastore -benchmem -run=^$ | tee "$RESULTS_DIR/datastore_results.txt"
            ;;
        "memory")
            echo "  ⚙️  Testing memory usage and allocation patterns..."
            go test -bench=BenchmarkMemory -benchmem -run=^$ | tee "$RESULTS_DIR/memory_results.txt"
            ;;
        "concurrent")
            echo "  ⚙️  Testing concurrent operations and contention..."
            go test -bench=BenchmarkConcurrent -benchmem -run=^$ | tee "$RESULTS_DIR/concurrent_results.txt"
            ;;
        "gateway")
            echo "  ⚙️  Testing HTTP gateway performance..."
            go test -bench=BenchmarkGateway -benchmem -run=^$ | tee "$RESULTS_DIR/gateway_results.txt"
            ;;
        *)
            echo "  ❌ Unknown category: $category"
            return 1
            ;;
    esac
    echo "  ✅ $category benchmarks completed"
    echo ""
}

# Run benchmarks for each category
IFS=',' read -ra CATEGORY_ARRAY <<< "$CATEGORIES"
for category in "${CATEGORY_ARRAY[@]}"; do
    # Trim whitespace
    category=$(echo "$category" | xargs)
    run_category_benchmarks "$category"
done

echo "🎉 All benchmarks completed successfully!"
echo ""
echo "📊 Results saved to: $RESULTS_DIR"
echo "📝 Individual result files:"
ls -la "$RESULTS_DIR"/*.txt 2>/dev/null || echo "   (No result files found)"

echo ""
echo "💡 To run specific benchmarks:"
echo "   ./scripts/run_benchmarks.sh block                    # Just block benchmarks"
echo "   ./scripts/run_benchmarks.sh datastore,memory         # Multiple categories"
echo "   go test -bench=BenchmarkBlock_CID -benchmem          # Specific pattern"
echo ""
echo "📈 To generate detailed reports:"
echo "   cd benchmarks && go run cmd/benchmark/main.go"