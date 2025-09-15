package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ipfs/go-cid"

	kubo_api "github.com/gosuda/boxo-starter-kit/99-kubo-api-demo/pkg"
)

// Helper function to parse CID
func ParseCID(cidStr string) (cid.Cid, error) {
	return cid.Parse(cidStr)
}

func main() {
	fmt.Println("=== Kubo HTTP API Demo ===")
	fmt.Println("This demo requires a running Kubo (IPFS) node at http://localhost:5001")

	ctx := context.Background()

	// Demo 1: Connect to Kubo node
	fmt.Println("\n1. Connecting to Kubo node:")

	kuboAPI := kubo_api.NewKuboAPI("") // Use default endpoint

	online, err := kuboAPI.IsOnline(ctx)
	if err != nil {
		fmt.Printf("   ❌ Failed to connect to Kubo node: %v\n", err)
		fmt.Println("   💡 Make sure IPFS daemon is running: 'ipfs daemon'")
		os.Exit(1)
	}

	if !online {
		fmt.Println("   ❌ Kubo node is not accessible")
		os.Exit(1)
	}

	fmt.Printf("   ✅ Connected to Kubo node\n")

	// Demo 2: Get node information
	fmt.Println("\n2. Getting node information:")
	nodeInfo, err := kuboAPI.GetNodeInfo(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to get node info: %v", err)
	} else {
		fmt.Printf("   🆔 Node ID: %s\n", nodeInfo.ID[:12]+"...")
		fmt.Printf("   📍 Addresses: %d addresses\n", len(nodeInfo.Addresses))
		fmt.Printf("   🏷️  Version: %s\n", nodeInfo.Version)
	}

	// Demo 3: Repository statistics
	fmt.Println("\n3. Repository statistics:")
	repoStats, err := kuboAPI.GetRepoStats(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to get repo stats: %v", err)
	} else {
		fmt.Printf("   💾 Repo size: %d bytes (%.2f MB)\n",
			repoStats.RepoSize, float64(repoStats.RepoSize)/1024/1024)
		fmt.Printf("   📦 Objects: %d\n", repoStats.NumObjects)
		fmt.Printf("   💿 Storage max: %d bytes\n", repoStats.StorageMax)
		fmt.Printf("   📁 Repo path: %s\n", repoStats.RepoPath)
	}

	// Demo 4: Add file content
	fmt.Println("\n4. Adding files to IPFS:")

	// Add sample files
	files := map[string][]byte{
		"hello.txt": []byte("Hello from Kubo API Demo!"),
		"readme.md": []byte("# IPFS Demo\n\nThis file was added via the Kubo HTTP API."),
		"data.json": []byte(`{"message": "IPFS rocks!", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`),
		"script.py": []byte("#!/usr/bin/env python3\nprint('Hello from IPFS!')"),
	}

	addedFiles := make(map[string]string) // filename -> CID

	for filename, content := range files {
		cid, err := kuboAPI.AddFile(ctx, filename, content)
		if err != nil {
			log.Printf("   ❌ Failed to add %s: %v", filename, err)
			continue
		}
		addedFiles[filename] = cid.String()
		fmt.Printf("   ✅ Added %s → %s\n", filename, cid.String()[:20]+"...")
	}

	// Demo 5: Retrieve files
	fmt.Println("\n5. Retrieving files from IPFS:")
	for filename, cidStr := range addedFiles {
		if len(cidStr) == 0 {
			continue
		}

		// Parse CID and get file
		cid, err := ParseCID(cidStr)
		if err != nil {
			log.Printf("   ❌ Invalid CID for %s: %v", filename, err)
			continue
		}

		content, err := kuboAPI.GetFile(ctx, cid)
		if err != nil {
			log.Printf("   ❌ Failed to get %s: %v", filename, err)
			continue
		}

		fmt.Printf("   ✅ Retrieved %s (%d bytes)\n", filename, len(content))
		if filename == "hello.txt" {
			fmt.Printf("      Content: %s\n", string(content))
		}
	}

	// Demo 6: Pin management
	fmt.Println("\n6. Pin management:")

	// Pin one of our files
	if helloCID, exists := addedFiles["hello.txt"]; exists && helloCID != "" {
		cid, err := ParseCID(helloCID)
		if err == nil {
			err = kuboAPI.PinAdd(ctx, cid)
			if err != nil {
				log.Printf("   ❌ Failed to pin hello.txt: %v", err)
			} else {
				fmt.Printf("   📌 Pinned hello.txt → %s\n", cid.String()[:20]+"...")
			}
		}
	}

	// List pins
	pins, err := kuboAPI.ListPins(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to list pins: %v", err)
	} else {
		fmt.Printf("   📋 Total pins: %d\n", len(pins))

		// Show first few pins
		count := 0
		for cid, pinInfo := range pins {
			if count >= 3 {
				fmt.Printf("      ... and %d more pins\n", len(pins)-3)
				break
			}
			fmt.Printf("   📌 %s (%s)\n", cid[:20]+"...", pinInfo.Type)
			count++
		}
	}

	// Demo 7: Network peers
	fmt.Println("\n7. Network information:")

	peers, err := kuboAPI.ListConnectedPeers(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to list peers: %v", err)
	} else {
		fmt.Printf("   🌐 Connected peers: %d\n", len(peers))

		// Show first few peers
		for i, peer := range peers {
			if i >= 3 {
				fmt.Printf("      ... and %d more peers\n", len(peers)-3)
				break
			}
			fmt.Printf("   🤝 %s\n", peer.ID[:12]+"...")
		}
	}

	// Get bootstrap peers
	bootstrap, err := kuboAPI.GetBootstrapPeers(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to get bootstrap peers: %v", err)
	} else {
		fmt.Printf("   🚀 Bootstrap peers: %d\n", len(bootstrap))
	}

	// Demo 8: Object statistics
	fmt.Println("\n8. Object information:")

	if helloCID, exists := addedFiles["hello.txt"]; exists && helloCID != "" {
		cid, err := ParseCID(helloCID)
		if err == nil {
			stat, err := kuboAPI.GetObjectStat(ctx, cid)
			if err != nil {
				log.Printf("   ❌ Failed to get object stat: %v", err)
			} else {
				fmt.Printf("   📊 Object statistics for hello.txt:\n")
				fmt.Printf("      Hash: %s\n", stat.Hash[:20]+"...")
				fmt.Printf("      Links: %d\n", stat.NumLinks)
				fmt.Printf("      Block size: %d bytes\n", stat.BlockSize)
				fmt.Printf("      Data size: %d bytes\n", stat.DataSize)
				fmt.Printf("      Cumulative size: %d bytes\n", stat.CumulativeSize)
			}
		}
	}

	// Demo 9: IPNS Keys (if available)
	fmt.Println("\n9. IPNS key management:")

	keys, err := kuboAPI.ListKeys(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to list keys: %v", err)
	} else {
		fmt.Printf("   🔑 Available keys: %d\n", len(keys))
		for _, key := range keys {
			fmt.Printf("   🔑 %s → %s\n", key.Name, key.ID[:12]+"...")
		}
	}

	// Try to create a demo key
	fmt.Printf("   🔨 Creating demo key...\n")
	demoKey, err := kuboAPI.CreateKey(ctx, "demo-key-"+time.Now().Format("150405"), "rsa")
	if err != nil {
		log.Printf("   ❌ Failed to create demo key: %v", err)
	} else {
		fmt.Printf("   ✅ Created key: %s → %s\n", demoKey.Name, demoKey.ID[:12]+"...")

		// Try to publish IPNS record with demo key
		if helloCID, exists := addedFiles["hello.txt"]; exists && helloCID != "" {
			cid, err := ParseCID(helloCID)
			if err == nil {
				publishResult, err := kuboAPI.PublishIPNS(ctx, cid, demoKey.Name, 24*time.Hour)
				if err != nil {
					log.Printf("   ❌ Failed to publish IPNS: %v", err)
				} else {
					fmt.Printf("   📝 Published IPNS record:\n")
					fmt.Printf("      Name: /ipns/%s\n", publishResult.Name[:12]+"...")
					fmt.Printf("      Value: %s\n", publishResult.Value)

					// Try to resolve it back
					resolved, err := kuboAPI.ResolveIPNS(ctx, "/ipns/"+publishResult.Name)
					if err != nil {
						log.Printf("   ❌ Failed to resolve IPNS: %v", err)
					} else {
						fmt.Printf("   ✅ Resolved IPNS: %s\n", resolved)
					}
				}
			}
		}
	}

	// Demo 10: Provider search
	fmt.Println("\n10. Provider search:")

	if helloCID, exists := addedFiles["hello.txt"]; exists && helloCID != "" {
		cid, err := ParseCID(helloCID)
		if err == nil {
			fmt.Printf("   🔍 Finding providers for hello.txt...\n")
			providers, err := kuboAPI.FindProviders(ctx, cid, 3)
			if err != nil {
				log.Printf("   ❌ Failed to find providers: %v", err)
			} else {
				fmt.Printf("   📡 Found %d providers:\n", len(providers))
				for _, provider := range providers {
					fmt.Printf("   🏪 %s\n", provider.ID[:12]+"...")
				}
			}
		}
	}

	// Demo 11: Garbage collection (optional)
	fmt.Println("\n11. Repository maintenance:")

	fmt.Printf("   🗑️  Running garbage collection...\n")
	gcResult, err := kuboAPI.GarbageCollect(ctx)
	if err != nil {
		log.Printf("   ❌ Failed to run GC: %v", err)
	} else {
		fmt.Printf("   ✅ Garbage collection completed\n")
		fmt.Printf("      Removed objects: %d\n", gcResult.TotalRemoved)
		if len(gcResult.RemovedKeys) > 0 && len(gcResult.RemovedKeys) <= 3 {
			for _, key := range gcResult.RemovedKeys {
				fmt.Printf("      🗑️  %s\n", key[:20]+"...")
			}
		} else if len(gcResult.RemovedKeys) > 3 {
			fmt.Printf("      🗑️  %s ... and %d more\n",
				gcResult.RemovedKeys[0][:20]+"...",
				len(gcResult.RemovedKeys)-1)
		}
	}

	fmt.Println("\n=== Demo completed! ===")
	fmt.Println("\nKey Features Demonstrated:")
	fmt.Println("  • Connection to Kubo IPFS node")
	fmt.Println("  • File addition and retrieval")
	fmt.Println("  • Pin management for data persistence")
	fmt.Println("  • Network peer discovery")
	fmt.Println("  • IPNS record publishing and resolution")
	fmt.Println("  • Repository statistics and maintenance")
	fmt.Println("  • Provider search and discovery")
	fmt.Println("  • Object metadata and statistics")
}
