package persistent_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

func TestPersistentBackends(t *testing.T) {
	ctx := context.TODO()
	data := []byte("example persistent block")

	for _, test := range []struct {
		name string
		typ  persistent.PersistentType
	}{
		{"file", persistent.File},
		{"badger", persistent.Badgerdb},
		{"pebble", persistent.Pebbledb},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, string(test.typ))

		pw, err := persistent.New(test.typ, path)
		require.NoError(t, err, "must construct persistent wrapper")

		// Put
		c, err := pw.PutRaw(ctx, data)
		require.NoError(t, err)

		// Has
		ok, err := pw.Has(ctx, c)
		require.NoError(t, err)
		assert.True(t, ok, "Has must be true after Put")

		// GetRaw
		got, err := pw.GetRaw(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, data, got)

		// GetSize
		size, err := pw.GetSize(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, len(data), size)

		// Delete
		err = pw.Delete(ctx, c)
		require.NoError(t, err)

		ok, err = pw.Has(ctx, c)
		require.NoError(t, err)
		assert.False(t, ok, "Has must be false after Delete")

		c2, err := pw.PutRaw(ctx, data)
		require.NoError(t, err)
		assert.True(t, c.Equals(c2), "same bytes → deterministic CID")

		// Cleanup
		err = pw.Close()
		require.NoError(t, err, "must close persistent wrapper")
	}
}
