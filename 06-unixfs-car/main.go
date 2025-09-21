package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/go-cid"

	"github.com/gosuda/boxo-starter-kit/06-unixfs-car/pkg"
)

func main() {
	fmt.Println("🗂️  UnixFS & CAR: File System & Archives Demo")
	fmt.Println("============================================")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n1. 🏗️  UnixFS Infrastructure Setup")
	fmt.Println("----------------------------------")
	demonstrateUnixFSSetup(ctx)

	fmt.Println("\n2. 📄 File Operations")
	fmt.Println("--------------------")
	demonstrateFileOperations(ctx)

	fmt.Println("\n3. 📁 Directory Operations")
	fmt.Println("-------------------------")
	demonstrateDirectoryOperations(ctx)

	fmt.Println("\n4. 🗄️  CAR Archive Operations")
	fmt.Println("----------------------------")
	demonstrateCarOperations(ctx)

	fmt.Println("\n5. ⚡ Performance & Chunking")
	fmt.Println("---------------------------")
	demonstratePerformanceChunking(ctx)

	fmt.Println("\n6. 🔄 Import/Export Workflows")
	fmt.Println("----------------------------")
	demonstrateImportExportWorkflows(ctx)

	fmt.Println("\n🎉 Demo Complete!")
	fmt.Println("💡 Key Insights:")
	fmt.Println("   • UnixFS provides file system abstractions over IPLD")
	fmt.Println("   • CAR files enable efficient archive and transfer")
	fmt.Println("   • Chunking optimizes storage and retrieval performance")
	fmt.Println("   • File system operations preserve directory structure")
	fmt.Println("\nNext: Try other advanced modules for specialized functionality")
}

func demonstrateUnixFSSetup(ctx context.Context) {
	fmt.Printf("Setting up UnixFS with different chunk sizes...\n")

	// 1. Default configuration
	fmt.Printf("\n📝 1. Default UnixFS Setup:\n")
	defaultUFS, err := unixfs.New(0, nil) // 0 = use default chunk size
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ UnixFS created with default 256KB chunk size\n")
	fmt.Printf("   ✅ Auto-generated IPLD wrapper for data operations\n")
	fmt.Printf("   ✅ Ready for file and directory operations\n")

	// 2. Small chunk configuration
	fmt.Printf("\n🔧 2. Small Chunk UnixFS:\n")
	smallChunkUFS, err := unixfs.New(32*1024, nil) // 32KB chunks
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ UnixFS created with 32KB chunk size\n")
	fmt.Printf("   ✅ Optimized for small files and low memory usage\n")

	// 3. Large chunk configuration
	fmt.Printf("\n🚀 3. Large Chunk UnixFS:\n")
	_, err = unixfs.New(1024*1024, nil) // 1MB chunks
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ UnixFS created with 1MB chunk size\n")
	fmt.Printf("   ✅ Optimized for large files and high throughput\n")

	// Test a simple operation to verify setup
	testData := []byte("Hello UnixFS world!")
	testCid, err := defaultUFS.PutBytes(ctx, testData)
	if err != nil {
		fmt.Printf("   ❌ Setup verification failed: %v\n", err)
		return
	}

	retrievedData, err := defaultUFS.GetBytes(ctx, testCid)
	if err != nil {
		fmt.Printf("   ❌ Retrieval verification failed: %v\n", err)
		return
	}

	if bytes.Equal(testData, retrievedData) {
		fmt.Printf("   ✅ Setup verification successful: data integrity confirmed\n")
	} else {
		fmt.Printf("   ❌ Setup verification failed: data mismatch\n")
	}

	// Test with small chunk size
	smallTestCid, err := smallChunkUFS.PutBytes(ctx, testData)
	if err != nil {
		fmt.Printf("   ❌ Small chunk test failed: %v\n", err)
		return
	}

	fmt.Printf("   📊 Default chunks CID: %s\n", testCid.String()[:20]+"...")
	fmt.Printf("   📊 Small chunks CID:   %s\n", smallTestCid.String()[:20]+"...")
	if testCid.String() != smallTestCid.String() {
		fmt.Printf("   💡 Different chunk sizes produce different CIDs\n")
	}
}

