package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gosuda/boxo-starter-kit/benchmarks"
)

func main() {
	var (
		patterns   = flag.String("patterns", "", "Comma-separated list of benchmark patterns (default: run all)")
		outputDir  = flag.String("output", "./benchmark_results", "Output directory for results")
		compare    = flag.String("compare", "", "Compare with baseline results file")
		verbose    = flag.Bool("verbose", false, "Verbose output")
		categories = flag.String("categories", "", "Run specific categories: block,datastore,gateway,memory,concurrent")
	)
	flag.Parse()

	var benchmarkPatterns []string

	// Parse categories if specified
	if *categories != "" {
		cats := strings.Split(*categories, ",")
		for _, cat := range cats {
			switch strings.TrimSpace(cat) {
			case "block":
				benchmarkPatterns = append(benchmarkPatterns, "BenchmarkBlock")
			case "datastore":
				benchmarkPatterns = append(benchmarkPatterns, "BenchmarkDatastore", "BenchmarkPersistentWrapper")
			case "gateway":
				benchmarkPatterns = append(benchmarkPatterns, "BenchmarkGateway", "BenchmarkTrustlessGateway")
			case "memory":
				benchmarkPatterns = append(benchmarkPatterns, "BenchmarkMemory")
			case "concurrent":
				benchmarkPatterns = append(benchmarkPatterns, "BenchmarkConcurrent")
			default:
				log.Printf("Unknown category: %s", cat)
			}
		}
	}

	// Parse individual patterns if specified
	if *patterns != "" {
		patterns := strings.Split(*patterns, ",")
		for _, pattern := range patterns {
			benchmarkPatterns = append(benchmarkPatterns, strings.TrimSpace(pattern))
		}
	}

	if *verbose {
		fmt.Printf("Running benchmarks with patterns: %v\n", benchmarkPatterns)
		fmt.Printf("Output directory: %s\n", *outputDir)
	}

	// Run benchmarks
	suite, err := benchmarks.RunBenchmarks(benchmarkPatterns, *outputDir)
	if err != nil {
		log.Fatalf("Failed to run benchmarks: %v", err)
	}

	fmt.Printf("\n✅ Benchmark completed successfully!\n")
	fmt.Printf("📊 Total benchmarks run: %d\n", len(suite.Results))

	// Print summary
	printSummary(suite)

	// Compare with baseline if specified
	if *compare != "" {
		err := compareWithBaseline(suite, *compare)
		if err != nil {
			log.Printf("Warning: Failed to compare with baseline: %v", err)
		}
	}
}

func printSummary(suite *benchmarks.BenchmarkSuite) {
	if len(suite.Results) == 0 {
		fmt.Println("No benchmark results to display")
		return
	}

	fmt.Println("\n📈 Quick Summary:")
	fmt.Println("==================")

	// Group by category and show top performers
	categories := make(map[string][]benchmarks.BenchmarkResult)
	for _, result := range suite.Results {
		category := extractCategory(result.Name)
		categories[category] = append(categories[category], result)
	}

	for category, results := range categories {
		if len(results) == 0 {
			continue
		}

		// Find fastest in category
		fastest := results[0]
		for _, result := range results {
			if result.NsPerOp < fastest.NsPerOp {
				fastest = result
			}
		}

		fmt.Printf("\n🏆 %s - Fastest: %s\n", category, fastest.Name)
		fmt.Printf("   ⏱️  %d ns/op\n", fastest.NsPerOp)
		if fastest.MBPerSec > 0 {
			fmt.Printf("   📊 %.2f MB/s\n", fastest.MBPerSec)
		}
		fmt.Printf("   💾 %d B/op, %d allocs/op\n", fastest.BytesPerOp, fastest.AllocsPerOp)
	}
}

func extractCategory(benchmarkName string) string {
	parts := strings.Split(benchmarkName, "_")
	if len(parts) > 1 {
		return strings.Title(strings.ToLower(strings.TrimPrefix(parts[0], "Benchmark")))
	}
	return "Other"
}

func compareWithBaseline(current *benchmarks.BenchmarkSuite, baselineFile string) error {
	fmt.Printf("\n🔍 Comparing with baseline: %s\n", baselineFile)
	fmt.Println("=====================================")

	// This is a placeholder for baseline comparison
	// In a real implementation, you would:
	// 1. Load the baseline results from the file
	// 2. Match benchmarks by name
	// 3. Calculate performance differences
	// 4. Generate a comparison report

	if _, err := os.Stat(baselineFile); os.IsNotExist(err) {
		return fmt.Errorf("baseline file not found: %s", baselineFile)
	}

	fmt.Println("⚠️  Baseline comparison not yet implemented")
	fmt.Println("   This feature will compare performance against historical results")
	fmt.Println("   and highlight regressions or improvements")

	return nil
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nBoxo Starter Kit Benchmark Runner\n")
		fmt.Fprintf(os.Stderr, "==================================\n\n")
		fmt.Fprintf(os.Stderr, "This tool runs comprehensive benchmarks comparing boxo-starter-kit\n")
		fmt.Fprintf(os.Stderr, "implementations with standard go-ipfs performance.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s                                    # Run all benchmarks\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -categories=block,datastore        # Run specific categories\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -patterns=BenchmarkBlock_CID       # Run specific patterns\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -output=./results -verbose         # Custom output with verbose mode\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nCategories:\n")
		fmt.Fprintf(os.Stderr, "  block      - Block creation, CID operations, validation\n")
		fmt.Fprintf(os.Stderr, "  datastore  - Storage backend performance (memory, badger, pebble)\n")
		fmt.Fprintf(os.Stderr, "  gateway    - HTTP gateway response times, throughput\n")
		fmt.Fprintf(os.Stderr, "  memory     - Memory usage analysis, leak detection\n")
		fmt.Fprintf(os.Stderr, "  concurrent - High concurrency scenarios, contention tests\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
	}
}
