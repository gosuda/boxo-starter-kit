package pin

import (
	"context"
	"fmt"

	ipfspinner "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/boxo/pinning/pinner/dspinner"

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
)

type PinnerWrapper struct {
	dagWrapper *dag.IpldWrapper
	ipfspinner.Pinner
}

func NewPinnerWrapper(ctx context.Context, dagWrapper *dag.IpldWrapper) (*PinnerWrapper, error) {
	if dagWrapper == nil {
		return nil, fmt.Errorf("dag wrapper cannot be nil")
	}

	pinner, err := dspinner.New(ctx, dagWrapper.BlockServiceWrapper.PersistentWrapper.Batching, dagWrapper)
	if err != nil {
		return nil, err
	}

	return &PinnerWrapper{
		dagWrapper: dagWrapper,
		Pinner:     pinner,
	}, nil
}

func (p *PinnerWrapper) Close() error {
	if p.Pinner == nil {
		return nil
	}
	err := p.Pinner.Flush(context.Background())
	if err != nil {
		return err
	}

	return p.dagWrapper.BlockServiceWrapper.Close()
}
