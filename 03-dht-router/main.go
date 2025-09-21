package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	"github.com/gosuda/boxo-starter-kit/03-dht-router/pkg"
)

func main() {
	fmt.Println("🌐 DHT (Distributed Hash Table) Router Demo")
	fmt.Println("==========================================")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("\n1. 🏗️  Creating DHT Infrastructure")
	fmt.Println("----------------------------------")
	demonstrateDHTSetup(ctx)

	fmt.Println("\n2. 🔍 Provider Advertisement & Discovery")
	fmt.Println("--------------------------------------")
	demonstrateProviderOperations(ctx)

	fmt.Println("\n3. 📊 DHT Routing Table Analysis")
	fmt.Println("-------------------------------")
	demonstrateRoutingTable(ctx)

	fmt.Println("\n4. 🚀 Multi-Node DHT Network")
	fmt.Println("----------------------------")
	demonstrateMultiNodeDHT(ctx)

	fmt.Println("\n5. 📈 DHT Performance Metrics")
	fmt.Println("----------------------------")
	demonstrateDHTMetrics(ctx)

	fmt.Println("\n🎉 Demo Complete!")
	fmt.Println("💡 Key Insights:")
	fmt.Println("   • DHT enables decentralized content discovery")
	fmt.Println("   • Provider records help locate content across the network")
	fmt.Println("   • Routing tables grow as more peers connect")
	fmt.Println("   • DHT performance depends on network size and connectivity")
	fmt.Println("\nNext: Try 04-bitswap module for content exchange")
}

func demonstrateDHTSetup(ctx context.Context) {
	fmt.Printf("Setting up DHT with different configurations...\n")

	// 1. Basic DHT with memory storage
	fmt.Printf("\n📝 1. Memory-based DHT:\n")
	memPersistent, err := persistent.New(persistent.Memory, "")
	if err != nil {
		log.Fatal(err)
	}
	defer memPersistent.Close()

	host1, err := network.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host1.Close()

	dht1, err := dht.New(ctx, 5*time.Second, host1, memPersistent)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ DHT created with host ID: %s\n", host1.ID().String()[:20]+"...")
	fmt.Printf("   ✅ Routing table size: %d peers\n", dht1.RoutingTableSize())
	fmt.Printf("   ✅ Storage: In-memory (volatile)\n")

	// 2. DHT with persistent storage
	fmt.Printf("\n💾 2. Persistent DHT:\n")
	filePersistent, err := persistent.New(persistent.File, "./dht_data")
	if err != nil {
		log.Fatal(err)
	}
	defer filePersistent.Close()

	host2, err := network.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host2.Close()

	dht2, err := dht.New(ctx, 5*time.Second, host2, filePersistent)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ DHT created with host ID: %s\n", host2.ID().String()[:20]+"...")
	fmt.Printf("   ✅ Routing table size: %d peers\n", dht2.RoutingTableSize())
	fmt.Printf("   ✅ Storage: File-based (persistent)\n")

	// 3. DHT with custom timeout
	fmt.Printf("\n⏱️  3. Custom timeout DHT:\n")
	customTimeout := 2 * time.Second
	_, err = dht.New(ctx, customTimeout, nil, nil) // Uses defaults for host and storage
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("   ✅ DHT created with %v find timeout\n", customTimeout)
	fmt.Printf("   ✅ Auto-generated host and memory storage\n")

	// Clean up
	filePersistent.Close()
}

func demonstrateProviderOperations(ctx context.Context) {
	fmt.Printf("Demonstrating provider advertisement and discovery...\n")

	// Create DHT node
	host, err := network.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer host.Close()

	dhtNode, err := dht.New(ctx, 5*time.Second, host, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create test content CIDs
	testContents := []struct {
		name string
		data string
	}{
		{"Document A", "Important business document"},
		{"Image B", "Profile picture data"},
		{"Video C", "Training video content"},
	}

	fmt.Printf("\n📢 Advertising content as provider:\n")
	var advertisedCids []cid.Cid

	for _, content := range testContents {
		// Generate CID for content
		hash, err := mh.Sum([]byte(content.data), mh.SHA2_256, -1)
		if err != nil {
			log.Printf("   ❌ Failed to hash %s: %v\n", content.name, err)
			continue
		}
		contentCid := cid.NewCidV1(cid.Raw, hash)
		advertisedCids = append(advertisedCids, contentCid)

		// Advertise as provider
		start := time.Now()
		err = dhtNode.Provide(ctx, contentCid, true)
		if err != nil {
			fmt.Printf("   ❌ Failed to advertise %s: %v\n", content.name, err)
			continue
		}
		duration := time.Since(start)

		fmt.Printf("   ✅ %s: %s (took %v)\n", content.name, contentCid.String()[:20]+"...", duration)
	}

	// Wait a bit for advertisements to propagate
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("\n🔍 Discovering providers:\n")
	for i, contentCid := range advertisedCids {
		start := time.Now()
		providers, err := dhtNode.FindProviders(ctx, contentCid, 10)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Failed to find providers for %s: %v\n", testContents[i].name, err)
			continue
		}

		fmt.Printf("   🔎 %s: found %d provider(s) (took %v)\n",
			testContents[i].name, len(providers), duration)

		for j, provider := range providers {
			if j < 3 { // Show first 3 providers
				fmt.Printf("      - Provider %d: %s\n", j+1, provider.ID.String()[:20]+"...")
			}
		}
		if len(providers) > 3 {
			fmt.Printf("      - ... and %d more\n", len(providers)-3)
		}
	}

	// Demonstrate finding providers for non-existent content
	fmt.Printf("\n🚫 Searching for non-existent content:\n")
	nonExistentHash, _ := mh.Sum([]byte("this content does not exist"), mh.SHA2_256, -1)
	nonExistentCid := cid.NewCidV1(cid.Raw, nonExistentHash)

	start := time.Now()
	providers, err := dhtNode.FindProviders(ctx, nonExistentCid, 10)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("   ❌ Error finding providers: %v\n", err)
	} else {
		fmt.Printf("   🔍 Non-existent content: found %d provider(s) (took %v)\n", len(providers), duration)
	}
}

