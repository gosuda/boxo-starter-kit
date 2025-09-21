package multifetcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/cbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p/core/peer"

	bitswap "github.com/gosuda/boxo-starter-kit/04-bitswap/pkg"
	graphsync "github.com/gosuda/boxo-starter-kit/15-graphsync/pkg"
	ipni "github.com/gosuda/boxo-starter-kit/17-ipni/pkg"
)

// FetchResult represents the result of a fetch operation
type FetchResult struct {
	Protocol  string
	Provider  string
	Data      []byte
	Error     error
	Duration  time.Duration
	CID       cid.Cid
}

// FetcherConfig contains configuration for the multifetcher
type FetcherConfig struct {
	MaxConcurrent    int           // Maximum concurrent fetchers
	Timeout          time.Duration // Overall timeout
	StaggerDelay     time.Duration // Delay between starting fetchers
	CancelOnFirstWin bool          // Cancel other fetchers on first success
}

// DefaultConfig returns sensible defaults for fetcher configuration
func DefaultConfig() FetcherConfig {
	return FetcherConfig{
		MaxConcurrent:    3,
		Timeout:          30 * time.Second,
		StaggerDelay:     150 * time.Millisecond,
		CancelOnFirstWin: true,
	}
}

// MultiFetcher orchestrates parallel fetching across multiple protocols
type MultiFetcher struct {
	config       FetcherConfig
	ipni         *ipni.IPNIWrapper
	graphsync    *graphsync.GraphSyncWrapper
	bitswap      *bitswap.BitswapWrapper
	httpFetcher  *HTTPFetcher
	mu           sync.RWMutex
	metrics      *Metrics
}

// Metrics tracks performance across protocols
type Metrics struct {
	mu                 sync.RWMutex
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	ProtocolStats      map[string]*ProtocolMetrics
}

type ProtocolMetrics struct {
	Attempts        int64
	Successes       int64
	Failures        int64
	AvgLatency      time.Duration
	TotalLatency    time.Duration
	BytesTransferred int64
}

// NewMultiFetcher creates a new multifetcher instance
func NewMultiFetcher(
	ipni *ipni.IPNIWrapper,
	graphsync *graphsync.GraphSyncWrapper,
	bitswap *bitswap.BitswapWrapper,
	config *FetcherConfig,
) *MultiFetcher {
	if config == nil {
		defaultConfig := DefaultConfig()
		config = &defaultConfig
	}

	return &MultiFetcher{
		config:      *config,
		ipni:        ipni,
		graphsync:   graphsync,
		bitswap:     bitswap,
		httpFetcher: NewHTTPFetcher(),
		metrics: &Metrics{
			ProtocolStats: map[string]*ProtocolMetrics{
				"bitswap":   {},
				"graphsync": {},
				"http":      {},
			},
		},
	}
}

// FetchBlock fetches a single block using the best available strategy
func (mf *MultiFetcher) FetchBlock(ctx context.Context, c cid.Cid) (*FetchResult, error) {
	mf.recordRequest()

	// Get ranked fetchers from IPNI
	intent := ipni.RouteIntent{
		Root:   c,
		Format: "raw",
		Scope:  "block",
	}

	rankedFetchers, found, err := mf.ipni.RankedFetchersByCID(ctx, c, intent)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers from IPNI: %w", err)
	}

	if !found || len(rankedFetchers) == 0 {
		// Fallback to direct bitswap if no providers found
		result := mf.fetchViaBitswap(ctx, c, "")
		if result.Error != nil {
			return result, result.Error
		}
		return result, nil
	}

	// Race multiple fetchers
	return mf.raceProtocols(ctx, c, rankedFetchers, nil)
}

// FetchDAG fetches a DAG using GraphSync with selector
func (mf *MultiFetcher) FetchDAG(ctx context.Context, root cid.Cid, selector ipld.Node) (*FetchResult, error) {
	mf.recordRequest()

	// Encode selector to CBOR for IPNI intent
	var selCBOR []byte
	if selector != nil {
		var err error
		selCBOR, err = encodeSelectorToCBOR(selector)
		if err != nil {
			// Log error but continue without selector
			selCBOR = nil
		}
	}

	intent := ipni.RouteIntent{
		Root:    root,
		Format:  "car",
		Scope:   "entity",
		SelCBOR: selCBOR,
	}

	rankedFetchers, found, err := mf.ipni.RankedFetchersByCID(ctx, root, intent)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers from IPNI: %w", err)
	}

	if !found || len(rankedFetchers) == 0 {
		// Fallback to direct graphsync
		result := mf.fetchViaGraphSync(ctx, root, "", selector)
		if result.Error != nil {
			return result, result.Error
		}
		return result, nil
	}

	return mf.raceProtocols(ctx, root, rankedFetchers, selector)
}

