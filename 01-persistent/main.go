package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	cid "github.com/ipfs/go-cid"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

func main() {
	fmt.Println("🗄️ Persistent Storage Backend Comparison")
	fmt.Println("========================================")

	ctx := context.Background()

	// Create test data directory
	testDir := "./data"
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanupTestData(testDir)

	fmt.Println("\n1. 🏃‍♂️ Quick Backend Comparison")
	fmt.Println("-------------------------------")
	demonstrateAllBackends(ctx, testDir)

	fmt.Println("\n2. ⚡ Performance Benchmarks")
	fmt.Println("---------------------------")
	benchmarkBackends(ctx, testDir)

	fmt.Println("\n3. 🔄 Data Migration Demo")
	fmt.Println("------------------------")
	demonstrateDataMigration(ctx, testDir)

	fmt.Println("\n4. 💾 Persistence Verification")
	fmt.Println("-----------------------------")
	demonstratePersistence(ctx, testDir)

	fmt.Println("\n5. 🎯 Backend Selection Guide")
	fmt.Println("----------------------------")
	displaySelectionGuide()

	fmt.Println("\n6. 📊 Storage Efficiency Comparison")
	fmt.Println("----------------------------------")
	demonstrateStorageEfficiency(ctx, testDir)

	fmt.Println("\n🎉 Demo Complete!")
	fmt.Println("Next: Try 03-dht-router module for distributed networking")
}

func demonstrateAllBackends(ctx context.Context, baseDir string) {
	backends := []struct {
		name        string
		pType       persistent.PersistentType
		path        string
		description string
	}{
		{"Memory", persistent.Memory, "", "In-memory storage (fastest, volatile)"},
		{"File", persistent.File, filepath.Join(baseDir, "file"), "File system storage (simple, portable)"},
		{"BadgerDB", persistent.Badgerdb, filepath.Join(baseDir, "badger"), "LSM-Tree database (write-optimized)"},
		{"PebbleDB", persistent.Pebbledb, filepath.Join(baseDir, "pebble"), "RocksDB-inspired (balanced performance)"},
	}

	testData := []byte("Hello, persistent world! This is a test block for IPFS storage.")
	var cids []cid.Cid

	for _, backend := range backends {
		fmt.Printf("\n🔧 Testing %s Backend\n", backend.name)
		fmt.Printf("   Description: %s\n", backend.description)

		// Create backend
		start := time.Now()
		p, err := persistent.New(backend.pType, backend.path)
		if err != nil {
			fmt.Printf("   ❌ Creation failed: %v\n", err)
			continue
		}
		creationTime := time.Since(start)

		fmt.Printf("   ✅ Created in %v\n", creationTime)

		// Store data
		start = time.Now()
		cidResult, err := p.PutV1Cid(ctx, testData, nil)
		if err != nil {
			fmt.Printf("   ❌ Store failed: %v\n", err)
			p.Close()
			continue
		}
		storeTime := time.Since(start)
		cids = append(cids, cidResult)

		fmt.Printf("   ✅ Stored in %v: %s\n", storeTime, cidResult.String())

		// Retrieve data
		start = time.Now()
		retrievedData, err := p.GetRaw(ctx, cidResult)
		if err != nil {
			fmt.Printf("   ❌ Retrieve failed: %v\n", err)
		} else {
			retrieveTime := time.Since(start)
			fmt.Printf("   ✅ Retrieved in %v: %s\n", retrieveTime, string(retrievedData[:min(50, len(retrievedData))]))
		}

		// Check storage location
		if backend.path != "" {
			if dirExists(backend.path) {
				fmt.Printf("   📁 Data stored at: %s\n", backend.path)
			}
		}

		p.Close()
	}

	// Verify all backends produced same CID
	fmt.Printf("\n🔍 CID Consistency Check:\n")
	if len(cids) > 1 {
		allSame := true
		for i := 1; i < len(cids); i++ {
			if !cids[0].Equals(cids[i]) {
				allSame = false
				break
			}
		}
		if allSame {
			fmt.Printf("   ✅ All backends produced identical CID: %s\n", cids[0].String())
		} else {
			fmt.Printf("   ❌ CID mismatch detected!\n")
		}
	}
}