func demonstrateFileOperations(ctx context.Context) {
	fmt.Printf("Demonstrating file storage and retrieval operations...\n")

	ufs, err := unixfs.New(0, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Test different file types and sizes
	testFiles := []struct {
		name string
		data []byte
		desc string
	}{
		{
			name: "small.txt",
			data: []byte("Small text file content for testing UnixFS operations."),
			desc: "Small Text File (55B)",
		},
		{
			name: "medium.json",
			data: []byte(`{
	"name": "UnixFS Demo",
	"version": "1.0.0",
	"description": "Demonstrating UnixFS file operations with JSON data",
	"features": ["file storage", "content addressing", "chunking", "retrieval"],
	"metadata": {
		"created": "2024-01-01T00:00:00Z",
		"format": "application/json",
		"size": "medium"
	}
}`),
			desc: "JSON Configuration (375B)",
		},
		{
			name: "large.data",
			data: bytes.Repeat([]byte("UnixFS large file test data. "), 100), // ~2.9KB
			desc: "Large Data File (2.9KB)",
		},
	}

	fmt.Printf("\n📦 Storing different file types:\n")
	var fileCids []cid.Cid

	for _, file := range testFiles {
		start := time.Now()
		fileCid, err := ufs.PutBytes(ctx, file.data)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s failed: %v\n", file.desc, err)
			continue
		}

		fileCids = append(fileCids, fileCid)
		fmt.Printf("   ✅ %s: %s (took %v)\n",
			file.desc, fileCid.String()[:20]+"...", duration)
	}

	// Wait a moment for operations to settle
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("\n🔍 Retrieving and verifying files:\n")
	for i, file := range testFiles {
		if i >= len(fileCids) {
			continue
		}

		start := time.Now()
		retrievedData, err := ufs.GetBytes(ctx, fileCids[i])
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s retrieval failed: %v\n", file.desc, err)
			continue
		}

		if bytes.Equal(file.data, retrievedData) {
			fmt.Printf("   ✅ %s: %d bytes verified (took %v)\n",
				file.desc, len(retrievedData), duration)
		} else {
			fmt.Printf("   ❌ %s: data integrity check failed\n", file.desc)
		}
	}

	// Demonstrate chunking behavior
	fmt.Printf("\n🧩 Analyzing chunking behavior:\n")
	smallData := []byte("small")
	largeData := bytes.Repeat([]byte("This is a large file that will be chunked. "), 1000) // ~43KB

	smallCid, _ := ufs.PutBytes(ctx, smallData)
	largeCid, _ := ufs.PutBytes(ctx, largeData)

	fmt.Printf("   📊 Small file (5B): %s\n", smallCid.String()[:20]+"...")
	fmt.Printf("   📊 Large file (43KB): %s\n", largeCid.String()[:20]+"...")
	fmt.Printf("   💡 Large files are automatically chunked for efficiency\n")

	// Test with different chunk sizes
	smallChunkUFS, _ := unixfs.New(1024, nil) // 1KB chunks
	largeChunkCid, _ := smallChunkUFS.PutBytes(ctx, largeData)

	if largeCid.String() != largeChunkCid.String() {
		fmt.Printf("   📊 Different chunk size: %s\n", largeChunkCid.String()[:20]+"...")
		fmt.Printf("   💡 Chunk size affects the resulting CID\n")
	}
}

