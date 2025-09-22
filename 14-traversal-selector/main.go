package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/traversal"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
	traversalselector "github.com/gosuda/boxo-starter-kit/14-traversal-selector/pkg"
)

func main() {
	fmt.Println("=== IPLD Traversal and Selector Demo ===")
	fmt.Println()

	ctx := context.Background()

	// Demo 1: Setup traversal selector wrapper
	fmt.Println("🔧 1. Setting up traversal selector wrapper:")

	// Create persistent storage
	store, err := persistent.New(persistent.Memory, "")
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// Create IPLD wrapper with DAG-CBOR codec
	prefix := block.NewV1Prefix(mc.DagCbor, 0, 0)
	ipldWrapper, err := ipldprime.NewDefault(prefix, store)
	if err != nil {
		log.Fatalf("Failed to create IPLD wrapper: %v", err)
	}

	// Create traversal selector wrapper
	traversalWrapper, err := traversalselector.New(ipldWrapper)
	if err != nil {
		log.Fatalf("Failed to create traversal wrapper: %v", err)
	}

	fmt.Printf("   ✅ Traversal wrapper created successfully\n")
	fmt.Printf("   🏗️ Features: Selective DAG traversal, visitor patterns, transform functions\n")
	fmt.Printf("   💾 Storage: In-memory backend\n")
	fmt.Printf("   🔗 Codec: DAG-CBOR for IPLD compatibility\n")
	fmt.Println()

	// Demo 2: Create a complex DAG structure for traversal
	fmt.Println("🏗️ 2. Creating complex DAG structure:")

	// Create a hierarchical document structure
	// Document -> Sections -> Paragraphs -> Words
	wordsData := []map[string]interface{}{
		{"text": "IPLD", "type": "keyword", "importance": 10},
		{"text": "traversal", "type": "concept", "importance": 8},
		{"text": "selector", "type": "concept", "importance": 9},
		{"text": "DAG", "type": "keyword", "importance": 10},
	}

	// Store individual word objects
	var wordCIDs []cid.Cid
	for i, word := range wordsData {
		wordCID, err := ipldWrapper.PutIPLDAny(ctx, word)
		if err != nil {
			log.Fatalf("Failed to store word %d: %v", i, err)
		}
		wordCIDs = append(wordCIDs, wordCID)
		fmt.Printf("   💾 Stored word '%s': %s\n", word["text"], wordCID)
	}

	// Create paragraphs that reference words
	paragraphsData := []map[string]interface{}{
		{
			"id":      1,
			"topic":   "introduction",
			"words":   []cid.Cid{wordCIDs[0], wordCIDs[3]}, // "IPLD", "DAG"
			"summary": "Introduction to IPLD and DAG concepts",
		},
		{
			"id":      2,
			"topic":   "technical",
			"words":   []cid.Cid{wordCIDs[1], wordCIDs[2]}, // "traversal", "selector"
			"summary": "Technical aspects of traversal and selectors",
		},
	}

	var paragraphCIDs []cid.Cid
	for i, paragraph := range paragraphsData {
		paragraphCID, err := ipldWrapper.PutIPLDAny(ctx, paragraph)
		if err != nil {
			log.Fatalf("Failed to store paragraph %d: %v", i, err)
		}
		paragraphCIDs = append(paragraphCIDs, paragraphCID)
		fmt.Printf("   📝 Stored paragraph on '%s': %s\n", paragraph["topic"], paragraphCID)
	}

	// Create sections that reference paragraphs
	sectionsData := []map[string]interface{}{
		{
			"title":      "Overview",
			"paragraphs": []cid.Cid{paragraphCIDs[0]},
			"metadata": map[string]interface{}{
				"author": "IPLD Team",
				"level":  1,
			},
		},
		{
			"title":      "Advanced Topics",
			"paragraphs": []cid.Cid{paragraphCIDs[1]},
			"metadata": map[string]interface{}{
				"author": "Technical Writer",
				"level":  2,
			},
		},
	}

	var sectionCIDs []cid.Cid
	for i, section := range sectionsData {
		sectionCID, err := ipldWrapper.PutIPLDAny(ctx, section)
		if err != nil {
			log.Fatalf("Failed to store section %d: %v", i, err)
		}
		sectionCIDs = append(sectionCIDs, sectionCID)
		fmt.Printf("   📋 Stored section '%s': %s\n", section["title"], sectionCID)
	}

	// Create root document
	documentData := map[string]interface{}{
		"title":    "IPLD Traversal Guide",
		"version":  "1.0",
		"sections": sectionCIDs,
		"metadata": map[string]interface{}{
			"created_at": "2024-01-01",
			"tags":       []interface{}{"ipld", "traversal", "tutorial"},
			"stats": map[string]interface{}{
				"total_sections":   len(sectionCIDs),
				"total_paragraphs": len(paragraphCIDs),
				"total_words":      len(wordCIDs),
			},
		},
	}

	documentCID, err := ipldWrapper.PutIPLDAny(ctx, documentData)
	if err != nil {
		log.Fatalf("Failed to store document: %v", err)
	}

	fmt.Printf("   📖 Stored root document: %s\n", documentCID)
	fmt.Printf("   🌳 DAG structure: Document -> Sections -> Paragraphs -> Words\n")
	fmt.Println()

	// Demo 3: Basic full DAG traversal
	fmt.Println("🌲 3. Full DAG traversal (no selector):")

	// Create selector for all nodes with matching
	allSelectorNode := traversalselector.SelectorAll(true)
	allSelector, err := traversalselector.CompileSelector(allSelectorNode)
	if err != nil {
		log.Fatalf("Failed to compile all selector: %v", err)
	}

	// Visit all nodes with detailed progress tracking
	visitFn, collector := traversalselector.NewVisitAll(documentCID)

	err = traversalWrapper.WalkMatching(ctx, documentCID, allSelector, visitFn)
	if err != nil {
		log.Fatalf("Failed to traverse DAG: %v", err)
	}

	fmt.Printf("   📊 Traversal Results:\n")
	fmt.Printf("     • Total nodes visited: %d\n", len(collector.Records))
	fmt.Printf("     • Root CID: %s\n", documentCID)

	// Display first few visited nodes
	fmt.Printf("   🔍 First 5 nodes visited:\n")
	for i, record := range collector.Records {
		if i >= 5 {
			break
		}
		nodeType := getNodeTypeDescription(record.Node)
		fmt.Printf("     %d. %s (%s)\n", i+1, record.Cid, nodeType)
	}
	fmt.Println()

	// Demo 4: Selective traversal with custom selectors
	fmt.Println("🎯 4. Selective traversal with custom selectors:")

	// Create selector for only metadata fields
	metadataSelectorNode := traversalselector.SelectorField("metadata")
	metadataSelector, err := traversalselector.CompileSelector(metadataSelectorNode)
	if err != nil {
		log.Fatalf("Failed to compile metadata selector: %v", err)
	}

	// Traverse only metadata
	metadataVisitFn, metadataCollector := traversalselector.NewAdvVisitAll(documentCID)

	err = traversalWrapper.WalkAdv(ctx, documentCID, metadataSelector, metadataVisitFn)
	if err != nil {
		log.Fatalf("Failed to traverse metadata: %v", err)
	}

	fmt.Printf("   📋 Metadata-only traversal:\n")
	fmt.Printf("     • Nodes visited: %d\n", len(metadataCollector.Records))
	fmt.Printf("     • Selected path: document/metadata\n")

	// Create selector for title field only
	titleSelectorNode := traversalselector.SelectorField("title")
	titleSelector, err := traversalselector.CompileSelector(titleSelectorNode)
	if err != nil {
		log.Fatalf("Failed to compile title selector: %v", err)
	}

	// Traverse title field
	titleVisitFn, titleCollector := traversalselector.NewAdvVisitAll(documentCID)

	err = traversalWrapper.WalkAdv(ctx, documentCID, titleSelector, titleVisitFn)
	if err != nil {
		log.Fatalf("Failed to traverse title: %v", err)
	}

	fmt.Printf("   📑 Title field traversal:\n")
	fmt.Printf("     • Nodes visited: %d\n", len(titleCollector.Records))
	fmt.Printf("     • Selected path: document/title\n")
	fmt.Println()

	// Demo 5: Advanced visitor patterns
	fmt.Println("🔍 5. Advanced visitor patterns:")

	// Stream-based visitor for large datasets
	streamVisitFn, visitStream := traversalselector.NewVisitStream(documentCID, 10)

	// Start traversal in goroutine
	go func() {
		defer visitStream.Close()
		err := traversalWrapper.WalkMatching(ctx, documentCID, allSelector, streamVisitFn)
		if err != nil {
			log.Printf("Stream traversal error: %v", err)
		}
	}()

	// Consume stream
	fmt.Printf("   🌊 Stream-based traversal:\n")
	visitCount := 0
	for record := range visitStream.C {
		visitCount++
		if visitCount <= 3 {
			nodeType := getNodeTypeDescription(record.Node)
			fmt.Printf("     • Streamed node %d: %s (%s)\n", visitCount, record.Cid, nodeType)
		}
	}
	fmt.Printf("     • Total streamed nodes: %d\n", visitCount)

	// Single node visitor (early termination)
	oneVisitFn, oneCollector := traversalselector.NewVisitOne(documentCID)

	err = traversalWrapper.WalkMatching(ctx, documentCID, allSelector, oneVisitFn)
	if err != nil {
		log.Fatalf("Failed in single node visit: %v", err)
	}

	fmt.Printf("   🎯 Single node visitor (early termination):\n")
	if oneCollector.Found {
		nodeType := getNodeTypeDescription(oneCollector.Rec.Node)
		fmt.Printf("     • First node found: %s (%s)\n", oneCollector.Rec.Cid, nodeType)
	}
	fmt.Println()

	// Demo 6: Transform operations during traversal
	fmt.Println("🔄 6. Transform operations during traversal:")

	// Create transform function that adds visit timestamps
	transformFn, transformCollector := traversalselector.NewTransformAll(
		documentCID,
		func(p traversal.Progress, n datamodel.Node) (datamodel.Node, error) {
			// For demonstration, just return the original node
			// In practice, you could modify the node here
			return n, nil
		},
	)

	// Transform traversal with depth limitation
	depthLimitSelectorNode := traversalselector.SelectorDepth(2, true)
	depthLimitSelector, err := traversalselector.CompileSelector(depthLimitSelectorNode)
	if err != nil {
		log.Fatalf("Failed to compile depth limit selector: %v", err)
	}

	_, err = traversalWrapper.WalkTransforming(ctx, documentCID, depthLimitSelector, transformFn)
	if err != nil {
		log.Fatalf("Failed to transform: %v", err)
	}

	fmt.Printf("   🔧 Transform traversal:\n")
	fmt.Printf("     • Nodes processed: %d\n", len(transformCollector.Records))
	fmt.Printf("     • Transform function: identity (demonstration)\n")
	fmt.Printf("     • Use case: Data migration, format conversion, validation\n")
	fmt.Println()

	// Demo 7: Performance and configuration analysis
	fmt.Println("📊 7. Performance and configuration analysis:")

	// Compare different traversal strategies
	strategies := []struct {
		name        string
		description string
		nodeCount   int
	}{
		{"Full Traversal", "Visit all nodes in DAG", len(collector.Records)},
		{"Metadata Only", "Visit metadata subtree", len(metadataCollector.Records)},
		{"Titles Only", "Visit section titles", len(titleCollector.Records)},
		{"Stream Based", "Streaming visitor pattern", visitCount},
		{"Transform", "Transform with identity function", len(transformCollector.Records)},
	}

	fmt.Printf("   📈 Traversal Strategy Comparison:\n")
	totalNodes := len(collector.Records)
	for _, strategy := range strategies {
		efficiency := float64(strategy.nodeCount) / float64(totalNodes) * 100
		fmt.Printf("     • %-15s: %2d nodes (%.1f%% of total) - %s\n",
			strategy.name, strategy.nodeCount, efficiency, strategy.description)
	}

	fmt.Printf("   🎯 Selector Benefits:\n")
	fmt.Printf("     • Bandwidth savings: Skip irrelevant data\n")
	fmt.Printf("     • Memory efficiency: Process only needed nodes\n")
	fmt.Printf("     • Network optimization: Fetch specific DAG parts\n")
	fmt.Printf("     • Application focus: Extract business-relevant data\n")
	fmt.Println()

	// Demo 8: Real-world usage patterns
	fmt.Println("🏆 8. Real-world usage patterns:")

	fmt.Printf("   📚 Common Use Cases:\n")
	fmt.Printf("     • Content indexing: Extract searchable metadata\n")
	fmt.Printf("     • Data migration: Transform legacy formats\n")
	fmt.Printf("     • Partial sync: Download specific document sections\n")
	fmt.Printf("     • Validation: Check structural integrity\n")
	fmt.Printf("     • Analytics: Collect usage statistics\n")

	fmt.Printf("\n   🔧 Configuration Options:\n")
	fmt.Printf("     • Budget limits: Control traversal depth/breadth\n")
	fmt.Printf("     • Link loading: Customize how links are resolved\n")
	fmt.Printf("     • Progress tracking: Monitor traversal state\n")
	fmt.Printf("     • Error handling: Graceful failure recovery\n")

	fmt.Printf("\n   ⚡ Performance Tips:\n")
	fmt.Printf("     • Use specific selectors to reduce I/O\n")
	fmt.Printf("     • Stream large datasets to reduce memory\n")
	fmt.Printf("     • Implement caching for repeated traversals\n")
	fmt.Printf("     • Consider parallelization for independent branches\n")
	fmt.Println()

	fmt.Println("✅ IPLD Traversal and Selector demo completed successfully!")
	fmt.Println()
	fmt.Println("🔗 Key concepts demonstrated:")
	fmt.Println("   • Selective DAG traversal with custom selectors")
	fmt.Println("   • Multiple visitor patterns (collect, stream, single)")
	fmt.Println("   • Transform operations during traversal")
	fmt.Println("   • Performance optimization through targeted selection")
	fmt.Println("   • Real-world usage patterns and configuration")
	fmt.Println("   • Integration with IPLD Prime and persistent storage")
	fmt.Println()
	fmt.Println("💡 Traversal selectors enable efficient, targeted access to")
	fmt.Println("   specific parts of large DAG structures in IPLD!")
}

