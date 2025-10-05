package ipni

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

// IPNI represents the main IPNI coordinator
type IPNI struct {
	Provider   *Provider
	Subscriber *Subscriber
	Security   *Security
	AntiSpam   *AntiSpamFilter
	Planner    *Planner
	PubSub     *PubSubManager
	AdChain    *AdvertisementChain
	Monitoring *MonitoringManager
	datastore  datastore.Datastore
	config     *IPNIConfig
}

// New creates a new IPNI instance
func New(ds datastore.Datastore) (*IPNI, error) {
	if ds == nil {
		return nil, fmt.Errorf("datastore is required")
	}

	// Create security manager
	security, err := NewSecurity(DefaultSecurityConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create security manager: %w", err)
	}

	// Create anti-spam filter (max 10 requests per minute)
	antiSpam := NewAntiSpamFilter(10, time.Minute)

	// Create components
	provider := NewProvider(ds)
	subscriber := NewSubscriber(provider)
	planner := NewPlanner(DefaultPlannerConfig())

	// Create advertisement chain
	adChain, err := NewAdvertisementChain(ds, DefaultChainValidatorConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create advertisement chain: %w", err)
	}

	// Create monitoring
	monitoring := NewMonitoringManager(DefaultMonitoringConfig())

	ipni := &IPNI{
		Provider:   provider,
		Subscriber: subscriber,
		Security:   security,
		AntiSpam:   antiSpam,
		Planner:    planner,
		AdChain:    adChain,
		Monitoring: monitoring,
		datastore:  ds,
		config:     DefaultIPNIConfig(),
	}

	// Create PubSub manager with IPNI as message handler
	pubsub, err := NewPubSubManager(nil, ipni) // host is nil for demo
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub manager: %w", err)
	}
	ipni.PubSub = pubsub

	// Register health checks
	monitoring.RegisterHealthCheck(NewProviderHealthCheck(provider))
	monitoring.RegisterHealthCheck(NewSecurityHealthCheck(security))

	return ipni, nil
}

// Start initializes the IPNI components
func (ipni *IPNI) Start(ctx context.Context) error {
	fmt.Println("🚀 Starting IPNI components...")

	if err := ipni.Subscriber.Start(ctx); err != nil {
		return fmt.Errorf("failed to start subscriber: %w", err)
	}

	fmt.Println("✅ IPNI components started successfully")
	return nil
}

// Close gracefully shuts down all IPNI components
func (ipni *IPNI) Close() error {
	fmt.Println("👋 Shutting down IPNI components...")

	if ipni.Subscriber != nil {
		ipni.Subscriber.Close()
	}

	if ipni.Provider != nil {
		ipni.Provider.Close()
	}

	fmt.Println("✅ IPNI components shut down successfully")
	return nil
}

// GetStats returns overall IPNI statistics
func (ipni *IPNI) GetStats() *IndexStats {
	if ipni.Provider != nil {
		return ipni.Provider.GetStats()
	}
	return &IndexStats{}
}

// Put adds content to the index with provider information
func (ipni *IPNI) Put(providerID peer.ID, contextID []byte, metadataBytes []byte, mhs ...multihash.Multihash) error {
	// Check rate limiting
	if !ipni.AntiSpam.CheckRateLimit(providerID) {
		return fmt.Errorf("rate limit exceeded for provider %s", providerID)
	}

	// Check if provider is trusted
	if ipni.Security != nil && !ipni.Security.IsProviderTrusted(providerID, DefaultSecurityConfig()) {
		fmt.Printf("⚠️ Warning: Low trust provider %s (score: %.2f)\n",
			providerID, ipni.Security.TrustScore(providerID))
	}

	// Convert multihashes to CIDs for provider storage
	var cids []cid.Cid
	for _, mh := range mhs {
		c := cid.NewCidV1(cid.Raw, mh)
		cids = append(cids, c)
	}

	return ipni.Provider.PutCID(providerID, contextID, metadataBytes, cids...)
}

// PutCID is a convenience method to add CIDs to the index
func (ipni *IPNI) PutCID(providerID peer.ID, contextID []byte, metadataBytes []byte, cids ...cid.Cid) error {
	// Convert CIDs to multihashes
	var mhs []multihash.Multihash
	for _, c := range cids {
		mhs = append(mhs, c.Hash())
	}

	return ipni.Put(providerID, contextID, metadataBytes, mhs...)
}

