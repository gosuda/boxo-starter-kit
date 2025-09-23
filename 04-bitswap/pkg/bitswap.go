package bitswap

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/bitswap"
	bnet "github.com/ipfs/boxo/bitswap/network"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	dht "github.com/gosuda/boxo-starter-kit/03-dht-router/pkg"
	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

var _ exchange.Interface = (*BitswapWrapper)(nil)

// BitswapWrapper represents a simplified IPFS node with block exchange capability
// This is an educational implementation focusing on core P2P concepts
type BitswapWrapper struct {
	HostWrapper       *network.HostWrapper
	PersistentWrapper *persistent.PersistentWrapper
	*bitswap.Bitswap

	// Metrics
	metrics *metrics.ComponentMetrics
}

// NewBitswap creates a new simplified bitswap node for educational purposes
func NewBitswap(ctx context.Context, dhtWrapper *dht.DHTWrapper, host *network.HostWrapper, persistentWrapper *persistent.PersistentWrapper) (*BitswapWrapper, error) {
	var err error
	if host == nil {
		host, err = network.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p host: %w", err)
		}
	}
	if persistentWrapper == nil {
		persistentWrapper, err = persistent.New(persistent.Memory, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create persistent storage: %w", err)
		}
	}
	if dhtWrapper == nil {
		dhtWrapper, err = dht.New(ctx, host, persistentWrapper)
		if err != nil {
			return nil, fmt.Errorf("failed to create DHT: %w", err)
		}
	}

	bsnet := bsnet.NewFromIpfsHost(host)
	bsnet = bnet.New(nil, bsnet, nil)
	bswap := bitswap.New(ctx, bsnet, dhtWrapper, persistentWrapper,
		bitswap.SetSendDontHaves(true),
		bitswap.ProviderSearchDelay(time.Second),
	)

	// Initialize metrics
	bitswapMetrics := metrics.NewComponentMetrics("bitswap")
	metrics.RegisterGlobalComponent(bitswapMetrics)

	node := &BitswapWrapper{
		HostWrapper:       host,
		PersistentWrapper: persistentWrapper,
		Bitswap:           bswap,
		metrics:           bitswapMetrics,
	}

	return node, nil
}

func (b *BitswapWrapper) Close() error {
	if err := b.Bitswap.Close(); err != nil {
		return err
	}
	if b.PersistentWrapper != nil {
		return b.PersistentWrapper.Close()
	}
	return nil
}

// It is only used for example, not scoped for production use
func (b *BitswapWrapper) PutBlockRaw(ctx context.Context, data []byte) (cid.Cid, error) {
	if len(data) == 0 {
		return cid.Undef, fmt.Errorf("empty data")
	}

	blk, err := block.NewBlock(data, nil)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to build block with cid: %w", err)
	}

	err = b.PersistentWrapper.Put(ctx, blk)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to build block with cid: %w", err)
	}

	if err := b.Bitswap.NotifyNewBlocks(ctx, blk); err != nil {
		return cid.Undef, fmt.Errorf("bitswap announce failed: %w", err)
	}

	return blk.Cid(), nil
}

// GetBlock retrieves a block by CID (simplified implementation)
func (b *BitswapWrapper) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return b.Bitswap.GetBlock(ctx, c)
}

func (b *BitswapWrapper) GetBlockRaw(ctx context.Context, c cid.Cid) ([]byte, error) {
	block, err := b.GetBlock(ctx, c)
	if err != nil {
		return nil, err
	}
	return block.RawData(), nil
}

// GetBlockFromPeer retrieves a block from a specific peer
func (b *BitswapWrapper) GetBlockFromPeer(ctx context.Context, c cid.Cid, targetPeer peer.ID) (blocks.Block, error) {
	start := time.Now()
	b.metrics.RecordRequest()

	// Check if we're already connected to the target peer
	connected := b.HostWrapper.Host.Network().Connectedness(targetPeer)
	if connected != 1 { // NotConnected = 0, Connected = 1
		// Try to connect to the peer
		peerAddrs := b.HostWrapper.Host.Peerstore().Addrs(targetPeer)
		if len(peerAddrs) > 0 {
			err := b.HostWrapper.Host.Connect(ctx, peer.AddrInfo{
				ID:    targetPeer,
				Addrs: peerAddrs,
			})
			if err != nil {
				b.metrics.RecordFailure(time.Since(start), "peer_connection_failed")
				return nil, fmt.Errorf("failed to connect to peer %s: %w", targetPeer, err)
			}
		}
	}

	// Create a session for targeted fetching
	session := b.Bitswap.NewSession(ctx)

	// Use the session to fetch the block
	// Note: This still relies on the underlying bitswap routing,
	// but sessions provide better performance for targeted requests
	block, err := session.GetBlock(ctx, c)
	if err != nil {
		b.metrics.RecordFailure(time.Since(start), "block_fetch_failed")
		return nil, fmt.Errorf("failed to get block %s from peer %s: %w", c, targetPeer, err)
	}

	b.metrics.RecordSuccess(time.Since(start), int64(len(block.RawData())))
	return block, nil
}

// GetBlockFromPeerRaw retrieves raw block data from a specific peer
func (b *BitswapWrapper) GetBlockFromPeerRaw(ctx context.Context, c cid.Cid, targetPeer peer.ID) ([]byte, error) {
	block, err := b.GetBlockFromPeer(ctx, c, targetPeer)
	if err != nil {
		return nil, err
	}
	return block.RawData(), nil
}

// RequestBlockFromPeer sends a block request to a specific peer without blocking
func (b *BitswapWrapper) RequestBlockFromPeer(ctx context.Context, c cid.Cid, targetPeer peer.ID) error {
	start := time.Now()
	b.metrics.RecordRequest()

	// Check connection
	connected := b.HostWrapper.Host.Network().Connectedness(targetPeer)
	if connected != 1 {
		peerAddrs := b.HostWrapper.Host.Peerstore().Addrs(targetPeer)
		if len(peerAddrs) > 0 {
			err := b.HostWrapper.Host.Connect(ctx, peer.AddrInfo{
				ID:    targetPeer,
				Addrs: peerAddrs,
			})
			if err != nil {
				b.metrics.RecordFailure(time.Since(start), "peer_connection_failed")
				return fmt.Errorf("failed to connect to peer %s: %w", targetPeer, err)
			}
		}
	}

	// Send want request (non-blocking)
	session := b.Bitswap.NewSession(ctx)

	// Start fetching in background
	go func() {
		_, err := session.GetBlock(ctx, c)
		if err != nil {
			// Log error but don't return it since this is async
			b.metrics.RecordFailure(time.Since(start), "async_block_fetch_failed")
		} else {
			b.metrics.RecordSuccess(time.Since(start), 0) // Size unknown in async mode
		}
	}()

	return nil
}

// IsConnectedToPeer checks if we're connected to a specific peer
func (b *BitswapWrapper) IsConnectedToPeer(peerID peer.ID) bool {
	return b.HostWrapper.Host.Network().Connectedness(peerID) == 1
}

// GetConnectedPeers returns a list of currently connected peers
func (b *BitswapWrapper) GetConnectedPeers() []peer.ID {
	return b.HostWrapper.Host.Network().Peers()
}
