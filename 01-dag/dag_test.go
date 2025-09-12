package dag_test

import (
	"context"
	"testing"

	dag "github.com/gosunuts/boxo-starter-kit/01-dag/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDagSingle(t *testing.T) {
	ctx := context.TODO()
	d := dag.New(nil)

	c1, err := d.PutAny(ctx, map[string]any{"name": "bob", "age": 30})
	require.NoError(t, err)
	c2, err := d.PutAny(ctx, map[string]any{"age": 30, "name": "bob"})
	require.NoError(t, err)
	assert.True(t, c1.Equals(c2), "same structure → same CID")

	// get any type
	data, err := d.GetAny(ctx, c1)
	require.NoError(t, err)
	m, ok := data.(map[string]any)
	require.True(t, ok, "data must be a map")
	assert.Equal(t, map[string]any{"name": "bob", "age": int64(30)}, m, "data must match original")

	// get node and lookup
	n, err := d.Get(ctx, c1)
	require.NoError(t, err)
	name, err := n.LookupByString("name")
	require.NoError(t, err)
	ns, err := name.AsString()
	require.NoError(t, err)
	assert.Equal(t, "bob", ns, "name must be 'bob'")
}

func TestDagNestedLinks(t *testing.T) {
	ctx := context.TODO()
	d := dag.New(nil)

	c1, err := d.PutAny(ctx, map[string]any{"name": "bob", "age": 30})
	require.NoError(t, err)
	c2, err := d.PutAny(ctx, map[string]any{"child": c1})
	require.NoError(t, err)
	c3, err := d.PutAny(ctx, map[string]any{"grandchild": c2})
	require.NoError(t, err)

	n1, resolved1, err := d.ResolvePath(ctx, c3, "grandchild")
	require.NoError(t, err)
	assert.True(t, resolved1.Equals(c2), "resolved must be c2 after following grandchild link")
	k, _ := n1.LookupByString("child")
	assert.Equal(t, "map", n1.Kind().String())
	assert.Equal(t, "link", k.Kind().String())

	n2, resolved2, err := d.ResolvePath(ctx, c3, "grandchild/child")
	require.NoError(t, err)
	assert.True(t, resolved2.Equals(c1), "resolved must be c1 after following child link")
	nameNode, err := n2.LookupByString("name")
	require.NoError(t, err)
	ns, _ := nameNode.AsString()
	assert.Equal(t, "bob", ns)

	n3, resolved3, err := d.ResolvePath(ctx, c3, "grandchild/child/name")
	require.NoError(t, err)
	assert.True(t, resolved3.Equals(c1), "resolved remains c1 at leaf value access")
	s, _ := n3.AsString()
	assert.Equal(t, "bob", s)
}
