package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
)

func main() {
	fmt.Println("=== IPLD Prime Library Demo ===")
	fmt.Println()

	ctx := context.Background()

	// Demo 1: Basic IPLD Prime setup
	fmt.Println("🔧 1. Setting up IPLD Prime wrapper:")

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

	fmt.Printf("   ✅ IPLD wrapper created with DAG-CBOR codec\n")
	fmt.Printf("   📦 Storage: In-memory backend\n")
	fmt.Printf("   🔗 CID Prefix: v%d-%x-%x-%x\n", prefix.Version, prefix.Codec, prefix.MhType, prefix.MhLength)
	fmt.Println()

	// Demo 2: Working with basic data types
	fmt.Println("📊 2. Storing basic data types:")

	// Store simple values
	basicData := map[string]interface{}{
		"message":   "Hello IPLD Prime!",
		"timestamp": 1640995200,
		"active":    true,
		"metadata":  []byte("binary data here"),
	}

	basicCID, err := ipldWrapper.PutIPLDAny(ctx, basicData)
	if err != nil {
		log.Fatalf("Failed to store basic data: %v", err)
	}

	fmt.Printf("   💾 Stored basic data: %s\n", basicCID)

	// Retrieve and verify
	retrieved, err := ipldWrapper.GetIPLDAny(ctx, basicCID)
	if err != nil {
		log.Fatalf("Failed to retrieve basic data: %v", err)
	}

	fmt.Printf("   📥 Retrieved data: %+v\n", retrieved)
	fmt.Println()

	// Demo 3: Complex nested structures
	fmt.Println("🏗️ 3. Working with complex structures:")

	// Create a more complex nested structure
	complexData := map[string]interface{}{
		"user": map[string]interface{}{
			"id":    12345,
			"name":  "Alice",
			"email": "alice@example.com",
			"preferences": map[string]interface{}{
				"theme":    "dark",
				"language": "en",
				"notifications": map[string]interface{}{
					"email": true,
					"push":  false,
				},
			},
		},
		"posts": []interface{}{
			map[string]interface{}{
				"id":    1,
				"title": "First Post",
				"tags":  []interface{}{"intro", "welcome"},
			},
			map[string]interface{}{
				"id":    2,
				"title": "IPLD Deep Dive",
				"tags":  []interface{}{"ipld", "technical"},
			},
		},
		"stats": map[string]interface{}{
			"total_posts": 2,
			"followers":   []interface{}{2001, 2002, 2003},
		},
	}

	complexCID, err := ipldWrapper.PutIPLDAny(ctx, complexData)
	if err != nil {
		log.Fatalf("Failed to store complex data: %v", err)
	}

	fmt.Printf("   💾 Stored complex structure: %s\n", complexCID)

	// Retrieve complex data
	complexRetrieved, err := ipldWrapper.GetIPLDAny(ctx, complexCID)
	if err != nil {
		log.Fatalf("Failed to retrieve complex data: %v", err)
	}

	fmt.Printf("   📥 Retrieved complex data successfully\n")
	fmt.Printf("   🔍 User name: %v\n", getNestedValue(complexRetrieved, "user", "name"))
	fmt.Printf("   🔍 Post count: %v\n", getNestedValue(complexRetrieved, "stats", "total_posts"))
	fmt.Println()

	// Demo 4: Working with IPLD Nodes directly
	fmt.Println("🎛️ 4. Direct IPLD Node manipulation:")

	// Create node manually
	nb := basicnode.Prototype.Map.NewBuilder()
	ma, err := nb.BeginMap(3)
	if err != nil {
		log.Fatalf("Failed to create map assembler: %v", err)
	}

	// Add fields
	err = ma.AssembleKey().AssignString("type")
	if err != nil {
		log.Fatalf("Failed to assign key: %v", err)
	}
	err = ma.AssembleValue().AssignString("document")
	if err != nil {
		log.Fatalf("Failed to assign value: %v", err)
	}

	err = ma.AssembleKey().AssignString("version")
	if err != nil {
		log.Fatalf("Failed to assign key: %v", err)
	}
	err = ma.AssembleValue().AssignInt(1)
	if err != nil {
		log.Fatalf("Failed to assign value: %v", err)
	}

	err = ma.AssembleKey().AssignString("content")
	if err != nil {
		log.Fatalf("Failed to assign key: %v", err)
	}
	err = ma.AssembleValue().AssignString("This is IPLD content")
	if err != nil {
		log.Fatalf("Failed to assign value: %v", err)
	}

	err = ma.Finish()
	if err != nil {
		log.Fatalf("Failed to finish map: %v", err)
	}

	node := nb.Build()
	nodeCID, err := ipldWrapper.PutIPLD(ctx, node)
	if err != nil {
		log.Fatalf("Failed to store node: %v", err)
	}

	fmt.Printf("   💾 Stored manual node: %s\n", nodeCID)

	// Retrieve and examine node
	retrievedNode, err := ipldWrapper.GetIPLD(ctx, nodeCID)
	if err != nil {
		log.Fatalf("Failed to retrieve node: %v", err)
	}

	fmt.Printf("   📥 Retrieved node kind: %s\n", retrievedNode.Kind())
	fmt.Printf("   📊 Node length: %d\n", retrievedNode.Length())

	// Walk through node fields
	fmt.Printf("   🔍 Node fields:\n")
	iter := retrievedNode.MapIterator()
	for !iter.Done() {
		key, value, err := iter.Next()
		if err != nil {
			log.Printf("Error iterating: %v", err)
			break
		}
		keyStr, _ := key.AsString()
		valueStr := getValueAsString(value)
		fmt.Printf("     • %s: %s\n", keyStr, valueStr)
	}
	fmt.Println()

	// Demo 5: Linking between IPLD objects
	fmt.Println("🔗 5. Creating links between IPLD objects:")

	// Create a document that references other documents
	authorData := map[string]interface{}{
		"name":  "Dr. Smith",
		"email": "dr.smith@university.edu",
		"bio":   "Computer Science Professor",
	}

	authorCID, err := ipldWrapper.PutIPLDAny(ctx, authorData)
	if err != nil {
		log.Fatalf("Failed to store author: %v", err)
	}

	// Create document with link to author
	documentData := map[string]interface{}{
		"title":   "Introduction to IPLD",
		"content": "IPLD (InterPlanetary Linked Data) is a data model...",
		"author":  authorCID, // This creates a link
		"tags":    []interface{}{"ipld", "education", "tutorial"},
		"metadata": map[string]interface{}{
			"created_at": "2024-01-01",
			"version":    1,
		},
	}

	documentCID, err := ipldWrapper.PutIPLDAny(ctx, documentData)
	if err != nil {
		log.Fatalf("Failed to store document: %v", err)
	}

	fmt.Printf("   💾 Stored author: %s\n", authorCID)
	fmt.Printf("   💾 Stored document with link: %s\n", documentCID)

	// Retrieve document and follow link
	retrievedDoc, err := ipldWrapper.GetIPLDAny(ctx, documentCID)
	if err != nil {
		log.Fatalf("Failed to retrieve document: %v", err)
	}

	fmt.Printf("   📄 Document title: %v\n", getNestedValue(retrievedDoc, "title"))

	// Extract author CID and retrieve author data
	if authorLink := getNestedValue(retrievedDoc, "author"); authorLink != nil {
		if authorCIDFromDoc, ok := authorLink.(cid.Cid); ok {
			retrievedAuthor, err := ipldWrapper.GetIPLDAny(ctx, authorCIDFromDoc)
			if err != nil {
				log.Printf("Failed to retrieve author: %v", err)
			} else {
				fmt.Printf("   👤 Author name: %v\n", getNestedValue(retrievedAuthor, "name"))
				fmt.Printf("   ✉️ Author email: %v\n", getNestedValue(retrievedAuthor, "email"))
			}
		}
	}
	fmt.Println()

	// Demo 6: Performance and statistics
	fmt.Println("📊 6. Performance summary:")

	allCIDs := []cid.Cid{basicCID, complexCID, nodeCID, authorCID, documentCID}
	fmt.Printf("   📈 Total objects stored: %d\n", len(allCIDs))
	fmt.Printf("   🗃️ Storage backend: In-memory\n")

	// Calculate total size (approximate)
	totalSize := int64(0)
	for _, c := range allCIDs {
		// This is a simplified size calculation
		totalSize += int64(len(c.Bytes()) + 100) // CID + approximate data
	}
	fmt.Printf("   💾 Approximate total size: %d bytes\n", totalSize)

	fmt.Println()
	fmt.Println("✅ IPLD Prime demo completed successfully!")
	fmt.Println()
	fmt.Println("🔗 Key concepts demonstrated:")
	fmt.Println("   • IPLD wrapper setup with LinkSystem")
	fmt.Println("   • Storing/retrieving various data types")
	fmt.Println("   • Working with complex nested structures")
	fmt.Println("   • Direct node manipulation with builders")
	fmt.Println("   • Creating and following links between objects")
	fmt.Println("   • Integration with persistent storage backends")
}

// Helper function to get nested values from interface{}
func getNestedValue(data interface{}, keys ...string) interface{} {
	current := data
	for _, key := range keys {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[key]
		} else {
			return nil
		}
	}
	return current
}

// Helper function to get value as string for display
func getValueAsString(value datamodel.Node) string {
	switch value.Kind() {
	case datamodel.Kind_String:
		if s, err := value.AsString(); err == nil {
			return fmt.Sprintf("\"%s\"", s)
		}
	case datamodel.Kind_Int:
		if i, err := value.AsInt(); err == nil {
			return fmt.Sprintf("%d", i)
		}
	case datamodel.Kind_Bool:
		if b, err := value.AsBool(); err == nil {
			return fmt.Sprintf("%t", b)
		}
	case datamodel.Kind_Bytes:
		if b, err := value.AsBytes(); err == nil {
			return fmt.Sprintf("<%d bytes>", len(b))
		}
	case datamodel.Kind_Link:
		if l, err := value.AsLink(); err == nil {
			return fmt.Sprintf("Link(%s)", l.String())
		}
	}
	return fmt.Sprintf("<%s>", value.Kind())
}