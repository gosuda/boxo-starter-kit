package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name        string  `json:"name"`
	Iterations  int     `json:"iterations"`
	NsPerOp     int64   `json:"ns_per_op"`
	MBPerSec    float64 `json:"mb_per_sec,omitempty"`
	BytesPerOp  int64   `json:"bytes_per_op"`
	AllocsPerOp int64   `json:"allocs_per_op"`
	Timestamp   string  `json:"timestamp"`
	GoVersion   string  `json:"go_version"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
}

// BenchmarkSuite contains multiple benchmark results
type BenchmarkSuite struct {
	Results   []BenchmarkResult `json:"results"`
	Timestamp string            `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

// RunBenchmarks executes all benchmarks and returns results
func RunBenchmarks(patterns []string, outputDir string) (*BenchmarkSuite, error) {
	if len(patterns) == 0 {
		patterns = []string{"."} // Run all benchmarks by default
	}

	suite := &BenchmarkSuite{
		Results:   make([]BenchmarkResult, 0),
		Timestamp: time.Now().Format(time.RFC3339),
		Metadata: map[string]string{
			"go_version": getGoVersion(),
			"os":         getOS(),
			"arch":       getArch(),
		},
	}

	for _, pattern := range patterns {
		results, err := runBenchmarkPattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to run benchmark pattern %s: %w", pattern, err)
		}
		suite.Results = append(suite.Results, results...)
	}

	// Sort results by name
	sort.Slice(suite.Results, func(i, j int) bool {
		return suite.Results[i].Name < suite.Results[j].Name
	})

	// Save results to file
	if outputDir != "" {
		err := saveResults(suite, outputDir)
		if err != nil {
			return nil, fmt.Errorf("failed to save results: %w", err)
		}
	}

	return suite, nil
}

func runBenchmarkPattern(pattern string) ([]BenchmarkResult, error) {
	cmd := exec.Command("go", "test", "-bench="+pattern, "-benchmem", "-run=^$", "./...")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("benchmark failed: %w, output: %s", err, string(output))
	}

	return parseBenchmarkOutput(string(output))
}

