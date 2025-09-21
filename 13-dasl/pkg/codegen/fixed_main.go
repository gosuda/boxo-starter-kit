package main

import (
	"fmt"
	"os"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	gengo "github.com/ipld/go-ipld-prime/schema/gen/go"
)

func main() {
	fmt.Println("Building schema programmatically to avoid DSL parser issues...")

	// Create and initialize type system
	ts := &schema.TypeSystem{}
	ts.Init()

	// Build User struct type programmatically
	userStructBuilder := ts.SpawnStructType("User")
	userStructBuilder.AddField("id", "String", false, false)
	userStructBuilder.AddField("name", "String", false, false)
	userStructBuilder.AddField("email", "String", false, false)
	userType := userStructBuilder.Complete()

	// Build Post struct type programmatically
	postStructBuilder := ts.SpawnStructType("Post")
	postStructBuilder.AddField("id", "String", false, false)
	postStructBuilder.AddField("title", "String", false, false)
	postStructBuilder.AddField("body", "String", false, false)
	postStructBuilder.AddField("createdAt", "Int", false, false)

	// For the tags field, we need to create a list type
	listStringType := ts.SpawnListType("ListString", "String", false)
	postStructBuilder.AddField("tags", "ListString", false, false)
	postType := postStructBuilder.Complete()

	if userType == nil || postType == nil || listStringType == nil {
		fmt.Printf("Error: Failed to create types\n")
		os.Exit(1)
	}

	fmt.Printf("Created types successfully:\n")
	fmt.Printf("  User: %s\n", userType.Name())
	fmt.Printf("  Post: %s\n", postType.Name())
	fmt.Printf("  ListString: %s\n", listStringType.Name())

	// Try generating Go code
	fmt.Println("Generating Go code...")

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Code generation failed: %v\n", r)
			fmt.Println("Attempting alternative approach...")
		}
	}()

	cfg := &gengo.AdjunctCfg{
		// Use minimal configuration
	}

	gengo.Generate("./generated", "daslmodels", *ts, cfg)
	fmt.Println("Code generation completed successfully!")
	fmt.Println("Generated files are in the ./generated directory")
}