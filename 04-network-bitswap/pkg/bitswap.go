package bitswap

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	dag "github.com/gosuda/boxo-starter-kit/02-dag-ipld/pkg"
)

// BitswapNode represents a simplified IPFS node with block exchange capability
// This is an educational implementation focusing on core P2P concepts
type BitswapNode struct {
	host       host.Host
	dagWrapper *dag.DagWrapper

	// Node info
	id        peer.ID
	addresses []multiaddr.Multiaddr

	// Statistics
	stats struct {
		mutex          sync.RWMutex
		BlocksSent     int64 `json:"blocks_sent"`
		BlocksReceived int64 `json:"blocks_received"`
		PeersConnected int   `json:"peers_connected"`
		WantListSize   int   `json:"want_list_size"`
	}
}

// NodeConfig configures a bitswap node
type NodeConfig struct {
	ListenAddrs    []string // Addresses to listen on (e.g., "/ip4/0.0.0.0/tcp/0")
	BootstrapPeers []string // Bootstrap peer addresses
}

// NewBitswapNode creates a new simplified bitswap node for educational purposes
func NewBitswapNode(ctx context.Context, dagWrapper *dag.DagWrapper, config NodeConfig) (*BitswapNode, error) {
	if dagWrapper == nil {
		return nil, fmt.Errorf("dag wrapper cannot be nil")
	}

	// Create libp2p host with default configuration
	var listenAddrs []multiaddr.Multiaddr
	for _, addr := range config.ListenAddrs {
		if addr == "" {
			addr = "/ip4/0.0.0.0/tcp/0" // Default listen address
		}
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid listen address %s: %w", addr, err)
		}
		listenAddrs = append(listenAddrs, ma)
	}

	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.EnableRelay(), // Enable relay for NAT traversal
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	node := &BitswapNode{
		host:       h,
		dagWrapper: dagWrapper,
		id:         h.ID(),
		addresses:  h.Addrs(),
	}

	return node, nil
}

// GetID returns the peer ID of this node
func (n *BitswapNode) GetID() peer.ID {
	return n.id
}

// GetAddresses returns the multiaddresses this node is listening on
func (n *BitswapNode) GetAddresses() []multiaddr.Multiaddr {
	return n.addresses
}

// GetFullAddresses returns the full multiaddresses including peer ID
func (n *BitswapNode) GetFullAddresses() []multiaddr.Multiaddr {
	var fullAddrs []multiaddr.Multiaddr
	peerPart, _ := multiaddr.NewMultiaddr("/p2p/" + n.id.String())

	for _, addr := range n.addresses {
		fullAddr := addr.Encapsulate(peerPart)
		fullAddrs = append(fullAddrs, fullAddr)
	}

	return fullAddrs
}

// ConnectToPeer connects to another bitswap node
func (n *BitswapNode) ConnectToPeer(ctx context.Context, addr multiaddr.Multiaddr) error {
	// Extract peer info from multiaddr
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("failed to parse peer address: %w", err)
	}

	// Connect to peer
	err = n.host.Connect(ctx, *info)
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", info.ID, err)
	}

	// Update stats
	n.updatePeerStats()

	return nil
}

// GetBlock retrieves a block by CID (simplified implementation)
func (n *BitswapNode) GetBlock(ctx context.Context, c cid.Cid) ([]byte, error) {
	if !c.Defined() {
		return nil, fmt.Errorf("invalid CID")
	}

	// Try to get block from local storage first
	data, err := n.dagWrapper.PersistentWrapper.GetRaw(ctx, c)
	if err == nil {
		// Update stats
		n.stats.mutex.Lock()
		n.stats.BlocksReceived++
		n.stats.mutex.Unlock()
		return data, nil
	}

	// In a full implementation, this would request the block from peers
	// For this educational version, we just return the local result
	return nil, fmt.Errorf("block not found locally (P2P exchange not implemented in this demo): %s", c.String())
}

// PutBlock stores a block and makes it available to peers
func (n *BitswapNode) PutBlock(ctx context.Context, data []byte) (cid.Cid, error) {
	if len(data) == 0 {
		return cid.Undef, fmt.Errorf("empty data")
	}

	// Store block in local storage
	c, err := n.dagWrapper.PersistentWrapper.Put(ctx, data)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to store block: %w", err)
	}

	// The bitswap will automatically serve this block to peers who request it

	// Update stats
	n.stats.mutex.Lock()
	n.stats.BlocksSent++
	n.stats.mutex.Unlock()

	return c, nil
}

// WantBlock adds a block to the want list (simplified implementation)
func (n *BitswapNode) WantBlock(ctx context.Context, c cid.Cid) error {
	if !c.Defined() {
		return fmt.Errorf("invalid CID")
	}

	// In a full bitswap implementation, this would:
	// 1. Add CID to want list
	// 2. Announce want to connected peers
	// 3. Wait for providers to respond
	// For this educational version, we simulate the behavior
	n.stats.mutex.Lock()
	n.stats.WantListSize++
	n.stats.mutex.Unlock()

	return nil
}

// GetConnectedPeers returns the list of connected peers
func (n *BitswapNode) GetConnectedPeers() []peer.ID {
	return n.host.Network().Peers()
}

// GetStats returns current bitswap statistics
func (n *BitswapNode) GetStats() BitswapStats {
	n.stats.mutex.RLock()
	defer n.stats.mutex.RUnlock()

	return BitswapStats{
		BlocksSent:     n.stats.BlocksSent,
		BlocksReceived: n.stats.BlocksReceived,
		PeersConnected: len(n.GetConnectedPeers()),
		WantListSize:   n.stats.WantListSize,
		NodeID:         n.id.String(),
	}
}

// BitswapStats contains bitswap node statistics
type BitswapStats struct {
	BlocksSent     int64  `json:"blocks_sent"`
	BlocksReceived int64  `json:"blocks_received"`
	PeersConnected int    `json:"peers_connected"`
	WantListSize   int    `json:"want_list_size"`
	NodeID         string `json:"node_id"`
}

// Close shuts down the bitswap node
func (n *BitswapNode) Close() error {
	if n.host != nil {
		return n.host.Close()
	}
	return nil
}

// updatePeerStats updates peer-related statistics
func (n *BitswapNode) updatePeerStats() {
	n.stats.mutex.Lock()
	defer n.stats.mutex.Unlock()

	n.stats.PeersConnected = len(n.GetConnectedPeers())
}

// Note: This simplified implementation focuses on demonstrating P2P networking concepts
// rather than full bitswap protocol implementation. In a production system,
// you would use the complete boxo bitswap package with proper routing,
// want-list management, and provider discovery.