func benchmarkBackends(ctx context.Context, baseDir string) {
	backends := []struct {
		name  string
		pType persistent.PersistentType
		path  string
	}{
		{"Memory", persistent.Memory, ""},
		{"File", persistent.File, filepath.Join(baseDir, "bench_file")},
		{"BadgerDB", persistent.Badgerdb, filepath.Join(baseDir, "bench_badger")},
		{"PebbleDB", persistent.Pebbledb, filepath.Join(baseDir, "bench_pebble")},
	}

	const numOperations = 1000
	const blockSize = 4096 // 4KB blocks

	fmt.Printf("Benchmarking %d operations with %d byte blocks:\n\n", numOperations, blockSize)

	results := make(map[string]BenchmarkResult)

	for _, backend := range backends {
		fmt.Printf("🔬 Benchmarking %s...\n", backend.name)

		result := benchmarkSingleBackend(ctx, backend.pType, backend.path, numOperations, blockSize)
		results[backend.name] = result

		fmt.Printf("   Write: %v/op (%d ops/sec)\n",
			result.AvgWriteTime, int(float64(numOperations)/result.TotalWriteTime.Seconds()))
		fmt.Printf("   Read:  %v/op (%d ops/sec)\n",
			result.AvgReadTime, int(float64(numOperations)/result.TotalReadTime.Seconds()))
		fmt.Printf("   Setup: %v\n", result.SetupTime)
	}

	// Performance comparison table
	fmt.Printf("\n📊 Performance Summary:\n")
	fmt.Printf("%-10s │ %12s │ %12s │ %10s\n", "Backend", "Write (μs/op)", "Read (μs/op)", "Setup (ms)")
	fmt.Printf("───────────┼──────────────┼──────────────┼───────────\n")

	for _, backend := range backends {
		result := results[backend.name]
		fmt.Printf("%-10s │ %12.0f │ %12.0f │ %10.1f\n",
			backend.name,
			float64(result.AvgWriteTime.Nanoseconds())/1000,
			float64(result.AvgReadTime.Nanoseconds())/1000,
			float64(result.SetupTime.Nanoseconds())/1000000)
	}
}

func demonstrateDataMigration(ctx context.Context, baseDir string) {
	fmt.Printf("Migrating data from Memory to File backend...\n")

	// Create source (memory) with test data
	source, err := persistent.New(persistent.Memory, "")
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	// Add test data to source
	testData := []struct {
		content string
		desc    string
	}{
		{"Document 1: Important contract", "Legal document"},
		{"Document 2: Product specifications", "Technical specs"},
		{"Document 3: Meeting notes from 2024", "Meeting minutes"},
	}

	var sourceCids []cid.Cid
	for _, data := range testData {
		cidResult, err := source.PutV1Cid(ctx, []byte(data.content), nil)
		if err != nil {
			log.Fatal(err)
		}
		sourceCids = append(sourceCids, cidResult)
		fmt.Printf("   📝 Added: %s -> %s\n", data.desc, cidResult.String()[:20]+"...")
	}

	// Create target (file) backend
	target, err := persistent.New(persistent.File, filepath.Join(baseDir, "migration"))
	if err != nil {
		log.Fatal(err)
	}
	defer target.Close()

	// Migration process
	fmt.Printf("\n🔄 Migrating %d blocks...\n", len(sourceCids))
	start := time.Now()

	migrated := 0
	for _, cidToMigrate := range sourceCids {
		// Read from source
		data, err := source.GetRaw(ctx, cidToMigrate)
		if err != nil {
			fmt.Printf("   ❌ Failed to read %s: %v\n", cidToMigrate.String(), err)
			continue
		}

		// Write to target
		targetCid, err := target.PutV1Cid(ctx, data, nil)
		if err != nil {
			fmt.Printf("   ❌ Failed to write %s: %v\n", cidToMigrate.String(), err)
			continue
		}

		// Verify CID consistency
		if !cidToMigrate.Equals(targetCid) {
			fmt.Printf("   ❌ CID mismatch: %s != %s\n", cidToMigrate.String(), targetCid.String())
			continue
		}

		migrated++
		fmt.Printf("   ✅ Migrated: %s\n", cidToMigrate.String()[:20]+"...")
	}

	migrationTime := time.Since(start)
	fmt.Printf("\n📊 Migration complete: %d/%d blocks in %v\n", migrated, len(sourceCids), migrationTime)

	// Verify migration by reading from target
	fmt.Printf("\n🔍 Verification: Reading from target...\n")
	for i, cidToVerify := range sourceCids {
		_, err := target.GetRaw(ctx, cidToVerify)
		if err != nil {
			fmt.Printf("   ❌ Verification failed for %s: %v\n", cidToVerify.String(), err)
		} else {
			fmt.Printf("   ✅ Verified: %s\n", testData[i].desc)
		}
	}
}

