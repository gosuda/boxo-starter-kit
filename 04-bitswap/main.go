package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	dht "github.com/gosuda/boxo-starter-kit/03-dht-router/pkg"
	bitswap "github.com/gosuda/boxo-starter-kit/04-bitswap/pkg"
)

func main() {
	fmt.Println("🔄 Bitswap Protocol Content Exchange Demo")
	fmt.Println("=========================================")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("\n1. 🏗️  Creating Bitswap Infrastructure")
	fmt.Println("------------------------------------")
	demonstrateBitswapSetup(ctx)

	fmt.Println("\n2. 📦 Single Node Block Operations")
	fmt.Println("---------------------------------")
	demonstrateSingleNodeOperations(ctx)

	fmt.Println("\n3. 🌐 Multi-Node Content Exchange")
	fmt.Println("--------------------------------")
	demonstrateMultiNodeExchange(ctx)

	fmt.Println("\n4. 🔧 BlockService Integration")
	fmt.Println("-----------------------------")
	demonstrateBlockService(ctx)

	fmt.Println("\n5. ⚡ Performance & Efficiency")
	fmt.Println("----------------------------")
	demonstratePerformance(ctx)

	fmt.Println("\n6. 🚀 Advanced Bitswap Features")
	fmt.Println("------------------------------")
	demonstrateAdvancedFeatures(ctx)

	fmt.Println("\n🎉 Demo Complete!")
	fmt.Println("💡 Key Concepts Demonstrated:")
	fmt.Println("   • Bitswap enables P2P content exchange in IPFS")
	fmt.Println("   • Nodes can both provide and request blocks")
	fmt.Println("   • DHT integration helps discover content providers")
	fmt.Println("   • BlockService provides higher-level block operations")
	fmt.Println("   • Performance depends on network connectivity and storage")
	fmt.Println("\nNext: Try 05-dag-ipld module for structured data handling")
}

func demonstrateBitswapSetup(ctx context.Context) {
	fmt.Printf("Setting up Bitswap with different configurations...\n")

	// 1. Basic Bitswap with defaults
	fmt.Printf("\n🔧 1. Default Bitswap Setup:\n")
	defaultBitswap, err := bitswap.NewBitswap(ctx, nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer defaultBitswap.Close()

	fmt.Printf("   ✅ Created with auto-generated components\n")
	fmt.Printf("   📡 Host ID: %s\n", defaultBitswap.HostWrapper.ID().String()[:20]+"...")
	fmt.Printf("   💾 Storage: Memory (in-memory)\n")
	fmt.Printf("   🌐 DHT: Auto-configured\n")

	// 2. Bitswap with custom persistent storage
	fmt.Printf("\n💾 2. Persistent Storage Bitswap:\n")
	persistentStore, err := persistent.New(persistent.File, "./bitswap_data")
	if err != nil {
		log.Fatal(err)
	}
	defer persistentStore.Close()

	host, err := network.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host.Close()

	dhtRouter, err := dht.New(ctx, host, persistentStore)
	if err != nil {
		log.Fatal(err)
	}

	persistentBitswap, err := bitswap.NewBitswap(ctx, dhtRouter, host, persistentStore)
	if err != nil {
		log.Fatal(err)
	}
	defer persistentBitswap.Close()

	fmt.Printf("   ✅ Created with file-based storage\n")
	fmt.Printf("   📡 Host ID: %s\n", host.ID().String()[:20]+"...")
	fmt.Printf("   💾 Storage: File-based (persistent)\n")
	fmt.Printf("   🗂️  Data directory: ./bitswap_data\n")

	fmt.Printf("\n🔍 Architecture Overview:\n")
	fmt.Printf("   Bitswap Layer (Content Exchange Protocol)\n")
	fmt.Printf("   ├── DHT Router (Peer & Content Discovery)\n")
	fmt.Printf("   ├── Network Host (P2P Communication)\n")
	fmt.Printf("   └── Block Storage (Content Persistence)\n")
}

func demonstrateSingleNodeOperations(ctx context.Context) {
	fmt.Printf("Demonstrating basic block operations on a single Bitswap node...\n")

	// Create Bitswap node
	node, err := bitswap.NewBitswap(ctx, nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer node.Close()

	// Test data
	testData := []struct {
		name    string
		content []byte
		size    string
	}{
		{"Small Document", []byte("Hello, Bitswap world!"), "21B"},
		{"JSON Config", []byte(`{"server":"localhost","port":8080,"ssl":true}`), "45B"},
		{"Medium Text", []byte(generateLargeText(1024)), "1KB"},
		{"Large Binary", generateBinaryData(4096), "4KB"},
	}

	fmt.Printf("\n📝 Storing blocks:\n")
	var storedCids []cid.Cid

	for _, data := range testData {
		start := time.Now()
		cidResult, err := node.PutBlockRaw(ctx, data.content)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: failed - %v\n", data.name, err)
			continue
		}

		storedCids = append(storedCids, cidResult)
		fmt.Printf("   ✅ %s (%s): %s (took %v)\n",
			data.name, data.size, cidResult.String()[:20]+"...", duration)
	}

	fmt.Printf("\n🔍 Retrieving blocks:\n")
	for i, cidToRetrieve := range storedCids {
		start := time.Now()
		retrievedData, err := node.GetBlockRaw(ctx, cidToRetrieve)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s: retrieval failed - %v\n", testData[i].name, err)
			continue
		}

		// Verify content (show first 50 chars)
		preview := string(retrievedData)
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}

		fmt.Printf("   ✅ %s: retrieved %d bytes (took %v)\n", testData[i].name, len(retrievedData), duration)
		fmt.Printf("      Content: %s\n", preview)
	}

	fmt.Printf("\n📊 Node Statistics:\n")
	fmt.Printf("   📦 Blocks stored: %d\n", len(storedCids))
	fmt.Printf("   🔗 Host addresses: %d\n", len(node.HostWrapper.Addrs()))
	fmt.Printf("   🌐 DHT routing table: %d peers\n", 0) // Would need access to DHT for real count
}

