package main

import (
	"context"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
)

func TestBlockStore(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{
			name: "raw block",
			data: []byte("hello raw block"),
		},
		{
			name: "empty block",
			data: []byte{},
		},
		{
			name: "large block",
			data: make([]byte, 1024*1024),
		},
	} {
		ctx := context.TODO()
		s := block.NewInMemory()

		c, err := s.PutV0Cid(ctx, test.data)
		require.NoError(t, err, "Put should not error", test.name)

		ok, err := s.Has(ctx, c)
		require.NoError(t, err)
		assert.True(t, ok, "Has must be true after Put", test.name)

		got, err := s.GetRaw(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, test.data, got, "Get must return the same bytes", test.name)

		size, err := s.GetSize(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, len(test.data), size, "GetSize must return the correct size", test.name)

		err = s.Delete(ctx, c)
		require.NoError(t, err, "Delete should not error", test.name)

		ok, err = s.Has(ctx, c)
		require.NoError(t, err)
		assert.False(t, ok, "Has must be false after Delete", test.name)

		c2, err := s.PutV0Cid(ctx, test.data)
		require.NoError(t, err, "Put should not error", test.name)
		assert.Equal(t, c, c2, "Put the same data must return the deterministic CID", test.name)
	}
}

func TestCidVersion(t *testing.T) {
	ctx := context.TODO()
	store := block.NewInMemory()
	data := []byte("cid version demo data")

	t.Run("v0_legacy", func(t *testing.T) {
		c0, err := store.PutV0Cid(ctx, data)
		require.NoError(t, err)
		c0b, err := store.PutV0Cid(ctx, data)
		require.NoError(t, err)

		// CIDv0 must always be identical for the same input bytes
		assert.True(t, c0.Equals(c0b), "CIDv0 must be deterministic for the same content")
		assert.Equal(t, c0.String(), c0b.String(), "CIDv0 string representation must be the same")
		assert.Equal(t, uint64(0), c0.Version())
		assert.True(t, strings.HasPrefix(c0.String(), "Qm"),
			"CIDv0 string representation should start with Qm (base58)")
	})

	t.Run("v1_diff_codec", func(t *testing.T) {
		// Store using default prefix (v1 + raw + sha2-256)
		cRaw, err := store.PutV1Cid(ctx, data, block.NewV1Prefix(0, 0, 0))
		require.NoError(t, err)

		// Same multihash, but explicitly labeled as dag-pb instead of raw
		cProtobuf := cid.NewCidV1(uint64(mc.DagPb), cRaw.Hash())

		// Different CIDs (different codec label)
		assert.False(t, cRaw.Equals(cProtobuf), "v1(raw) and v1(dag-pb) should be different CIDs")
		// But same multihash underneath (same content)
		assert.Equal(t, cRaw.Hash(), cProtobuf.Hash(), "same content → same multihash")

		// Blockstore uses multihash as the key → both resolve to the same data
		out1, err := store.GetRaw(ctx, cRaw)
		require.NoError(t, err)
		out2, err := store.GetRaw(ctx, cProtobuf)
		require.NoError(t, err)

		assert.Equal(t, data, out1)
		assert.Equal(t, data, out2)
	})

	t.Run("v1_diff_hash", func(t *testing.T) {
		// Store with default prefix (v1 + raw + sha2-256)
		cRaw, err := store.PutV1Cid(ctx, data, block.NewV1Prefix(0, 0, 0))
		require.NoError(t, err)

		// Build a CID with BLAKE3 hash instead of SHA2-256
		sumB3, err := mh.Sum(data, mh.BLAKE3, 32)
		require.NoError(t, err)
		cB3 := cid.NewCidV1(uint64(mc.Raw), sumB3)

		// Different hash algorithm → completely different multihash → different block key
		assert.NotEqual(t, cRaw.Hash(), cB3.Hash(), "different hash algo → different multihash → different block key")

		// Not found because we never stored a block under the BLAKE3 multihash
		_, err = store.Get(ctx, cB3)
		require.Error(t, err, "not found until we Put the BLAKE3-identified block")

		// Now put it explicitly with BLAKE3
		cB3Put, err := store.PutV1Cid(ctx, data, block.NewV1Prefix(0, mh.BLAKE3, 32))
		require.NoError(t, err)
		assert.Equal(t, cB3, cB3Put, "same prefix + hash → same CID")

		// Now we can get it
		out, err := store.GetRaw(ctx, cB3)
		require.NoError(t, err)
		assert.Equal(t, data, out)
	})
}

func TestAllKeysChan(t *testing.T) {
	ctx := context.TODO()
	store := block.NewInMemory()

	data := [][]byte{
		[]byte("block 1"),
		[]byte("block 2"),
		[]byte("block 3"),
	}

	var cids []cid.Cid
	for _, d := range data {
		c, err := store.PutV1Cid(ctx, d, nil)
		require.NoError(t, err)
		cids = append(cids, c)
	}

	ch, err := store.AllKeysChan(ctx)
	require.NoError(t, err)

	// cid can be different, because of cid encode format
	var gotCids []cid.Cid
	for c := range ch {
		gotCids = append(gotCids, c)
	}

	for _, gotcid := range gotCids {
		gotData, err := store.GetRaw(ctx, gotcid)
		require.NoError(t, err)
		require.Contains(t, data, gotData, "AllKeysChan must return only stored CIDs")
	}

	require.Equal(t, len(cids), len(gotCids), "AllKeysChan must return all stored CIDs")
	require.ElementsMatch(t, cids, gotCids, "AllKeysChan must return all stored CIDs")
}
