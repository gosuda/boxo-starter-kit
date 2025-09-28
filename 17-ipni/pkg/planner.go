package ipni

import (
	"context"
	"sort"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

// Planner handles provider selection and query planning
type Planner struct {
	config      *PlannerConfig
	healthScorer HealthScorer
}

// PlannerConfig holds planner configuration
type PlannerConfig struct {
	MaxProviders       int                            `json:"max_providers"`
	PreferredProtocols []TransportProtocol           `json:"preferred_protocols"`
	ProtocolScores     map[TransportProtocol]float64 `json:"protocol_scores"`
	HealthWeight       float64                       `json:"health_weight"`
	DistanceWeight     float64                       `json:"distance_weight"`
	ReputationWeight   float64                       `json:"reputation_weight"`
	DefaultTimeout     time.Duration                 `json:"default_timeout"`
}

// DefaultPlannerConfig returns default planner configuration
func DefaultPlannerConfig() *PlannerConfig {
	return &PlannerConfig{
		MaxProviders:       10,
		PreferredProtocols: []TransportProtocol{ProtocolHTTP, ProtocolBitswap, ProtocolGraphSync},
		ProtocolScores: map[TransportProtocol]float64{
			ProtocolHTTP:      1.0,
			ProtocolBitswap:   0.8,
			ProtocolGraphSync: 0.9,
			ProtocolCAR:       0.7,
		},
		HealthWeight:     0.4,
		DistanceWeight:   0.3,
		ReputationWeight: 0.3,
		DefaultTimeout:   30 * time.Second,
	}
}

// HealthScorer interface for provider health assessment
type HealthScorer interface {
	Score(providerID peer.ID) float64
	IsHealthy(providerID peer.ID) bool
}

// BasicHealthScorer provides simple health scoring
type BasicHealthScorer struct {
	healthMap map[peer.ID]float64
}

// NewBasicHealthScorer creates a new basic health scorer
func NewBasicHealthScorer() *BasicHealthScorer {
	return &BasicHealthScorer{
		healthMap: make(map[peer.ID]float64),
	}
}

// Score returns health score for a provider
func (s *BasicHealthScorer) Score(providerID peer.ID) float64 {
	if score, exists := s.healthMap[providerID]; exists {
		return score
	}
	// Default score for unknown providers
	return 0.7
}

// IsHealthy checks if a provider is considered healthy
func (s *BasicHealthScorer) IsHealthy(providerID peer.ID) bool {
	return s.Score(providerID) > 0.5
}

// SetHealth sets health score for a provider
func (s *BasicHealthScorer) SetHealth(providerID peer.ID, score float64) {
	s.healthMap[providerID] = score
}

// NewPlanner creates a new query planner
func NewPlanner(config *PlannerConfig) *Planner {
	if config == nil {
		config = DefaultPlannerConfig()
	}

	return &Planner{
		config:       config,
		healthScorer: NewBasicHealthScorer(),
	}
}

// SetHealthScorer sets a custom health scorer
func (p *Planner) SetHealthScorer(scorer HealthScorer) {
	p.healthScorer = scorer
}

// Plan creates an optimal retrieval plan for given content
func (p *Planner) Plan(ctx context.Context, mh multihash.Multihash, intent QueryIntent) ([]RankedProvider, bool, error) {
	// This would normally query providers for the multihash
	// For demo purposes, we'll create mock providers
	providers := p.generateMockProviders(mh)

	if len(providers) == 0 {
		return nil, false, nil
	}

	// Rank providers based on multiple factors
	rankedProviders := p.rankProviders(providers, intent)

	// Limit to max providers
	maxProviders := intent.MaxProviders
	if maxProviders == 0 {
		maxProviders = p.config.MaxProviders
	}

	if len(rankedProviders) > maxProviders {
		rankedProviders = rankedProviders[:maxProviders]
	}

	return rankedProviders, len(rankedProviders) > 0, nil
}

// RankedFetchers returns a simplified prioritized list of providers
func (p *Planner) RankedFetchers(ctx context.Context, mh multihash.Multihash, intent QueryIntent) ([]RankedFetcher, bool, error) {
	rankedProviders, found, err := p.Plan(ctx, mh, intent)
	if err != nil || !found {
		return nil, found, err
	}

	var fetchers []RankedFetcher
	for i, rp := range rankedProviders {
		// Select best protocol for this provider
		protocol := p.selectBestProtocol(rp.Provider, intent.PreferredProtocols)

		fetcher := RankedFetcher{
			Provider: rp.Provider,
			Protocol: protocol,
			Score:    rp.Score,
			Priority: i + 1,
		}
		fetchers = append(fetchers, fetcher)
	}

	return fetchers, len(fetchers) > 0, nil
}

// RankedFetchersByCID returns a simplified prioritized list of providers by CID
func (p *Planner) RankedFetchersByCID(ctx context.Context, c cid.Cid, intent QueryIntent) ([]RankedFetcher, bool, error) {
	return p.RankedFetchers(ctx, c.Hash(), intent)
}

// rankProviders scores and sorts providers based on multiple factors
func (p *Planner) rankProviders(providers []ProviderInfo, intent QueryIntent) []RankedProvider {
	var ranked []RankedProvider

	for _, provider := range providers {
		score := p.calculateProviderScore(provider, intent)

		ranked = append(ranked, RankedProvider{
			Provider: provider,
			Score:    score,
		})
	}

	// Sort by score (highest first)
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	// Assign ranks
	for i := range ranked {
		ranked[i].Rank = i + 1
	}

	return ranked
}

// calculateProviderScore computes a composite score for a provider
func (p *Planner) calculateProviderScore(provider ProviderInfo, intent QueryIntent) float64 {
	var score float64

	// Health score
	healthScore := p.healthScorer.Score(provider.ProviderID)
	score += healthScore * p.config.HealthWeight

	// Protocol preference score
	protocolScore := p.getProtocolScore(provider, intent.PreferredProtocols)
	score += protocolScore * 0.3

	// Recency score (prefer recent providers)
	recencyScore := p.getRecencyScore(provider.LastSeen)
	score += recencyScore * 0.2

	// Reputation score (based on metadata or past performance)
	reputationScore := p.getReputationScore(provider)
	score += reputationScore * p.config.ReputationWeight

	// Normalize to 0-1 range
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// getProtocolScore returns score based on protocol preference
func (p *Planner) getProtocolScore(provider ProviderInfo, preferredProtocols []TransportProtocol) float64 {
	// Get protocol from metadata
	protocol := ProtocolBitswap // default
	if protocolStr, exists := provider.Metadata["protocol"]; exists {
		protocol = TransportProtocol(protocolStr)
	}

	// Check if protocol is in preferred list
	for i, preferred := range preferredProtocols {
		if protocol == preferred {
			// Higher score for more preferred protocols
			return 1.0 - (float64(i) * 0.1)
		}
	}

	// Use config score if available
	if score, exists := p.config.ProtocolScores[protocol]; exists {
		return score
	}

	return 0.5 // default neutral score
}

// getRecencyScore returns score based on how recently provider was seen
func (p *Planner) getRecencyScore(lastSeen time.Time) float64 {
	age := time.Since(lastSeen)

	// Score decreases with age
	if age < time.Hour {
		return 1.0
	} else if age < 24*time.Hour {
		return 0.8
	} else if age < 7*24*time.Hour {
		return 0.6
	} else {
		return 0.4
	}
}

// getReputationScore returns reputation score from metadata
func (p *Planner) getReputationScore(provider ProviderInfo) float64 {
	// Look for reputation in metadata
	if repStr, exists := provider.Metadata["reputation"]; exists {
		// Parse reputation string (simplified)
		switch repStr {
		case "high":
			return 1.0
		case "medium":
			return 0.7
		case "low":
			return 0.4
		}
	}

	// Default neutral reputation
	return 0.6
}

// selectBestProtocol selects the best protocol for a provider
func (p *Planner) selectBestProtocol(provider ProviderInfo, preferredProtocols []TransportProtocol) TransportProtocol {
	// Get provider's protocol from metadata
	if protocolStr, exists := provider.Metadata["protocol"]; exists {
		providerProtocol := TransportProtocol(protocolStr)

		// Check if it's in preferred list
		for _, preferred := range preferredProtocols {
			if providerProtocol == preferred {
				return providerProtocol
			}
		}

		return providerProtocol
	}

	// Return first preferred protocol as fallback
	if len(preferredProtocols) > 0 {
		return preferredProtocols[0]
	}

	return ProtocolBitswap // ultimate fallback
}

// generateMockProviders creates mock providers for demonstration
func (p *Planner) generateMockProviders(mh multihash.Multihash) []ProviderInfo {
	// Generate deterministic but varied mock providers based on multihash
	hash := mh.String()
	providers := []ProviderInfo{}

	// Create 3-5 mock providers
	numProviders := 3 + (len(hash) % 3)

	for i := 0; i < numProviders; i++ {
		providerID := peer.ID("12D3KooWMock" + hash[i%10:i%10+4])

		// Vary protocols and metadata
		var protocol TransportProtocol
		var reputation string

		switch i % 3 {
		case 0:
			protocol = ProtocolHTTP
			reputation = "high"
		case 1:
			protocol = ProtocolBitswap
			reputation = "medium"
		case 2:
			protocol = ProtocolGraphSync
			reputation = "low"
		}

		provider := ProviderInfo{
			ProviderID: providerID,
			ContextID:  []byte("context-" + string(rune('A'+i))),
			Addresses:  []string{"/ip4/192.168.1." + string(rune('1'+i)) + "/tcp/4001"},
			Metadata: map[string]string{
				"protocol":   string(protocol),
				"reputation": reputation,
				"region":     "us-west",
			},
			LastSeen: time.Now().Add(-time.Duration(i) * time.Hour),
			TTL:      24 * time.Hour,
		}

		providers = append(providers, provider)
	}

	return providers
}

// GetConfig returns the planner configuration
func (p *Planner) GetConfig() *PlannerConfig {
	return p.config
}

// UpdateConfig updates the planner configuration
func (p *Planner) UpdateConfig(config *PlannerConfig) {
	p.config = config
}

// GetProviderScore returns the calculated score for a specific provider
func (p *Planner) GetProviderScore(provider ProviderInfo, intent QueryIntent) float64 {
	return p.calculateProviderScore(provider, intent)
}