// raceProtocols runs multiple fetchers in parallel according to the plan
func (mf *MultiFetcher) raceProtocols(ctx context.Context, c cid.Cid, fetchers []ipni.RankedFetcher, selector ipld.Node) (*FetchResult, error) {
	if len(fetchers) == 0 {
		return nil, fmt.Errorf("no fetchers available")
	}

	// Create context with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, mf.config.Timeout)
	defer cancel()

	// Result channel
	resultCh := make(chan *FetchResult, len(fetchers))
	var wg sync.WaitGroup

	// Limit concurrent fetchers
	semaphore := make(chan struct{}, mf.config.MaxConcurrent)

	// Start fetchers with stagger
	for i, fetcher := range fetchers {
		// Apply stagger delay
		if i > 0 {
			time.Sleep(mf.config.StaggerDelay)
		}

		wg.Add(1)
		go func(f ipni.RankedFetcher, idx int) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-fetchCtx.Done():
				return
			}

			var result *FetchResult
			switch f.Proto {
			case ipni.TBitswap:
				result = mf.fetchViaBitswap(fetchCtx, c, f.ProviderID)
			case ipni.TGraphSync:
				result = mf.fetchViaGraphSync(fetchCtx, c, f.ProviderID, selector)
			case ipni.THTTP:
				result = mf.fetchViaHTTP(fetchCtx, c, f.ProviderID, f.Meta)
			default:
				result = &FetchResult{
					Protocol: string(f.Proto),
					Provider: f.ProviderID,
					Error:    fmt.Errorf("unsupported protocol: %s", f.Proto),
					CID:      c,
				}
			}

			select {
			case resultCh <- result:
			case <-fetchCtx.Done():
			}
		}(fetcher, i)
	}

	// Close result channel when all goroutines finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	var firstSuccess *FetchResult
	var lastError error

	for result := range resultCh {
		mf.recordResult(result)

		if result.Error == nil {
			if mf.config.CancelOnFirstWin {
				cancel() // Cancel other fetchers
				return result, nil
			}
			if firstSuccess == nil {
				firstSuccess = result
			}
		} else {
			lastError = result.Error
		}
	}

	if firstSuccess != nil {
		return firstSuccess, nil
	}

	mf.recordFailure()
	return nil, fmt.Errorf("all fetchers failed, last error: %w", lastError)
}

// fetchViaBitswap fetches using Bitswap protocol
func (mf *MultiFetcher) fetchViaBitswap(ctx context.Context, c cid.Cid, providerID string) *FetchResult {
	start := time.Now()
	result := &FetchResult{
		Protocol: "bitswap",
		Provider: providerID,
		CID:      c,
	}

	// Parse peer ID from provider string
	peerID, err := peer.Decode(providerID)
	if err != nil {
		result.Error = fmt.Errorf("invalid peer ID %s: %w", providerID, err)
		result.Duration = time.Since(start)
		return result
	}

	// Fetch block via Bitswap from specific peer
	block, err := mf.bitswap.GetBlockFromPeer(ctx, c, peerID)
	if err != nil {
		result.Error = err
	} else {
		result.Data = block.RawData()
	}

	result.Duration = time.Since(start)
	return result
}

