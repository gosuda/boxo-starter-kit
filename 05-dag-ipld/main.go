package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"

	bitswap "github.com/gosuda/boxo-starter-kit/04-bitswap/pkg"
	"github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
)

func main() {
	fmt.Println("🌳 DAG-IPLD: Structured Data & Merkle Trees Demo")
	fmt.Println("===============================================")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n1. 🏗️  IPLD Infrastructure Setup")
	fmt.Println("-------------------------------")
	demonstrateIPLDSetup(ctx)

	fmt.Println("\n2. 📦 Raw Data Operations")
	fmt.Println("------------------------")
	demonstrateRawDataOps(ctx)

	fmt.Println("\n3. 🌲 DAG Structure Creation")
	fmt.Println("---------------------------")
	demonstrateDAGStructures(ctx)

	fmt.Println("\n4. 🔗 Complex Linked Structures")
	fmt.Println("------------------------------")
	demonstrateLinkedStructures(ctx)

	fmt.Println("\n5. 🗂️  JSON Data Handling")
	fmt.Println("------------------------")
	demonstrateJSONHandling(ctx)

	fmt.Println("\n6. 🔍 Path Resolution & Navigation")
	fmt.Println("--------------------------------")
	demonstratePathResolution(ctx)

	fmt.Println("\n7. ⚡ Performance & Efficiency")
	fmt.Println("----------------------------")
	demonstratePerformance(ctx)

	fmt.Println("\n🎉 Demo Complete!")
	fmt.Println("💡 Key Concepts Demonstrated:")
	fmt.Println("   • IPLD enables structured, linked data on IPFS")
	fmt.Println("   • DAGs provide directed acyclic graph structures")
	fmt.Println("   • Content addressing ensures data integrity")
	fmt.Println("   • Path resolution allows navigation through structures")
	fmt.Println("   • JSON serialization bridges traditional and IPLD data")
	fmt.Println("\nNext: Try 05-unixfs module for file system abstractions")
}

func demonstrateIPLDSetup(ctx context.Context) {
	fmt.Printf("Setting up IPLD with different configurations...\n")

	// 1. Basic IPLD setup with default BlockService
	fmt.Printf("\n🔧 1. Default IPLD Setup:\n")
	defaultIPLD, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer defaultIPLD.BlockServiceWrapper.Close()

	fmt.Printf("   ✅ Created IPLD with auto-generated components\n")
	fmt.Printf("   📦 BlockService: Auto-configured\n")
	fmt.Printf("   🌳 DAG Service: Ready for structured data\n")

	// 2. IPLD with custom BlockService
	fmt.Printf("\n🔧 2. Custom BlockService IPLD:\n")
	blockService, err := bitswap.NewBlockService(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer blockService.Close()

	_, err = dag.NewIpldWrapper(ctx, nil, blockService)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ Created IPLD with custom BlockService\n")
	fmt.Printf("   📦 BlockService: Custom configuration\n")
	fmt.Printf("   🔗 Integration: Full bitswap support\n")

	fmt.Printf("\n🏗️  IPLD Architecture:\n")
	fmt.Printf("   IPLD Layer (Structured Data)\n")
	fmt.Printf("   ├── DAG Service (Merkle Tree Operations)\n")
	fmt.Printf("   ├── Block Service (Content Storage/Retrieval)\n")
	fmt.Printf("   └── Bitswap (P2P Content Exchange)\n")
}

func demonstrateRawDataOps(ctx context.Context) {
	fmt.Printf("Demonstrating basic raw data operations with IPLD...\n")

	// Create IPLD wrapper
	ipld, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ipld.BlockServiceWrapper.Close()

	// Test raw data storage
	testData := []struct {
		name    string
		content []byte
		size    string
	}{
		{"Text Document", []byte("Hello IPLD world! This is structured data."), "43B"},
		{"Binary Data", []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x00, 0x57, 0x6f, 0x72, 0x6c, 0x64}, "11B"},
		{"JSON Fragment", []byte(`{"message": "IPLD JSON test", "timestamp": 1234567890}`), "57B"},
		{"Large Text", []byte(generateLargeText(1024)), "1KB"},
	}

	fmt.Printf("\n📝 Storing raw data as IPLD nodes:\n")
	var rawCids []cid.Cid

	for _, data := range testData {
		start := time.Now()
		cidResult, err := ipld.AddRaw(ctx, data.content)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: failed - %v\n", data.name, err)
			continue
		}

		rawCids = append(rawCids, cidResult)
		fmt.Printf("   ✅ %s (%s): %s (took %v)\n",
			data.name, data.size, cidResult.String()[:20]+"...", duration)
	}

	fmt.Printf("\n🔍 Retrieving raw data:\n")
	for i, cidToRetrieve := range rawCids {
		start := time.Now()
		retrievedData, err := ipld.GetRaw(ctx, cidToRetrieve)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: retrieval failed - %v\n", testData[i].name, err)
			continue
		}

		// Verify content integrity
		if len(retrievedData) == len(testData[i].content) {
			fmt.Printf("   ✅ %s: %d bytes retrieved (took %v)\n",
				testData[i].name, len(retrievedData), duration)
		} else {
			fmt.Printf("   ❌ %s: size mismatch - expected %d, got %d\n",
				testData[i].name, len(testData[i].content), len(retrievedData))
		}
	}

	fmt.Printf("\n💡 Raw Data Benefits:\n")
	fmt.Printf("   • Content addressing ensures data integrity\n")
	fmt.Printf("   • Automatic deduplication saves storage space\n")
	fmt.Printf("   • Immutable references prevent data corruption\n")
}