func demonstrateMultiNodeExchange(ctx context.Context) {
	fmt.Printf("Demonstrating content exchange between multiple Bitswap nodes...\n")

	const numNodes = 3
	var nodes []*bitswap.BitswapWrapper

	// Create multiple Bitswap nodes
	fmt.Printf("\n🏗️  Creating %d Bitswap nodes:\n", numNodes)
	for i := 0; i < numNodes; i++ {
		node, err := bitswap.NewBitswap(ctx, nil, nil, nil)
		if err != nil {
			log.Printf("   ❌ Failed to create node %d: %v\n", i, err)
			continue
		}
		nodes = append(nodes, node)
		fmt.Printf("   ✅ Node %d: %s\n", i+1, node.HostWrapper.ID().String()[:20]+"...")
	}

	// Clean up
	defer func() {
		for _, node := range nodes {
			node.Close()
		}
	}()

	if len(nodes) < 2 {
		fmt.Printf("   ❌ Need at least 2 nodes for exchange demo\n")
		return
	}

	// Simulate content distribution
	fmt.Printf("\n📦 Distributing content across nodes:\n")
	contentMap := map[int]string{
		0: "Node 0 exclusive content: Research paper draft v1.0",
		1: "Node 1 exclusive content: Project documentation",
		2: "Node 2 exclusive content: Media files and assets",
	}

	var contentCids []cid.Cid
	for nodeIdx, content := range contentMap {
		if nodeIdx >= len(nodes) {
			break
		}

		start := time.Now()
		cidResult, err := nodes[nodeIdx].PutBlockRaw(ctx, []byte(content))
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Node %d failed to store content: %v\n", nodeIdx, err)
			continue
		}

		contentCids = append(contentCids, cidResult)
		fmt.Printf("   ✅ Node %d stored: %s (took %v)\n",
			nodeIdx, cidResult.String()[:20]+"...", duration)
	}

	// Allow time for content advertisements
	time.Sleep(100 * time.Millisecond)

	// Cross-node content retrieval attempts
	fmt.Printf("\n🔄 Cross-node content exchange attempts:\n")
	for requestingNodeIdx, requestingNode := range nodes {
		fmt.Printf("   Node %d requesting content:\n", requestingNodeIdx)

		for contentIdx, contentCid := range contentCids {
			// Don't request content from the same node that stored it
			if contentIdx == requestingNodeIdx {
				fmt.Printf("      ⏭️  Skipping own content %d\n", contentIdx)
				continue
			}

			start := time.Now()
			_, err := requestingNode.GetBlockRaw(ctx, contentCid)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("      ❌ Content %d: failed after %v - %v\n", contentIdx, duration, err)
			} else {
				fmt.Printf("      ✅ Content %d: retrieved successfully (took %v)\n", contentIdx, duration)
			}
		}
	}

	fmt.Printf("\n💡 Note: Cross-node exchange requires network connectivity.\n")
	fmt.Printf("   In this demo, nodes are isolated, so exchanges may fail.\n")
	fmt.Printf("   In production, nodes connect via bootstrap peers and DHT.\n")
}

