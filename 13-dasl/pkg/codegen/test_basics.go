package main

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/schema"
)

func testBasics() {
	// Test basic type system creation
	ts := schema.TypeSystem{}
	ts.Init()

	fmt.Printf("Initial types in system:\n")
	for name, typ := range ts.GetTypes() {
		fmt.Printf("  - %s (%s)\n", name, typ.TypeKind())
	}

	// Test MustTypeSystem
	ts2 := schema.MustTypeSystem()
	fmt.Printf("\nMustTypeSystem types:\n")
	for name, typ := range ts2.GetTypes() {
		fmt.Printf("  - %s (%s)\n", name, typ.TypeKind())
	}
}

func main() {
	testBasics()
}
