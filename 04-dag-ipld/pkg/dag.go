package dag

import (
	"fmt"

	"github.com/ipfs/boxo/ipld/merkledag"
	format "github.com/ipfs/go-ipld-format"

	bitswap "github.com/gosuda/boxo-starter-kit/03-bitswap-blockservice/pkg"
)

// legacy merkledag service wrapper
type DagServiceWrapper struct {
	format.DAGService
}

func NewDagServiceWrapper(blockserviceWrapper *bitswap.BlockServiceWrapper) (*DagServiceWrapper, error) {
	var err error
	if blockserviceWrapper == nil {
		blockserviceWrapper, err = bitswap.NewBlockService(nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create BlockService wrapper: %w", err)
		}
	}
	merkledagService := merkledag.NewDAGService(blockserviceWrapper)

	return &DagServiceWrapper{
		DAGService: merkledagService,
	}, nil
}