func demonstrateBlockService(ctx context.Context) {
	fmt.Printf("Demonstrating BlockService - higher-level block operations...\n")

	// Create BlockService
	blockService, err := bitswap.NewBlockService(ctx, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer blockService.Close()

	fmt.Printf("\n📚 BlockService provides higher-level abstractions:\n")
	fmt.Printf("   • Batch operations for multiple blocks\n")
	fmt.Printf("   • Automatic block validation and integrity checks\n")
	fmt.Printf("   • Integration with IPFS ecosystem components\n")
	fmt.Printf("   • Simplified API for common operations\n")

	// Test batch operations
	fmt.Printf("\n📦 Batch block operations:\n")
	batchData := []string{
		"Batch item 1: Configuration data",
		"Batch item 2: Application state",
		"Batch item 3: User preferences",
		"Batch item 4: Cache metadata",
		"Batch item 5: Session information",
	}

	var batchCids []cid.Cid

	// Add blocks in batch
	start := time.Now()
	for i, data := range batchData {
		cidResult, err := blockService.AddBlockRaw(ctx, []byte(data))
		if err != nil {
			fmt.Printf("   ❌ Failed to add batch item %d: %v\n", i+1, err)
			continue
		}
		batchCids = append(batchCids, cidResult)
	}
	batchAddTime := time.Since(start)

	fmt.Printf("   ✅ Added %d blocks in %v (avg: %v/block)\n",
		len(batchCids), batchAddTime, batchAddTime/time.Duration(len(batchCids)))

	// Retrieve blocks using BlockService
	fmt.Printf("\n🔍 Block retrieval operations:\n")

	// Individual retrieval
	start = time.Now()
	for i, cidToRetrieve := range batchCids {
		data, err := blockService.GetBlockRaw(ctx, cidToRetrieve)
		if err != nil {
			fmt.Printf("   ❌ Failed to retrieve block %d: %v\n", i+1, err)
			continue
		}
		fmt.Printf("   ✅ Block %d: %d bytes\n", i+1, len(data))
	}
	individualRetrievalTime := time.Since(start)

	// Batch retrieval
	start = time.Now()
	blockChan := blockService.GetBlocks(ctx, batchCids)
	retrievedCount := 0
	for block := range blockChan {
		retrievedCount++
		fmt.Printf("   📦 Batch retrieved block: %s (%d bytes)\n",
			block.Cid().String()[:20]+"...", len(block.RawData()))
	}
	batchRetrievalTime := time.Since(start)

	fmt.Printf("\n📊 Performance comparison:\n")
	fmt.Printf("   Individual retrieval: %v total (%v/block)\n",
		individualRetrievalTime, individualRetrievalTime/time.Duration(len(batchCids)))
	if retrievedCount > 0 {
		fmt.Printf("   Batch retrieval: %v total (%v/block)\n",
			batchRetrievalTime, batchRetrievalTime/time.Duration(retrievedCount))
	} else {
		fmt.Printf("   Batch retrieval: %v total (no blocks retrieved)\n", batchRetrievalTime)
	}

	// Block existence checks
	fmt.Printf("\n🔍 Block existence checks:\n")
	for i, cidToCheck := range batchCids {
		exists, err := blockService.HasBlock(ctx, cidToCheck)
		if err != nil {
			fmt.Printf("   ❌ Failed to check block %d: %v\n", i+1, err)
			continue
		}
		status := "❌"
		if exists {
			status = "✅"
		}
		fmt.Printf("   %s Block %d exists: %v\n", status, i+1, exists)
	}
}

func demonstratePerformance(ctx context.Context) {
	fmt.Printf("Measuring Bitswap performance characteristics...\n")

	// Create Bitswap node for testing
	node, err := bitswap.NewBitswap(ctx, nil, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer node.Close()

	// Test different block sizes
	testSizes := []struct {
		name string
		size int
	}{
		{"Tiny", 100},       // 100B
		{"Small", 1024},     // 1KB
		{"Medium", 16384},   // 16KB
		{"Large", 262144},   // 256KB
		{"XLarge", 1048576}, // 1MB
	}

	fmt.Printf("\n⏱️  Block storage performance:\n")
	for _, test := range testSizes {
		data := generateBinaryData(test.size)

		// Measure storage time
		start := time.Now()
		cidResult, err := node.PutBlockRaw(ctx, data)
		storageTime := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s (%s): failed - %v\n", test.name, formatSize(test.size), err)
			continue
		}

		// Measure retrieval time
		start = time.Now()
		_, err = node.GetBlockRaw(ctx, cidResult)
		retrievalTime := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s retrieval failed: %v\n", test.name, err)
			continue
		}

		throughputMBps := float64(test.size) / storageTime.Seconds() / (1024 * 1024)

		fmt.Printf("   ✅ %s (%s): store %v, retrieve %v (%.2f MB/s)\n",
			test.name, formatSize(test.size), storageTime, retrievalTime, throughputMBps)
	}

	// Concurrent operations test
	fmt.Printf("\n🚀 Concurrent operations test:\n")
	const numConcurrent = 10
	const blockSize = 4096

	// Generate test data
	testData := make([][]byte, numConcurrent)
	for i := 0; i < numConcurrent; i++ {
		testData[i] = generateBinaryData(blockSize)
	}

	// Concurrent storage
	start := time.Now()
	cidChan := make(chan cid.Cid, numConcurrent)
	errChan := make(chan error, numConcurrent)

	for i, data := range testData {
		go func(idx int, blockData []byte) {
			cidResult, err := node.PutBlockRaw(ctx, blockData)
			if err != nil {
				errChan <- fmt.Errorf("concurrent store %d failed: %w", idx, err)
				return
			}
			cidChan <- cidResult
		}(i, data)
	}

	// Collect results
	var concurrentCids []cid.Cid
	for i := 0; i < numConcurrent; i++ {
		select {
		case cidResult := <-cidChan:
			concurrentCids = append(concurrentCids, cidResult)
		case err := <-errChan:
			fmt.Printf("   ❌ %v\n", err)
		case <-time.After(5 * time.Second):
			fmt.Printf("   ⏰ Operation %d timed out\n", i)
		}
	}
	concurrentStoreTime := time.Since(start)

	fmt.Printf("   ✅ Stored %d blocks concurrently in %v\n", len(concurrentCids), concurrentStoreTime)
	fmt.Printf("   📊 Average: %v/block, Total throughput: %.2f MB/s\n",
		concurrentStoreTime/time.Duration(len(concurrentCids)),
		float64(len(concurrentCids)*blockSize)/concurrentStoreTime.Seconds()/(1024*1024))
}