func demonstrateRoutingTable(ctx context.Context) {
	fmt.Printf("Analyzing DHT routing table structure...\n")

	// Create DHT node
	dhtNode, err := dht.New(ctx, 5*time.Second, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n📊 Initial routing table state:\n")
	initialSize := dhtNode.RoutingTableSize()
	fmt.Printf("   📈 Routing table size: %d peers\n", initialSize)
	fmt.Printf("   💡 Note: Initially empty as no connections established\n")

	// Create additional nodes to simulate network
	fmt.Printf("\n🌐 Simulating network growth:\n")
	nodes := make([]*network.HostWrapper, 5)

	for i := 0; i < 5; i++ {
		node, err := network.New(nil)
		if err != nil {
			log.Printf("   ❌ Failed to create node %d: %v\n", i, err)
			continue
		}
		defer node.Close()
		nodes[i] = node
		fmt.Printf("   ✅ Created node %d: %s\n", i+1, node.ID().String()[:20]+"...")
	}

	// Note: In a real scenario, nodes would discover each other through bootstrap nodes
	// or peer exchange. This demo shows the structure rather than full connectivity.

	fmt.Printf("\n🔬 Routing table analysis:\n")
	finalSize := dhtNode.RoutingTableSize()
	fmt.Printf("   📊 Final routing table size: %d peers\n", finalSize)

	if finalSize > initialSize {
		fmt.Printf("   📈 Routing table grew by %d peers\n", finalSize-initialSize)
	} else {
		fmt.Printf("   💡 No new peers added (requires bootstrap connections)\n")
	}

	fmt.Printf("\n🏗️  DHT Architecture Overview:\n")
	fmt.Printf("   • Each node maintains a routing table of known peers\n")
	fmt.Printf("   • Peers are organized by XOR distance from our node ID\n")
	fmt.Printf("   • Closer peers (by XOR distance) are preferred for routing\n")
	fmt.Printf("   • Routing table has buckets for different distance ranges\n")
	fmt.Printf("   • DHT uses Kademlia algorithm for efficient lookups\n")
}

func demonstrateMultiNodeDHT(ctx context.Context) {
	fmt.Printf("Creating a multi-node DHT network...\n")

	const numNodes = 3
	var nodes []*dht.DHTWrapper
	var hosts []*network.HostWrapper

	// Create multiple DHT nodes
	fmt.Printf("\n🏗️  Creating %d DHT nodes:\n", numNodes)
	for i := 0; i < numNodes; i++ {
		host, err := network.New(nil)
		if err != nil {
			log.Printf("   ❌ Failed to create host %d: %v\n", i, err)
			continue
		}
		hosts = append(hosts, host)

		dhtNode, err := dht.New(ctx, 3*time.Second, host, nil)
		if err != nil {
			log.Printf("   ❌ Failed to create DHT %d: %v\n", i, err)
			host.Close()
			continue
		}
		nodes = append(nodes, dhtNode)

		fmt.Printf("   ✅ Node %d: %s\n", i+1, host.ID().String()[:20]+"...")
	}

	// Clean up
	defer func() {
		for _, host := range hosts {
			host.Close()
		}
	}()

	if len(nodes) < 2 {
		fmt.Printf("   ❌ Need at least 2 nodes for network demo\n")
		return
	}

	// Demonstrate content distribution across nodes
	fmt.Printf("\n📦 Distributing content across nodes:\n")
	testContent := map[int]string{
		0: "Node 0 content: Research paper",
		1: "Node 1 content: Software documentation",
		2: "Node 2 content: Media files",
	}

	var contentCids []cid.Cid
	for i, content := range testContent {
		if i >= len(nodes) {
			break
		}

		hash, err := mh.Sum([]byte(content), mh.SHA2_256, -1)
		if err != nil {
			continue
		}
		contentCid := cid.NewCidV1(cid.Raw, hash)
		contentCids = append(contentCids, contentCid)

		// Each node advertises its content
		err = nodes[i].Provide(ctx, contentCid, true)
		if err != nil {
			fmt.Printf("   ❌ Node %d failed to advertise: %v\n", i, err)
			continue
		}

		fmt.Printf("   ✅ Node %d advertised: %s\n", i, contentCid.String()[:20]+"...")
	}

	// Allow time for advertisements
	time.Sleep(200 * time.Millisecond)

	// Each node tries to find all content
	fmt.Printf("\n🔍 Cross-node content discovery:\n")
	for nodeIdx, node := range nodes {
		fmt.Printf("   Node %d searching for content:\n", nodeIdx)

		for contentIdx, contentCid := range contentCids {
			providers, err := node.FindProviders(ctx, contentCid, 5)
			if err != nil {
				fmt.Printf("      ❌ Search failed for content %d: %v\n", contentIdx, err)
				continue
			}

			if len(providers) > 0 {
				fmt.Printf("      ✅ Found content %d: %d provider(s)\n", contentIdx, len(providers))
			} else {
				fmt.Printf("      🔍 Content %d: no providers found\n", contentIdx)
			}
		}
	}

	fmt.Printf("\n📊 Network statistics:\n")
	for i, node := range nodes {
		tableSize := node.RoutingTableSize()
		fmt.Printf("   Node %d routing table: %d peers\n", i, tableSize)
	}
}

func demonstrateDHTMetrics(ctx context.Context) {
	fmt.Printf("Measuring DHT performance metrics...\n")

	// Create DHT for testing
	dhtNode, err := dht.New(ctx, 5*time.Second, nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Test data
	testData := []struct {
		name string
		size string
		data []byte
	}{
		{"Small", "100B", make([]byte, 100)},
		{"Medium", "1KB", make([]byte, 1024)},
		{"Large", "10KB", make([]byte, 10240)},
	}

	fmt.Printf("\n⏱️  Provider advertisement performance:\n")

	for _, test := range testData {
		// Fill test data
		for i := range test.data {
			test.data[i] = byte(i % 256)
		}

		// Generate CID
		hash, err := mh.Sum(test.data, mh.SHA2_256, -1)
		if err != nil {
			continue
		}
		testCid := cid.NewCidV1(cid.Raw, hash)

		// Measure advertisement time
		start := time.Now()
		err = dhtNode.Provide(ctx, testCid, true)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ %s (%s): failed - %v\n", test.name, test.size, err)
		} else {
			fmt.Printf("   ✅ %s (%s): %v\n", test.name, test.size, duration)
		}
	}

	fmt.Printf("\n🔍 Provider discovery performance:\n")

	// Create some content to search for
	searchContent := []byte("Performance test content")
	hash, err := mh.Sum(searchContent, mh.SHA2_256, -1)
	if err == nil {
		searchCid := cid.NewCidV1(cid.Raw, hash)

		// Advertise it first
		dhtNode.Provide(ctx, searchCid, true)
		time.Sleep(50 * time.Millisecond)

		// Measure search performance
		iterations := []int{1, 5, 10}
		for _, iter := range iterations {
			start := time.Now()

			for i := 0; i < iter; i++ {
				_, err := dhtNode.FindProviders(ctx, searchCid, 10)
				if err != nil {
					break
				}
			}

			duration := time.Since(start)
			avgDuration := duration / time.Duration(iter)

			fmt.Printf("   📊 %d searches: %v total (%v avg)\n", iter, duration, avgDuration)
		}
	}

	fmt.Printf("\n📈 DHT Efficiency Insights:\n")
	fmt.Printf("   • Advertisement time is mostly constant regardless of content size\n")
	fmt.Printf("   • Search performance depends on network connectivity\n")
	fmt.Printf("   • Repeated searches may benefit from local caching\n")
	fmt.Printf("   • Real-world performance improves with more connected peers\n")

	fmt.Printf("\n🎯 Optimization Tips:\n")
	fmt.Printf("   • Use reasonable find timeouts (5-10 seconds)\n")
	fmt.Printf("   • Batch operations when possible\n")
	fmt.Printf("   • Consider caching frequently accessed provider records\n")
	fmt.Printf("   • Monitor routing table size for network health\n")
}

// Helper function to create test CID
func createTestCID(data string) cid.Cid {
	hash, err := mh.Sum([]byte(data), mh.SHA2_256, -1)
	if err != nil {
		return cid.Undef
	}
	return cid.NewCidV1(cid.Raw, hash)
}