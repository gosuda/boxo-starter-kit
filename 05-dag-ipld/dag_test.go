package main

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/require"

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
)

func TestDagServiceAddGet(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), time.Second*5)
	defer timeout()

	d, err := dag.NewDagServiceWrapper(ctx, nil, nil)
	require.NoError(t, err)

	// make a dag-pb node
	payload := []byte("hello dag-pb")

	cid, err := d.AddRaw(ctx, payload)
	require.NoError(t, err)

	// get back
	got, err := d.Get(ctx, cid)
	require.NoError(t, err)
	require.Equal(t, cid, got.Cid())
	require.Equal(t, payload, got.RawData())

	// clean up
	err = d.Remove(ctx, cid)
	require.NoError(t, err)

	_, err = d.Get(ctx, cid)
	require.Error(t, err, "must error after delete")
}

// helper: new leaf ProtoNode with data
func newLeaf(data string) format.Node {
	return merkledag.NewRawNode([]byte(data))
}

// helper: add a named link from parent -> child (child must have a CID)
func addNamedLink(t *testing.T, parent *merkledag.ProtoNode, name string, child format.Node) {
	t.Helper()
	l, err := format.MakeLink(child)
	require.NoError(t, err)
	err = parent.AddRawLink(name, l)
	require.NoError(t, err)
}

func TestPutAndGetNode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	w, err := dag.NewIpldWrapper(ctx, nil, nil)
	require.NoError(t, err)

	c1, err := w.AddRaw(ctx, []byte("hello"))
	require.NoError(t, err)
	require.NotEqual(t, cid.Undef, c1)

	got, err := w.GetNode(ctx, c1)
	require.NoError(t, err)
	require.Equal(t, c1, got.Cid())
}

func TestResolvePath_ByName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	w, err := dag.NewIpldWrapper(ctx, nil, nil)
	require.NoError(t, err)

	// Build: root --a--> mid --b--> leaf
	leaf := newLeaf("leaf")
	leafCID, err := w.PutNode(ctx, leaf)
	require.NoError(t, err)

	mid := merkledag.NodeWithData(nil)
	addNamedLink(t, mid, "b", leaf)
	_, err = w.PutNode(ctx, mid)
	require.NoError(t, err)

	root := merkledag.NodeWithData(nil)
	addNamedLink(t, root, "a", mid)
	rootCID, err := w.PutNode(ctx, root)
	require.NoError(t, err)

	// Resolve "a/b" → leaf
	gotNode, gotCID, err := w.ResolvePath(ctx, rootCID, "a/b")
	require.NoError(t, err)
	require.Equal(t, leafCID, gotCID)
	require.Equal(t, leafCID, gotNode.Cid())
}

func TestResolvePath_ByIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	w, err := dag.NewIpldWrapper(ctx, nil, nil)
	require.NoError(t, err)

	// Build same chain but use indices "0/0"
	leaf := newLeaf("leaf-index")
	leafCID, err := w.PutNode(ctx, leaf)
	require.NoError(t, err)

	mid := merkledag.NodeWithData(nil)
	addNamedLink(t, mid, "only", leaf) // single link => index 0
	_, err = w.PutNode(ctx, mid)
	require.NoError(t, err)

	root := merkledag.NodeWithData(nil)
	addNamedLink(t, root, "first", mid) // single link => index 0
	rootCID, err := w.PutNode(ctx, root)
	require.NoError(t, err)

	gotNode, gotCID, err := w.ResolvePath(ctx, rootCID, "0/0")
	require.NoError(t, err)
	require.Equal(t, leafCID, gotCID)
	require.Equal(t, leafCID, gotNode.Cid())
}

func TestResolvePath_MixedNameIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	w, err := dag.NewIpldWrapper(ctx, nil, nil)
	require.NoError(t, err)

	leaf1 := newLeaf("L1")
	leaf1CID, err := w.PutNode(ctx, leaf1)
	require.NoError(t, err)

	leaf2 := newLeaf("L2")
	_, err = w.PutNode(ctx, leaf2)
	require.NoError(t, err)

	// mid has two links: [x: leaf1, y: leaf2]
	mid := merkledag.NodeWithData(nil)
	addNamedLink(t, mid, "x", leaf1)
	addNamedLink(t, mid, "y", leaf2)
	_, err = w.PutNode(ctx, mid)
	require.NoError(t, err)

	// root has one link "m" → mid
	root := merkledag.NodeWithData(nil)
	addNamedLink(t, root, "m", mid)
	rootCID, err := w.PutNode(ctx, root)
	require.NoError(t, err)

	// "m/0" should pick the first link under mid => leaf1
	gotNode, gotCID, err := w.ResolvePath(ctx, rootCID, "m/0")
	require.NoError(t, err)
	require.Equal(t, leaf1CID, gotCID)
	require.Equal(t, leaf1CID, gotNode.Cid())
}

func TestResolvePath_EmptyPathReturnsRoot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	w, err := dag.NewIpldWrapper(ctx, nil, nil)
	require.NoError(t, err)

	root := merkledag.NodeWithData(nil)
	rootCID, err := w.PutNode(ctx, root)
	require.NoError(t, err)

	gotNode, gotCID, err := w.ResolvePath(ctx, rootCID, "")
	require.NoError(t, err)
	require.Equal(t, rootCID, gotCID)
	require.Equal(t, rootCID, gotNode.Cid())
}

func TestResolvePath_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	w, err := dag.NewIpldWrapper(ctx, nil, nil)
	require.NoError(t, err)

	leaf := newLeaf("leaf")
	_, err = w.PutNode(ctx, leaf)
	require.NoError(t, err)

	root := merkledag.NodeWithData(nil) // no links
	rootCID, err := w.PutNode(ctx, root)
	require.NoError(t, err)

	// missing link name
	_, _, err = w.ResolvePath(ctx, rootCID, "missing")
	require.Error(t, err, "expected error for missing link name")

	// index out of range
	_, _, err = w.ResolvePath(ctx, rootCID, "0")
	require.Error(t, err, "expected error for index out of range")
}
