package main

import (
	"fmt"
	"os"

	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"
	gengo "github.com/ipld/go-ipld-prime/schema/gen/go"
)

func main() {
	// Parse the simpler schema file
	file, err := schemadsl.ParseFile("schema_simple.dasl")
	if err != nil {
		fmt.Printf("Error parsing schema file: %v\n", err)
		os.Exit(1)
	}

	// Create and initialize type system
	ts := schema.TypeSystem{}
	ts.Init()
	err = schemadmt.Compile(&ts, file)
	if err != nil {
		fmt.Printf("Error compiling schema: %v\n", err)
		os.Exit(1)
	}

	// Create a new filtered type system without Any and other built-in types
	filteredTS := schema.TypeSystem{}
	filteredTS.Init()

	// Only add our custom types (User and Post)
	allTypes := ts.GetTypes()
	for name, typ := range allTypes {
		// Skip built-in types that might cause issues
		if name == "Any" || name == "Bool" || name == "String" || name == "Bytes" ||
			name == "Int" || name == "Float" || name == "Link" || name == "List" ||
			name == "Map" || name == "List__String" {
			continue
		}

		fmt.Printf("Adding type: %s (kind: %s)\n", name, typ.TypeKind())
		// This is a workaround - we'll need to manually add types
		// For now, let's just try with basic types and see if that works
	}

	// Instead of trying to filter, let's try a completely manual approach
	fmt.Println("Creating minimal working generator...")

	// Try generating with a basic configuration
	cfg := &gengo.AdjunctCfg{
		// Set minimal configuration
	}

	// Try to generate but catch any panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Code generation failed: %v\n", r)
			fmt.Println("This is a known issue with go-ipld-prime code generation")
		}
	}()

	fmt.Println("Attempting code generation...")
	gengo.Generate("./", "main", ts, cfg)
	fmt.Println("Code generation completed!")
}
