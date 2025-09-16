package bitswap

import (
	"context"
	"fmt"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type BlockServiceWrapper struct {
	blockservice.BlockService
}

func NewBlockService(persistentWrapper *persistent.PersistentWrapper, bitswapWrapper *BitswapWrapper) (*BlockServiceWrapper, error) {
	var err error
	if persistentWrapper == nil {
		if bitswapWrapper != nil && bitswapWrapper.PersistentWrapper != nil {
			// Try to use the one from bitswap if available
			persistentWrapper = bitswapWrapper.PersistentWrapper
		} else {
			// Otherwise, create a new in-memory one
			persistentWrapper, err = persistent.New(persistent.Memory, "")
			if err != nil {
				return nil, err
			}
		}
	}
	if bitswapWrapper == nil {
		bitswapWrapper, err = NewBitswap(context.TODO(), nil, persistentWrapper)
		if err != nil {
			return nil, fmt.Errorf("init bitswap: %w", err)
		}
	}

	bs := blockservice.New(persistentWrapper, bitswapWrapper)

	return &BlockServiceWrapper{
		BlockService: bs,
	}, nil
}

func (b *BlockServiceWrapper) Close() error {
	if b.BlockService == nil {
		return nil
	}
	return b.BlockService.Close()
}

func (b *BlockServiceWrapper) GetBlockRaw(ctx context.Context, cid cid.Cid) ([]byte, error) {
	blk, err := b.BlockService.GetBlock(ctx, cid)
	if err != nil {
		return nil, err
	}
	return blk.RawData(), nil
}

func (b *BlockServiceWrapper) GetBlock(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	return b.BlockService.GetBlock(ctx, cid)
}

func (b *BlockServiceWrapper) GetBlocks(ctx context.Context, cids []cid.Cid) <-chan blocks.Block {
	return b.BlockService.GetBlocks(ctx, cids)
}

func (b *BlockServiceWrapper) AddBlockRaw(ctx context.Context, payload []byte) (cid.Cid, error) {
	c, err := block.ComputeCID(payload, nil)
	if err != nil {
		return cid.Undef, err
	}

	blk, err := blocks.NewBlockWithCid(payload, c)
	if err != nil {
		return cid.Undef, err
	}
	err = b.AddBlock(ctx, blk)
	if err != nil {
		return cid.Undef, err
	}
	return c, nil
}

func (b *BlockServiceWrapper) AddBlock(ctx context.Context, block blocks.Block) error {
	return b.BlockService.AddBlock(ctx, block)
}

func (b *BlockServiceWrapper) AddBlocks(ctx context.Context, blocks []blocks.Block) error {
	return b.BlockService.AddBlocks(ctx, blocks)
}

func (b *BlockServiceWrapper) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	return b.BlockService.DeleteBlock(ctx, cid)
}

func (b *BlockServiceWrapper) HasBlock(ctx context.Context, cid cid.Cid) (bool, error) {
	return b.Blockstore().Has(ctx, cid)
}
