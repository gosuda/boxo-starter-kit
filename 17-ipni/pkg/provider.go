package ipni

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Provider handles provider management and indexing
type Provider struct {
	datastore datastore.Datastore

	// In-memory index for fast lookups
	providerIndex map[string][]ProviderInfo
	indexMutex    sync.RWMutex

	// Statistics
	stats *IndexStats

	// Configuration
	config *IPNIConfig
}

// NewProvider creates a new provider component
func NewProvider(ds datastore.Datastore) *Provider {
	return &Provider{
		datastore:     ds,
		providerIndex: make(map[string][]ProviderInfo),
		stats: &IndexStats{
			LastUpdate: time.Now(),
		},
		config: DefaultIPNIConfig(),
	}
}

// ProviderID returns a mock provider ID
func (p *Provider) ProviderID() peer.ID {
	// Return a mock peer ID for demo purposes
	return peer.ID("12D3KooWDemo")
}

// PutCID adds CIDs to the index
func (p *Provider) PutCID(providerID peer.ID, contextID []byte, metadataBytes []byte, cids ...cid.Cid) error {
	p.indexMutex.Lock()
	defer p.indexMutex.Unlock()

	// Parse metadata
	var metadata map[string]string
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			metadata = map[string]string{"raw": string(metadataBytes)}
		}
	} else {
		metadata = make(map[string]string)
	}

	// Create provider info
	providerInfo := ProviderInfo{
		ProviderID: providerID,
		ContextID:  contextID,
		Addresses:  []string{"/ip4/127.0.0.1/tcp/4001"},
		Metadata:   metadata,
		LastSeen:   time.Now(),
		TTL:        p.config.DefaultTTL,
	}

	// Add to index
	for _, c := range cids {
		key := c.Hash().String()
		providers := p.providerIndex[key]

		// Check if provider already exists
		found := false
		for i, existing := range providers {
			if existing.ProviderID == providerID {
				providers[i] = providerInfo
				found = true
				break
			}
		}

		if !found {
			providers = append(providers, providerInfo)
		}

		// Limit providers per multihash
		if len(providers) > p.config.MaxProvidersPerMultihash {
			providers = providers[:p.config.MaxProvidersPerMultihash]
		}

		p.providerIndex[key] = providers
	}

	// Update statistics
	p.stats.TotalEntries = int64(len(p.providerIndex))
	p.stats.LastUpdate = time.Now()

	return nil
}

// GetProvidersByCID finds providers for a given CID
func (p *Provider) GetProvidersByCID(c cid.Cid) ([]ProviderInfo, bool, error) {
	p.indexMutex.RLock()
	defer p.indexMutex.RUnlock()

	providers, found := p.providerIndex[c.Hash().String()]
	if !found || len(providers) == 0 {
		return nil, false, nil
	}

	// Filter out expired providers
	var validProviders []ProviderInfo
	now := time.Now()
	for _, provider := range providers {
		if now.Sub(provider.LastSeen) < provider.TTL {
			validProviders = append(validProviders, provider)
		}
	}

	if len(validProviders) == 0 {
		return nil, false, nil
	}

	p.stats.QueryCount++
	return validProviders, true, nil
}

// GetStats returns index statistics
func (p *Provider) GetStats() *IndexStats {
	p.indexMutex.RLock()
	defer p.indexMutex.RUnlock()

	// Count unique providers
	providerSet := make(map[peer.ID]struct{})
	totalProviders := int64(0)
	totalMultihashes := int64(len(p.providerIndex))

	for _, providers := range p.providerIndex {
		for _, provider := range providers {
			if _, exists := providerSet[provider.ProviderID]; !exists {
				providerSet[provider.ProviderID] = struct{}{}
				totalProviders++
			}
		}
	}

	stats := *p.stats
	stats.TotalProviders = totalProviders
	stats.TotalMultihashes = totalMultihashes

	return &stats
}

// Close gracefully shuts down the provider
func (p *Provider) Close() error {
	return nil
}