package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"

	dag "github.com/gosuda/boxo-starter-kit/04-dag-ipld/pkg"
	ipns "github.com/gosuda/boxo-starter-kit/08-ipns/pkg"
)

func main() {
	fmt.Println("=== IPNS (InterPlanetary Name System) Demo ===")

	ctx := context.Background()

	// Demo 1: Setup IPNS manager
	fmt.Println("\n1. Setting up IPNS manager:")

	// Create DAG wrapper for content storage
	dagWrapper, err := dag.NewIpldWrapper(nil, nil)
	if err != nil {
		log.Fatalf("Failed to create DAG wrapper: %v", err)
	}
	defer dagWrapper.BlockServiceWrapper.Close()

	// Create IPNS manager
	ipnsManager := ipns.NewIPNSManager(dagWrapper)

	fmt.Printf("   ✅ IPNS manager ready\n")

	// Demo 2: Create sample content
	fmt.Println("\n2. Creating sample content:")
	contentCIDs := createSampleContent(ctx, dagWrapper)

	// Demo 3: Generate keys and publish IPNS records
	fmt.Println("\n3. Generating keys and publishing IPNS records:")

	// Generate key for a personal website
	websiteKeyName := "my-website"
	websitePeerID, err := ipnsManager.GenerateKey(ctx, websiteKeyName)
	if err != nil {
		log.Fatalf("Failed to generate website key: %v", err)
	}

	fmt.Printf("   🔑 Generated key '%s' → %s\n", websiteKeyName, websitePeerID.String()[:12]+"...")

	// Publish initial version pointing to home page
	websiteRecord, err := ipnsManager.PublishIPNS(ctx, websiteKeyName, contentCIDs["homepage"], 24*time.Hour)
	if err != nil {
		log.Fatalf("Failed to publish website IPNS: %v", err)
	}

	fmt.Printf("   📝 Published /ipns/%s → %s\n",
		websiteRecord.Name[:12]+"...",
		websiteRecord.Value[:25]+"...")

	// Generate key for a blog
	blogKeyName := "my-blog"
	_, err = ipnsManager.GenerateKey(ctx, blogKeyName)
	if err != nil {
		log.Fatalf("Failed to generate blog key: %v", err)
	}

	// Publish blog pointing to first post
	blogRecord, err := ipnsManager.PublishIPNS(ctx, blogKeyName, contentCIDs["post-v1"], 12*time.Hour)
	if err != nil {
		log.Fatalf("Failed to publish blog IPNS: %v", err)
	}

	fmt.Printf("   📝 Published /ipns/%s → %s\n",
		blogRecord.Name[:12]+"...",
		blogRecord.Value[:25]+"...")

	// Demo 4: Resolve IPNS names
	fmt.Println("\n4. Resolving IPNS names:")
	testIPNSResolution(ctx, ipnsManager, websiteRecord.Name, "website")
	testIPNSResolution(ctx, ipnsManager, blogRecord.Name, "blog")

	// Demo 5: Update IPNS records
	fmt.Println("\n5. Updating IPNS records:")

	// Update website to point to updated homepage
	time.Sleep(1 * time.Second) // Ensure different timestamp
	updatedWebsiteRecord, err := ipnsManager.UpdateIPNS(ctx, websiteKeyName, contentCIDs["homepage-v2"], 24*time.Hour)
	if err != nil {
		log.Printf("Failed to update website: %v", err)
	} else {
		fmt.Printf("   🔄 Updated website (seq %d) → %s\n",
			updatedWebsiteRecord.Sequence,
			updatedWebsiteRecord.Value[:25]+"...")
	}

	// Update blog to point to newer post
	time.Sleep(1 * time.Second) // Ensure different timestamp
	updatedBlogRecord, err := ipnsManager.UpdateIPNS(ctx, blogKeyName, contentCIDs["post-v2"], 12*time.Hour)
	if err != nil {
		log.Printf("Failed to update blog: %v", err)
	} else {
		fmt.Printf("   🔄 Updated blog (seq %d) → %s\n",
			updatedBlogRecord.Sequence,
			updatedBlogRecord.Value[:25]+"...")
	}

	// Demo 6: List all IPNS records
	fmt.Println("\n6. Listing all IPNS records:")
	records, err := ipnsManager.ListIPNSRecords(ctx)
	if err != nil {
		log.Printf("Failed to list records: %v", err)
	} else {
		for i, record := range records {
			fmt.Printf("   📋 Record %d:\n", i+1)
			fmt.Printf("      Name: /ipns/%s\n", record.Name[:12]+"...")
			fmt.Printf("      Value: %s\n", record.Value[:30]+"...")
			fmt.Printf("      Sequence: %d\n", record.Sequence)
			fmt.Printf("      TTL: %d seconds (%.1f hours)\n", record.TTL, float64(record.TTL)/3600)
			fmt.Printf("      Updated: %s ago\n", time.Since(record.UpdatedAt).Round(time.Second))
			fmt.Printf("      Status: %s\n", getRecordStatus(record))
		}
	}

	// Demo 7: IPNS Statistics
	fmt.Println("\n7. IPNS Statistics:")
	stats, err := ipnsManager.GetStats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		fmt.Printf("   📊 IPNS Statistics:\n")
		fmt.Printf("      Total records: %d\n", stats.TotalRecords)
		fmt.Printf("      Active records: %d\n", stats.ActiveRecords)
		fmt.Printf("      Expired records: %d\n", stats.ExpiredRecords)
		fmt.Printf("      Total keys: %d\n", stats.TotalKeys)
		if !stats.OldestRecord.IsZero() {
			fmt.Printf("      Oldest record: %s ago\n", time.Since(stats.OldestRecord).Round(time.Second))
		}
		if !stats.NewestRecord.IsZero() {
			fmt.Printf("      Newest record: %s ago\n", time.Since(stats.NewestRecord).Round(time.Second))
		}
	}

	// Demo 8: Demonstrate name validation
	fmt.Println("\n8. IPNS name validation:")
	testNameValidation(websiteRecord.Name)
	testNameValidation("invalid-name-format")

	// Demo 9: Path formatting
	fmt.Println("\n9. Path formatting examples:")
	fmt.Printf("   Raw peer ID: %s\n", websiteRecord.Name[:20]+"...")
	fmt.Printf("   IPNS path: %s\n", ipns.FormatIPNSPath(websiteRecord.Name)[:25]+"...")
	fmt.Printf("   Resolution: %s → %s\n",
		ipns.FormatIPNSPath(websiteRecord.Name)[:25]+"...",
		websiteRecord.Value[:30]+"...")

	fmt.Println("\n=== Demo completed! ===")
	fmt.Println("\nKey Concepts Demonstrated:")
	fmt.Println("  • IPNS record creation and management")
	fmt.Println("  • Cryptographic key generation")
	fmt.Println("  • Name resolution and updates")
	fmt.Println("  • Record versioning with sequence numbers")
	fmt.Println("  • TTL and expiration handling")
	fmt.Println("  • Statistics and monitoring")
}

