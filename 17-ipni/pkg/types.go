package indexer

import (
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Protocol represents the transfer protocol supported by a provider
type Protocol string

const (
	ProtocolBitswap   Protocol = "bitswap"
	ProtocolHTTP      Protocol = "http"
	ProtocolGraphSync Protocol = "graphsync"
)

// ProviderRecord represents a single announcement from a provider
// "I (provider) have this content (CID) and you can get it via these protocols"
type ProviderRecord struct {
	ProviderID peer.ID    // Who has the content
	ContentID  cid.Cid    // What content they have
	Protocols  []Protocol // How to get it (bitswap, http, graphsync)
	Addresses  []string   // Where to reach the provider (multiaddrs)
	Timestamp  time.Time  // When this was announced
	TTL        time.Duration // How long this record is valid
}

// IsExpired checks if the provider record has expired
func (p *ProviderRecord) IsExpired() bool {
	if p.TTL == 0 {
		return false // Never expires
	}
	return time.Since(p.Timestamp) > p.TTL
}

// SupportsProtocol checks if the provider supports a given protocol
func (p *ProviderRecord) SupportsProtocol(proto Protocol) bool {
	for _, p := range p.Protocols {
		if p == proto {
			return true
		}
	}
	return false
}
