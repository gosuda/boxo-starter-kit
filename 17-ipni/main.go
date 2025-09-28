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

	"github.com/gosuda/boxo-starter-kit/17-ipni/pkg"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multihash"
)

func main() {
	fmt.Println("=== IPNI (InterPlanetary Network Indexer) Demo ===")

	ctx := context.Background()

	// Demo 1: Create IPNI system
	fmt.Println("\n1. Setting up IPNI system:")

	// Create in-memory datastore
	ds := datastore.NewMapDatastore()

	// Create IPNI instance
	ipniInstance, err := ipni.New(ds)
	if err != nil {
		log.Fatalf("Failed to create IPNI: %v", err)
	}
	defer ipniInstance.Close()

	// Start IPNI components
	if err := ipniInstance.Start(ctx); err != nil {
		log.Fatalf("Failed to start IPNI: %v", err)
	}

	// Start PubSub manager for messaging demos
	if err := ipniInstance.PubSub.Start(ctx); err != nil {
		log.Fatalf("Failed to start PubSub: %v", err)
	}

	fmt.Printf("   ✅ IPNI system ready\n")
	fmt.Printf("   📊 Provider ID: %s\n", ipniInstance.Provider.ProviderID())
	fmt.Printf("   🔐 Security Peer ID: %s\n", ipniInstance.Security.GetPeerID())

	// Demo 2: Add sample content to index
	fmt.Println("\n2. Adding sample content to index:")
	sampleCIDs := createSampleContent(ctx, ipniInstance)

	// Demo 3: Demonstrate provider lookup
	fmt.Println("\n3. Demonstrating provider lookup:")
	demonstrateProviderLookup(ipniInstance, sampleCIDs)

	// Demo 4: Test security features
	fmt.Println("\n4. Testing security features:")
	demonstrateSecurityFeatures(ipniInstance, sampleCIDs)

	// Demo 5: Show query planning and ranking
	fmt.Println("\n5. Demonstrating query planning:")
	demonstrateQueryPlanning(ipniInstance, sampleCIDs)

	// Demo 6: Show PubSub messaging
	fmt.Println("\n6. Testing PubSub messaging:")
	demonstratePubSubMessaging(ctx, ipniInstance)

	// Demo 7: Display system metrics
	fmt.Println("\n7. System metrics and monitoring:")
	displaySystemMetrics(ipniInstance)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Show usage examples
	showUsageExamples()

	// Demo 8: Wait for shutdown signal
	fmt.Println("\n8. IPNI system is running! Press Ctrl+C to stop...")
	<-sigChan

	fmt.Println("\n📤 Shutting down IPNI system...")
	fmt.Println("=== Demo completed! ===")
}

func createSampleContent(ctx context.Context, ipniInstance *ipni.IPNI) map[string]cid.Cid {
	sampleCIDs := make(map[string]cid.Cid)
	providerID := ipniInstance.Provider.ProviderID()

	// Create various types of content with different metadata
	contents := map[string]struct {
		data     string
		metadata map[string]string
	}{
		"document.txt": {
			data: "Hello, IPNI World! This is a sample document.",
			metadata: map[string]string{
				"content-type": "text/plain",
				"protocol":     "bitswap",
				"size":         "45",
			},
		},
		"video.mp4": {
			data: "Mock video content for IPNI indexing demonstration",
			metadata: map[string]string{
				"content-type": "video/mp4",
				"protocol":     "http",
				"quality":      "720p",
				"duration":     "120",
			},
		},
		"dataset.json": {
			data: `{"type": "dataset", "records": 1000, "format": "json"}`,
			metadata: map[string]string{
				"content-type": "application/json",
				"protocol":     "graphsync",
				"schema":       "v1.0",
			},
		},
		"archive.car": {
			data: "CAR archive content with multiple blocks",
			metadata: map[string]string{
				"content-type": "application/car",
				"protocol":     "car",
				"blocks":       "500",
			},
		},
	}

	fmt.Printf("   Creating sample content:\n")
	for filename, content := range contents {
		// Compute CID for the content
		hash, err := multihash.Sum([]byte(content.data), multihash.SHA2_256, -1)
		if err != nil {
			log.Printf("   ❌ Failed to compute hash for %s: %v", filename, err)
			continue
		}

		c := cid.NewCidV1(cid.Raw, hash)

		// Store in provider index
		contextID := []byte("demo-context-" + filename)
		err = ipniInstance.Provider.PutCID(providerID, contextID, nil, c)
		if err != nil {
			log.Printf("   ❌ Failed to index %s: %v", filename, err)
			continue
		}

		sampleCIDs[filename] = c
		fmt.Printf("   ✅ %s → %s\n", filename, c.String()[:20]+"...")
	}

	fmt.Printf("   ✅ Sample content indexed\n")
	return sampleCIDs
}