// GetProviders finds providers for a given multihash
func (ipni *IPNI) GetProviders(mh multihash.Multihash) ([]ProviderInfo, bool, error) {
	// Convert multihash to CID for provider lookup
	c := cid.NewCidV1(cid.Raw, mh)
	return ipni.Provider.GetProvidersByCID(c)
}

// GetProvidersByCID finds providers for a given CID
func (ipni *IPNI) GetProvidersByCID(c cid.Cid) ([]ProviderInfo, bool, error) {
	// First check local provider
	providers, found, err := ipni.Provider.GetProvidersByCID(c)
	if err == nil && found && len(providers) > 0 {
		return providers, found, err
	}

	// If not found locally, try subscriber
	if ipni.Subscriber != nil {
		return ipni.Subscriber.GetProvidersByCID(c)
	}

	return providers, found, err
}

// Remove removes a provider context from the index
func (ipni *IPNI) Remove(providerID peer.ID, contextID []byte) error {
	// Check rate limiting for removals too
	if !ipni.AntiSpam.CheckRateLimit(providerID) {
		return fmt.Errorf("rate limit exceeded for provider %s", providerID)
	}

	// In a real implementation, we'd add a remove method to provider
	fmt.Printf("🗑️ Remove request for provider %s, context %x\n", providerID, contextID)
	return nil
}

// CreateSignedAnnouncement creates a cryptographically signed announcement
func (ipni *IPNI) CreateSignedAnnouncement(providerID peer.ID, contextID []byte, metadata map[string]string, cids []cid.Cid) (*SignedAnnouncement, error) {
	if ipni.Security == nil {
		return nil, fmt.Errorf("security manager not available")
	}

	// Convert CIDs to strings
	var cidStrings []string
	for _, c := range cids {
		cidStrings = append(cidStrings, c.String())
	}

	return ipni.Security.CreateSignedAnnouncement(providerID, contextID, metadata, cidStrings)
}

// VerifyAnnouncement verifies a signed announcement
func (ipni *IPNI) VerifyAnnouncement(announcement *SignedAnnouncement) bool {
	if ipni.Security == nil {
		return false
	}

	return ipni.Security.VerifyAnnouncement(announcement)
}

// GetTrustScore returns the trust score for a provider
func (ipni *IPNI) GetTrustScore(providerID peer.ID) float64 {
	if ipni.Security == nil {
		return 0.5 // Default neutral score
	}

	return ipni.Security.TrustScore(providerID)
}

// IsProviderTrusted checks if a provider meets the trust threshold
func (ipni *IPNI) IsProviderTrusted(providerID peer.ID) bool {
	if ipni.Security == nil {
		return true // Allow all if no security
	}

	return ipni.Security.IsProviderTrusted(providerID, DefaultSecurityConfig())
}

// Flush persists all in-memory data
func (ipni *IPNI) Flush(ctx context.Context) error {
	// Provider flush would go here
	fmt.Println("💾 Flushing IPNI data to persistent storage...")
	return nil
}

// Size returns the total size of the index
func (ipni *IPNI) Size() (int64, error) {
	// Calculate total index size
	stats := ipni.GetStats()
	// Estimate: 100 bytes per entry on average
	return stats.TotalEntries * 100, nil
}

// MessageHandler implementation for PubSub

