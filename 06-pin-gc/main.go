package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"

	dag "github.com/gosuda/boxo-starter-kit/04-dag-ipld/pkg"
	pin "github.com/gosuda/boxo-starter-kit/06-pin-gc/pkg"
)

func main() {
	fmt.Println("=== Pin & Garbage Collection Demo ===")

	ctx := context.Background()

	// Create DAG wrapper (this will create persistent storage)
	dagWrapper, err := dag.New(nil, "")
	if err != nil {
		log.Fatalf("Failed to create DAG wrapper: %v", err)
	}
	defer dagWrapper.Close()

	// Create pin manager
	pinManager, err := pin.NewPinManager(dagWrapper)
	if err != nil {
		log.Fatalf("Failed to create pin manager: %v", err)
	}
	defer pinManager.Close()

	// Demo 1: Create and pin some content
	fmt.Println("1. Creating and pinning content:")
	cids := createSampleContent(ctx, dagWrapper)

	// Pin some content
	fmt.Printf("\n   Pinning content...\n")

	// Pin first CID directly
	err = pinManager.Pin(ctx, cids[0], pin.PinOptions{
		Name:      "important-document",
		Recursive: false,
	})
	if err != nil {
		log.Printf("Failed to pin %s: %v", cids[0].String(), err)
	}

	// Pin second CID recursively
	err = pinManager.Pin(ctx, cids[1], pin.PinOptions{
		Name:      "project-data",
		Recursive: true,
	})
	if err != nil {
		log.Printf("Failed to pin %s: %v", cids[1].String(), err)
	}

	// Demo 2: List all pins
	fmt.Println("\n2. Listing all pins:")
	pins, err := pinManager.ListPins(ctx)
	if err != nil {
		log.Printf("Failed to list pins: %v", err)
	} else {
		for _, pinInfo := range pins {
			fmt.Printf("   📌 %s (%s) - %s\n",
				pinInfo.CID.String()[:20]+"...",
				pinInfo.Type.String(),
				pinInfo.Name)
		}
	}

	// Demo 3: Check pin status
	fmt.Println("\n3. Checking pin status:")
	for i, c := range cids {
		isPinned, err := pinManager.IsPinned(ctx, c)
		if err != nil {
			log.Printf("Failed to check pin status for %s: %v", c.String(), err)
			continue
		}

		status := "❌ Not pinned"
		if isPinned {
			pinType, _ := pinManager.GetPinType(ctx, c)
			status = fmt.Sprintf("✅ Pinned (%s)", pinType.String())
		}

		fmt.Printf("   CID %d: %s - %s\n", i+1, c.String()[:20]+"...", status)
	}

	// Demo 4: Show statistics before GC
	fmt.Println("\n4. Statistics before garbage collection:")
	stats, err := pinManager.GetStats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		printStats(stats)
	}

	// Demo 5: Run garbage collection
	fmt.Println("\n5. Running garbage collection:")
	gcResult, err := pinManager.RunGC(ctx)
	if err != nil {
		log.Printf("Failed to run GC: %v", err)
	} else {
		printGCResult(gcResult)
	}

	// Demo 6: Show statistics after GC
	fmt.Println("\n6. Statistics after garbage collection:")
	stats, err = pinManager.GetStats(ctx)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
	} else {
		printStats(stats)
	}

	// Demo 7: Verify pinned content still exists
	fmt.Println("\n7. Verifying pinned content still exists:")
	for _, pinInfo := range pins {
		exists, err := dagWrapper.Has(ctx, pinInfo.CID)
		if err != nil {
			log.Printf("Failed to check existence of %s: %v", pinInfo.CID.String(), err)
			continue
		}

		status := "❌ Missing"
		if exists {
			status = "✅ Exists"
		}

		fmt.Printf("   %s (%s): %s\n",
			pinInfo.CID.String()[:20]+"...",
			pinInfo.Type.String(),
			status)
	}

	// Demo 8: Unpin and run GC again
	fmt.Println("\n8. Unpinning one item and running GC again:")

	err = pinManager.Unpin(ctx, cids[0], false)
	if err != nil {
		log.Printf("Failed to unpin: %v", err)
	}

	gcResult2, err := pinManager.RunGC(ctx)
	if err != nil {
		log.Printf("Failed to run second GC: %v", err)
	} else {
		fmt.Printf("\n   Second GC Results:\n")
		printGCResult(gcResult2)
	}

	fmt.Println("\n=== Demo completed! ===")
}