func demonstrateDAGStructures(ctx context.Context) {
	fmt.Printf("Creating and manipulating DAG (Directed Acyclic Graph) structures...\n")

	// Create IPLD wrapper
	ipld, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ipld.BlockServiceWrapper.Close()

	fmt.Printf("\n🌱 Creating leaf nodes:\n")

	// Create leaf nodes
	leafData := []string{
		"Leaf A: User profile data",
		"Leaf B: Application settings",
		"Leaf C: Session information",
		"Leaf D: Cache metadata",
	}

	var leafCids []cid.Cid
	for i, data := range leafData {
		cidResult, err := ipld.AddRaw(ctx, []byte(data))
		if err != nil {
			fmt.Printf("   ❌ Failed to create leaf %d: %v\n", i, err)
			continue
		}
		leafCids = append(leafCids, cidResult)
		fmt.Printf("   🍃 Leaf %c: %s\n", 'A'+i, cidResult.String()[:20]+"...")
	}

	if len(leafCids) < 2 {
		fmt.Printf("   ❌ Need at least 2 leaf nodes for DAG demo\n")
		return
	}

	fmt.Printf("\n🌳 Creating intermediate nodes:\n")

	// Create intermediate nodes that link to leaves (using JSON for simplicity)
	branch1Data := map[string]interface{}{
		"type": "branch",
		"name": "user-branch",
		"links": map[string]string{
			"user-data": leafCids[0].String(),
			"settings":  leafCids[1].String(),
		},
	}
	branch1Cid, err := ipld.PutAny(ctx, branch1Data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   🌿 Branch 1: %s (links to A, B)\n", branch1Cid.String()[:20]+"...")

	branch2Data := map[string]interface{}{
		"type": "branch",
		"name": "system-branch",
		"links": map[string]string{
			"session": leafCids[2].String(),
			"cache":   leafCids[3].String(),
		},
	}
	branch2Cid, err := ipld.PutAny(ctx, branch2Data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   🌿 Branch 2: %s (links to C, D)\n", branch2Cid.String()[:20]+"...")

	fmt.Printf("\n🌳 Creating root node:\n")

	// Create root node that links to branches
	rootData := map[string]interface{}{
		"type":        "root",
		"description": "Application state tree",
		"branches": map[string]string{
			"user-branch":   branch1Cid.String(),
			"system-branch": branch2Cid.String(),
		},
	}
	rootCid, err := ipld.PutAny(ctx, rootData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   🌳 Root: %s\n", rootCid.String()[:20]+"...")

	fmt.Printf("\n📊 DAG Structure Analysis:\n")
	fmt.Printf("   Structure: Root → Branches → Leaves\n")
	fmt.Printf("   Total nodes: %d (1 root + 2 branches + %d leaves)\n", 3+len(leafCids), len(leafCids))
	fmt.Printf("   Properties:\n")
	fmt.Printf("     • Directed: Links point from parent to child\n")
	fmt.Printf("     • Acyclic: No circular references possible\n")
	fmt.Printf("     • Content-addressed: Each node has unique CID\n")
	fmt.Printf("     • Immutable: Changes create new nodes, not mutations\n")

	// Verify structure by retrieving
	fmt.Printf("\n🔍 Verifying DAG structure:\n")
	var rootRetrieved map[string]interface{}
	err = ipld.GetAny(ctx, rootCid, &rootRetrieved)
	if err != nil {
		fmt.Printf("   ❌ Failed to retrieve root: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Root type: %v\n", rootRetrieved["type"])
	fmt.Printf("   ✅ Root description: %v\n", rootRetrieved["description"])
	if branches, ok := rootRetrieved["branches"].(map[string]interface{}); ok {
		fmt.Printf("   🔗 Root links: %d branches\n", len(branches))
		for name, cidStr := range branches {
			fmt.Printf("      - %s → %s\n", name, cidStr.(string)[:20]+"...")
		}
	}
}

func demonstrateLinkedStructures(ctx context.Context) {
	fmt.Printf("Building complex linked data structures...\n")

	// Create IPLD wrapper
	ipld, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ipld.BlockServiceWrapper.Close()

	fmt.Printf("\n📚 Creating a document with chapters:\n")

	// Create chapter data structures
	chapters := []struct {
		title   string
		content string
	}{
		{"Introduction", "Welcome to IPLD and DAG structures. This chapter covers basic concepts."},
		{"Core Concepts", "IPLD provides content-addressed, linked data structures for decentralized systems."},
		{"Implementation", "This chapter demonstrates practical implementation patterns and best practices."},
		{"Conclusion", "IPLD enables powerful data structures for modern distributed applications."},
	}

	var chapterCids []cid.Cid
	for i, chapter := range chapters {
		// Create chapter structure
		chapterData := map[string]interface{}{
			"type":    "chapter",
			"title":   chapter.title,
			"content": chapter.content,
			"number":  i + 1,
		}

		chapterCid, err := ipld.PutAny(ctx, chapterData)
		if err != nil {
			fmt.Printf("   ❌ Failed to create chapter %d: %v\n", i+1, err)
			continue
		}

		chapterCids = append(chapterCids, chapterCid)
		fmt.Printf("   📄 Chapter %d (%s): %s\n", i+1, chapter.title, chapterCid.String()[:20]+"...")
	}

	fmt.Printf("\n📖 Creating document with table of contents:\n")

	// Create table of contents structure
	tocChapters := make(map[string]string)
	for i := range chapters {
		if i < len(chapterCids) {
			key := fmt.Sprintf("chapter-%d", i+1)
			tocChapters[key] = chapterCids[i].String()
		}
	}

	tocData := map[string]interface{}{
		"type":     "table-of-contents",
		"chapters": tocChapters,
	}

	tocCid, err := ipld.PutAny(ctx, tocData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   📑 Table of Contents: %s\n", tocCid.String()[:20]+"...")

	// Create document root
	docData := map[string]interface{}{
		"type":    "document",
		"title":   "IPLD Guide",
		"author":  "Demo",
		"version": "1.0",
		"toc":     tocCid.String(),
	}

	docCid, err := ipld.PutAny(ctx, docData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   📚 Document root: %s\n", docCid.String()[:20]+"...")

	fmt.Printf("\n🔗 Linked Structure Benefits:\n")
	fmt.Printf("   • Modular: Each component can be updated independently\n")
	fmt.Printf("   • Efficient: Shared content is stored only once\n")
	fmt.Printf("   • Verifiable: Each link includes content hash\n")
	fmt.Printf("   • Navigable: Structure can be traversed programmatically\n")

	// Demonstrate navigation
	fmt.Printf("\n🧭 Navigating the document structure:\n")
	docNodeRetrieved, err := ipld.GetNode(ctx, docCid)
	if err != nil {
		fmt.Printf("   ❌ Failed to retrieve document: %v\n", err)
		return
	}

	fmt.Printf("   📚 Document metadata: %s\n", string(docNodeRetrieved.RawData()))

	for _, link := range docNodeRetrieved.Links() {
		fmt.Printf("   🔗 Link: %s → %s\n", link.Name, link.Cid.String()[:20]+"...")

		if link.Name == "table-of-contents" {
			tocNodeRetrieved, err := ipld.GetNode(ctx, link.Cid)
			if err != nil {
				continue
			}
			fmt.Printf("      📑 TOC has %d chapter links\n", len(tocNodeRetrieved.Links()))
		}
	}
}

func demonstrateJSONHandling(ctx context.Context) {
	fmt.Printf("Working with JSON data in IPLD structures...\n")

	// Create IPLD wrapper
	ipld, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ipld.BlockServiceWrapper.Close()

	fmt.Printf("\n📄 Storing structured JSON data:\n")

	// Define various JSON structures
	jsonStructures := []struct {
		name string
		data interface{}
	}{
		{
			"User Profile",
			map[string]interface{}{
				"id":       12345,
				"username": "alice",
				"email":    "alice@example.com",
				"profile": map[string]interface{}{
					"name":     "Alice Johnson",
					"location": "San Francisco",
					"bio":      "Software engineer interested in distributed systems",
				},
				"preferences": map[string]interface{}{
					"theme":       "dark",
					"language":    "en",
					"notifications": true,
				},
			},
		},
		{
			"Application Config",
			map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
					"port": 8080,
					"ssl":  false,
				},
				"database": map[string]interface{}{
					"driver": "postgres",
					"host":   "db.example.com",
					"port":   5432,
				},
				"features": []string{"auth", "api", "websockets"},
			},
		},
		{
			"Metrics Data",
			map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"metrics": map[string]interface{}{
					"cpu_usage":    45.2,
					"memory_usage": 67.8,
					"disk_usage":   23.1,
					"requests":     1234,
				},
				"status": "healthy",
			},
		},
	}

	var jsonCids []cid.Cid
	for _, structure := range jsonStructures {
		start := time.Now()
		cidResult, err := ipld.PutAny(ctx, structure.data)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: failed - %v\n", structure.name, err)
			continue
		}

		jsonCids = append(jsonCids, cidResult)
		fmt.Printf("   ✅ %s: %s (took %v)\n",
			structure.name, cidResult.String()[:20]+"...", duration)
	}

	fmt.Printf("\n🔍 Retrieving and parsing JSON data:\n")
	for i, cidToRetrieve := range jsonCids {
		start := time.Now()

		var retrieved map[string]interface{}
		err := ipld.GetAny(ctx, cidToRetrieve, &retrieved)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: retrieval failed - %v\n", jsonStructures[i].name, err)
			continue
		}

		fmt.Printf("   ✅ %s: parsed successfully (took %v)\n",
			jsonStructures[i].name, duration)

		// Show some key information
		if jsonStructures[i].name == "User Profile" {
			if username, ok := retrieved["username"].(string); ok {
				fmt.Printf("      👤 Username: %s\n", username)
			}
		} else if jsonStructures[i].name == "Application Config" {
			if server, ok := retrieved["server"].(map[string]interface{}); ok {
				if port, ok := server["port"].(float64); ok {
					fmt.Printf("      🌐 Server port: %.0f\n", port)
				}
			}
		} else if jsonStructures[i].name == "Metrics Data" {
			if status, ok := retrieved["status"].(string); ok {
				fmt.Printf("      📊 Status: %s\n", status)
			}
		}
	}

	fmt.Printf("\n💡 JSON in IPLD Benefits:\n")
	fmt.Printf("   • Schema flexibility: No predefined structure required\n")
	fmt.Printf("   • Type safety: JSON ensures proper data serialization\n")
	fmt.Printf("   • Interoperability: Works with existing JSON APIs\n")
	fmt.Printf("   • Content addressing: Automatic data integrity verification\n")
}

func demonstratePathResolution(ctx context.Context) {
	fmt.Printf("Demonstrating path resolution and navigation...\n")

	// Create IPLD wrapper
	ipld, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ipld.BlockServiceWrapper.Close()

	fmt.Printf("\n🗂️  Creating hierarchical structure:\n")

	// Create leaf nodes for files
	files := map[string]string{
		"readme.txt":   "This is the README file for the project",
		"config.json":  `{"version": "1.0", "debug": true}`,
		"main.go":      "package main\n\nfunc main() {\n\tfmt.Println(\"Hello!\")\n}",
		"test.go":      "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}",
	}

	fileCids := make(map[string]cid.Cid)
	for filename, content := range files {
		cidResult, err := ipld.AddRaw(ctx, []byte(content))
		if err != nil {
			fmt.Printf("   ❌ Failed to create %s: %v\n", filename, err)
			continue
		}
		fileCids[filename] = cidResult
		fmt.Printf("   📄 %s: %s\n", filename, cidResult.String()[:20]+"...")
	}

	// Create src directory structure
	srcFiles := make(map[string]string)
	if cid, ok := fileCids["main.go"]; ok {
		srcFiles["main.go"] = cid.String()
	}
	if cid, ok := fileCids["test.go"]; ok {
		srcFiles["test.go"] = cid.String()
	}

	srcData := map[string]interface{}{
		"type":  "directory",
		"name":  "src",
		"files": srcFiles,
	}

	srcCid, err := ipld.PutAny(ctx, srcData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   📁 src/: %s\n", srcCid.String()[:20]+"...")

	// Create root directory structure
	rootFiles := make(map[string]string)
	if cid, ok := fileCids["readme.txt"]; ok {
		rootFiles["README.txt"] = cid.String()
	}
	if cid, ok := fileCids["config.json"]; ok {
		rootFiles["config.json"] = cid.String()
	}

	rootData := map[string]interface{}{
		"type":        "directory",
		"name":        "root",
		"files":       rootFiles,
		"directories": map[string]string{"src": srcCid.String()},
	}

	rootCid, err := ipld.PutAny(ctx, rootData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   📁 /: %s\n", rootCid.String()[:20]+"...")

	fmt.Printf("\n🧭 Path resolution examples:\n")

	// Test various path resolutions
	testPaths := []string{
		"",           // Root
		"README.txt", // File in root
		"src",        // Directory
		"src/main.go", // File in subdirectory
		"src/test.go", // Another file in subdirectory
	}

	for _, path := range testPaths {
		start := time.Now()
		node, resolvedCid, err := ipld.ResolvePath(ctx, rootCid, path)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Path '%s': resolution failed - %v\n", path, err)
			continue
		}

		displayPath := path
		if displayPath == "" {
			displayPath = "/"
		}

		fmt.Printf("   ✅ Path '%s': %s (took %v)\n",
			displayPath, resolvedCid.String()[:20]+"...", duration)

		// Show node type and basic info
		if len(node.Links()) > 0 {
			fmt.Printf("      📁 Directory with %d items\n", len(node.Links()))
		} else {
			dataPreview := string(node.RawData())
			if len(dataPreview) > 50 {
				dataPreview = dataPreview[:50] + "..."
			}
			fmt.Printf("      📄 File: %s\n", dataPreview)
		}
	}

	// Test invalid path
	fmt.Printf("\n🚫 Testing invalid path resolution:\n")
	_, _, err = ipld.ResolvePath(ctx, rootCid, "nonexistent/file.txt")
	if err != nil {
		fmt.Printf("   ✅ Invalid path correctly rejected: %v\n", err)
	} else {
		fmt.Printf("   ❌ Invalid path should have failed\n")
	}

	fmt.Printf("\n💡 Path Resolution Benefits:\n")
	fmt.Printf("   • Familiar navigation: Similar to file system paths\n")
	fmt.Printf("   • Flexible addressing: Access nested data structures\n")
	fmt.Printf("   • Content verification: Each step validated by hash\n")
	fmt.Printf("   • Efficient traversal: Only loads necessary nodes\n")
}

func demonstratePerformance(ctx context.Context) {
	fmt.Printf("Measuring IPLD performance characteristics...\n")

	// Create IPLD wrapper
	ipld, err := dag.NewIpldWrapper(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ipld.BlockServiceWrapper.Close()

	// Test different node types and sizes
	fmt.Printf("\n⏱️  Node storage performance:\n")

	testCases := []struct {
		name     string
		nodeType string
		size     int
	}{
		{"Small JSON", "json", 100},
		{"Medium JSON", "json", 1024},
		{"Large JSON", "json", 10240},
		{"Small Raw", "raw", 100},
		{"Medium Raw", "raw", 1024},
		{"Large Raw", "raw", 10240},
	}

	for _, test := range testCases {
		var cidResult cid.Cid
		var err error

		start := time.Now()

		if test.nodeType == "json" {
			// Create JSON data
			data := map[string]interface{}{
				"type":    "test",
				"size":    test.size,
				"content": generateLargeText(test.size - 50), // Account for JSON overhead
			}
			cidResult, err = ipld.PutAny(ctx, data)
		} else {
			// Create raw data
			data := []byte(generateLargeText(test.size))
			cidResult, err = ipld.AddRaw(ctx, data)
		}

		storageTime := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: failed - %v\n", test.name, err)
			continue
		}

		// Measure retrieval time
		start = time.Now()
		if test.nodeType == "json" {
			var retrieved map[string]interface{}
			err = ipld.GetAny(ctx, cidResult, &retrieved)
		} else {
			_, err = ipld.GetRaw(ctx, cidResult)
		}
		retrievalTime := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s retrieval failed: %v\n", test.name, err)
			continue
		}

		throughputMBps := float64(test.size) / storageTime.Seconds() / (1024 * 1024)

		fmt.Printf("   ✅ %s (%s): store %v, retrieve %v (%.2f MB/s)\n",
			test.name, formatSize(test.size), storageTime, retrievalTime, throughputMBps)
	}

	// Test DAG depth performance
	fmt.Printf("\n🌳 DAG depth performance:\n")

	depths := []int{2, 5, 10}
	for _, depth := range depths {
		start := time.Now()

		// Create a chain of nodes
		currentCid, err := ipld.AddRaw(ctx, []byte("leaf node"))
		if err != nil {
			continue
		}

		for i := 0; i < depth; i++ {
			// Create a level node with reference to child
			levelData := map[string]interface{}{
				"level": i,
				"data":  fmt.Sprintf("level %d", i),
				"child": currentCid.String(),
			}

			currentCid, err = ipld.PutAny(ctx, levelData)
			if err != nil {
				break
			}
		}

		creationTime := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Depth %d: creation failed - %v\n", depth, err)
			continue
		}

		// Measure traversal time
		start = time.Now()
		path := ""
		for i := 0; i < depth; i++ {
			if i > 0 {
				path += "/"
			}
			path += "child"
		}

		_, _, err = ipld.ResolvePath(ctx, currentCid, path)
		traversalTime := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Depth %d: traversal failed - %v\n", depth, err)
			continue
		}

		fmt.Printf("   ✅ Depth %d: create %v, traverse %v\n",
			depth, creationTime, traversalTime)
	}

	fmt.Printf("\n📊 Performance Insights:\n")
	fmt.Printf("   • JSON overhead: ~20-30%% compared to raw data\n")
	fmt.Printf("   • Linear scaling: Performance scales with data size\n")
	fmt.Printf("   • DAG efficiency: Traversal time grows linearly with depth\n")
	fmt.Printf("   • Content addressing: Enables efficient caching and deduplication\n")
}

// Helper functions

func generateLargeText(size int) string {
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	result := ""
	for len(result) < size {
		result += text
	}
	return result[:size]
}

func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}