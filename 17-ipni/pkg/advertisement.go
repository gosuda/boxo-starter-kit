package ipni

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

// AdvertisementChain manages the chain of advertisements
type AdvertisementChain struct {
	datastore datastore.Datastore
	head      *cid.Cid
	entries   map[cid.Cid]*Advertisement
	validator *ChainValidator
	stats     *ChainStats
	mutex     sync.RWMutex
}

// ChainValidator validates advertisement chains
type ChainValidator struct {
	config *ChainValidatorConfig
}

// ChainValidatorConfig holds validator configuration
type ChainValidatorConfig struct {
	MaxChainLength   int           `json:"max_chain_length"`
	MaxAge           time.Duration `json:"max_age"`
	RequireSignature bool          `json:"require_signature"`
	MaxEntrySize     int           `json:"max_entry_size"`
}

// DefaultChainValidatorConfig returns default validator configuration
func DefaultChainValidatorConfig() *ChainValidatorConfig {
	return &ChainValidatorConfig{
		MaxChainLength:   1000,
		MaxAge:           7 * 24 * time.Hour, // 7 days
		RequireSignature: false,              // Simplified for demo
		MaxEntrySize:     1024 * 1024,        // 1MB
	}
}

// ChainStats tracks advertisement chain statistics
type ChainStats struct {
	TotalAdvertisements int64     `json:"total_advertisements"`
	ChainLength         int       `json:"chain_length"`
	LastUpdate          time.Time `json:"last_update"`
	OldestEntry         time.Time `json:"oldest_entry"`
	ChainSize           int64     `json:"chain_size_bytes"`
}

// NewAdvertisementChain creates a new advertisement chain
func NewAdvertisementChain(ds datastore.Datastore, config *ChainValidatorConfig) (*AdvertisementChain, error) {
	if ds == nil {
		return nil, fmt.Errorf("datastore is required")
	}

	if config == nil {
		config = DefaultChainValidatorConfig()
	}

	validator := &ChainValidator{config: config}

	chain := &AdvertisementChain{
		datastore: ds,
		entries:   make(map[cid.Cid]*Advertisement),
		validator: validator,
		stats: &ChainStats{
			LastUpdate: time.Now(),
		},
	}

	// Load existing chain from datastore
	if err := chain.loadChain(); err != nil {
		return nil, fmt.Errorf("failed to load existing chain: %w", err)
	}

	return chain, nil
}

// AddAdvertisement adds a new advertisement to the chain
func (ac *AdvertisementChain) AddAdvertisement(ctx context.Context, ad *Advertisement) (*cid.Cid, error) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	// Validate advertisement
	if err := ac.validator.ValidateAdvertisement(ad); err != nil {
		return nil, fmt.Errorf("advertisement validation failed: %w", err)
	}

	// Set previous pointer to current head
	if ac.head != nil {
		prevStr := ac.head.String()
		ad.Previous = &prevStr
	}

	// Set timestamp if not provided
	if ad.Timestamp.IsZero() {
		ad.Timestamp = time.Now()
	}

	// Create CID for advertisement
	adCID, err := ac.createAdvertisementCID(ad)
	if err != nil {
		return nil, fmt.Errorf("failed to create advertisement CID: %w", err)
	}

	// Store in datastore
	adData, err := json.Marshal(ad)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal advertisement: %w", err)
	}

	key := datastore.NewKey("/ipni/ads/" + adCID.String())
	if err := ac.datastore.Put(ctx, key, adData); err != nil {
		return nil, fmt.Errorf("failed to store advertisement: %w", err)
	}

	// Update in-memory structures
	ac.entries[*adCID] = ad
	ac.head = adCID

	// Update statistics
	ac.updateStats()

	fmt.Printf("📄 Added advertisement %s to chain\n", adCID.String()[:12]+"...")
	return adCID, nil
}

// GetAdvertisement retrieves an advertisement by CID
func (ac *AdvertisementChain) GetAdvertisement(ctx context.Context, adCID cid.Cid) (*Advertisement, error) {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	// Check in-memory cache first
	if ad, exists := ac.entries[adCID]; exists {
		return ad, nil
	}

	// Load from datastore
	key := datastore.NewKey("/ipni/ads/" + adCID.String())
	data, err := ac.datastore.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("advertisement not found: %w", err)
	}

	var ad Advertisement
	if err := json.Unmarshal(data, &ad); err != nil {
		return nil, fmt.Errorf("failed to unmarshal advertisement: %w", err)
	}

	// Cache in memory
	ac.entries[adCID] = &ad

	return &ad, nil
}

// GetChainHead returns the current chain head
func (ac *AdvertisementChain) GetChainHead() *cid.Cid {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()
	return ac.head
}

// WalkChain walks the advertisement chain with a visitor function
func (ac *AdvertisementChain) WalkChain(ctx context.Context, visitor func(*Advertisement) error) error {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	if ac.head == nil {
		return nil // Empty chain
	}

	current := ac.head
	visited := make(map[cid.Cid]bool)

	for current != nil {
		// Prevent infinite loops
		if visited[*current] {
			break
		}
		visited[*current] = true

		// Get advertisement
		ad, err := ac.GetAdvertisement(ctx, *current)
		if err != nil {
			return fmt.Errorf("failed to get advertisement %s: %w", current, err)
		}

		// Visit advertisement
		if err := visitor(ad); err != nil {
			return err
		}

		// Move to previous
		if ad.Previous != nil {
			prevCID, err := cid.Parse(*ad.Previous)
			if err != nil {
				return fmt.Errorf("invalid previous CID: %w", err)
			}
			current = &prevCID
		} else {
			current = nil
		}
	}

	return nil
}

