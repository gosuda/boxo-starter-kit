package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/multiformats/go-multihash"

	indexer "github.com/gosuda/boxo-starter-kit/17-ipni/pkg"
)

func main() {
	fmt.Println("=== Simple IPFS Indexer Demo ===")
	fmt.Println("Demonstrating Provider/Subscriber pattern in 3 simple steps\n")

	ctx := context.Background()

	// Step 1: Create the Index (the "phonebook")
	fmt.Println("📚 Step 1: Creating the Index")
	index := indexer.NewSimpleIndex()
	defer index.Close()
	fmt.Println("   ✅ Index created - this is like a phonebook mapping content to providers\n")

	// Step 2: Create Providers (people who HAVE content)
	fmt.Println("👤 Step 2: Creating Providers")

	// Create libp2p hosts for providers
	provider1Host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		log.Fatal(err)
	}
	defer provider1Host.Close()

	provider2Host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		log.Fatal(err)
	}
	defer provider2Host.Close()

	// Create simple providers
	provider1 := indexer.NewSimpleProvider(provider1Host, index)
	provider2 := indexer.NewSimpleProvider(provider2Host, index)

	fmt.Printf("   ✅ Provider 1: %s\n", provider1Host.ID().String()[:20]+"...")
	fmt.Printf("   ✅ Provider 2: %s\n\n", provider2Host.ID().String()[:20]+"...")

	// Step 3: Providers announce content
	fmt.Println("📢 Step 3: Providers Announce Content")

	// Create some example CIDs
	cid1 := createExampleCID("hello world")
	cid2 := createExampleCID("ipfs rocks")
	cid3 := createExampleCID("decentralized storage")

	// Provider 1 announces: "I have cid1 via bitswap and http"
	err = provider1.Announce(ctx, cid1, []indexer.Protocol{
		indexer.ProtocolBitswap,
		indexer.ProtocolHTTP,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✅ Provider 1 announced: %s\n", cid1.String()[:20]+"...")
	fmt.Println("      Protocols: bitswap, http")

	// Provider 1 also announces cid2
	err = provider1.Announce(ctx, cid2, []indexer.Protocol{
		indexer.ProtocolGraphSync,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✅ Provider 1 announced: %s\n", cid2.String()[:20]+"...")
	fmt.Println("      Protocols: graphsync")

	// Provider 2 announces the same cid1 but with different protocols
	err = provider2.Announce(ctx, cid1, []indexer.Protocol{
		indexer.ProtocolGraphSync,
		indexer.ProtocolHTTP,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✅ Provider 2 announced: %s\n", cid1.String()[:20]+"...")
	fmt.Println("      Protocols: graphsync, http")

	// Provider 2 announces cid3
	err = provider2.Announce(ctx, cid3, []indexer.Protocol{
		indexer.ProtocolBitswap,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✅ Provider 2 announced: %s\n\n", cid3.String()[:20]+"...")
	fmt.Println("      Protocols: bitswap")

	// Step 4: Check Index Stats
	fmt.Println("📊 Step 4: Index Statistics")
	stats := index.Stats()
	fmt.Printf("   Total Content:   %d unique CIDs\n", stats.TotalContent)
	fmt.Printf("   Total Providers: %d providers\n", stats.TotalProviders)
	fmt.Printf("   Total Records:   %d announcements\n\n", stats.TotalRecords)

	// Step 5: Create Subscriber (someone LOOKING for content)
	fmt.Println("🔍 Step 5: Creating Subscriber")
	subscriberHost, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		log.Fatal(err)
	}
	defer subscriberHost.Close()

	subscriber := indexer.NewSimpleSubscriber(subscriberHost, index)
	fmt.Printf("   ✅ Subscriber: %s\n\n", subscriberHost.ID().String()[:20]+"...")

	// Step 6: Subscriber queries for providers
	fmt.Println("❓ Step 6: Finding Providers for Content")

	// Query 1: Who has cid1?
	fmt.Printf("\n   Query: Who has %s?\n", cid1.String()[:20]+"...")
	providers, err := subscriber.FindProviders(ctx, cid1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Answer: %d providers found\n", len(providers))
	for i, p := range providers {
		fmt.Printf("      %d. Provider %s\n", i+1, p.ProviderID.String()[:20]+"...")
		fmt.Printf("         Protocols: %v\n", p.Protocols)
		fmt.Printf("         Addresses: %d multiaddrs\n", len(p.Addresses))
	}

	// Query 2: Who has cid2?
	fmt.Printf("\n   Query: Who has %s?\n", cid2.String()[:20]+"...")
	providers, err = subscriber.FindProviders(ctx, cid2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Answer: %d provider found\n", len(providers))
	for i, p := range providers {
		fmt.Printf("      %d. Provider %s (protocols: %v)\n", i+1, p.ProviderID.String()[:20]+"...", p.Protocols)
	}

	// Query 3: Who has cid1 and supports HTTP?
	fmt.Printf("\n   Query: Who has %s via HTTP?\n", cid1.String()[:20]+"...")
	httpProviders, err := subscriber.FindProvidersByProtocol(ctx, cid1, indexer.ProtocolHTTP)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Answer: %d providers with HTTP\n", len(httpProviders))
	for i, p := range httpProviders {
		fmt.Printf("      %d. Provider %s\n", i+1, p.ProviderID.String()[:20]+"...")
	}

	// Step 7: Get best provider
	fmt.Println("\n🏆 Step 7: Finding Best Provider")
	best, err := subscriber.GetBestProvider(ctx, cid1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Best provider for %s:\n", cid1.String()[:20]+"...")
	fmt.Printf("   Provider: %s\n", best.ProviderID.String()[:20]+"...")
	fmt.Printf("   Protocols: %v\n", best.Protocols)
	fmt.Println("   (Selected based on: most protocols, most recent)")

	// Step 8: Demonstrate TTL and expiration
	fmt.Println("\n⏰ Step 8: Testing TTL (Time To Live)")
	shortTTLCID := createExampleCID("temporary content")

	// Set short TTL for demonstration
	provider1.SetTTL(2 * time.Second)
	err = provider1.Announce(ctx, shortTTLCID, []indexer.Protocol{indexer.ProtocolBitswap})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   ✅ Announced with 2-second TTL: %s\n", shortTTLCID.String()[:20]+"...")

	// Find immediately
	providers, _ = subscriber.FindProviders(ctx, shortTTLCID)
	fmt.Printf("   Found %d provider(s) immediately\n", len(providers))

	// Wait for expiration
	fmt.Println("   Waiting 3 seconds for TTL expiration...")
	time.Sleep(3 * time.Second)

	// Try to find again
	stats = index.Stats()
	fmt.Printf("   Expired records in index: %d\n", stats.ExpiredRecords)

	// Cleanup expired
	removed := index.CleanupExpired()
	fmt.Printf("   ✅ Cleaned up %d expired record(s)\n", removed)

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📋 Summary")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("This simple indexer demonstrates:")
	fmt.Println("  1. Providers announce what content they have")
	fmt.Println("  2. Index stores the mapping (CID -> Providers)")
	fmt.Println("  3. Subscribers query to find who has content")
	fmt.Println("  4. Protocol-specific queries (bitswap, http, graphsync)")
	fmt.Println("  5. TTL-based expiration for freshness")
	fmt.Println("\nThis simplified version: ~750 lines")
	fmt.Println("Previous version (17-ipni-old): 2,600+ lines")
	fmt.Println("Educational value: Easy to understand in 10 minutes!")
	fmt.Println(strings.Repeat("=", 60))
}

// Helper function to create example CIDs
func createExampleCID(data string) cid.Cid {
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}

	c, err := pref.Sum([]byte(data))
	if err != nil {
		log.Fatal(err)
	}

	return c
}
