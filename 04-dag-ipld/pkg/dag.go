package dag

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	bitswap "github.com/gosuda/boxo-starter-kit/03-bitswap-blockservice/pkg"
)

// legacy merkledag service wrapper
type DagServiceWrapper struct {
	prefix              *cid.Prefix
	BlockServiceWrapper *bitswap.BlockServiceWrapper
	format.DAGService
}

func NewDagServiceWrapper(prefix *cid.Prefix, blockserviceWrapper *bitswap.BlockServiceWrapper) (*DagServiceWrapper, error) {
	var err error
	if prefix == nil {
		prefix = block.NewV1Prefix(mc.Protobuf, 0, 0)
	}
	if blockserviceWrapper == nil {
		blockserviceWrapper, err = bitswap.NewBlockService(nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create BlockService wrapper: %w", err)
		}
	}
	merkledagService := merkledag.NewDAGService(blockserviceWrapper)

	return &DagServiceWrapper{
		prefix:              prefix,
		BlockServiceWrapper: blockserviceWrapper,
		DAGService:          merkledagService,
	}, nil
}

func (d *DagServiceWrapper) AddRaw(ctx context.Context, payload []byte) (cid.Cid, error) {
	pn := merkledag.NewRawNode(payload)
	err := d.DAGService.Add(ctx, pn)
	if err != nil {
		return cid.Undef, err
	}
	return pn.Cid(), nil
}

func (d *DagServiceWrapper) GetRaw(ctx context.Context, c cid.Cid) ([]byte, error) {
	nd, err := d.DAGService.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return nd.RawData(), nil
}
