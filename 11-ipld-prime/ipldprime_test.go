package main

import (
	"context"
	"testing"
	"time"

	ipld "github.com/gosuda/boxo-starter-kit/11-ipld-prime/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPLD(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), time.Second*5)
	defer timeout()

	d, err := ipld.NewDefault(nil, nil)
	require.NoError(t, err)

	c1, err := d.PutIPLDAny(ctx, map[string]any{"name": "bob", "age": 30})
	require.NoError(t, err)
	c2, err := d.PutIPLDAny(ctx, map[string]any{"age": 30, "name": "bob"})
	require.NoError(t, err)
	assert.True(t, c1.Equals(c2), "same structure → same CID")

	// get any type
	data, err := d.GetIPLDAny(ctx, c1)
	require.NoError(t, err)
	m, ok := data.(map[string]any)
	require.True(t, ok, "data must be a map")
	assert.Equal(t, map[string]any{"name": "bob", "age": int64(30)}, m, "data must match original")

	// get node and lookup
	n, err := d.GetIPLD(ctx, c1)
	require.NoError(t, err)
	name, err := n.LookupByString("name")
	require.NoError(t, err)
	ns, err := name.AsString()
	require.NoError(t, err)
	assert.Equal(t, "bob", ns, "name must be 'bob'")
}

func TestIpldLink(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), time.Second*5)
	defer timeout()

	d, err := ipld.NewDefault(nil, nil)
	require.NoError(t, err)
	// leaf1
	l1b := map[string]any{
		"name": "leaf1",
		"age":  30,
	}
	lnk1, err := d.PutIPLDAny(ctx, l1b)
	require.NoError(t, err)

	// leaf2
	l2b := map[string]any{
		"name": "leaf2",
	}
	lnk2, err := d.PutIPLDAny(ctx, l2b)
	require.NoError(t, err)

	// root
	rb := map[string]any{
		"L1": lnk1,
		"L2": lnk2,
	}

	rl, err := d.PutIPLDAny(ctx, rb)
	require.NoError(t, err)

	// resolve L1
	n1, resolved1, err := d.ResolvePath(ctx, rl, "L1")
	require.NoError(t, err)
	assert.True(t, resolved1.Equals(lnk1), "resolved must be lnk1 after following L1 link")
	k, _ := n1.LookupByString("name")
	val, err := k.AsString()
	require.NoError(t, err)
	assert.Equal(t, "leaf1", val)
	assert.Equal(t, "map", n1.Kind().String())
	assert.Equal(t, "string", k.Kind().String())

	// resolve L2
	n2, resolved2, err := d.ResolvePath(ctx, rl, "L2")
	require.NoError(t, err)
	assert.True(t, resolved2.Equals(lnk2), "resolved must be lnk2 after following L2 link")
	k2, _ := n2.LookupByString("name")
	val2, err := k2.AsString()
	require.NoError(t, err)
	assert.Equal(t, "leaf2", val2)
	assert.Equal(t, "map", n2.Kind().String())
	assert.Equal(t, "string", k2.Kind().String())

	// resolve L1/name
	n3, resolved3, err := d.ResolvePath(ctx, rl, "L1/name")
	require.NoError(t, err)
	assert.True(t, resolved3.Equals(lnk1), "resolved remains lnk1 at leaf value access")
	s, _ := n3.AsString()
	assert.Equal(t, "leaf1", s)

	// resolve L2/name
	n4, resolved4, err := d.ResolvePath(ctx, rl, "L2/name")
	require.NoError(t, err)
	assert.True(t, resolved4.Equals(lnk2), "resolved remains lnk2 at leaf value access")
	s2, _ := n4.AsString()
	assert.Equal(t, "leaf2", s2)

}
