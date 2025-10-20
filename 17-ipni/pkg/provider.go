package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// SimpleProvider announces content to the network
// "I have this content and you can get it from me"
type SimpleProvider struct {
	host      host.Host
	content   map[cid.Cid]*ContentInfo // What content this provider has
	index     *SimpleIndex             // Where to announce
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	defaultTTL time.Duration
}

// ContentInfo stores metadata about content being provided
type ContentInfo struct {
	CID       cid.Cid
	Protocols []Protocol
	AddedAt   time.Time
}

// NewSimpleProvider creates a new content provider
func NewSimpleProvider(h host.Host, index *SimpleIndex) *SimpleProvider {
	ctx, cancel := context.WithCancel(context.Background())

	return &SimpleProvider{
		host:       h,
		content:    make(map[cid.Cid]*ContentInfo),
		index:      index,
		ctx:        ctx,
		cancel:     cancel,
		defaultTTL: 24 * time.Hour, // Default: records valid for 24 hours
	}
}

// Announce tells the network "I have this content"
// This is the main function providers call to advertise their content
func (p *SimpleProvider) Announce(ctx context.Context, c cid.Cid, protocols []Protocol) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store locally
	p.content[c] = &ContentInfo{
		CID:       c,
		Protocols: protocols,
		AddedAt:   time.Now(),
	}

	// Create provider record
	record := &ProviderRecord{
		ProviderID: p.host.ID(),
		ContentID:  c,
		Protocols:  protocols,
		Addresses:  multiaddrsToStrings(p.host.Addrs()),
		Timestamp:  time.Now(),
		TTL:        p.defaultTTL,
	}

	// Announce to index
	if p.index != nil {
		return p.index.Announce(record)
	}

	return nil
}

// Remove removes a content announcement
func (p *SimpleProvider) Remove(ctx context.Context, c cid.Cid) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.content, c)

	// Remove from index
	if p.index != nil {
		return p.index.Remove(c, p.host.ID())
	}

	return nil
}

// ListContent returns all content this provider is announcing
func (p *SimpleProvider) ListContent() []cid.Cid {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cids := make([]cid.Cid, 0, len(p.content))
	for c := range p.content {
		cids = append(cids, c)
	}
	return cids
}

// GetContentInfo returns info about a specific content
func (p *SimpleProvider) GetContentInfo(c cid.Cid) (*ContentInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info, exists := p.content[c]
	if !exists {
		return nil, fmt.Errorf("content not found: %s", c.String())
	}

	return info, nil
}

// HasContent checks if the provider has specific content
func (p *SimpleProvider) HasContent(c cid.Cid) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.content[c]
	return exists
}

// SetTTL sets the default TTL for new announcements
func (p *SimpleProvider) SetTTL(ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defaultTTL = ttl
}

// Close stops the provider
func (p *SimpleProvider) Close() error {
	p.cancel()
	return nil
}

// Stats returns provider statistics
func (p *SimpleProvider) Stats() ProviderStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProviderStats{
		ProviderID:     p.host.ID(),
		ContentCount:   len(p.content),
		Protocols:      p.getUniqueProtocols(),
		AnnouncementTTL: p.defaultTTL,
	}
}

// ProviderStats contains provider statistics
type ProviderStats struct {
	ProviderID      peer.ID
	ContentCount    int
	Protocols       []Protocol
	AnnouncementTTL time.Duration
}

func (p *SimpleProvider) getUniqueProtocols() []Protocol {
	protocolSet := make(map[Protocol]bool)
	for _, info := range p.content {
		for _, proto := range info.Protocols {
			protocolSet[proto] = true
		}
	}

	protocols := make([]Protocol, 0, len(protocolSet))
	for proto := range protocolSet {
		protocols = append(protocols, proto)
	}
	return protocols
}

// Helper function to convert multiaddrs to strings
func multiaddrsToStrings(addrs []multiaddr.Multiaddr) []string {
	result := make([]string, len(addrs))
	for i, addr := range addrs {
		result[i] = addr.String()
	}
	return result
}