func demonstrateProviderLookup(ipniInstance *ipni.IPNI, sampleCIDs map[string]cid.Cid) {
	fmt.Printf("   Testing provider lookup for indexed content:\n")

	for filename, c := range sampleCIDs {
		providers, found, err := ipniInstance.Provider.GetProvidersByCID(c)
		if err != nil {
			fmt.Printf("   ❌ Error looking up %s: %v\n", filename, err)
			continue
		}

		if found && len(providers) > 0 {
			fmt.Printf("   ✅ %s: Found %d provider(s)\n", filename, len(providers))
			for _, provider := range providers {
				fmt.Printf("      📍 Provider: %s (last seen: %s)\n",
					provider.ProviderID, provider.LastSeen.Format("15:04:05"))
			}
		} else {
			fmt.Printf("   ❌ %s: No providers found\n", filename)
		}
	}

	// Test lookup for non-existent content
	fmt.Printf("   Testing lookup for non-existent content:\n")
	hash, _ := multihash.Sum([]byte("non-existent-content"), multihash.SHA2_256, -1)
	nonExistentCID := cid.NewCidV1(cid.Raw, hash)

	providers, found, err := ipniInstance.Provider.GetProvidersByCID(nonExistentCID)
	if err != nil {
		fmt.Printf("   ❌ Error: %v\n", err)
	} else if !found || len(providers) == 0 {
		fmt.Printf("   ✅ Correctly returned no providers for non-existent content\n")
	}
}

