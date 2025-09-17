package ipni

import (
	"context"
	"os"

	dag "github.com/gosuda/boxo-starter-kit/04-dag-ipld/pkg"
	"github.com/ipfs/go-cid"
	"github.com/ipni/go-indexer-core"
	"github.com/ipni/go-indexer-core/cache/radixcache"
	"github.com/ipni/go-indexer-core/engine"
	"github.com/ipni/go-indexer-core/store/pebble"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

type IPNIWrapper struct {
	dagWrapper *dag.IpldWrapper
	Engine     *engine.Engine
}

func NewIPNIWrapper(path string, dagWrapper *dag.IpldWrapper) (*IPNIWrapper, error) {
	if path == "" {
		path = os.TempDir() + "ipni"
	}

	store, err := pebble.New(path, nil)
	if err != nil {
		return nil, err
	}
	// 4 MB
	cache := radixcache.New(4 * 1024 * 1024)

	eng := engine.New(store, engine.WithCache(cache), engine.WithCacheOnPut(true))
	return &IPNIWrapper{
		dagWrapper: dagWrapper,
		Engine:     eng,
	}, nil
}

func (w *IPNIWrapper) Close() error {
	if w.dagWrapper != nil {
		w.dagWrapper.BlockServiceWrapper.Close()
	}

	return w.Engine.Close()
}

func (w *IPNIWrapper) Flush() error                   { return w.Engine.Flush() }
func (w *IPNIWrapper) Size() (int64, error)           { return w.Engine.Size() }
func (w *IPNIWrapper) Stats() (*indexer.Stats, error) { return w.Engine.Stats() }

func (w *IPNIWrapper) PutMultihashes(ctx context.Context, val indexer.Value, mhs ...multihash.Multihash) error {
	if len(mhs) == 0 {
		return nil
	}
	return w.Engine.Put(val, mhs...)
}

func (w *IPNIWrapper) PutCID(ctx context.Context, val indexer.Value, c cid.Cid) error {
	return w.Engine.Put(val, c.Hash())
}

func (w *IPNIWrapper) Remove(ctx context.Context, val indexer.Value, mhs ...multihash.Multihash) error {
	return w.Engine.Remove(val, mhs...)
}

func (w *IPNIWrapper) RemoveProvider(ctx context.Context, id peer.ID) error {
	return w.Engine.RemoveProvider(ctx, id)
}

func (w *IPNIWrapper) RemoveProviderContext(ctx context.Context, id peer.ID, contextID []byte) error {
	return w.Engine.RemoveProviderContext(id, contextID)
}

func (w *IPNIWrapper) GetProvidersByCID(ctx context.Context, c cid.Cid) ([]indexer.Value, bool, error) {
	return w.Engine.Get(c.Hash())
}

func (w *IPNIWrapper) GetProviders(ctx context.Context, mh multihash.Multihash) ([]indexer.Value, bool, error) {
	return w.Engine.Get(mh)
}
