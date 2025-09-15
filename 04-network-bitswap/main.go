package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"

	bitswap "github.com/gosuda/boxo-starter-kit/04-network-bitswap/pkg"
)

func main() {
	fmt.Println("=== Network Bitswap Demo ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Demo 1: Create two bitswap nodes
	fmt.Println("\n1. Creating bitswap nodes:")

	// Create bitswap nodes
	node1, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		log.Fatalf("Failed to create bitswap node 1: %v", err)
	}
	defer node1.Close()

	node2, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		log.Fatalf("Failed to create bitswap node 2: %v", err)
	}
	defer node2.Close()

	fmt.Printf("   ✅ Node 1 created: %s\n", node1.GetID().String()[:12]+"...")
	fmt.Printf("   ✅ Node 2 created: %s\n", node2.GetID().String()[:12]+"...")

	// Demo 2: Connect the nodes
	fmt.Println("\n2. Connecting nodes:")
	node2Addrs := node2.GetFullAddresses()
	if len(node2Addrs) > 0 {
		err = node1.ConnectToPeer(ctx, node2Addrs[0])
		if err != nil {
			log.Printf("Failed to connect nodes: %v", err)
		} else {
			fmt.Printf("   ✅ Node 1 connected to Node 2\n")
		}
	}

	// Wait for connection to establish
	time.Sleep(1 * time.Second)

	// Demo 3: Store content in node 1
	fmt.Println("\n3. Storing content in Node 1:")
	testContent := []byte("Hello, Bitswap World! This is a test message for P2P block exchange.")
	contentCID, err := node1.PutBlock(ctx, testContent)
	if err != nil {
		log.Fatalf("Failed to store content: %v", err)
	}
	fmt.Printf("   ✅ Content stored with CID: %s\n", contentCID.String()[:20]+"...")

	// Demo 4: Retrieve content from node 2 (should fetch from node 1)
	fmt.Println("\n4. Retrieving content from Node 2 (via bitswap):")

	// Give some time for the network to propagate
	time.Sleep(1 * time.Second)

	retrievedContent, err := node2.GetBlock(ctx, contentCID)
	if err != nil {
		log.Printf("Failed to retrieve content: %v", err)
		fmt.Printf("   ❌ Content retrieval failed (this is expected in demo mode)\n")
	} else {
		fmt.Printf("   ✅ Content retrieved: %s\n", string(retrievedContent))
	}

	// Demo 5: Show statistics
	fmt.Println("\n5. Node statistics:")
	stats1 := node1.GetStats()
	stats2 := node2.GetStats()

	fmt.Printf("   Node 1 Stats:\n")
	fmt.Printf("      Blocks sent: %d\n", stats1.BlocksSent)
	fmt.Printf("      Blocks received: %d\n", stats1.BlocksReceived)
	fmt.Printf("      Connected peers: %d\n", stats1.PeersConnected)
	fmt.Printf("      Node ID: %s...\n", stats1.NodeID[:12])

	fmt.Printf("   Node 2 Stats:\n")
	fmt.Printf("      Blocks sent: %d\n", stats2.BlocksSent)
	fmt.Printf("      Blocks received: %d\n", stats2.BlocksReceived)
	fmt.Printf("      Connected peers: %d\n", stats2.PeersConnected)
	fmt.Printf("      Node ID: %s...\n", stats2.NodeID[:12])

	// Demo 6: Multiple block exchange
	fmt.Println("\n6. Multiple block exchange test:")

	var cids []cid.Cid
	testMessages := []string{
		"First message for bitswap exchange",
		"Second message with different content",
		"Third message to test multiple blocks",
	}

	// Store multiple blocks in node 1
	for i, message := range testMessages {
		c, err := node1.PutBlock(ctx, []byte(message))
		if err != nil {
			log.Printf("Failed to store block %d: %v", i, err)
			continue
		}
		cids = append(cids, c)
		fmt.Printf("   ✅ Stored block %d: %s...\n", i+1, c.String()[:20])
	}

	// Try to retrieve from node 2
	fmt.Println("\n   Attempting to retrieve blocks from Node 2:")
	for i, c := range cids {
		content, err := node2.GetBlock(ctx, c)
		if err != nil {
			fmt.Printf("   ❌ Failed to retrieve block %d: %v\n", i+1, err)
		} else {
			fmt.Printf("   ✅ Retrieved block %d: %s\n", i+1, string(content))
		}
	}

	// Demo 7: Final statistics
	fmt.Println("\n7. Final statistics:")
	finalStats1 := node1.GetStats()
	finalStats2 := node2.GetStats()

	fmt.Printf("   📊 Total Exchange Summary:\n")
	fmt.Printf("      Node 1 → Node 2: %d blocks\n", finalStats1.BlocksSent)
	fmt.Printf("      Node 2 ← Node 1: %d blocks\n", finalStats2.BlocksReceived)
	fmt.Printf("      Network connections: %d peers each\n", finalStats1.PeersConnected)

	fmt.Println("\n=== Demo completed! ===")
	fmt.Println("\nNote: Some retrieval operations may fail in this demo environment")
	fmt.Println("due to simplified routing. In a full IPFS network, content")
	fmt.Println("discovery and retrieval would work seamlessly.")
}