// Helper function to describe node types for display
func getNodeTypeDescription(node datamodel.Node) string {
	switch node.Kind() {
	case datamodel.Kind_Map:
		return fmt.Sprintf("Map(%d fields)", node.Length())
	case datamodel.Kind_List:
		return fmt.Sprintf("List(%d items)", node.Length())
	case datamodel.Kind_String:
		if s, err := node.AsString(); err == nil {
			if len(s) > 20 {
				return fmt.Sprintf("String(\"%s...\")", s[:17])
			}
			return fmt.Sprintf("String(\"%s\")", s)
		}
		return "String"
	case datamodel.Kind_Int:
		if i, err := node.AsInt(); err == nil {
			return fmt.Sprintf("Int(%d)", i)
		}
		return "Int"
	case datamodel.Kind_Bool:
		if b, err := node.AsBool(); err == nil {
			return fmt.Sprintf("Bool(%t)", b)
		}
		return "Bool"
	case datamodel.Kind_Bytes:
		if b, err := node.AsBytes(); err == nil {
			return fmt.Sprintf("Bytes(%d bytes)", len(b))
		}
		return "Bytes"
	case datamodel.Kind_Link:
		if l, err := node.AsLink(); err == nil {
			return fmt.Sprintf("Link(%s)", l.String()[:12]+"...")
		}
		return "Link"
	default:
		return string(node.Kind())
	}
}
