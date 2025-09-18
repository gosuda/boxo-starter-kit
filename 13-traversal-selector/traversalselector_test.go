package main

import (
	"context"
	"testing"

	ts "github.com/gosuda/boxo-starter-kit/13-traversal-selector/pkg"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func buildBinaryTree(t *testing.T, d *ts.TraversalSelectorWrapper) cid.Cid {
	ctx := context.Background()

	leaf := func(name string) cid.Cid {
		c, err := d.IpldWrapper.PutIPLDAny(ctx, map[string]any{
			"name": name,
			"leaf": true,
		})
		require.NoError(t, err)
		return c
	}
	ll := leaf("ll")
	lr := leaf("lr")
	rl := leaf("rl")
	rr := leaf("rr")

	left, err := d.IpldWrapper.PutIPLDAny(ctx, map[string]cid.Cid{
		"0": ll,
		"1": lr,
	})
	require.NoError(t, err)

	right, err := d.IpldWrapper.PutIPLDAny(ctx, map[string]cid.Cid{
		"0": rl,
		"1": rr,
	})
	require.NoError(t, err)

	root, err := d.IpldWrapper.PutIPLDAny(ctx, map[string]cid.Cid{
		"L": left,
		"R": right,
	})
	require.NoError(t, err)

	return root
}

func TestSelectAll(t *testing.T) {
	ctx := context.Background()

	ipld, err := ts.New(nil)
	require.NoError(t, err)

	root := buildBinaryTree(t, ipld)

	t.Run("SelectorAll(match=true)", func(t *testing.T) {
		sel, err := ts.SelectorAll(true)
		require.NoError(t, err)

		visit, col := ts.NewVisitAll(root, 0)
		err = ipld.WalkMatchingCid(ctx, root, sel, visit)
		require.NoError(t, err)

		require.Equal(t, 7, len(col.Records))
	})

	t.Run("SelectorAll(match=false)", func(t *testing.T) {
		sel, err := ts.SelectorAll(false)
		require.NoError(t, err)

		visit, col := ts.NewVisitAll(root, 0)
		err = ipld.WalkMatchingCid(ctx, root, sel, visit)
		require.NoError(t, err)

		require.Equal(t, 0, len(col.Records))
	})
}

func TestSelectorDepth(t *testing.T) {
	ctx := context.Background()

	ipld, err := ts.New(nil)
	require.NoError(t, err)

	root := buildBinaryTree(t, ipld)

	t.Run("Depth=0, match=true", func(t *testing.T) {
		sel, err := ts.SelectorDepth(0, true)
		require.NoError(t, err)

		visit, col := ts.NewVisitAll(root, 0)
		err = ipld.WalkMatchingCid(ctx, root, sel, visit)
		require.NoError(t, err)

		require.Equal(t, 1, len(col.Records)) // root만
	})

	t.Run("Depth=1, match=true => root + level1(2) = 3", func(t *testing.T) {
		sel, err := ts.SelectorDepth(1, true)
		require.NoError(t, err)

		visit, col := ts.NewVisitAll(root, 0)
		err = ipld.WalkMatchingCid(ctx, root, sel, visit)
		require.NoError(t, err)

		require.Equal(t, 3, len(col.Records))
	})

	t.Run("Depth=2, match=true", func(t *testing.T) {
		sel, err := ts.SelectorDepth(2, true)
		require.NoError(t, err)

		visit, col := ts.NewVisitAll(root, 0)
		err = ipld.WalkMatchingCid(ctx, root, sel, visit)
		require.NoError(t, err)

		require.Equal(t, 7, len(col.Records))
	})

	t.Run("Depth=2, match=false", func(t *testing.T) {
		sel, err := ts.SelectorDepth(2, false)
		require.NoError(t, err)

		visit, col := ts.NewVisitAll(root, 0)
		err = ipld.WalkMatchingCid(ctx, root, sel, visit)
		require.NoError(t, err)

		require.Equal(t, 0, len(col.Records))
	})
}
