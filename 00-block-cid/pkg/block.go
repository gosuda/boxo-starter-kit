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

type Store struct {
	bs blockstore.Blockstore
}

func NewInMemory() *Store {
	mds := dssync.MutexWrap(ds.NewMapDatastore())
	return New(mds)
}

func New(ds ds.Batching, opts ...blockstore.Option) *Store {
	bs := blockstore.NewBlockstore(ds, opts...)
	return &Store{bs: bs}
}

func (s *Store) Put(ctx context.Context, data []byte) (cid.Cid, error) {
	blk := blockformat.NewBlock(data)
	if err := s.bs.Put(ctx, blk); err != nil {
		return cid.Undef, err
	}

	return blk.Cid(), nil
}

func (s *Store) PutV1Cid(ctx context.Context, data []byte, prefix *cid.Prefix) (cid.Cid, error) {
	if prefix.Version != 0 {
		prefix.Version = 1
	}
	if prefix.Codec == 0 {
		prefix.Codec = uint64(mc.Raw)
	}
	if prefix.MhType == 0 {
		prefix.MhType = mh.SHA2_256
	}
	if prefix.MhLength == 0 {
		prefix.MhLength = -1
	}

	c, err := prefix.Sum(data)
	if err != nil {
		return cid.Undef, err
	}
	blk, err := blockformat.NewBlockWithCid(data, c)
	if err != nil {
		return cid.Undef, err
	}
	if err := s.bs.Put(ctx, blk); err != nil {
		return cid.Undef, err
	}
	return blk.Cid(), nil
}

func (s *Store) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return s.bs.Has(ctx, c)
}

func (s *Store) Get(ctx context.Context, c cid.Cid) (blockformat.Block, error) {
	return s.bs.Get(ctx, c)
}

func (s *Store) GetRaw(ctx context.Context, c cid.Cid) ([]byte, error) {
	blk, err := s.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return blk.RawData(), nil
}

func (s *Store) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	return s.bs.GetSize(ctx, c)
}

func (s *Store) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return s.bs.AllKeysChan(ctx)
}

func (s *Store) Delete(ctx context.Context, c cid.Cid) error {
	return s.bs.DeleteBlock(ctx, c)
}
