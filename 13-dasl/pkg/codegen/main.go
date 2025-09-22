package main

import (
	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"
	gengo "github.com/ipld/go-ipld-prime/schema/gen/go"
)

func main() {
	file, err := schemadsl.ParseFile("schema.dasl")
	if err != nil {
		panic(err)
	}

	ts := schema.TypeSystem{}
	ts.Init()
	err = schemadmt.Compile(&ts, file)
	if err != nil {
		panic(err)
	}

	// It is not working with the latest version.
	// https://github.com/ipld/go-ipld-prime/issues/528
	// TODO: Update the code to work.
	gengo.Generate("./", "main", ts, &gengo.AdjunctCfg{})
}
