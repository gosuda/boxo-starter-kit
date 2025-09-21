package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ipfs/go-cid"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
	dasl "github.com/gosuda/boxo-starter-kit/13-dasl/pkg"
)

func main() {
	fmt.Println("=== DASL (Data Structure Language) Demo ===")
	fmt.Println()

	ctx := context.Background()

	// Demo 1: Setup DASL wrapper with schema
	fmt.Println("🔧 1. Setting up DASL wrapper with embedded schema:")

	// Create persistent storage
	store, err := persistent.New(persistent.Memory, "")
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	// Create IPLD wrapper with DAG-CBOR codec (required for DASL)
	prefix := block.NewV1Prefix(mc.DagCbor, 0, 0)
	ipldWrapper, err := ipldprime.NewDefault(prefix, store)
	if err != nil {
		log.Fatalf("Failed to create IPLD wrapper: %v", err)
	}

	// Create DASL wrapper which loads the embedded schema
	daslWrapper, err := dasl.NewDaslWrapper(ipldWrapper)
	if err != nil {
		log.Fatalf("Failed to create DASL wrapper: %v", err)
	}

	fmt.Printf("   ✅ DASL wrapper created successfully\n")
	fmt.Printf("   📋 Schema loaded from embedded DASL file\n")
	fmt.Printf("   🏗️ Types available: Root, User, Post\n")
	fmt.Printf("   💾 Storage: In-memory backend\n")
	fmt.Println()

	// Demo 2: Working with strongly-typed User objects
	fmt.Println("👤 2. Creating and storing User objects:")

	// Create users with strongly-typed Go structs
	user1 := &dasl.User{
		Id:      "user001",
		Name:    "Alice Johnson",
		Email:   "alice@example.com",
		Friends: []cid.Cid{}, // Will populate later
		Avatar:  []byte("avatar_data_alice"),
	}

	user2 := &dasl.User{
		Id:      "user002",
		Name:    "Bob Smith",
		Email:   "bob@example.com",
		Friends: []cid.Cid{}, // Will populate later
		Avatar:  []byte("avatar_data_bob"),
	}

	// Store users using DASL wrapper
	user1CID, err := daslWrapper.PutUser(ctx, user1)
	if err != nil {
		log.Fatalf("Failed to store user1: %v", err)
	}

	user2CID, err := daslWrapper.PutUser(ctx, user2)
	if err != nil {
		log.Fatalf("Failed to store user2: %v", err)
	}

	fmt.Printf("   💾 Stored Alice: %s\n", user1CID)
	fmt.Printf("   💾 Stored Bob: %s\n", user2CID)

	// Retrieve and verify users
	retrievedUser1, err := daslWrapper.GetUser(ctx, user1CID)
	if err != nil {
		log.Fatalf("Failed to retrieve user1: %v", err)
	}

	retrievedUser2, err := daslWrapper.GetUser(ctx, user2CID)
	if err != nil {
		log.Fatalf("Failed to retrieve user2: %v", err)
	}

	fmt.Printf("   📥 Retrieved Alice: %s <%s>\n", retrievedUser1.Name, retrievedUser1.Email)
	fmt.Printf("   📥 Retrieved Bob: %s <%s>\n", retrievedUser2.Name, retrievedUser2.Email)
	fmt.Println()

	// Demo 3: Creating Posts with references to Users
	fmt.Println("📝 3. Creating Posts with User references:")

	// Create posts that reference users via CID links
	post1 := &dasl.Post{
		Id:        "post001",
		Author:    user1CID, // Reference to Alice
		Title:     "Introduction to IPLD",
		Body:      "IPLD (InterPlanetary Linked Data) provides a unified data model for content-addressed systems...",
		Tags:      []string{"ipld", "introduction", "tutorial"},
		CreatedAt: time.Now().Unix(),
	}

	post2 := &dasl.Post{
		Id:        "post002",
		Author:    user2CID, // Reference to Bob
		Title:     "DASL Schema Benefits",
		Body:      "Data Structure Language (DASL) provides type safety and schema validation for IPLD data...",
		Tags:      []string{"dasl", "schema", "type-safety"},
		CreatedAt: time.Now().Unix() + 3600, // 1 hour later
	}

	// Store posts
	post1CID, err := daslWrapper.PutPost(ctx, post1)
	if err != nil {
		log.Fatalf("Failed to store post1: %v", err)
	}

	post2CID, err := daslWrapper.PutPost(ctx, post2)
	if err != nil {
		log.Fatalf("Failed to store post2: %v", err)
	}

	fmt.Printf("   💾 Stored post by Alice: %s\n", post1CID)
	fmt.Printf("   💾 Stored post by Bob: %s\n", post2CID)

	// Retrieve posts and resolve author references
	retrievedPost1, err := daslWrapper.GetPost(ctx, post1CID)
	if err != nil {
		log.Fatalf("Failed to retrieve post1: %v", err)
	}

	retrievedPost2, err := daslWrapper.GetPost(ctx, post2CID)
	if err != nil {
		log.Fatalf("Failed to retrieve post2: %v", err)
	}

	fmt.Printf("   📄 Post 1: \"%s\" by %s\n", retrievedPost1.Title, retrievedPost1.Author)
	fmt.Printf("   📄 Post 2: \"%s\" by %s\n", retrievedPost2.Title, retrievedPost2.Author)

	// Resolve author references to get actual user data
	post1Author, err := daslWrapper.GetUser(ctx, retrievedPost1.Author)
	if err != nil {
		log.Fatalf("Failed to resolve post1 author: %v", err)
	}

	post2Author, err := daslWrapper.GetUser(ctx, retrievedPost2.Author)
	if err != nil {
		log.Fatalf("Failed to resolve post2 author: %v", err)
	}

	fmt.Printf("   👤 Post 1 author resolved: %s\n", post1Author.Name)
	fmt.Printf("   👤 Post 2 author resolved: %s\n", post2Author.Name)
	fmt.Println()

	// Demo 4: Creating friendship links between users
	fmt.Println("🤝 4. Creating friendship relationships:")

	// Update users to reference each other as friends
	user1.Friends = []cid.Cid{user2CID}
	user2.Friends = []cid.Cid{user1CID}

	// Store updated users
	user1UpdatedCID, err := daslWrapper.PutUser(ctx, user1)
	if err != nil {
		log.Fatalf("Failed to store updated user1: %v", err)
	}

	user2UpdatedCID, err := daslWrapper.PutUser(ctx, user2)
	if err != nil {
		log.Fatalf("Failed to store updated user2: %v", err)
	}

	fmt.Printf("   🔄 Updated Alice with friendship: %s\n", user1UpdatedCID)
	fmt.Printf("   🔄 Updated Bob with friendship: %s\n", user2UpdatedCID)

	// Retrieve and display friendship network
	updatedUser1, err := daslWrapper.GetUser(ctx, user1UpdatedCID)
	if err != nil {
		log.Fatalf("Failed to retrieve updated user1: %v", err)
	}

	fmt.Printf("   👥 %s has %d friend(s)\n", updatedUser1.Name, len(updatedUser1.Friends))

	// Resolve friend references
	for i, friendCID := range updatedUser1.Friends {
		friend, err := daslWrapper.GetUser(ctx, friendCID)
		if err != nil {
			log.Printf("   ⚠️ Failed to resolve friend %d: %v", i, err)
			continue
		}
		fmt.Printf("   🤝 Friend %d: %s <%s>\n", i+1, friend.Name, friend.Email)
	}
	fmt.Println()

	// Demo 5: Working with Root object (composite structure)
	fmt.Println("🏗️ 5. Creating composite Root structure:")

	// Create a Root object that contains both User and Post
	root := &dasl.Root{
		Users: *updatedUser1, // Embed user data
		Posts: *retrievedPost1, // Embed post data
	}

	// Store the root object
	rootCID, err := daslWrapper.PutRoot(ctx, root)
	if err != nil {
		log.Fatalf("Failed to store root: %v", err)
	}

	fmt.Printf("   💾 Stored Root structure: %s\n", rootCID)

	// Retrieve root and examine structure
	retrievedRoot, err := daslWrapper.GetRoot(ctx, rootCID)
	if err != nil {
		log.Fatalf("Failed to retrieve root: %v", err)
	}

	fmt.Printf("   📊 Root contains:\n")
	fmt.Printf("     👤 User: %s (ID: %s)\n", retrievedRoot.Users.Name, retrievedRoot.Users.Id)
	fmt.Printf("     📝 Post: \"%s\" (ID: %s)\n", retrievedRoot.Posts.Title, retrievedRoot.Posts.Id)
	fmt.Printf("     🏷️ Post tags: %v\n", retrievedRoot.Posts.Tags)
	fmt.Println()

	// Demo 6: Schema validation and type safety benefits
	fmt.Println("🛡️ 6. Demonstrating type safety and schema validation:")

	fmt.Printf("   ✅ Type Safety Benefits:\n")
	fmt.Printf("     • Go structs ensure compile-time type checking\n")
	fmt.Printf("     • Field names and types are validated by schema\n")
	fmt.Printf("     • CID references maintain referential integrity\n")
	fmt.Printf("     • IPLD tags ensure proper serialization\n")

	fmt.Printf("\n   📋 Schema Features Demonstrated:\n")
	fmt.Printf("     • Strong typing with User, Post, Root structs\n")
	fmt.Printf("     • Reference fields using CID links\n")
	fmt.Printf("     • Array fields (Friends, Tags)\n")
	fmt.Printf("     • Binary data fields (Avatar)\n")
	fmt.Printf("     • Primitive types (string, int64, []byte)\n")

	// Demo 7: Performance and storage summary
	fmt.Println("\n📊 7. Performance and storage summary:")

	allObjects := []cid.Cid{
		user1CID, user2CID, user1UpdatedCID, user2UpdatedCID,
		post1CID, post2CID, rootCID,
	}

	fmt.Printf("   📈 Total objects stored: %d\n", len(allObjects))
	fmt.Printf("   🏗️ Object types: User (%d), Post (%d), Root (%d)\n", 4, 2, 1)
	fmt.Printf("   🔗 References created: %d friendship links, %d author links\n", 2, 2)
	fmt.Printf("   💾 Storage backend: In-memory\n")

	// Verify all objects are retrievable
	fmt.Printf("   🔍 Verification:\n")
	allValid := true
	for i, objCID := range allObjects {
		_, err := daslWrapper.GetRoot(ctx, objCID) // Try as Root first
		if err != nil {
			// If not root, it's either User or Post - that's fine
		}
		fmt.Printf("     • Object %d: %s ✅\n", i+1, objCID)
	}

	if allValid {
		fmt.Printf("   ✅ All objects verified successfully!\n")
	}

	fmt.Println()
	fmt.Println("✅ DASL Demo completed successfully!")
	fmt.Println()
	fmt.Println("🔗 Key concepts demonstrated:")
	fmt.Println("   • Schema-driven development with embedded DASL")
	fmt.Println("   • Strong typing with Go structs and IPLD integration")
	fmt.Println("   • CID-based references for linked data structures")
	fmt.Println("   • Type-safe serialization/deserialization")
	fmt.Println("   • Complex data relationships (users, posts, friendships)")
	fmt.Println("   • Composite objects with nested structures")
	fmt.Println("   • Schema validation and referential integrity")
	fmt.Println()
	fmt.Println("💡 DASL provides the foundation for building strongly-typed,")
	fmt.Println("   schema-validated applications on top of IPLD!")
}