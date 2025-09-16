package block

import (
	blockformat "github.com/ipfs/go-block-format"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
)

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
		Codec:    uint64(mcType),
		MhType:   mhType,
		MhLength: mhLength,
	}
}

func ComputeCID(data []byte, prefix *cid.Prefix) (cid.Cid, error) {
	if prefix == nil {
		// default to v1, raw, sha2-256
		prefix = NewV1Prefix(0, 0, 0)
	}
	return prefix.Sum(data)
}

func NewBlock(data []byte, prefix *cid.Prefix) (blocks.Block, error) {
	c, err := ComputeCID(data, prefix)
	if err != nil {
		return nil, err
	}
	return blockformat.NewBlockWithCid(data, c)
}

func ToV1Block(b blocks.Block) (blocks.Block, error) {
	if b.Cid().Version() == 1 {
		return b, nil
	}
	prefix := NewV1Prefix(
		mc.Code(b.Cid().Prefix().Codec),
		b.Cid().Prefix().MhType,
		b.Cid().Prefix().MhLength,
	)
	newCid, err := prefix.Sum(b.RawData())
	if err != nil {
		return nil, err
	}
	return blockformat.NewBlockWithCid(b.RawData(), newCid)
}
