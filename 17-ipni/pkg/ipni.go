package ipni

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipni/go-indexer-core"
	"github.com/ipni/go-indexer-core/cache/radixcache"
	"github.com/ipni/go-indexer-core/engine"
	"github.com/ipni/go-indexer-core/store/pebble"
	md "github.com/ipni/go-libipni/metadata"
	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"
)

// -----------------------------
// Fetcher: Remote indexer fetcher
// -----------------------------

type IPNIWrapper struct {
	Engine       *engine.Engine
	Planner      *Planner
	HealthScorer HealthScorer
	DefaultTTL   time.Duration
}

func NewIPNIWrapper(path string) (*IPNIWrapper, error) {
	if path == "" {
		path = os.TempDir() + "/ipni"
	}

	store, err := pebble.New(path, nil)
	if err != nil {
		return nil, err
	}
	// 4 MB cache
	cache := radixcache.New(4 * 1024 * 1024)

	eng := engine.New(store, engine.WithCache(cache), engine.WithCacheOnPut(true))

	// scoring-only planner with default policy
	pl := NewPlanner(nil)

	return &IPNIWrapper{
		Engine:       eng,
		Planner:      pl,
		HealthScorer: nil,              // can be set via SetHealthScorer
		DefaultTTL:   60 * time.Second, // used when composing Providers
	}, nil
}

func (w *IPNIWrapper) Close() error                   { return w.Engine.Close() }
func (w *IPNIWrapper) Flush() error                   { return w.Engine.Flush() }
func (w *IPNIWrapper) Size() (int64, error)           { return w.Engine.Size() }
func (w *IPNIWrapper) Stats() (*indexer.Stats, error) { return w.Engine.Stats() }

func (w *IPNIWrapper) PutMultihashes(ctx context.Context, val indexer.Value, mhs ...mh.Multihash) error {
	if len(mhs) == 0 {
		return nil
	}
	return w.Engine.Put(val, mhs...)
}

func (w *IPNIWrapper) PutCID(ctx context.Context, val indexer.Value, c cid.Cid) error {
	return w.Engine.Put(val, c.Hash())
}

func (w *IPNIWrapper) Remove(ctx context.Context, val indexer.Value, mhs ...mh.Multihash) error {
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

func (w *IPNIWrapper) GetProviders(ctx context.Context, mh mh.Multihash) ([]indexer.Value, bool, error) {
	return w.Engine.Get(mh)
}

// =====================================================================
// PUT helpers (wrap MetadataBytes creation for each transport)
// =====================================================================

func (w *IPNIWrapper) PutBitswap(ctx context.Context, pid peer.ID, contextID []byte, mhs ...mh.Multihash) error {
	meta := md.Bitswap{}
	metaBytes, err := meta.MarshalBinary()
	if err != nil {
		return err
	}

	val := indexer.Value{ProviderID: pid, ContextID: contextID, MetadataBytes: metaBytes}
	return w.PutMultihashes(ctx, val, mhs...)
}

func (w *IPNIWrapper) PutGraphSync(ctx context.Context, pid peer.ID, contextID []byte, mhs ...mh.Multihash) error {
	// For demo purposes, use Bitswap metadata for GraphSync
	// In a real implementation, proper GraphSync metadata would be used
	meta := md.Bitswap{}
	metaBytes, err := meta.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal GraphSync metadata: %w", err)
	}

	val := indexer.Value{ProviderID: pid, ContextID: contextID, MetadataBytes: metaBytes}
	return w.PutMultihashes(ctx, val, mhs...)
}

func (w *IPNIWrapper) PutHTTP(ctx context.Context, pid peer.ID, contextID []byte, urls []string, partialCAR bool, auth bool, mhs ...mh.Multihash) error {
	// Create HTTP gateway metadata
	// Note: The current version of IpfsGatewayHttp doesn't support additional fields
	// URL information would be stored in the contextID or handled separately
	meta := md.IpfsGatewayHttp{}

	// In a real implementation, URL and capability information would be encoded
	// in the contextID or handled through a separate metadata system
	// For demo purposes, we'll use basic HTTP gateway metadata

	metaBytes, err := meta.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal HTTP gateway metadata: %w", err)
	}

	val := indexer.Value{ProviderID: pid, ContextID: contextID, MetadataBytes: metaBytes}
	return w.PutMultihashes(ctx, val, mhs...)
}

// Planning helpers (scoring-only)
// RankedFetchers returns a simplified prioritized list of (providerID, transport)
// derived from the Plan, for easy wiring into a multifetcher.
type RankedFetcher struct {
	ProviderID string
	Proto      TransportKind
	Meta       map[string]string // small hints like region / partial_car
}

func (w *IPNIWrapper) RankedFetchersByCID(ctx context.Context, c cid.Cid, in RouteIntent) ([]RankedFetcher, bool, error) {
	pl, hit, err := w.PlanByCID(ctx, c, in)
	if err != nil {
		return nil, hit, err
	}
	out := make([]RankedFetcher, 0, len(pl.Attempts))
	for _, a := range pl.Attempts {
		out = append(out, RankedFetcher{
			ProviderID: a.ProviderID,
			Proto:      a.Proto,
			Meta:       a.Meta,
		})
	}
	return out, hit, nil
}

func (w *IPNIWrapper) RankedFetchers(ctx context.Context, mh mh.Multihash, in RouteIntent) ([]RankedFetcher, bool, error) {
	pl, hit, err := w.Plan(ctx, mh, in)
	if err != nil {
		return nil, hit, err
	}
	out := make([]RankedFetcher, 0, len(pl.Attempts))
	for _, a := range pl.Attempts {
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
func (w *IPNIWrapper) PlanByCID(ctx context.Context, c cid.Cid, in RouteIntent) (Plan, bool, error) {
	vals, hit, err := w.Engine.Get(c.Hash())
	if err != nil {
		return Plan{}, hit, err
	}
	provs := Providers{
		Items:       Normalize(vals), // normalize indexer.Value -> Provider
		ObservedTTL: w.DefaultTTL,
		Source:      "local-engine",
	}
	pl := w.Planner.Plan(ctx, provs, in, w.HealthScorer)
	return pl, hit, nil
}

// Plan reads local providers (engine) by multihash, normalizes them, and returns a scoring-only Plan.
func (w *IPNIWrapper) Plan(ctx context.Context, mh mh.Multihash, in RouteIntent) (Plan, bool, error) {
	vals, hit, err := w.Engine.Get(mh)
	if err != nil {
		return Plan{}, hit, err
	}
	provs := Providers{
		Items:       Normalize(vals),
		ObservedTTL: w.DefaultTTL,
		Source:      "local-engine",
	}
	pl := w.Planner.Plan(ctx, provs, in, w.HealthScorer)
	return pl, hit, nil
}