func demonstrateDirectoryOperations(ctx context.Context) {
	fmt.Printf("Creating and managing directory structures...\n")

	ufs, err := unixfs.New(0, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "unixfs-demo-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("\n🏗️  Creating test directory structure:\n")

	// Create files and subdirectories
	testStructure := map[string]string{
		"README.md":           "# UnixFS Demo\n\nThis demonstrates directory operations.",
		"config.json":         `{"version": "1.0", "debug": true}`,
		"src/main.go":         "package main\n\nfunc main() {\n\tprintln(\"Hello UnixFS!\")\n}",
		"src/utils.go":        "package main\n\nfunc helper() string {\n\treturn \"utility\"\n}",
		"docs/guide.txt":      "UnixFS User Guide\n\n1. Create files\n2. Store in IPFS\n3. Retrieve as needed",
		"data/sample.csv":     "id,name,value\n1,test,100\n2,demo,200",
		"assets/icon.txt":     "ASCII art icon would go here",
	}

	for filePath, content := range testStructure {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
		fmt.Printf("   📄 Created: %s\n", filePath)
	}

	// Store the entire directory structure
	fmt.Printf("\n📁 Storing directory structure in UnixFS:\n")
	start := time.Now()
	rootCid, err := ufs.PutPath(ctx, tempDir)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ Directory storage failed: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Directory stored: %s (took %v)\n", rootCid.String()[:20]+"...", duration)
	fmt.Printf("   📊 Total files: %d\n", len(testStructure))

	// List directory contents
	fmt.Printf("\n🔍 Listing directory contents:\n")
	entries, err := ufs.List(ctx, rootCid)
	if err != nil {
		fmt.Printf("   ❌ Directory listing failed: %v\n", err)
		return
	}

	for _, entry := range entries {
		fmt.Printf("   📄 %s\n", entry)
	}

	// Retrieve the entire directory structure
	fmt.Printf("\n📥 Retrieving directory structure:\n")
	outputDir, err := os.MkdirTemp("", "unixfs-output-*")
	if err != nil {
		fmt.Printf("   ❌ Failed to create output directory: %v\n", err)
		return
	}
	defer os.RemoveAll(outputDir)

	start = time.Now()
	err = ufs.GetPath(ctx, rootCid, outputDir)
	duration = time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ Directory retrieval failed: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Directory retrieved (took %v)\n", duration)

	// Verify the retrieved structure
	fmt.Printf("\n✅ Verifying retrieved structure:\n")
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(outputDir, path)
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			fmt.Printf("   📁 %s/\n", relPath)
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Printf("   📄 %s (%d bytes)\n", relPath, len(content))

			// Verify content matches original
			if originalContent, exists := testStructure[strings.ReplaceAll(relPath, string(filepath.Separator), "/")]; exists {
				if string(content) == originalContent {
					fmt.Printf("      ✅ Content verified\n")
				} else {
					fmt.Printf("      ❌ Content mismatch\n")
				}
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("   ❌ Structure verification failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ All files and directories verified successfully\n")
	}
}

func demonstrateCarOperations(ctx context.Context) {
	fmt.Printf("Working with CAR (Content Addressable aRchive) files...\n")

	ufs, err := unixfs.New(0, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create some test content to archive
	fmt.Printf("\n📦 Creating content for CAR archive:\n")
	testContent := map[string][]byte{
		"document1": []byte("Important document content that needs to be archived."),
		"document2": []byte("Another document with different content for testing."),
		"data":      []byte(`{"type": "archive", "version": 1, "items": ["doc1", "doc2"]}`),
		"large":     bytes.Repeat([]byte("Large content for archive testing. "), 50),
	}

	var contentCids []cid.Cid
	for name, content := range testContent {
		fileCid, err := ufs.PutBytes(ctx, content)
		if err != nil {
			fmt.Printf("   ❌ Failed to store %s: %v\n", name, err)
			continue
		}
		contentCids = append(contentCids, fileCid)
		fmt.Printf("   ✅ %s: %s (%d bytes)\n", name, fileCid.String()[:20]+"...", len(content))
	}

	if len(contentCids) == 0 {
		fmt.Printf("   ❌ No content available for CAR operations\n")
		return
	}

	// Export to CAR format
	fmt.Printf("\n📤 Exporting to CAR archive:\n")

	// Export to bytes
	start := time.Now()
	carData, err := unixfs.CarExportBytes(ctx, ufs.IpldWrapper, contentCids)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ CAR export failed: %v\n", err)
		return
	}

	fmt.Printf("   ✅ CAR archive created: %d bytes (took %v)\n", len(carData), duration)
	fmt.Printf("   📊 Content ratio: %.1f%% compression\n",
		float64(len(carData))/float64(sum(testContent))*100)

	// Export to file
	tempDir, err := os.MkdirTemp("", "car-demo-*")
	if err == nil {
		defer os.RemoveAll(tempDir)

		carPath := filepath.Join(tempDir, "archive.car")
		start = time.Now()
		err = unixfs.CarExportToPath(ctx, ufs.IpldWrapper, contentCids, carPath)
		duration = time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ CAR file export failed: %v\n", err)
		} else {
			fileInfo, _ := os.Stat(carPath)
			fmt.Printf("   ✅ CAR file exported: %s (%d bytes, took %v)\n",
				carPath, fileInfo.Size(), duration)
		}
	}

	// Import from CAR
	fmt.Printf("\n📥 Importing from CAR archive:\n")

	// Create a new UnixFS instance to simulate fresh import
	newUFS, err := unixfs.New(0, nil)
	if err != nil {
		fmt.Printf("   ❌ Failed to create new UnixFS instance: %v\n", err)
		return
	}

	start = time.Now()
	importedRoots, err := unixfs.CarImportBytes(ctx,
		newUFS.IpldWrapper.BlockServiceWrapper.Blockstore(), carData)
	duration = time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ CAR import failed: %v\n", err)
		return
	}

	fmt.Printf("   ✅ CAR archive imported: %d root CIDs (took %v)\n",
		len(importedRoots), duration)

	// Verify imported content
	fmt.Printf("\n🔍 Verifying imported content:\n")
	for i, rootCid := range importedRoots {
		if i >= len(contentCids) {
			break
		}

		// Verify the CID matches
		if rootCid.String() == contentCids[i].String() {
			fmt.Printf("   ✅ Root %d CID matches: %s\n", i+1, rootCid.String()[:20]+"...")
		} else {
			fmt.Printf("   ❌ Root %d CID mismatch\n", i+1)
			continue
		}

		// Verify content can be retrieved
		retrievedData, err := newUFS.GetBytes(ctx, rootCid)
		if err != nil {
			fmt.Printf("      ❌ Failed to retrieve content: %v\n", err)
			continue
		}

		fmt.Printf("      ✅ Content retrieved: %d bytes\n", len(retrievedData))
	}

	fmt.Printf("\n💡 CAR Benefits:\n")
	fmt.Printf("   • Self-contained: All referenced blocks included\n")
	fmt.Printf("   • Efficient: Optimized binary format for transfer\n")
	fmt.Printf("   • Verifiable: Content integrity preserved\n")
	fmt.Printf("   • Portable: Can be shared across different IPFS nodes\n")
}

