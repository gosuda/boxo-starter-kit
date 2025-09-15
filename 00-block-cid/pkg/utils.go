package block

import (
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