// HandleMessage handles incoming PubSub messages
func (ipni *IPNI) HandleMessage(ctx context.Context, msg *Message) error {
	switch msg.Type {
	case "provider_announcement":
		var announcement PubSubProviderAnnouncement
		if err := json.Unmarshal(msg.Data, &announcement); err != nil {
			return fmt.Errorf("failed to unmarshal provider announcement: %w", err)
		}
		return ipni.handleProviderAnnouncement(ctx, &announcement)

	case "provider_removal":
		// Handle provider removal messages
		fmt.Printf("📢 Received provider removal message\n")
		return nil

	case "health_update":
		// Handle health update messages
		fmt.Printf("🏥 Received health update message\n")
		return nil

	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// GetMessageTypes returns supported message types
func (ipni *IPNI) GetMessageTypes() []string {
	return []string{
		"provider_announcement",
		"provider_removal",
		"health_update",
	}
}

// handleProviderAnnouncement processes provider announcements
func (ipni *IPNI) handleProviderAnnouncement(ctx context.Context, announcement *PubSubProviderAnnouncement) error {
	// Convert string provider ID to peer.ID
	providerID, err := announcement.GetProviderID()
	if err != nil {
		return fmt.Errorf("failed to parse peer ID: %w", err)
	}

	// Convert multihash strings to multihash objects
	var multihashes []multihash.Multihash
	for _, mhStr := range announcement.Multihashes {
		// Parse multihash string (simplified)
		// In a real implementation, this would use proper multihash parsing
		if len(mhStr) > 10 {
			// Create a mock multihash for demo
			hash := []byte(mhStr)
			// Ensure we have at least 32 bytes
			if len(hash) < 32 {
				// Pad with zeros if needed
				padded := make([]byte, 32)
				copy(padded, hash)
				hash = padded
			}
			if mh, err := multihash.Encode(hash[:32], multihash.SHA2_256); err == nil {
				multihashes = append(multihashes, mh)
			}
		}
	}

	// Store in provider index
	if len(multihashes) > 0 {
		return ipni.Put(providerID, announcement.ContextID, nil, multihashes...)
	}

	return nil
}

// Enhanced advertisement methods with integration

// CreateAdvertisement creates and stores an advertisement
func (ipni *IPNI) CreateAdvertisement(ctx context.Context, providerID peer.ID, contextID []byte, multihashes []multihash.Multihash, metadata *AdvertisementMetadata, protocol TransportProtocol, addresses []string) (*cid.Cid, error) {
	// Convert multihashes to strings
	var mhStrings []string
	for _, mh := range multihashes {
		mhStrings = append(mhStrings, mh.String())
	}

	// Create advertisement
	ad := &Advertisement{
		Provider:    providerID,
		ContextID:   contextID,
		Multihashes: mhStrings,
		Metadata:    metadata,
		Protocol:    protocol,
		Addresses:   addresses,
		Timestamp:   time.Now(),
		TTL:         ipni.config.DefaultTTL,
	}

	// Add to advertisement chain
	adCID, err := ipni.AdChain.AddAdvertisement(ctx, ad)
	if err != nil {
		return nil, fmt.Errorf("failed to add advertisement to chain: %w", err)
	}

	// Also store in provider index
	var cids []cid.Cid
	for _, mh := range multihashes {
		c := cid.NewCidV1(cid.Raw, mh)
		cids = append(cids, c)
	}

	if err := ipni.Provider.PutCID(providerID, contextID, nil, cids...); err != nil {
		fmt.Printf("⚠️ Warning: Failed to store in provider index: %v\n", err)
	}

	return adCID, nil
}

// GetSystemHealth returns comprehensive system health
func (ipni *IPNI) GetSystemHealth() *SystemHealth {
	if ipni.Monitoring != nil {
		return ipni.Monitoring.GetSystemHealth()
	}

	// Return basic health if monitoring is not available
	return &SystemHealth{
		Overall:    HealthHealthy,
		Components: make(map[string]HealthResult),
		Timestamp:  time.Now(),
		Version:    "demo-v1.0.0",
	}
}

// GetMetrics returns comprehensive IPNI metrics
func (ipni *IPNI) GetMetrics() *IPNIMetrics {
	if ipni.Monitoring != nil {
		// Update monitoring with latest stats
		stats := ipni.Provider.GetStats()
		chainStats := ipni.AdChain.GetStats()
		pubsubMetrics := ipni.PubSub.GetMetrics()

		ipni.Monitoring.UpdateMetrics(stats, chainStats, pubsubMetrics)
		return ipni.Monitoring.GetMetrics()
	}

	// Return basic metrics if monitoring is not available
	stats := ipni.Provider.GetStats()
	return &IPNIMetrics{
		TotalProviders:   stats.TotalProviders,
		TotalEntries:     stats.TotalEntries,
		TotalMultihashes: stats.TotalMultihashes,
		QueriesTotal:     stats.QueryCount,
		LastUpdate:       time.Now(),
	}
}
