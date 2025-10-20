package indexer

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// SimpleSubscriber finds providers for content
// "Who has this content and how can I get it?"
type SimpleSubscriber struct {
	host   host.Host
	index  *SimpleIndex // Where to query for providers
	cache  map[cid.Cid][]*ProviderRecord // Local cache
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSimpleSubscriber creates a new content subscriber
func NewSimpleSubscriber(h host.Host, index *SimpleIndex) *SimpleSubscriber {
	ctx, cancel := context.WithCancel(context.Background())

	return &SimpleSubscriber{
		host:   h,
		index:  index,
		cache:  make(map[cid.Cid][]*ProviderRecord),
		ctx:    ctx,
		cancel: cancel,
	}
}

// FindProviders finds all providers for a specific content
// This is the main function subscribers use to discover who has content
func (s *SimpleSubscriber) FindProviders(ctx context.Context, c cid.Cid) ([]*ProviderRecord, error) {
	// Check cache first
	if cached := s.getCached(c); len(cached) > 0 {
		return cached, nil
	}

	// Query index
	if s.index == nil {
		return nil, fmt.Errorf("no index configured")
	}

	providers, err := s.index.Query(c)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Filter out expired records
	active := make([]*ProviderRecord, 0)
	for _, p := range providers {
		if !p.IsExpired() {
			active = append(active, p)
		}
	}

	// Update cache
	s.updateCache(c, active)

	return active, nil
}

// FindProvidersByProtocol finds providers that support a specific protocol
func (s *SimpleSubscriber) FindProvidersByProtocol(ctx context.Context, c cid.Cid, proto Protocol) ([]*ProviderRecord, error) {
	all, err := s.FindProviders(ctx, c)
	if err != nil {
		return nil, err
	}

	matching := make([]*ProviderRecord, 0)
	for _, p := range all {
		if p.SupportsProtocol(proto) {
			matching = append(matching, p)
		}
	}

	return matching, nil
}

// GetBestProvider returns the "best" provider based on simple heuristics
// Priority: 1) Not expired, 2) Most protocols, 3) Most recent
func (s *SimpleSubscriber) GetBestProvider(ctx context.Context, c cid.Cid) (*ProviderRecord, error) {
	providers, err := s.FindProviders(ctx, c)
	if err != nil {
		return nil, err
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers found for %s", c.String())
	}

	best := providers[0]
	for _, p := range providers[1:] {
		// Prefer more protocols
		if len(p.Protocols) > len(best.Protocols) {
			best = p
			continue
		}
		// If same protocol count, prefer more recent
		if len(p.Protocols) == len(best.Protocols) && p.Timestamp.After(best.Timestamp) {
			best = p
		}
	}

	return best, nil
}

// Subscribe starts monitoring for new providers of specific content
// In a real implementation, this would listen to pubsub updates
func (s *SimpleSubscriber) Subscribe(ctx context.Context, c cid.Cid) (<-chan *ProviderRecord, error) {
	ch := make(chan *ProviderRecord, 10)

	// Simplified: just query once
	// In real implementation, would subscribe to pubsub for updates
	go func() {
		defer close(ch)

		providers, err := s.FindProviders(ctx, c)
		if err != nil {
			return
		}

		for _, p := range providers {
			select {
			case ch <- p:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// ClearCache removes all cached provider records
func (s *SimpleSubscriber) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[cid.Cid][]*ProviderRecord)
}

// ClearCacheFor removes cached records for specific content
func (s *SimpleSubscriber) ClearCacheFor(c cid.Cid) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, c)
}

// Close stops the subscriber
func (s *SimpleSubscriber) Close() error {
	s.cancel()
	return nil
}

// Stats returns subscriber statistics
func (s *SimpleSubscriber) Stats() SubscriberStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SubscriberStats{
		SubscriberID:   s.host.ID(),
		CachedContent:  len(s.cache),
		TotalProviders: s.countTotalProviders(),
	}
}

// SubscriberStats contains subscriber statistics
type SubscriberStats struct {
	SubscriberID   peer.ID
	CachedContent  int
	TotalProviders int
}

func (s *SimpleSubscriber) getCached(c cid.Cid) []*ProviderRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[c]
}

func (s *SimpleSubscriber) updateCache(c cid.Cid, providers []*ProviderRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[c] = providers
}

func (s *SimpleSubscriber) countTotalProviders() int {
	count := 0
	for _, providers := range s.cache {
		count += len(providers)
	}
	return count
}