func parseBenchmarkOutput(output string) ([]BenchmarkResult, error) {
	lines := strings.Split(output, "\n")
	var results []BenchmarkResult

	// Regex to parse benchmark lines
	// Example: BenchmarkBlock_CIDCreation_Small-8   	  100000	     10234 ns/op	    1024 B/op	       8 allocs/op
	benchRegex := regexp.MustCompile(`^(Benchmark\w+)-(\d+)\s+(\d+)\s+(\d+)\s+ns/op(?:\s+(\d+(?:\.\d+)?)\s+MB/s)?\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		matches := benchRegex.FindStringSubmatch(line)
		if len(matches) < 7 {
			continue
		}

		iterations, _ := strconv.Atoi(matches[3])
		nsPerOp, _ := strconv.ParseInt(matches[4], 10, 64)
		bytesPerOp, _ := strconv.ParseInt(matches[6], 10, 64)
		allocsPerOp, _ := strconv.ParseInt(matches[7], 10, 64)

		var mbPerSec float64
		if len(matches) > 5 && matches[5] != "" {
			mbPerSec, _ = strconv.ParseFloat(matches[5], 64)
		}

		result := BenchmarkResult{
			Name:        matches[1],
			Iterations:  iterations,
			NsPerOp:     nsPerOp,
			MBPerSec:    mbPerSec,
			BytesPerOp:  bytesPerOp,
			AllocsPerOp: allocsPerOp,
			Timestamp:   time.Now().Format(time.RFC3339),
			GoVersion:   getGoVersion(),
			OS:          getOS(),
			Arch:        getArch(),
		}

		results = append(results, result)
	}

	return results, nil
}

func saveResults(suite *BenchmarkSuite, outputDir string) error {
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return err
	}

	// Save JSON results
	jsonFile := filepath.Join(outputDir, fmt.Sprintf("benchmark_results_%s.json",
		time.Now().Format("20060102_150405")))

	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(jsonFile, data, 0644)
	if err != nil {
		return err
	}

	// Save human-readable report
	reportFile := filepath.Join(outputDir, fmt.Sprintf("benchmark_report_%s.md",
		time.Now().Format("20060102_150405")))

	report := generateMarkdownReport(suite)
	err = os.WriteFile(reportFile, []byte(report), 0644)
	if err != nil {
		return err
	}

	fmt.Printf("Results saved to:\n")
	fmt.Printf("  JSON: %s\n", jsonFile)
	fmt.Printf("  Report: %s\n", reportFile)

	return nil
}

func generateMarkdownReport(suite *BenchmarkSuite) string {
	var sb strings.Builder

	sb.WriteString("# Benchmark Results\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", suite.Timestamp))
	sb.WriteString(fmt.Sprintf("**Go Version:** %s\n", suite.Metadata["go_version"]))
	sb.WriteString(fmt.Sprintf("**OS/Arch:** %s/%s\n\n", suite.Metadata["os"], suite.Metadata["arch"]))

	// Group results by category
	categories := groupBenchmarksByCategory(suite.Results)

	for category, results := range categories {
		sb.WriteString(fmt.Sprintf("## %s\n\n", category))
		sb.WriteString("| Benchmark | Iterations | ns/op | MB/s | B/op | allocs/op |\n")
		sb.WriteString("|-----------|------------|-------|------|------|----------|\n")

		for _, result := range results {
			mbPerSecStr := ""
			if result.MBPerSec > 0 {
				mbPerSecStr = fmt.Sprintf("%.2f", result.MBPerSec)
			}

			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %s | %d | %d |\n",
				result.Name,
				result.Iterations,
				result.NsPerOp,
				mbPerSecStr,
				result.BytesPerOp,
				result.AllocsPerOp,
			))
		}
		sb.WriteString("\n")
	}

	// Performance insights
	sb.WriteString("## Performance Insights\n\n")
	sb.WriteString(generatePerformanceInsights(suite.Results))

	return sb.String()
}

func groupBenchmarksByCategory(results []BenchmarkResult) map[string][]BenchmarkResult {
	categories := make(map[string][]BenchmarkResult)

	for _, result := range results {
		category := extractCategory(result.Name)
		categories[category] = append(categories[category], result)
	}

	return categories
}

func extractCategory(benchmarkName string) string {
	parts := strings.Split(benchmarkName, "_")
	if len(parts) > 1 {
		return strings.Title(strings.ToLower(parts[1]))
	}
	return "Other"
}

func generatePerformanceInsights(results []BenchmarkResult) string {
	var sb strings.Builder

	// Find fastest and slowest operations
	if len(results) > 0 {
		sort.Slice(results, func(i, j int) bool {
			return results[i].NsPerOp < results[j].NsPerOp
		})

		fastest := results[0]
		slowest := results[len(results)-1]

		sb.WriteString(fmt.Sprintf("**Fastest Operation:** %s (%d ns/op)\n", fastest.Name, fastest.NsPerOp))
		sb.WriteString(fmt.Sprintf("**Slowest Operation:** %s (%d ns/op)\n\n", slowest.Name, slowest.NsPerOp))
	}

	// Memory usage insights
	sort.Slice(results, func(i, j int) bool {
		return results[i].BytesPerOp < results[j].BytesPerOp
	})

	if len(results) > 0 {
		leastMemory := results[0]
		mostMemory := results[len(results)-1]

		sb.WriteString(fmt.Sprintf("**Least Memory Usage:** %s (%d B/op)\n", leastMemory.Name, leastMemory.BytesPerOp))
		sb.WriteString(fmt.Sprintf("**Most Memory Usage:** %s (%d B/op)\n\n", mostMemory.Name, mostMemory.BytesPerOp))
	}

	// Allocation insights
	sort.Slice(results, func(i, j int) bool {
		return results[i].AllocsPerOp < results[j].AllocsPerOp
	})

	if len(results) > 0 {
		fewestAllocs := results[0]
		mostAllocs := results[len(results)-1]

		sb.WriteString(fmt.Sprintf("**Fewest Allocations:** %s (%d allocs/op)\n", fewestAllocs.Name, fewestAllocs.AllocsPerOp))
		sb.WriteString(fmt.Sprintf("**Most Allocations:** %s (%d allocs/op)\n\n", mostAllocs.Name, mostAllocs.AllocsPerOp))
	}

	return sb.String()
}

func getGoVersion() string {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func getOS() string {
	cmd := exec.Command("go", "env", "GOOS")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func getArch() string {
	cmd := exec.Command("go", "env", "GOARCH")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