func createSampleContent(ctx context.Context, dagWrapper *dag.DagWrapper) []cid.Cid {
	var cids []cid.Cid

	// Create various types of content
	contents := []map[string]any{
		{
			"type":    "document",
			"title":   "Important Document",
			"content": "This is a very important document that should be pinned.",
			"size":    1024,
		},
		{
			"type": "project",
			"name": "MyProject",
			"files": []string{
				"main.go",
				"README.md",
				"LICENSE",
			},
			"metadata": map[string]any{
				"version": "1.0.0",
				"author":  "Developer",
			},
		},
		{
			"type":        "temp_data",
			"description": "Temporary data that can be garbage collected",
			"created":     time.Now().Unix(),
			"data":        make([]byte, 2048), // Some binary data
		},
		{
			"type": "cache",
			"entries": []map[string]any{
				{"key": "cache1", "value": "data1"},
				{"key": "cache2", "value": "data2"},
				{"key": "cache3", "value": "data3"},
			},
		},
		{
			"type":    "logs",
			"entries": generateLogEntries(100),
		},
	}

	fmt.Printf("   Creating %d content items:\n", len(contents))

	for i, content := range contents {
		c, err := dagWrapper.PutAny(ctx, content)
		if err != nil {
			log.Printf("Failed to create content %d: %v", i, err)
			continue
		}

		cids = append(cids, c)
		fmt.Printf("   ✅ Created %s: %s\n",
			content["type"].(string),
			c.String()[:20]+"...")
	}

	return cids
}

func generateLogEntries(count int) []map[string]any {
	entries := make([]map[string]any, count)

	for i := 0; i < count; i++ {
		entries[i] = map[string]any{
			"timestamp": time.Now().Add(-time.Duration(i) * time.Minute).Unix(),
			"level":     []string{"INFO", "WARN", "ERROR"}[i%3],
			"message":   fmt.Sprintf("Log entry %d", i),
			"source":    "application",
		}
	}

	return entries
}

func printStats(stats *pin.PinStats) {
	fmt.Printf("   📊 Pin Statistics:\n")
	fmt.Printf("      Direct pins: %d\n", stats.DirectPins)
	fmt.Printf("      Recursive pins: %d\n", stats.RecursivePins)
	fmt.Printf("      Indirect pins: ~%d (estimated)\n", stats.IndirectPins)

	if !stats.LastGC.IsZero() {
		fmt.Printf("      Last GC: %s\n", stats.LastGC.Format("2006-01-02 15:04:05"))
		fmt.Printf("      Last GC duration: %v\n", stats.GCDuration)
		fmt.Printf("      Last reclaimed: %.2f KB\n", float64(stats.ReclaimedBytes)/1024)
	} else {
		fmt.Printf("      Last GC: Never run\n")
	}
}

func printGCResult(result *pin.GCResult) {
	fmt.Printf("   📈 GC Results:\n")
	fmt.Printf("      Blocks before: %d\n", result.BlocksBefore)
	fmt.Printf("      Blocks after: %d\n", result.BlocksAfter)
	fmt.Printf("      Deleted blocks: %d\n", result.DeletedBlocks)
	fmt.Printf("      Reclaimed space: %.2f KB\n", float64(result.ReclaimedBytes)/1024)
	fmt.Printf("      Duration: %v\n", result.Duration)
	fmt.Printf("      Pinned blocks: %d\n", result.PinnedBlocks)

	if result.BlocksBefore > 0 {
		efficiency := float64(result.DeletedBlocks) / float64(result.BlocksBefore) * 100
		fmt.Printf("      Cleanup efficiency: %.1f%%\n", efficiency)
	}
}