// FindAdvertisementsByProvider finds advertisements by provider
func (ac *AdvertisementChain) FindAdvertisementsByProvider(ctx context.Context, providerID peer.ID) ([]*Advertisement, error) {
	var results []*Advertisement

	err := ac.WalkChain(ctx, func(ad *Advertisement) error {
		if ad.Provider == providerID {
			results = append(results, ad)
		}
		return nil
	})

	return results, err
}

// FindAdvertisementsByContent finds advertisements by content
func (ac *AdvertisementChain) FindAdvertisementsByContent(ctx context.Context, mh multihash.Multihash) ([]*Advertisement, error) {
	var results []*Advertisement
	mhStr := mh.String()

	err := ac.WalkChain(ctx, func(ad *Advertisement) error {
		for _, adMh := range ad.Multihashes {
			if adMh == mhStr {
				results = append(results, ad)
				break
			}
		}
		return nil
	})

	return results, err
}

// GetStats returns chain statistics
func (ac *AdvertisementChain) GetStats() *ChainStats {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	stats := *ac.stats
	return &stats
}

// ValidateChain validates the entire advertisement chain
func (ac *AdvertisementChain) ValidateChain(ctx context.Context) error {
	return ac.validator.ValidateChain(ctx, ac)
}

// createAdvertisementCID creates a CID for an advertisement
func (ac *AdvertisementChain) createAdvertisementCID(ad *Advertisement) (*cid.Cid, error) {
	// Create deterministic hash of advertisement
	adData, err := json.Marshal(ad)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(adData)
	mh, err := multihash.Encode(hash[:], multihash.SHA2_256)
	if err != nil {
		return nil, err
	}

	c := cid.NewCidV1(cid.Raw, mh)
	return &c, nil
}

// loadChain loads existing chain from datastore
func (ac *AdvertisementChain) loadChain() error {
	// In a real implementation, this would load the chain from datastore
	// For demo purposes, we'll start with an empty chain
	fmt.Println("📄 Loaded advertisement chain (empty)")
	return nil
}

// updateStats updates chain statistics
func (ac *AdvertisementChain) updateStats() {
	ac.stats.TotalAdvertisements = int64(len(ac.entries))
	ac.stats.LastUpdate = time.Now()

	// Calculate chain length and size
	chainLength := 0
	chainSize := int64(0)
	oldestTime := time.Now()

	for _, ad := range ac.entries {
		chainLength++
		// Estimate size (simplified)
		chainSize += int64(len(ad.Multihashes)*50 + 200) // rough estimate

		if ad.Timestamp.Before(oldestTime) {
			oldestTime = ad.Timestamp
		}
	}

	ac.stats.ChainLength = chainLength
	ac.stats.ChainSize = chainSize
	if chainLength > 0 {
		ac.stats.OldestEntry = oldestTime
	}
}

// Validator methods

// ValidateAdvertisement validates a single advertisement
func (cv *ChainValidator) ValidateAdvertisement(ad *Advertisement) error {
	if ad == nil {
		return fmt.Errorf("advertisement is nil")
	}

	if ad.Provider == "" {
		return fmt.Errorf("provider ID is required")
	}

	if len(ad.Multihashes) == 0 {
		return fmt.Errorf("at least one multihash is required")
	}

	// Check age
	if cv.config.MaxAge > 0 && time.Since(ad.Timestamp) > cv.config.MaxAge {
		return fmt.Errorf("advertisement too old: %v", time.Since(ad.Timestamp))
	}

	// Check size (simplified)
	if cv.config.MaxEntrySize > 0 {
		estimatedSize := len(ad.Multihashes)*50 + 200 // rough estimate
		if estimatedSize > cv.config.MaxEntrySize {
			return fmt.Errorf("advertisement too large: %d bytes", estimatedSize)
		}
	}

	return nil
}

// ValidateChain validates the entire chain
func (cv *ChainValidator) ValidateChain(ctx context.Context, chain *AdvertisementChain) error {
	if chain.head == nil {
		return nil // Empty chain is valid
	}

	chainLength := 0
	current := chain.head
	visited := make(map[cid.Cid]bool)

	for current != nil {
		// Check chain length limit
		chainLength++
		if cv.config.MaxChainLength > 0 && chainLength > cv.config.MaxChainLength {
			return fmt.Errorf("chain too long: %d entries", chainLength)
		}

		// Prevent infinite loops
		if visited[*current] {
			return fmt.Errorf("circular reference detected in chain")
		}
		visited[*current] = true

		// Get and validate advertisement
		ad, err := chain.GetAdvertisement(ctx, *current)
		if err != nil {
			return fmt.Errorf("failed to get advertisement %s: %w", current, err)
		}

		if err := cv.ValidateAdvertisement(ad); err != nil {
			return fmt.Errorf("invalid advertisement %s: %w", current, err)
		}

		// Move to previous
		if ad.Previous != nil {
			prevCID, err := cid.Parse(*ad.Previous)
			if err != nil {
				return fmt.Errorf("invalid previous CID in %s: %w", current, err)
			}
			current = &prevCID
		} else {
			current = nil
		}
	}

	fmt.Printf("✅ Chain validation passed: %d entries\n", chainLength)
	return nil
}