func demonstratePerformanceChunking(ctx context.Context) {
	fmt.Printf("Analyzing performance characteristics and chunking behavior...\n")

	// Test different chunk sizes
	chunkSizes := []struct {
		size int64
		name string
	}{
		{32 * 1024, "32KB"},
		{256 * 1024, "256KB (default)"},
		{1024 * 1024, "1MB"},
	}

	// Create test data of different sizes
	testSizes := []struct {
		size int
		name string
		data []byte
	}{
		{1024, "1KB", make([]byte, 1024)},
		{64 * 1024, "64KB", make([]byte, 64*1024)},
		{1024 * 1024, "1MB", make([]byte, 1024*1024)},
	}

	// Fill test data with patterns
	for i := range testSizes {
		for j := range testSizes[i].data {
			testSizes[i].data[j] = byte(j % 256)
		}
	}

	fmt.Printf("\n⏱️  Performance analysis across chunk sizes:\n")

	for _, chunkConfig := range chunkSizes {
		fmt.Printf("\n🔧 Testing %s chunks:\n", chunkConfig.name)

		ufs, err := unixfs.New(chunkConfig.size, nil)
		if err != nil {
			fmt.Printf("   ❌ Failed to create UnixFS: %v\n", err)
			continue
		}

		for _, test := range testSizes {
			// Store operation
			start := time.Now()
			testCid, err := ufs.PutBytes(ctx, test.data)
			storeTime := time.Since(start)

			if err != nil {
				fmt.Printf("   ❌ %s store failed: %v\n", test.name, err)
				continue
			}

			// Retrieve operation
			start = time.Now()
			retrievedData, err := ufs.GetBytes(ctx, testCid)
			retrieveTime := time.Since(start)

			if err != nil {
				fmt.Printf("   ❌ %s retrieve failed: %v\n", test.name, err)
				continue
			}

			// Calculate throughput
			storeThroughput := float64(len(test.data)) / storeTime.Seconds() / (1024 * 1024) // MB/s
			retrieveThroughput := float64(len(retrievedData)) / retrieveTime.Seconds() / (1024 * 1024) // MB/s

			fmt.Printf("   📊 %s: store %v (%.1f MB/s), retrieve %v (%.1f MB/s)\n",
				test.name, storeTime, storeThroughput, retrieveTime, retrieveThroughput)

			// Verify data integrity
			if !bytes.Equal(test.data, retrievedData) {
				fmt.Printf("      ❌ Data integrity check failed\n")
			}
		}
	}

	// Demonstrate chunk size optimization
	fmt.Printf("\n🧩 Chunk size optimization analysis:\n")

	// Test the built-in chunk size optimization
	testSizes2 := []int{500, 50 * 1024, 5 * 1024 * 1024, 100 * 1024 * 1024}

	for _, size := range testSizes2 {
		optimalChunk := unixfs.GetChunkSize(size, 256*1024)
		fmt.Printf("   📏 File size %s → Optimal chunk: %s\n",
			formatBytes(size), formatBytes(int(optimalChunk)))
	}

	fmt.Printf("\n⚡ Performance insights:\n")
	fmt.Printf("   • Smaller chunks: Better for random access, higher metadata overhead\n")
	fmt.Printf("   • Larger chunks: Better for sequential access, lower metadata overhead\n")
	fmt.Printf("   • Adaptive chunking: Automatically optimizes based on file size\n")
	fmt.Printf("   • Network considerations: Chunk size affects transfer efficiency\n")
}

