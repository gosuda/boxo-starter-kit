package main

import (
	"fmt"
	"os"

	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"
)

func main() {
	// Parse the simpler schema file
	file, err := schemadsl.ParseFile("schema_simple.dasl")
	if err != nil {
		fmt.Printf("Error parsing schema file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed schema file successfully\n")
	fmt.Printf("File content: %+v\n", file)

	// Create and initialize type system
	ts := schema.TypeSystem{}
	ts.Init()
	err = schemadmt.Compile(&ts, file)
	if err != nil {
		fmt.Printf("Error compiling schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Schema compiled successfully\n")
	fmt.Printf("Types in schema: %v\n", ts.GetTypes())

	// List all types
	for name, typ := range ts.GetTypes() {
		fmt.Printf("Type: %s, Kind: %s\n", name, typ.TypeKind())
	}

	fmt.Println("Schema compilation completed without generating code")
}