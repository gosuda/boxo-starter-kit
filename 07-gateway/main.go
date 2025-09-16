package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ipfs/boxo/files"

	dag "github.com/gosuda/boxo-starter-kit/04-dag-ipld/pkg"
	unixfs "github.com/gosuda/boxo-starter-kit/05-unixfs-car/pkg"
	gateway "github.com/gosuda/boxo-starter-kit/07-gateway/pkg"
)

func main() {
	fmt.Println("=== IPFS HTTP Gateway Demo ===")

	ctx := context.Background()

	// Demo 1: Create storage and UnixFS system
	fmt.Println("\n1. Setting up storage and UnixFS system:")

	// Create DAG wrapper
	dagWrapper, err := dag.NewIpldWrapper(nil, nil)
	if err != nil {
		log.Fatalf("Failed to create DAG wrapper: %v", err)
	}
	defer dagWrapper.BlockServiceWrapper.Close()

	// Create UnixFS system
	unixfsSystem, err := unixfs.New(256*1024, nil) // 256KB chunks
	if err != nil {
		log.Fatalf("Failed to create UnixFS system: %v", err)
	}
	defer unixfsSystem.BlockServiceWrapper.Close()

	fmt.Printf("   ✅ Storage and UnixFS system ready\n")

	// Demo 2: Add some sample content
	fmt.Println("\n2. Adding sample content:")
	sampleFiles := createSampleContent(ctx, unixfsSystem)

	// Demo 3: Create and start gateway
	fmt.Println("\n3. Starting HTTP Gateway:")

	config := gateway.GatewayConfig{
		Port: 8080,
	}
	gw := gateway.NewGateway(dagWrapper, unixfsSystem, config)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start gateway in goroutine
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("Gateway error: %v", err)
		}
	}()

	// Give gateway time to start
	time.Sleep(500 * time.Millisecond)

	// Demo 4: Show usage examples
	showUsageExamples(sampleFiles)

	// Demo 5: Wait for shutdown signal
	fmt.Println("\n5. Gateway is running! Press Ctrl+C to stop...")
	<-sigChan

	fmt.Println("\n📤 Shutting down gateway...")
	if err := gw.Stop(); err != nil {
		log.Printf("Error stopping gateway: %v", err)
	}

	fmt.Println("=== Demo completed! ===")
}

func createSampleContent(ctx context.Context, unixfsSystem *unixfs.UnixFsWrapper) map[string]string {
	sampleFiles := make(map[string]string)

	// Create various types of content
	contents := map[string][]byte{
		"hello.txt": []byte("Hello, IPFS Gateway!\nThis is a sample text file."),
		"data.json": []byte(`{
  "message": "Hello from IPFS!",
  "timestamp": "2024-01-01T00:00:00Z",
  "data": {
    "numbers": [1, 2, 3, 4, 5],
    "nested": {
      "key": "value"
    }
  }
}`),
		"style.css": []byte(`body {
    font-family: Arial, sans-serif;
    max-width: 800px;
    margin: 50px auto;
    padding: 20px;
}

.header {
    text-align: center;
    color: #333;
    border-bottom: 2px solid #0066cc;
    padding-bottom: 20px;
}

.content {
    margin-top: 30px;
    line-height: 1.6;
}`),
		"index.html": []byte(`<!DOCTYPE html>
<html>
<head>
    <title>IPFS Sample Page</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <div class="header">
        <h1>🌐 Welcome to IPFS!</h1>
        <p>This page is served from IPFS via HTTP Gateway</p>
    </div>
    <div class="content">
        <h2>Features Demonstrated:</h2>
        <ul>
            <li>HTML content serving</li>
            <li>CSS stylesheets</li>
            <li>JSON data files</li>
            <li>Directory listings</li>
        </ul>
        <p>Try exploring the <a href="./">directory listing</a> to see all files.</p>
    </div>
</body>
</html>`),
	}

	fmt.Printf("   Creating sample files:\n")
	for filename, content := range contents {
		// Create file node from content
		fileReader := strings.NewReader(string(content))
		fileNode := files.NewReaderFile(fileReader)

		cid, err := unixfsSystem.Put(ctx, fileNode)
		if err != nil {
			log.Printf("   ❌ Failed to add %s: %v", filename, err)
			continue
		}
		sampleFiles[filename] = cid.String()
		fmt.Printf("   ✅ %s → %s\n", filename, cid.String()[:20]+"...")
	}

	// Create a sample directory
	dirFiles := map[string][]byte{
		"docs/README.md": []byte("# Sample Directory\n\nThis is a sample directory structure in IPFS.\n"),
		"docs/guide.md":  []byte("# User Guide\n\n## Getting Started\n\n1. First step\n2. Second step\n3. Done!\n"),
		"src/main.go":    []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello from IPFS!\")\n}\n"),
		"src/utils.go":   []byte("package main\n\nfunc helper() string {\n    return \"utility function\"\n}\n"),
	}

	fmt.Printf("   Creating sample directory:\n")
	for path, content := range dirFiles {
		// Create file node from content
		fileReader := strings.NewReader(string(content))
		fileNode := files.NewReaderFile(fileReader)

		cid, err := unixfsSystem.Put(ctx, fileNode)
		if err != nil {
			log.Printf("   ❌ Failed to add %s: %v", path, err)
			continue
		}
		sampleFiles[path] = cid.String()
		fmt.Printf("   ✅ %s → %s\n", path, cid.String()[:20]+"...")
	}

	fmt.Printf("   ✅ Sample content created\n")

	return sampleFiles
}

func showUsageExamples(sampleFiles map[string]string) {
	fmt.Println("\n4. Usage Examples:")
	fmt.Println("   🌐 Gateway is now running at http://localhost:8080")
	fmt.Println()

	fmt.Println("   📄 Individual Files:")
	for filename, cid := range sampleFiles {
		if filename != "root-directory" && !contains(filename, "/") {
			fmt.Printf("      %s: http://localhost:8080/ipfs/%s\n", filename, cid)
		}
	}

	fmt.Println("\n   📁 Directory Structure:")
	for path, cid := range sampleFiles {
		if contains(path, "/") {
			fmt.Printf("      %s: http://localhost:8080/ipfs/%s\n", path, cid)
		}
	}

	fmt.Println("\n   🔧 API Examples:")
	fmt.Println("      Add file: curl -X POST http://localhost:8080/api/v0/add -F \"file=@example.txt\"")
	fmt.Println("      Get stats: curl \"http://localhost:8080/api/v0/object/stat?cid=<CID>\"")

	fmt.Println("\n   💡 Try These:")
	fmt.Println("      • Visit http://localhost:8080 for the gateway homepage")
	fmt.Println("      • Browse directories by clicking links")
	fmt.Println("      • View source of HTML/CSS files")
	fmt.Println("      • Upload new files via API")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
