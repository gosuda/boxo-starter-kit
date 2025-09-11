package block

import (
	"context"

	blockstore "github.com/ipfs/boxo/blockstore"
	blockformat "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
)

type Store struct {
	bs blockstore.Blockstore
}

func NewInMemory() *Store {
	mds := dssync.MutexWrap(ds.NewMapDatastore())
	bs := blockstore.NewBlockstore(mds)
	return &Store{bs: bs}
}

func (s *Store) Put(ctx context.Context, data []byte) (cid.Cid, error) {
	blk := blockformat.NewBlock(data)
	if err := s.bs.Put(ctx, blk); err != nil {
		return cid.Undef, err
	}
	return blk.Cid(), nil
}

func (s *Store) Get(ctx context.Context, c cid.Cid) ([]byte, error) {
	blk, err := s.bs.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return blk.RawData(), nil
}
