package main

import (
	"context"
	"fmt"
	"testing"

	ts "github.com/gosuda/boxo-starter-kit/14-traversal-selector/pkg"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/require"
)

func buildBinaryTree(t *testing.T, ctx context.Context, d *ts.TraversalSelectorWrapper, level int, prefix string) cid.Cid {
	if prefix == "" {
		prefix = "root"
	}

	if level <= 1 {
		c, err := d.IpldWrapper.PutIPLDAny(ctx, map[string]any{
			"name": fmt.Sprintf("%s", prefix),
			"leaf": true,
		})
		require.NoError(t, err)
		return c
	}

	left := buildBinaryTree(t, ctx, d, level-1, prefix+"L")
	right := buildBinaryTree(t, ctx, d, level-1, prefix+"R")

	node, err := d.IpldWrapper.PutIPLDAny(ctx, map[string]any{
		"name": fmt.Sprintf("%s", prefix),
		"leaf": false,
		"L":    left,
		"R":    right,
	})
	require.NoError(t, err)

	return node
}

func loadBool(t *testing.T, n datamodel.Node, key string) bool {
	v, err := n.LookupByString(key)
	require.NoError(t, err)
	b, err := v.AsBool()
	require.NoError(t, err)
	return b
}

func loadString(t *testing.T, n datamodel.Node, key string) string {
	v, err := n.LookupByString(key)
	require.NoError(t, err)
	s, err := v.AsString()
	require.NoError(t, err)
	return s
}

func loadLink(t *testing.T, n datamodel.Node, key string) cid.Cid {
	v, err := n.LookupByString(key)
	require.NoError(t, err)
	l, err := v.AsLink()
	require.NoError(t, err)
	cl, ok := l.(cidlink.Link)
	require.True(t, ok)
	return cl.Cid
}

func TestWalkOneNode(t *testing.T) {
	ctx := context.Background()
	w, err := ts.New(nil)
	require.NoError(t, err)

	root := buildBinaryTree(t, ctx, w, 2, "root")

	visit, state := ts.NewVisitOne(root)
	err = w.WalkOneNode(ctx, root, visit)
	require.NoError(t, err)

	require.True(t, state.Found)

	isLeaf := loadBool(t, state.Rec.Node, "leaf")
	require.Equal(t, false, isLeaf)

	name := loadString(t, state.Rec.Node, "name")
	require.Equal(t, "root", name)

	L := loadLink(t, state.Rec.Node, "L")
	R := loadLink(t, state.Rec.Node, "R")
	require.NotEqual(t, cid.Undef, L)
	require.NotEqual(t, cid.Undef, R)
}

func TestWalkMatchingAll(t *testing.T) {
	ctx := context.Background()
	w, _ := ts.New(nil)
	root := buildBinaryTree(t, ctx, w, 3, "root")

	sel, err := ts.CompileSelector(ts.SelectorAll(true))
	require.NoError(t, err)

	visit, col := ts.NewVisitAll(root)
	err = w.WalkMatching(ctx, root, sel, visit)
	require.NoError(t, err)

	// 15 = 1 link + 2 link + 4 link + 14 leaves ( 8 links + 6 leaves )
	require.Equal(t, 21, len(col.Records))

	// for _, rec := range col.Records {
	// 	val, err := ipldprime.NodeToAny(rec.Node)
	// 	require.NoError(t, err)
	// 	fmt.Printf("%v\n", val)
	// }
}