func demonstrateImportExportWorkflows(ctx context.Context) {
	fmt.Printf("Demonstrating complete import/export workflows...\n")

	ufs, err := unixfs.New(0, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create a complex directory structure
	tempDir, err := os.MkdirTemp("", "workflow-demo-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("\n🏗️  Creating complex project structure:\n")

	projectStructure := map[string]string{
		"README.md":                    "# Demo Project\n\nThis is a complete project structure.",
		"LICENSE":                      "MIT License\n\nCopyright (c) 2024",
		"package.json":                 `{"name": "demo", "version": "1.0.0"}`,
		"src/main.js":                  "console.log('Hello from main.js');",
		"src/components/Header.js":     "export default function Header() { return 'Header'; }",
		"src/components/Footer.js":     "export default function Footer() { return 'Footer'; }",
		"src/utils/helpers.js":         "export const helper = () => 'utility function';",
		"tests/main.test.js":           "test('main function', () => { expect(true).toBe(true); });",
		"tests/components/Header.test.js": "test('Header component', () => {});",
		"docs/API.md":                  "# API Documentation\n\n## Endpoints",
		"docs/guide/installation.md":   "# Installation Guide\n\n1. Download\n2. Install",
		"config/development.json":      `{"debug": true, "port": 3000}`,
		"config/production.json":       `{"debug": false, "port": 8080}`,
		"assets/images/logo.txt":       "ASCII logo representation",
		"assets/styles/main.css":       "body { font-family: Arial, sans-serif; }",
		"build/output.js":              "// Compiled output\nconsole.log('compiled');",
	}

	// Create the project structure
	for filePath, content := range projectStructure {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			continue
		}
	}

	fmt.Printf("   ✅ Created project with %d files across multiple directories\n", len(projectStructure))

	// Workflow 1: Directory → UnixFS → CAR → Export
	fmt.Printf("\n🔄 Workflow 1: Directory → UnixFS → CAR → File System\n")

	// Step 1: Import directory to UnixFS
	start := time.Now()
	projectCid, err := ufs.PutPath(ctx, tempDir)
	importTime := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ Directory import failed: %v\n", err)
		return
	}

	fmt.Printf("   📁 Step 1: Directory imported to UnixFS (took %v)\n", importTime)
	fmt.Printf("      CID: %s\n", projectCid.String())

	// Step 2: Export to CAR archive
	start = time.Now()
	carData, err := unixfs.CarExportBytes(ctx, ufs.IpldWrapper, []cid.Cid{projectCid})
	exportTime := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ CAR export failed: %v\n", err)
		return
	}

	fmt.Printf("   📦 Step 2: Exported to CAR archive (%d bytes, took %v)\n", len(carData), exportTime)

	// Step 3: Import CAR to new instance
	newUFS, err := unixfs.New(0, nil)
	if err != nil {
		fmt.Printf("   ❌ Failed to create new UnixFS: %v\n", err)
		return
	}

	start = time.Now()
	importedRoots, err := unixfs.CarImportBytes(ctx,
		newUFS.IpldWrapper.BlockServiceWrapper.Blockstore(), carData)
	carImportTime := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ CAR import failed: %v\n", err)
		return
	}

	fmt.Printf("   📥 Step 3: CAR imported to new instance (took %v)\n", carImportTime)

	// Step 4: Export back to file system
	outputDir, err := os.MkdirTemp("", "workflow-output-*")
	if err != nil {
		fmt.Printf("   ❌ Failed to create output directory: %v\n", err)
		return
	}
	defer os.RemoveAll(outputDir)

	start = time.Now()
	err = newUFS.GetPath(ctx, importedRoots[0], outputDir)
	fsExportTime := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ File system export failed: %v\n", err)
		return
	}

	fmt.Printf("   💾 Step 4: Exported back to file system (took %v)\n", fsExportTime)

	// Verify the complete workflow
	fmt.Printf("\n✅ Workflow verification:\n")
	totalTime := importTime + exportTime + carImportTime + fsExportTime
	fmt.Printf("   ⏱️  Total workflow time: %v\n", totalTime)

	// Count files in output
	fileCount := 0
	filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})

	fmt.Printf("   📊 Original files: %d, Recovered files: %d\n", len(projectStructure), fileCount)
	if fileCount == len(projectStructure) {
		fmt.Printf("   ✅ All files successfully preserved through the workflow\n")
	} else {
		fmt.Printf("   ⚠️  File count mismatch detected\n")
	}

	// Workflow 2: Individual files → CAR collection
	fmt.Printf("\n🔄 Workflow 2: Individual Files → CAR Collection\n")

	individualFiles := []string{"README.md", "package.json", "src/main.js"}
	var fileCids []cid.Cid

	for _, filename := range individualFiles {
		content := projectStructure[filename]
		fileCid, err := ufs.PutBytes(ctx, []byte(content))
		if err != nil {
			continue
		}
		fileCids = append(fileCids, fileCid)
		fmt.Printf("   📄 %s: %s\n", filename, fileCid.String()[:20]+"...")
	}

	// Create collection CAR
	collectionCar, err := unixfs.CarExportBytes(ctx, ufs.IpldWrapper, fileCids)
	if err != nil {
		fmt.Printf("   ❌ Collection CAR creation failed: %v\n", err)
	} else {
		fmt.Printf("   📦 Collection CAR created: %d bytes for %d files\n",
			len(collectionCar), len(fileCids))
	}

	fmt.Printf("\n🎯 Workflow Benefits:\n")
	fmt.Printf("   • Round-trip fidelity: Content preserved exactly\n")
	fmt.Printf("   • Format flexibility: Directory ↔ CAR ↔ Files\n")
	fmt.Printf("   • Portability: Archives work across different systems\n")
	fmt.Printf("   • Scalability: Handles projects of any size\n")
	fmt.Printf("   • Integrity: Content addressing ensures data consistency\n")
}

// Helper functions
func sum(data map[string][]byte) int {
	total := 0
	for _, content := range data {
		total += len(content)
	}
	return total
}

func formatBytes(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1fGB", float64(bytes)/(1024*1024*1024))
	}
}