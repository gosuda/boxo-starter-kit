package ipni

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipni/go-indexer-core"
	"github.com/ipni/go-indexer-core/cache/radixcache"
	"github.com/ipni/go-indexer-core/engine"
	"github.com/ipni/go-indexer-core/store/memory"
	"github.com/ipni/go-indexer-core/store/pebble"
	md "github.com/ipni/go-libipni/metadata"
	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
)

type IPNIWrapper struct {
	Engine *engine.Engine

	Provider   *ProviderWrapper
	Subscriber *SubscriberWrapper
}

func New(path, topic string, persistentWrapper *persistent.PersistentWrapper, hostWrapper *network.HostWrapper, ipldWrapper *ipldprime.IpldWrapper) (*IPNIWrapper, error) {
	var err error
	var store indexer.Interface
	if path == "" {
		store = memory.New()
	} else {
		store, err = pebble.New(path, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create pebble store: %w", err)
		}
	}

	if persistentWrapper == nil {
		persistentWrapper, err = persistent.New(persistent.Memory, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create persistent wrapper: %w", err)
		}
	}
	if hostWrapper == nil {
		hostWrapper, err = network.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p host: %w", err)
		}
	}
	if ipldWrapper == nil {
		ipldWrapper, err = ipldprime.NewDefault(nil, persistentWrapper)
		if err != nil {
			return nil, fmt.Errorf("failed to create ipld wrapper: %w", err)
		}
	}

	provider, err := NewProviderWrapper(path, topic, persistentWrapper, hostWrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	subscriber, err := NewSubscriberWrapper(hostWrapper, ipldWrapper, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	cache := radixcache.New(4 * 1024 * 1024)
	eng := engine.New(store, engine.WithCache(cache), engine.WithCacheOnPut(true))

	return &IPNIWrapper{
		Engine:     eng,
		Provider:   provider,
		Subscriber: subscriber,
	}, nil
}

func (w *IPNIWrapper) Start(ctx context.Context) error {
	// start provider
	if err := w.Provider.Start(ctx); err != nil {
		return fmt.Errorf("provider engine start: %w", err)
	}

	// start subscriber
	if err := w.Subscriber.Start(ctx, w.Put, w.Remove); err != nil {
		return fmt.Errorf("subscriber start: %w", err)
	}
	return nil
}

func (w *IPNIWrapper) Close() error                   { return w.Engine.Close() }
func (w *IPNIWrapper) Flush() error                   { return w.Engine.Flush() }
func (w *IPNIWrapper) Size() (int64, error)           { return w.Engine.Size() }
func (w *IPNIWrapper) Stats() (*indexer.Stats, error) { return w.Engine.Stats() }

func (w *IPNIWrapper) PutMultihashes(val indexer.Value, mhs ...mh.Multihash) error {
	if len(mhs) == 0 {
		return nil
	}
	return w.Engine.Put(val, mhs...)
}

func (w *IPNIWrapper) Put(providerID peer.ID, contextID []byte, metadataBytes []byte, mhs ...mh.Multihash) error {

	val := indexer.Value{ProviderID: providerID, ContextID: contextID, MetadataBytes: metadataBytes}
	return w.PutMultihashes(val, mhs...)
}

func (w *IPNIWrapper) PutCID(providerID peer.ID, contextID []byte, metadataBytes []byte, c ...cid.Cid) error {
	if len(c) == 0 {
		return nil
	}
	mhs := make([]mh.Multihash, 0, len(c))
	for _, c := range c {
		mhs = append(mhs, c.Hash())
	}

	return w.Put(providerID, contextID, metadataBytes, mhs...)
}

var (
	bitswapMeta, _ = md.Bitswap{}.MarshalBinary()
	httpMeta, _    = md.IpfsGatewayHttp{}.MarshalBinary()
)

func (w *IPNIWrapper) PutBitswap(pid peer.ID, contextID []byte, c ...cid.Cid) error {
	return w.PutCID(pid, contextID, bitswapMeta, c...)
}

func (w *IPNIWrapper) PutGraphSyncFilecoin(
	pid peer.ID,
	piece cid.Cid,
	verifiedDeal bool,
	fastRetrieval bool,
	contextID []byte,
	c ...cid.Cid,
) error {
	meta := md.GraphsyncFilecoinV1{
		PieceCID:      piece,
		VerifiedDeal:  verifiedDeal,
		FastRetrieval: fastRetrieval,
	}
	metaBytes, err := meta.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal GraphSyncFilecoin metadata: %w", err)
	}
	return w.PutCID(pid, contextID, metaBytes, c...)
}