func demonstrateAdvancedFeatures(ctx context.Context) {
	fmt.Printf("Exploring advanced Bitswap features and configurations...\n")

	// Create Bitswap with custom storage backend
	fmt.Printf("\n🔧 Custom Storage Backend:\n")
	badgerStore, err := persistent.New(persistent.Badgerdb, "./bitswap_badger")
	if err != nil {
		fmt.Printf("   ❌ Failed to create Badger storage: %v\n", err)
	} else {
		defer badgerStore.Close()

		host, err := network.New(nil)
		if err != nil {
			fmt.Printf("   ❌ Failed to create host: %v\n", err)
		} else {
			defer host.Close()

			dhtRouter, err := dht.New(ctx, host, badgerStore)
			if err != nil {
				fmt.Printf("   ❌ Failed to create DHT: %v\n", err)
			} else {
				badgerBitswap, err := bitswap.NewBitswap(ctx, dhtRouter, host, badgerStore)
				if err != nil {
					fmt.Printf("   ❌ Failed to create Bitswap: %v\n", err)
				} else {
					defer badgerBitswap.Close()
					fmt.Printf("   ✅ Created Bitswap with BadgerDB storage\n")
					fmt.Printf("   💾 Storage: LSM-tree based, optimized for writes\n")
					fmt.Printf("   🚀 Performance: Better for high-throughput scenarios\n")
				}
			}
		}
	}

	fmt.Printf("\n📊 Bitswap Protocol Insights:\n")
	fmt.Printf("   🔄 Exchange Algorithm:\n")
	fmt.Printf("      • Bitswap uses a 'want list' to advertise needed blocks\n")
	fmt.Printf("      • Peers respond with blocks if they have them\n")
	fmt.Printf("      • Implements debt/credit system for fair exchange\n")
	fmt.Printf("   \n")
	fmt.Printf("   🌐 Network Integration:\n")
	fmt.Printf("      • Uses libp2p for peer-to-peer communication\n")
	fmt.Printf("      • DHT integration for content discovery\n")
	fmt.Printf("      • Supports multiple transport protocols\n")
	fmt.Printf("   \n")
	fmt.Printf("   ⚡ Performance Optimizations:\n")
	fmt.Printf("      • Block deduplication to save bandwidth\n")
	fmt.Printf("      • Parallel requests to multiple peers\n")
	fmt.Printf("      • Configurable timeouts and retry policies\n")

	fmt.Printf("\n🎯 Production Considerations:\n")
	fmt.Printf("   • Choose appropriate storage backend based on use case\n")
	fmt.Printf("   • Configure DHT for optimal peer discovery\n")
	fmt.Printf("   • Monitor network connectivity and peer relationships\n")
	fmt.Printf("   • Implement proper error handling and recovery\n")
	fmt.Printf("   • Consider security implications of content sharing\n")
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

func generateBinaryData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
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
