package dag

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"

	bitswap "github.com/gosuda/boxo-starter-kit/04-bitswap/pkg"
)

// legacy merkledag service wrapper
type DagServiceWrapper struct {
	BlockServiceWrapper *bitswap.BlockServiceWrapper
	format.DAGService
}

func NewDagServiceWrapper(ctx context.Context, blockserviceWrapper *bitswap.BlockServiceWrapper) (*DagServiceWrapper, error) {
	var err error

	if blockserviceWrapper == nil {
		blockserviceWrapper, err = bitswap.NewBlockService(ctx, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create BlockService wrapper: %w", err)
		}
	}
	merkledagService := merkledag.NewDAGService(blockserviceWrapper)

	return &DagServiceWrapper{
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