func demonstratePersistence(ctx context.Context, baseDir string) {
	persistentPath := filepath.Join(baseDir, "persistence_test")
	testData := []byte("This data should survive between sessions")

	fmt.Printf("Testing data persistence with File backend...\n")

	// Session 1: Store data
	fmt.Printf("\n📝 Session 1: Storing data...\n")
	p1, err := persistent.New(persistent.File, persistentPath)
	if err != nil {
		log.Fatal(err)
	}

	storedCid, err := p1.PutV1Cid(ctx, testData, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✅ Stored: %s\n", storedCid.String())

	p1.Close() // Simulate application shutdown

	// Session 2: Retrieve data
	fmt.Printf("\n🔍 Session 2: Retrieving data after restart...\n")
	p2, err := persistent.New(persistent.File, persistentPath)
	if err != nil {
		log.Fatal(err)
	}
	defer p2.Close()

	// Check if data exists
	exists, err := p2.Has(ctx, storedCid)
	if err != nil {
		log.Fatal(err)
	}

	if exists {
		retrievedData, err := p2.GetRaw(ctx, storedCid)
		if err != nil {
			fmt.Printf("   ❌ Failed to retrieve: %v\n", err)
		} else {
			fmt.Printf("   ✅ Retrieved: %s\n", string(retrievedData))
			fmt.Printf("   ✅ Data persisted across sessions!\n")
		}
	} else {
		fmt.Printf("   ❌ Data not found after restart\n")
	}

	// Show storage directory
	if dirExists(persistentPath) {
		fmt.Printf("   📁 Persistent storage: %s\n", persistentPath)

		// List some files in the storage directory
		entries, err := os.ReadDir(persistentPath)
		if err == nil && len(entries) > 0 {
			fmt.Printf("   📂 Storage contains %d entries\n", len(entries))
		}
	}
}

func displaySelectionGuide() {
	fmt.Printf("🎯 Backend Selection Guide:\n\n")

	scenarios := []struct {
		scenario string
		backend  string
		reason   string
	}{
		{
			"Unit Testing & Development",
			"Memory",
			"Fastest startup, no cleanup needed",
		},
		{
			"Small Applications (<1GB)",
			"File",
			"Simple, debuggable, portable",
		},
		{
			"Write-Heavy Workloads",
			"BadgerDB",
			"LSM-tree optimized for writes",
		},
		{
			"Large Scale Production (>10GB)",
			"PebbleDB",
			"Best overall performance and stability",
		},
		{
			"Embedded Applications",
			"BadgerDB",
			"No external dependencies",
		},
		{
			"High Reliability Requirements",
			"PebbleDB",
			"Battle-tested in CockroachDB",
		},
	}

	for _, s := range scenarios {
		fmt.Printf("📋 %s\n", s.scenario)
		fmt.Printf("   👉 Recommended: %s\n", s.backend)
		fmt.Printf("   💡 Reason: %s\n\n", s.reason)
	}

	fmt.Printf("⚠️  Important Considerations:\n")
	fmt.Printf("   • Memory: Data lost on restart\n")
	fmt.Printf("   • File: Limited concurrent access\n")
	fmt.Printf("   • BadgerDB: Requires occasional GC\n")
	fmt.Printf("   • PebbleDB: More memory usage\n")
}

func demonstrateStorageEfficiency(ctx context.Context, baseDir string) {
	// Test data with different characteristics
	testDataSets := []struct {
		name string
		data []byte
		desc string
	}{
		{"Small text", []byte("Hello World"), "Small text (11 bytes)"},
		{"JSON data", []byte(`{"name":"test","data":"compressed content","numbers":[1,2,3,4,5]}`), "JSON structure (70 bytes)"},
		{"Binary data", make([]byte, 1024), "Binary data (1KB)"},
		{"Large text", []byte(generateLargeText(4096)), "Large text (4KB)"},
	}

	// Initialize binary data
	for i := range testDataSets[2].data {
		testDataSets[2].data[i] = byte(i % 256)
	}

	backends := []struct {
		name  string
		pType persistent.PersistentType
		path  string
	}{
		{"File", persistent.File, filepath.Join(baseDir, "eff_file")},
		{"BadgerDB", persistent.Badgerdb, filepath.Join(baseDir, "eff_badger")},
		{"PebbleDB", persistent.Pebbledb, filepath.Join(baseDir, "eff_pebble")},
	}

	fmt.Printf("📊 Storage Efficiency Comparison:\n\n")

	for _, backend := range backends {
		fmt.Printf("🗃️  %s Backend:\n", backend.name)

		// Clean and create backend
		os.RemoveAll(backend.path)
		p, err := persistent.New(backend.pType, backend.path)
		if err != nil {
			fmt.Printf("   ❌ Failed to create: %v\n", err)
			continue
		}

		totalDataSize := 0
		for _, testData := range testDataSets {
			cidResult, err := p.PutV1Cid(ctx, testData.data, nil)
			if err != nil {
				fmt.Printf("   ❌ Failed to store %s: %v\n", testData.name, err)
				continue
			}
			totalDataSize += len(testData.data)
			fmt.Printf("   ✅ %s: %s\n", testData.desc, cidResult.String()[:20]+"...")
		}

		p.Close()

		// Measure storage usage
		if dirExists(backend.path) {
			storageSize := getDirSize(backend.path)
			overhead := float64(storageSize-int64(totalDataSize)) / float64(totalDataSize) * 100

			fmt.Printf("   📏 Data size: %s\n", formatSize(int64(totalDataSize)))
			fmt.Printf("   💾 Storage size: %s\n", formatSize(storageSize))
			fmt.Printf("   📊 Overhead: %.1f%%\n", overhead)
		}
		fmt.Println()
	}
}

// Helper types and functions

type BenchmarkResult struct {
	SetupTime      time.Duration
	TotalWriteTime time.Duration
	TotalReadTime  time.Duration
	AvgWriteTime   time.Duration
	AvgReadTime    time.Duration
}

func benchmarkSingleBackend(ctx context.Context, pType persistent.PersistentType, path string, numOps, blockSize int) BenchmarkResult {
	// Clean up previous test data
	if path != "" {
		os.RemoveAll(path)
	}

	// Setup
	start := time.Now()
	p, err := persistent.New(pType, path)
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()
	setupTime := time.Since(start)

	// Generate test data
	testData := make([][]byte, numOps)
	var cids []cid.Cid

	for i := 0; i < numOps; i++ {
		data := make([]byte, blockSize)
		// Fill with deterministic but varied data
		for j := range data {
			data[j] = byte((i + j) % 256)
		}
		testData[i] = data
	}

	// Write benchmark
	start = time.Now()
	for i, data := range testData {
		cidResult, err := p.PutV1Cid(ctx, data, nil)
		if err != nil {
			log.Fatalf("Write %d failed: %v", i, err)
		}
		cids = append(cids, cidResult)
	}
	totalWriteTime := time.Since(start)

	// Read benchmark
	start = time.Now()
	for i, cidToRead := range cids {
		_, err := p.GetRaw(ctx, cidToRead)
		if err != nil {
			log.Fatalf("Read %d failed: %v", i, err)
		}
	}
	totalReadTime := time.Since(start)

	return BenchmarkResult{
		SetupTime:      setupTime,
		TotalWriteTime: totalWriteTime,
		TotalReadTime:  totalReadTime,
		AvgWriteTime:   totalWriteTime / time.Duration(numOps),
		AvgReadTime:    totalReadTime / time.Duration(numOps),
	}
}

func generateLargeText(size int) string {
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	result := ""
	for len(result) < size {
		result += text
	}
	return result[:size]
}

func cleanupTestData(dir string) {
	fmt.Printf("\n🧹 Cleaning up test data...\n")
	err := os.RemoveAll(dir)
	if err != nil {
		fmt.Printf("   ⚠️ Cleanup warning: %v\n", err)
	} else {
		fmt.Printf("   ✅ Test data cleaned\n")
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
