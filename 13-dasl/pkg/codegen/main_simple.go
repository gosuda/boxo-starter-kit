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

	// Generate Go code
	fmt.Println("Generating Go code from DASL schema...")
	gengo.Generate("./", "main", ts, &gengo.AdjunctCfg{})
	fmt.Println("Code generation completed successfully!")
}