func (w *IPNIWrapper) PutHTTP(pid peer.ID, contextID []byte, c ...cid.Cid) error {
	return w.PutCID(pid, contextID, httpMeta, c...)
}

func (w *IPNIWrapper) RemoveMultihashes(val indexer.Value, mhs ...mh.Multihash) error {
	return w.Engine.Remove(val, mhs...)
}

func (w *IPNIWrapper) Remove(id peer.ID, contextID []byte) error {
	return w.Engine.RemoveProviderContext(id, contextID)
}

func (w *IPNIWrapper) RemoveProvider(ctx context.Context, id peer.ID) error {
	return w.Engine.RemoveProvider(ctx, id)
}

func (w *IPNIWrapper) GetProvidersByCID(c cid.Cid) ([]indexer.Value, bool, error) {
	return w.Engine.Get(c.Hash())
}

func (w *IPNIWrapper) GetProviders(mh mh.Multihash) ([]indexer.Value, bool, error) {
	return w.Engine.Get(mh)
}

// Planning helpers (scoring-only)
// RankedFetchers returns a simplified prioritized list of (providerID, transport)
// derived from the Plan, for easy wiring into a multifetcher.
type RankedFetcher struct {
	ProviderID string
	Proto      TransportKind
	Meta       map[string]string // small hints like region / partial_car
}

func (w *IPNIWrapper) RankedFetchersByCID(ctx context.Context, c cid.Cid, intent Intent) ([]RankedFetcher, bool, error) {
	attempts, hit, err := w.PlanByCID(ctx, c, intent)
	if err != nil {
		return nil, hit, err
	}
	out := make([]RankedFetcher, 0, len(attempts))
	for _, a := range attempts {
		out = append(out, RankedFetcher{
			ProviderID: a.ProviderID,
			Proto:      a.Proto,
			Meta:       a.Meta,
		})
	}
	return out, hit, nil
}

func (w *IPNIWrapper) RankedFetchers(ctx context.Context, mh mh.Multihash, intent Intent) ([]RankedFetcher, bool, error) {
	attempts, hit, err := w.Plan(ctx, mh, intent)
	if err != nil {
		return nil, hit, err
	}
	out := make([]RankedFetcher, 0, len(attempts))
	for _, a := range attempts {
		out = append(out, RankedFetcher{
			ProviderID: a.ProviderID,
			Proto:      a.Proto,
			Meta:       a.Meta,
		})
	}
	return out, hit, nil
}

// PlanByCID reads local providers (engine), normalizes them, and returns a scoring-only Plan.
// This does NOT fetch from a remote indexer and does NOT execute any network transfer.
func (w *IPNIWrapper) PlanByCID(ctx context.Context, c cid.Cid, intent Intent) ([]Attempt, bool, error) {
	vals, hit, err := w.Engine.Get(c.Hash())
	if err != nil {
		return nil, hit, err
	}
	pl := Plan(vals, intent, nil)
	if pl == nil {
		hit = false
	}

	return pl, hit, nil
}

// Plan reads local providers (engine) by multihash, normalizes them, and returns a scoring-only Plan.
func (w *IPNIWrapper) Plan(ctx context.Context, mh mh.Multihash, intent Intent) ([]Attempt, bool, error) {
	vals, hit, err := w.Engine.Get(mh)
	if err != nil {
		return nil, hit, err
	}
	pl := Plan(vals, intent, nil)
	if pl == nil {
		hit = false
	}

	return pl, hit, nil
}
