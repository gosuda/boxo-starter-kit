package indexer

import (
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

// SimpleIndex is the "phonebook" that maps content to providers
// "For this CID, here are all the providers who have it"
type SimpleIndex struct {
	// Map: CID -> List of providers who have it
	content map[cid.Cid][]*ProviderRecord

	// Reverse map: ProviderID -> List of CIDs they provide
	providers map[peer.ID][]cid.Cid

	mu sync.RWMutex

	// Configuration
	maxProvidersPerContent int
	cleanupInterval        time.Duration
	stopCleanup            chan struct{}
}

// NewSimpleIndex creates a new content index
func NewSimpleIndex() *SimpleIndex {
	idx := &SimpleIndex{
		content:                make(map[cid.Cid][]*ProviderRecord),
		providers:              make(map[peer.ID][]cid.Cid),
		maxProvidersPerContent: 20, // Limit to prevent spam
		cleanupInterval:        10 * time.Minute,
		stopCleanup:            make(chan struct{}),
	}

	// Start background cleanup of expired records
	go idx.cleanupLoop()

	return idx
}

// Announce adds a provider announcement to the index
// "Provider X has content C available via protocols P"
func (idx *SimpleIndex) Announce(record *ProviderRecord) error {
	if record == nil {
		return fmt.Errorf("nil provider record")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	c := record.ContentID
	providerID := record.ProviderID

	// Get existing providers for this content
	existing := idx.content[c]

	// Check if this provider already announced this content
	updated := false
	for i, p := range existing {
		if p.ProviderID == providerID {
			// Update existing record
			existing[i] = record
			updated = true
			break
		}
	}

	if !updated {
		// Add new provider
		existing = append(existing, record)

		// Enforce max providers limit
		if len(existing) > idx.maxProvidersPerContent {
			// Keep most recent ones
			existing = existing[len(existing)-idx.maxProvidersPerContent:]
		}
	}

	idx.content[c] = existing

	// Update reverse index
	if !containsCID(idx.providers[providerID], c) {
		idx.providers[providerID] = append(idx.providers[providerID], c)
	}

	return nil
}

// Query finds all providers for a specific content
func (idx *SimpleIndex) Query(c cid.Cid) ([]*ProviderRecord, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	providers := idx.content[c]
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers found for %s", c.String())
	}

	// Return copy to avoid external modification
	result := make([]*ProviderRecord, len(providers))
	copy(result, providers)

	return result, nil
}

// Remove removes a provider's announcement for specific content
func (idx *SimpleIndex) Remove(c cid.Cid, providerID peer.ID) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove from content map
	providers := idx.content[c]
	filtered := make([]*ProviderRecord, 0)
	for _, p := range providers {
		if p.ProviderID != providerID {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		delete(idx.content, c)
	} else {
		idx.content[c] = filtered
	}

	// Remove from reverse index
	cids := idx.providers[providerID]
	filtered2 := make([]cid.Cid, 0)
	for _, existingCID := range cids {
		if !existingCID.Equals(c) {
			filtered2 = append(filtered2, existingCID)
		}
	}

	if len(filtered2) == 0 {
		delete(idx.providers, providerID)
	} else {
		idx.providers[providerID] = filtered2
	}

	return nil
}

// RemoveProvider removes all announcements from a provider
func (idx *SimpleIndex) RemoveProvider(providerID peer.ID) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Get all content from this provider
	cids := idx.providers[providerID]

	// Remove from content map
	for _, c := range cids {
		providers := idx.content[c]
		filtered := make([]*ProviderRecord, 0)
		for _, p := range providers {
			if p.ProviderID != providerID {
				filtered = append(filtered, p)
			}
		}

		if len(filtered) == 0 {
			delete(idx.content, c)
		} else {
			idx.content[c] = filtered
		}
	}

	// Remove from providers map
	delete(idx.providers, providerID)

	return nil
}

// ListContent returns all content IDs in the index
func (idx *SimpleIndex) ListContent() []cid.Cid {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	cids := make([]cid.Cid, 0, len(idx.content))
	for c := range idx.content {
		cids = append(cids, c)
	}
	return cids
}

// ListProviders returns all provider IDs in the index
func (idx *SimpleIndex) ListProviders() []peer.ID {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	providers := make([]peer.ID, 0, len(idx.providers))
	for p := range idx.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetProviderContent returns all content announced by a specific provider
func (idx *SimpleIndex) GetProviderContent(providerID peer.ID) ([]cid.Cid, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	cids := idx.providers[providerID]
	if len(cids) == 0 {
		return nil, fmt.Errorf("provider not found: %s", providerID.String())
	}

	// Return copy
	result := make([]cid.Cid, len(cids))
	copy(result, cids)
	return result, nil
}

// Stats returns index statistics
func (idx *SimpleIndex) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	totalRecords := 0
	expiredRecords := 0

	for _, providers := range idx.content {
		for _, p := range providers {
			totalRecords++
			if p.IsExpired() {
				expiredRecords++
			}
		}
	}

	return IndexStats{
		TotalContent:   len(idx.content),
		TotalProviders: len(idx.providers),
		TotalRecords:   totalRecords,
		ExpiredRecords: expiredRecords,
	}
}

// IndexStats contains index statistics
type IndexStats struct {
	TotalContent   int
	TotalProviders int
	TotalRecords   int
	ExpiredRecords int
}

// CleanupExpired removes all expired provider records
func (idx *SimpleIndex) CleanupExpired() int {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	removed := 0

	// Check each content's providers
	for c, providers := range idx.content {
		active := make([]*ProviderRecord, 0)
		for _, p := range providers {
			if !p.IsExpired() {
				active = append(active, p)
			} else {
				removed++
			}
		}

		if len(active) == 0 {
			delete(idx.content, c)
		} else {
			idx.content[c] = active
		}
	}

	// Rebuild provider index
	idx.rebuildProviderIndex()

	return removed
}

// Close stops the index cleanup goroutine
func (idx *SimpleIndex) Close() error {
	close(idx.stopCleanup)
	return nil
}

// cleanupLoop periodically removes expired records
func (idx *SimpleIndex) cleanupLoop() {
	ticker := time.NewTicker(idx.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			removed := idx.CleanupExpired()
			if removed > 0 {
				// Could add logging here
				_ = removed
			}
		case <-idx.stopCleanup:
			return
		}
	}
}

// rebuildProviderIndex rebuilds the provider -> content reverse index
func (idx *SimpleIndex) rebuildProviderIndex() {
	idx.providers = make(map[peer.ID][]cid.Cid)

	for c, providers := range idx.content {
		for _, p := range providers {
			if !containsCID(idx.providers[p.ProviderID], c) {
				idx.providers[p.ProviderID] = append(idx.providers[p.ProviderID], c)
			}
		}
	}
}

// Helper function
func containsCID(slice []cid.Cid, c cid.Cid) bool {
	for _, item := range slice {
		if item.Equals(c) {
			return true
		}
	}
	return false
}