func demonstrateSecurityFeatures(ipniInstance *ipni.IPNI, sampleCIDs map[string]cid.Cid) {
	providerID := ipniInstance.Provider.ProviderID()

	// Test trust scoring
	fmt.Printf("   Testing trust scoring:\n")
	trustScore := ipniInstance.GetTrustScore(providerID)
	fmt.Printf("   🛡️ Provider trust score: %.3f\n", trustScore)

	if ipniInstance.IsProviderTrusted(providerID) {
		fmt.Printf("   ✅ Provider is trusted (above threshold)\n")
	} else {
		fmt.Printf("   ⚠️ Provider trust below threshold\n")
	}

	// Test signed announcements
	fmt.Printf("   Testing signed announcements:\n")

	// Get first CID for announcement
	var firstCID cid.Cid
	for _, c := range sampleCIDs {
		firstCID = c
		break
	}

	metadata := map[string]string{
		"protocol":    "bitswap",
		"version":     "demo-v1",
		"timestamp":   time.Now().Format(time.RFC3339),
		"description": "Sample provider announcement",
	}

	announcement, err := ipniInstance.CreateSignedAnnouncement(
		providerID, []byte("demo-context"), metadata, []cid.Cid{firstCID})
	if err != nil {
		fmt.Printf("   ❌ Failed to create signed announcement: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Created signed announcement\n")
	fmt.Printf("      📝 Signature length: %d bytes\n", len(announcement.Signature))
	fmt.Printf("      🔑 Public key length: %d bytes\n", len(announcement.PublicKey))

	// Verify the announcement
	if ipniInstance.VerifyAnnouncement(announcement) {
		fmt.Printf("   ✅ Signature verification successful\n")
	} else {
		fmt.Printf("   ❌ Signature verification failed\n")
	}

	// Test anti-spam filtering
	fmt.Printf("   Testing anti-spam protection:\n")

	// Simulate rapid requests from the same provider
	allowed := 0
	blocked := 0
	for i := 0; i < 10; i++ {
		if ipniInstance.AntiSpam.CheckRateLimit(providerID) {
			allowed++
		} else {
			blocked++
		}
	}

	fmt.Printf("   📊 Requests: %d allowed, %d blocked\n", allowed, blocked)
	if blocked > 0 {
		fmt.Printf("   ✅ Rate limiting is working\n")
	}
}

func demonstrateQueryPlanning(ipniInstance *ipni.IPNI, sampleCIDs map[string]cid.Cid) {
	// Get first CID for query planning demo
	var queryCID cid.Cid
	for _, c := range sampleCIDs {
		queryCID = c
		break
	}

	// Test different query intents
	queryIntents := []struct {
		name   string
		intent ipni.QueryIntent
	}{
		{
			name: "High Quality Video",
			intent: ipni.QueryIntent{
				PreferredProtocols: []ipni.TransportProtocol{ipni.ProtocolHTTP, ipni.ProtocolCAR},
				MaxProviders:       5,
				RequireHealthy:     true,
				PreferLocal:        false,
			},
		},
		{
			name: "Fast Local Access",
			intent: ipni.QueryIntent{
				PreferredProtocols: []ipni.TransportProtocol{ipni.ProtocolBitswap},
				MaxProviders:       3,
				RequireHealthy:     true,
				PreferLocal:        true,
			},
		},
		{
			name: "Best Availability",
			intent: ipni.QueryIntent{
				PreferredProtocols: []ipni.TransportProtocol{ipni.ProtocolHTTP, ipni.ProtocolBitswap, ipni.ProtocolGraphSync},
				MaxProviders:       10,
				RequireHealthy:     false,
				PreferLocal:        false,
			},
		},
	}

	fmt.Printf("   Testing query planning strategies:\n")
	for _, test := range queryIntents {
		rankedFetchers, found, err := ipniInstance.Planner.RankedFetchersByCID(context.Background(), queryCID, test.intent)
		if err != nil {
			fmt.Printf("   ❌ %s: %v\n", test.name, err)
			continue
		}

		if !found || len(rankedFetchers) == 0 {
			fmt.Printf("   ❌ %s: No providers found\n", test.name)
			continue
		}

		fmt.Printf("   📋 %s:\n", test.name)
		fmt.Printf("      🎯 Found %d ranked provider(s)\n", len(rankedFetchers))

		for i, fetcher := range rankedFetchers {
			if i >= 3 { // Show only top 3
				break
			}
			fmt.Printf("      %d. Score: %.3f, Provider: %s, Protocol: %s\n",
				fetcher.Priority, fetcher.Score, fetcher.Provider.ProviderID, fetcher.Protocol)
		}
	}
}

func demonstratePubSubMessaging(ctx context.Context, ipniInstance *ipni.IPNI) {
	fmt.Printf("   Testing PubSub messaging system:\n")

	// Create a sample provider announcement
	announcement := &ipni.PubSubProviderAnnouncement{
		ProviderID:  ipniInstance.Provider.ProviderID(),
		ContextID:   []byte("demo-pubsub-context"),
		Metadata:    map[string]string{"protocol": "demo", "version": "1.0"},
		Multihashes: []string{"QmSampleHash1", "QmSampleHash2"},
		Protocol:    ipni.ProtocolBitswap,
		Addresses:   []string{"/ip4/127.0.0.1/tcp/4001"},
		TTL:         time.Hour * 24,
	}

	// Publish the announcement
	err := ipniInstance.PubSub.PublishProviderAnnouncement(ctx, announcement)
	if err != nil {
		fmt.Printf("   ❌ Failed to publish announcement: %v\n", err)
		return
	}

	fmt.Printf("   ✅ Published provider announcement\n")
	fmt.Printf("      📢 Provider: %s\n", announcement.ProviderID)
	fmt.Printf("      🏷️ Protocol: %s\n", announcement.Protocol)
	fmt.Printf("      📦 Multihashes: %d\n", len(announcement.Multihashes))

	// Get PubSub metrics
	metrics := ipniInstance.PubSub.GetMetrics()
	fmt.Printf("   📊 PubSub metrics:\n")
	fmt.Printf("      📨 Messages sent: %d\n", metrics.MessagesSent)
	fmt.Printf("      📥 Messages received: %d\n", metrics.MessagesReceived)
	fmt.Printf("      📋 Topics: %d\n", metrics.TopicCount)
	fmt.Printf("      👥 Subscribers: %d\n", metrics.SubscriberCount)

	// Get active topics
	topics := ipniInstance.PubSub.GetTopics()
	fmt.Printf("      🏷️ Active topics: %s\n", strings.Join(topics, ", "))
}

func displaySystemMetrics(ipniInstance *ipni.IPNI) {
	// Get comprehensive system stats
	stats := ipniInstance.GetStats()
	fmt.Printf("   📊 System Statistics:\n")
	fmt.Printf("      👥 Total providers: %d\n", stats.TotalProviders)
	fmt.Printf("      📚 Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("      🔍 Total queries: %d\n", stats.QueryCount)
	fmt.Printf("      🗂️ Total multihashes: %d\n", stats.TotalMultihashes)
	fmt.Printf("      🕐 Last update: %s\n", stats.LastUpdate.Format("15:04:05"))

	// Get comprehensive metrics
	monitoring := ipniInstance.GetMetrics()
	fmt.Printf("   🔍 Monitoring Metrics:\n")
	fmt.Printf("      📈 Query rate: %.2f queries/sec\n", float64(monitoring.QueriesTotal))
	fmt.Printf("      ⚡ Average query latency: %.2fms\n", monitoring.QueryLatencyMS)
	fmt.Printf("      🎯 Cache hit rate: %.1f%%\n", monitoring.CacheHitRate*100)
	fmt.Printf("      💾 Index size: %.1fMB\n", float64(monitoring.IndexSizeBytes)/(1024*1024))
	fmt.Printf("      🌡️ Successful queries: %d/%d\n", monitoring.QueriesSuccessful, monitoring.QueriesTotal)

	// Get subscriber stats
	subscriberStats := ipniInstance.Subscriber.GetSubscriptionStats()
	fmt.Printf("   📡 Subscriber Statistics:\n")
	for key, value := range subscriberStats {
		fmt.Printf("      %s: %v\n", key, value)
	}
}

func showUsageExamples() {
	fmt.Println("\n📖 IPNI Usage Examples:")
	fmt.Println("   🌐 This demo shows key IPNI concepts:")
	fmt.Println()

	fmt.Println("   📄 Content Indexing:")
	fmt.Println("      • Content is identified by cryptographic CIDs")
	fmt.Println("      • Providers register availability for specific content")
	fmt.Println("      • Metadata describes protocols and capabilities")

	fmt.Println("\n   🔍 Provider Discovery:")
	fmt.Println("      • Clients query the index using CIDs")
	fmt.Println("      • IPNI returns ranked list of providers")
	fmt.Println("      • Query planning optimizes provider selection")

	fmt.Println("\n   🔐 Security Features:")
	fmt.Println("      • Ed25519 signatures verify provider announcements")
	fmt.Println("      • Trust scoring prevents malicious providers")
	fmt.Println("      • Rate limiting protects against spam")

	fmt.Println("\n   📡 Real-time Synchronization:")
	fmt.Println("      • PubSub enables instant network updates")
	fmt.Println("      • Advertisement chains provide audit trails")
	fmt.Println("      • Gossip protocols distribute information")

	fmt.Println("\n   🎯 Production Use Cases:")
	fmt.Println("      • Content discovery in IPFS networks")
	fmt.Println("      • Provider routing for distributed storage")
	fmt.Println("      • Decentralized content delivery networks")

	fmt.Println("\n   💡 Try This:")
	fmt.Println("      • Run with different content types")
	fmt.Println("      • Experiment with query strategies")
	fmt.Println("      • Monitor system metrics")
	fmt.Println("      • Test security verification")
}