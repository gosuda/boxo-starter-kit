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

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
)

var _ exchange.Interface = (*BitswapWrapper)(nil)

// BitswapWrapper represents a simplified IPFS node with block exchange capability
// This is an educational implementation focusing on core P2P concepts
type BitswapWrapper struct {
	HostWrapper       *network.HostWrapper
	PersistentWrapper *persistent.PersistentWrapper
	*bitswap.Bitswap
}

// NewBitswap creates a new simplified bitswap node for educational purposes
func NewBitswap(ctx context.Context, host *network.HostWrapper, persistentWrapper *persistent.PersistentWrapper) (*BitswapWrapper, error) {
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

	bsnet := bsnet.NewFromIpfsHost(host)
	bsnet = bnet.New(nil, bsnet, nil)
	bswap := bitswap.New(ctx, bsnet, nil, persistentWrapper,
		bitswap.SetSendDontHaves(true),
		bitswap.ProviderSearchDelay(time.Second*5),
	)

	node := &BitswapWrapper{
		HostWrapper:       host,
		PersistentWrapper: persistentWrapper,
		Bitswap:           bswap,
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
