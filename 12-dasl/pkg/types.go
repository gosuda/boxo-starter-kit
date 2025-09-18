package dasl

import (
	_ "embed"

	"github.com/ipfs/go-cid"
)

//go:embed codegen/schema.dasl
var schemaDasl string

type User struct {
	Id      string    `ipld:"id"`
	Name    string    `ipld:"name"`
	Email   string    `ipld:"email"`
	Friends []cid.Cid `ipld:"friends"`
	Avatar  []byte    `ipld:"avatar"`
}

type Post struct {
	Id        string   `ipld:"id"`
	Author    cid.Cid  `ipld:"author"`
	Title     string   `ipld:"title"`
	Body      string   `ipld:"body"`
	Tags      []string `ipld:"tags"`
	CreatedAt int64    `ipld:"createdAt"`
}

type Root struct {
	Users User `ipld:"user"`
	Posts Post `ipld:"post"`
}