// fetchViaGraphSync fetches using GraphSync protocol
func (mf *MultiFetcher) fetchViaGraphSync(ctx context.Context, c cid.Cid, providerID string, selector ipld.Node) *FetchResult {
	start := time.Now()
	result := &FetchResult{
		Protocol: "graphsync",
		Provider: providerID,
		CID:      c,
	}

	// GraphSync requires a valid peer ID
	if providerID == "" {
		result.Error = fmt.Errorf("GraphSync requires a provider ID")
		result.Duration = time.Since(start)
		return result
	}

	// Convert providerID to peer.ID
	targetPeer, err := peer.Decode(providerID)
	if err != nil {
		result.Error = fmt.Errorf("invalid provider ID: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Use default selector if none provided
	if selector == nil {
		var err error
		selector, err = createSimpleAllSelector()
		if err != nil {
			// Log error but continue without selector
			selector = nil
		}
	}

	// Fetch via GraphSync
	success, err := mf.graphsync.Fetch(ctx, targetPeer, c, selector)
	if err != nil {
		result.Error = err
	} else if !success {
		result.Error = fmt.Errorf("graphsync fetch returned false")
	} else {
		// For GraphSync, we don't return raw data but indicate success
		result.Data = []byte("graphsync_success")
	}

	result.Duration = time.Since(start)
	return result
}

// fetchViaHTTP fetches using HTTP protocol
func (mf *MultiFetcher) fetchViaHTTP(ctx context.Context, c cid.Cid, providerID string, meta map[string]string) *FetchResult {
	start := time.Now()
	result := &FetchResult{
		Protocol: "http",
		Provider: providerID,
		CID:      c,
	}

	// Extract URL from metadata
	url, ok := meta["url"]
	if !ok {
		result.Error = fmt.Errorf("no URL provided in metadata")
		result.Duration = time.Since(start)
		return result
	}

	// Check if partial CAR is supported
	partialCAR := meta["partial_car"] == "true"

	// Fetch via HTTP
	data, err := mf.httpFetcher.Fetch(ctx, url, c, partialCAR)
	if err != nil {
		result.Error = err
	} else {
		result.Data = data
	}

	result.Duration = time.Since(start)
	return result
}

// GetMetrics returns current performance metrics
func (mf *MultiFetcher) GetMetrics() *Metrics {
	mf.metrics.mu.RLock()
	defer mf.metrics.mu.RUnlock()

	// Deep copy metrics
	metrics := &Metrics{
		TotalRequests:      mf.metrics.TotalRequests,
		SuccessfulRequests: mf.metrics.SuccessfulRequests,
		FailedRequests:     mf.metrics.FailedRequests,
		ProtocolStats:      make(map[string]*ProtocolMetrics),
	}

	for proto, stats := range mf.metrics.ProtocolStats {
		metrics.ProtocolStats[proto] = &ProtocolMetrics{
			Attempts:         stats.Attempts,
			Successes:        stats.Successes,
			Failures:         stats.Failures,
			AvgLatency:       stats.AvgLatency,
			TotalLatency:     stats.TotalLatency,
			BytesTransferred: stats.BytesTransferred,
		}
	}

	return metrics
}

// recordRequest increments the total request counter
func (mf *MultiFetcher) recordRequest() {
	mf.metrics.mu.Lock()
	defer mf.metrics.mu.Unlock()
	mf.metrics.TotalRequests++
}

// recordResult records the result of a fetch operation
func (mf *MultiFetcher) recordResult(result *FetchResult) {
	mf.metrics.mu.Lock()
	defer mf.metrics.mu.Unlock()

	stats, ok := mf.metrics.ProtocolStats[result.Protocol]
	if !ok {
		stats = &ProtocolMetrics{}
		mf.metrics.ProtocolStats[result.Protocol] = stats
	}

	stats.Attempts++
	stats.TotalLatency += result.Duration

	if result.Error == nil {
		mf.metrics.SuccessfulRequests++
		stats.Successes++
		stats.BytesTransferred += int64(len(result.Data))
	} else {
		stats.Failures++
	}

	// Update average latency
	if stats.Attempts > 0 {
		stats.AvgLatency = stats.TotalLatency / time.Duration(stats.Attempts)
	}
}

// recordFailure increments the failed request counter
func (mf *MultiFetcher) recordFailure() {
	mf.metrics.mu.Lock()
	defer mf.metrics.mu.Unlock()
	mf.metrics.FailedRequests++
}

// Close cleans up resources
func (mf *MultiFetcher) Close() error {
	// Close underlying components if needed
	return nil
}

// encodeSelectorToCBOR encodes an IPLD selector to CBOR bytes
func encodeSelectorToCBOR(selector ipld.Node) ([]byte, error) {
	if selector == nil {
		return nil, nil
	}

	// Create a buffer to hold the CBOR data
	var buf []byte

	// Encode the selector node to CBOR
	err := cbor.Encode(selector, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to encode selector to CBOR: %w", err)
	}

	return buf, nil
}

// createDefaultSelector creates a default "match all" selector
func createDefaultSelector() (ipld.Node, error) {
	// Create a basic "match all" selector
	// This selector will match the entire DAG starting from the root
	nb := basicnode.Prototype.Map.NewBuilder()
	ma, err := nb.BeginMap(1)
	if err != nil {
		return nil, err
	}

	// Add "a" (all) key to match all children recursively
	err = ma.AssembleKey().AssignString("a")
	if err != nil {
		return nil, err
	}

	// Add recursive matcher
	recursiveNb := basicnode.Prototype.Map.NewBuilder()
	recursiveMa, err := recursiveNb.BeginMap(1)
	if err != nil {
		return nil, err
	}

	err = recursiveMa.AssembleKey().AssignString(":")
	if err != nil {
		return nil, err
	}

	recursiveValueNb := basicnode.Prototype.Map.NewBuilder()
	recursiveValueMa, err := recursiveValueNb.BeginMap(1)
	if err != nil {
		return nil, err
	}

	err = recursiveValueMa.AssembleKey().AssignString("a")
	if err != nil {
		return nil, err
	}
	err = recursiveValueMa.AssembleValue().AssignString("*")
	if err != nil {
		return nil, err
	}
	err = recursiveValueMa.Finish()
	if err != nil {
		return nil, err
	}

	recursiveValue := recursiveValueNb.Build()
	err = recursiveMa.AssembleValue().AssignNode(recursiveValue)
	if err != nil {
		return nil, err
	}
	err = recursiveMa.Finish()
	if err != nil {
		return nil, err
	}

	recursive := recursiveNb.Build()
	err = ma.AssembleValue().AssignNode(recursive)
	if err != nil {
		return nil, err
	}

	err = ma.Finish()
	if err != nil {
		return nil, err
	}

	return nb.Build(), nil
}

// createSimpleAllSelector creates a simplified "all" selector
func createSimpleAllSelector() (ipld.Node, error) {
	// Create a simple selector that matches everything
	nb := basicnode.Prototype.String.NewBuilder()
	err := nb.AssignString("*")
	if err != nil {
		return nil, err
	}
	return nb.Build(), nil
}