func createSampleContent(ctx context.Context, dagWrapper *dag.IpldWrapper) map[string]cid.Cid {
	cids := make(map[string]cid.Cid)

	// Create sample content for demonstration
	contents := map[string]map[string]any{
		"homepage": {
			"type":  "website",
			"title": "My Personal Website",
			"content": map[string]any{
				"header":  "Welcome to My Site",
				"body":    "This is my personal homepage hosted on IPFS.",
				"footer":  "Powered by IPFS and IPNS",
				"version": "1.0",
			},
		},
		"homepage-v2": {
			"type":  "website",
			"title": "My Personal Website - Updated",
			"content": map[string]any{
				"header":  "Welcome to My Updated Site",
				"body":    "This is my updated personal homepage with new content!",
				"footer":  "Powered by IPFS and IPNS",
				"news":    "Check out my new blog posts!",
				"version": "2.0",
			},
		},
		"post-v1": {
			"type":   "blog-post",
			"title":  "My First Blog Post",
			"author": "IPNS Demo",
			"date":   time.Now().Format("2006-01-02"),
			"content": "This is my first blog post published using IPNS. " +
				"IPNS allows me to update content while keeping the same address!",
			"tags": []string{"ipfs", "ipns", "blogging"},
		},
		"post-v2": {
			"type":   "blog-post",
			"title":  "Updated: My First Blog Post",
			"author": "IPNS Demo",
			"date":   time.Now().Format("2006-01-02"),
			"content": "This is my UPDATED blog post! With IPNS, I can modify content " +
				"and publish updates while keeping the same /ipns/ address. " +
				"This demonstrates the power of mutable content in IPFS.",
			"tags":   []string{"ipfs", "ipns", "blogging", "updates"},
			"update": "Added more details about IPNS benefits!",
		},
		"profile": {
			"type": "profile",
			"name": "IPNS Demo User",
			"bio":  "Demonstrating IPNS capabilities",
			"links": map[string]any{
				"website": "/ipns/my-website",
				"blog":    "/ipns/my-blog",
			},
			"interests": []string{"IPFS", "decentralization", "P2P networks"},
		},
	}

	fmt.Printf("   Creating sample content:\n")
	for name, content := range contents {
		c, err := dagWrapper.PutAny(ctx, content)
		if err != nil {
			log.Printf("   ❌ Failed to create %s: %v", name, err)
			continue
		}
		cids[name] = c
		fmt.Printf("   ✅ %s → %s\n", name, c.String()[:20]+"...")
	}

	return cids
}

func testIPNSResolution(ctx context.Context, manager *ipns.IPNSManager, name string, description string) {
	resolved, err := manager.ResolveIPNS(ctx, name)
	if err != nil {
		fmt.Printf("   ❌ Failed to resolve %s: %v\n", description, err)
		return
	}

	fmt.Printf("   ✅ Resolved %s:\n", description)
	fmt.Printf("      /ipns/%s\n", name[:12]+"...")
	fmt.Printf("      ↓\n")
	fmt.Printf("      %s\n", resolved[:35]+"...")
}

func getRecordStatus(record *ipns.IPNSRecord) string {
	now := time.Now()
	expirationTime := record.CreatedAt.Add(time.Duration(record.TTL) * time.Second)

	if now.After(expirationTime) {
		return fmt.Sprintf("❌ EXPIRED (%s ago)",
			now.Sub(expirationTime).Round(time.Second))
	}

	remaining := expirationTime.Sub(now)
	if remaining < time.Hour {
		return fmt.Sprintf("⚠️  EXPIRING SOON (%.0f minutes left)",
			remaining.Minutes())
	}

	return fmt.Sprintf("✅ ACTIVE (%.1f hours left)",
		remaining.Hours())
}

func testNameValidation(name string) {
	err := ipns.ValidateIPNSName(name)
	if err != nil {
		fmt.Printf("   ❌ Invalid IPNS name: %s (%v)\n", name[:12]+"...", err)
	} else {
		fmt.Printf("   ✅ Valid IPNS name: %s\n", name[:12]+"...")
	}
}
