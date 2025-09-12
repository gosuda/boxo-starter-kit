package block

import (
	"context"

	blockstore "github.com/ipfs/boxo/blockstore"
	blockformat "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
)

type BlockWrapper struct {
	blockstore.Blockstore
}

func NewInMemory() *BlockWrapper {
	mds := dssync.MutexWrap(ds.NewMapDatastore())
	return New(mds)
}

func New(ds ds.Batching, opts ...blockstore.Option) *BlockWrapper {
	bs := blockstore.NewBlockstore(ds, opts...)
	return &BlockWrapper{Blockstore: bs}
}

func (s *BlockWrapper) Put(ctx context.Context, data []byte) (cid.Cid, error) {
	blk := blockformat.NewBlock(data)
	if err := s.Blockstore.Put(ctx, blk); err != nil {
		return cid.Undef, err
	}

	return blk.Cid(), nil
}

func (s *BlockWrapper) PutV1Cid(ctx context.Context, data []byte, prefix *cid.Prefix) (cid.Cid, error) {
	if prefix == nil {
		// default to v1, raw, sha2-256
		prefix = NewV1Prefix(0, 0, 0)
	}
	c, err := prefix.Sum(data)
	if err != nil {
		return cid.Undef, err
	}
	blk, err := blockformat.NewBlockWithCid(data, c)
	if err != nil {
		return cid.Undef, err
	}
	if err := s.Blockstore.Put(ctx, blk); err != nil {
		return cid.Undef, err
	}
	return blk.Cid(), nil
}

func (s *BlockWrapper) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return s.Blockstore.Has(ctx, c)
}

func (s *BlockWrapper) Get(ctx context.Context, c cid.Cid) (blockformat.Block, error) {
	return s.Blockstore.Get(ctx, c)
}

func (s *BlockWrapper) GetRaw(ctx context.Context, c cid.Cid) ([]byte, error) {
	blk, err := s.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return blk.RawData(), nil
}

func (s *BlockWrapper) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	return s.Blockstore.GetSize(ctx, c)
}

func (s *BlockWrapper) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return s.Blockstore.AllKeysChan(ctx)
}

func (s *BlockWrapper) Delete(ctx context.Context, c cid.Cid) error {
	return s.Blockstore.DeleteBlock(ctx, c)
}

func NewV1Prefix(mcType mc.Code, mhType uint64, mhLength int) *cid.Prefix {
	if mcType == 0 {
		mcType = mc.Raw
	}
	if mhType == 0 {
		mhType = mh.SHA2_256
	}
	if mhLength == 0 {
		mhLength = -1
	}

	return &cid.Prefix{
		Version:  1,
		Codec:    uint64(mc.Raw),
		MhType:   mhType,
		MhLength: mhLength,
	}
}
