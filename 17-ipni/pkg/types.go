package ipni

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ProviderInfo represents information about a content provider
type ProviderInfo struct {
	ProviderID peer.ID           `json:"provider_id"`
	ContextID  []byte            `json:"context_id"`
	Addresses  []string          `json:"addresses"`
	Metadata   map[string]string `json:"metadata"`
	LastSeen   time.Time         `json:"last_seen"`
	TTL        time.Duration     `json:"ttl"`
}

// IndexStats represents indexing statistics
type IndexStats struct {
	TotalProviders   int64     `json:"total_providers"`
	TotalEntries     int64     `json:"total_entries"`
	LastUpdate       time.Time `json:"last_update"`
	QueryCount       int64     `json:"query_count"`
	TotalMultihashes int64     `json:"total_multihashes"`
}

// IPNIConfig holds configuration for IPNI
type IPNIConfig struct {
	DefaultTTL               time.Duration `json:"default_ttl"`
	MaxProvidersPerMultihash int           `json:"max_providers_per_multihash"`
}

// DefaultIPNIConfig returns default configuration
func DefaultIPNIConfig() *IPNIConfig {
	return &IPNIConfig{
		DefaultTTL:               24 * time.Hour,
		MaxProvidersPerMultihash: 20,
	}
}

// Value represents the value stored for each multihash entry
type Value struct {
	ProviderID    peer.ID `json:"provider_id"`
	ContextID     []byte  `json:"context_id"`
	MetadataBytes []byte  `json:"metadata_bytes"`
}

// Transport protocols
type TransportProtocol string

const (
	ProtocolBitswap   TransportProtocol = "bitswap"
	ProtocolHTTP      TransportProtocol = "http"
	ProtocolGraphSync TransportProtocol = "graphsync"
	ProtocolCAR       TransportProtocol = "car"
)

// Query Intent for provider selection
type QueryIntent struct {
	PreferredProtocols []TransportProtocol `json:"preferred_protocols"`
	MaxProviders       int                 `json:"max_providers"`
	RequireHealthy     bool                `json:"require_healthy"`
	PreferLocal        bool                `json:"prefer_local"`
}

// Advertisement represents a provider advertisement
type Advertisement struct {
	Provider    peer.ID                `json:"provider"`
	ContextID   []byte                 `json:"context_id"`
	Multihashes []string               `json:"multihashes"`
	Metadata    *AdvertisementMetadata `json:"metadata"`
	Protocol    TransportProtocol      `json:"protocol"`
	Addresses   []string               `json:"addresses"`
	Timestamp   time.Time              `json:"timestamp"`
	TTL         time.Duration          `json:"ttl"`
	Previous    *string                `json:"previous,omitempty"`
}

// AdvertisementMetadata contains metadata for advertisements
type AdvertisementMetadata struct {
	ContentType      string                 `json:"content_type"`
	ProtocolData     map[string]interface{} `json:"protocol_data"`
	ProviderMeta     map[string]string      `json:"provider_meta"`
	QualityScore     float64                `json:"quality_score"`
	ReliabilityScore float64                `json:"reliability_score"`
}

// PubSub message types
type PubSubProviderAnnouncement struct {
	ProviderID  peer.ID           `json:"provider_id"`
	ContextID   []byte            `json:"context_id"`
	Metadata    map[string]string `json:"metadata"`
	Multihashes []string          `json:"multihashes"`
	Protocol    TransportProtocol `json:"protocol"`
	Addresses   []string          `json:"addresses"`
	TTL         time.Duration     `json:"ttl"`
}

// Gossip message types
type GossipMessageType string

const (
	GossipTypeAdvertisement  GossipMessageType = "advertisement"
	GossipTypeProviderUpdate GossipMessageType = "provider_update"
	GossipTypeHeartbeat      GossipMessageType = "heartbeat"
	GossipTypePeerDiscovery  GossipMessageType = "peer_discovery"
	GossipTypeChainUpdate    GossipMessageType = "chain_update"
)

// Monitoring types
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
)

// Provider ranking
type RankedProvider struct {
	Provider ProviderInfo `json:"provider"`
	Score    float64      `json:"score"`
	Rank     int          `json:"rank"`
}

// Fetcher for retrieval
type RankedFetcher struct {
	Provider ProviderInfo      `json:"provider"`
	Protocol TransportProtocol `json:"protocol"`
	Score    float64           `json:"score"`
	Priority int               `json:"priority"`
}

// Cache configuration
type CacheConfig struct {
	Enabled     bool          `json:"enabled"`
	Size        int64         `json:"size_bytes"`
	TTL         time.Duration `json:"ttl"`
	Compression bool          `json:"compression"`
}
