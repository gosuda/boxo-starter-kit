package bitswap

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/bitswap"
	bsnet "github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

var _ exchange.Interface = (*BitswapWrapper)(nil)

// BitswapWrapper represents a simplified IPFS node with block exchange capability
// This is an educational implementation focusing on core P2P concepts
type BitswapWrapper struct {
	persistentWrapper *persistent.PersistentWrapper
	*bitswap.Bitswap
}

// NewBitswapNode creates a new simplified bitswap node for educational purposes
func NewBitswapNode(ctx context.Context, host host.Host, persistentWrapper *persistent.PersistentWrapper) (*BitswapWrapper, error) {
	var err error
	if host == nil {
		host, err = libp2p.New(
			libp2p.ListenAddrs([]multiaddr.Multiaddr{multiaddr.StringCast("/ip4/0.0.0.0/tcp/0")}...),
			libp2p.EnableRelay(), // Enable relay for NAT traversal
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p host: %w", err)
		}
	}

	bsnet := bsnet.NewFromIpfsHost(host)
	bswap := bitswap.New(ctx, bsnet, nil, persistentWrapper)

	node := &BitswapWrapper{
		Bitswap:           bswap,
		persistentWrapper: persistentWrapper,
	}

	return node, nil
}

func (b *BitswapWrapper) Close() error {
	if err := b.Bitswap.Close(); err != nil {
		return err
	}
	if b.persistentWrapper != nil {
		return b.persistentWrapper.Close()
	}
	return nil
}

func (b *BitswapWrapper) PutBlockRaw(ctx context.Context, data []byte) (cid.Cid, error) {
	if len(data) == 0 {
		return cid.Undef, fmt.Errorf("empty data")
	}

	c, err := b.persistentWrapper.PutRaw(ctx, data)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to store block: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to build block with cid: %w", err)
	}

	if err := b.Bitswap.NotifyNewBlocks(ctx, blk); err != nil {
		return cid.Undef, fmt.Errorf("bitswap announce failed: %w", err)
	}

	return c, nil
}

// GetBlock retrieves a block by CID (simplified implementation)
func (b *BitswapWrapper) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return b.Bitswap.GetBlock(ctx, c)
}

func (b *BitswapWrapper) GetBlockRaw(ctx context.Context, c cid.Cid) ([]byte, error) {
	block, err := b.Bitswap.GetBlock(ctx, c)
	if err != nil {
		return nil, err
	}
	return block.RawData(), nil
}
