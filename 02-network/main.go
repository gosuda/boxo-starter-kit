package main

import (
	"context"
	"fmt"
	"log"
	"time"

	host "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	"github.com/ipfs/go-cid"
)

func main() {
	fmt.Println("=== Network Demo ===")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Demo 1: Create two libp2p nodes
	fmt.Println("\n1. Creating libp2p nodes:")

	// Create libp2p nodes
	node1, err := host.New(&host.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		log.Fatalf("Failed to create libp2p node 1: %v", err)
	}
	defer node1.Close()

	node2, err := host.New(&host.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		log.Fatalf("Failed to create libp2p node 2: %v", err)
	}
	defer node2.Close()

	fmt.Printf("   ✅ Node 1 created: %s\n", node1.ID().String()[:12]+"...")
	fmt.Printf("   ✅ Node 2 created: %s\n", node2.ID().String()[:12]+"...")

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
	testContent := []byte("Hello, libp2p World! This is a test message for P2P block exchange.")
	c, err := node1.Send(ctx, node2.ID(), testContent)
	if err != nil {
		log.Fatalf("Failed to store content: %v", err)
	}
	fmt.Printf("   ✅ Content stored...\n")

	// Demo 4: Retrieve content from node 2 (should fetch from node 1)
	fmt.Println("\n4. Retrieving content from Node 2:")

	// Give some time for the network to propagate
	time.Sleep(1 * time.Second)

	_, retrievedContent, err := node2.Receive(ctx, c)
	if err != nil {
		log.Printf("Failed to retrieve content: %v", err)
		fmt.Printf("   ❌ Content retrieval failed (this is expected in demo mode)\n")
	} else {
		fmt.Printf("   ✅ Content retrieved: %s\n", string(retrievedContent))
	}

	// Demo 5: Multiple block exchange
	fmt.Println("\n5. Multiple block exchange test:")

	var cids []cid.Cid
	testMessages := []string{
		"First message for libp2p exchange",
		"Second message with different content",
		"Third message to test multiple blocks",
	}

	// Store multiple blocks in node 1
	for i, message := range testMessages {
		c, err := node1.Send(ctx, node2.ID(), []byte(message))
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
		_, content, err := node2.Receive(ctx, c)
		if err != nil {
			fmt.Printf("   ❌ Failed to retrieve block %d: %v\n", i+1, err)
		} else {
			fmt.Printf("   ✅ Retrieved block %d: %s\n", i+1, string(content))
		}
	}
}
