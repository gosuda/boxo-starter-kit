package traversalselector

import (
	"context"
	"fmt"

	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	basicnode "github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
)

type TraversalSelectorWrapper struct {
	*ipldprime.IpldWrapper
}

func New(ipld *ipldprime.IpldWrapper) (*TraversalSelectorWrapper, error) {
	var err error
	if ipld == nil {
		ipld, err = ipldprime.NewDefault(nil, nil)
		if err != nil {
			return nil, err
		}
	}
	return &TraversalSelectorWrapper{
		IpldWrapper: ipld,
	}, nil
}

func (d *TraversalSelectorWrapper) defaultTraversalProgress() traversal.Progress {
	return traversal.Progress{
		Cfg: &traversal.Config{
			LinkSystem: d.LinkSystem,
			LinkTargetNodePrototypeChooser: func(_ datamodel.Link, lc linking.LinkContext) (datamodel.NodePrototype, error) {
				return basicnode.Prototype.Any, nil
			},
			LinkVisitOnlyOnce: true,
		},
	}
}

func (d *TraversalSelectorWrapper) WalkOneNode(
	ctx context.Context,
	root cid.Cid,
	visit traversal.VisitFn,
) error {
	node, err := d.GetIPLD(ctx, root)
	if err != nil {
		return fmt.Errorf("load root %s: %w", root, err)
	}
	prog := d.defaultTraversalProgress()
	return prog.WalkLocal(node, visit)
}

func (d *TraversalSelectorWrapper) WalkMatching(
	ctx context.Context,
	root cid.Cid,
	sel selector.Selector,
	visit traversal.VisitFn,
) error {
	node, err := d.GetIPLD(ctx, root)
	if err != nil {
		return fmt.Errorf("load root %s: %w", root, err)
	}
	prog := d.defaultTraversalProgress()
	return prog.WalkMatching(node, sel, visit)
}

func (d *TraversalSelectorWrapper) WalkAdv(
	ctx context.Context,
	root cid.Cid,
	sel selector.Selector,
	visit traversal.AdvVisitFn,
) error {
	node, err := d.GetIPLD(ctx, root)
	if err != nil {
		return fmt.Errorf("load root %s: %w", root, err)
	}
	prog := d.defaultTraversalProgress()
	return prog.WalkAdv(node, sel, visit)
}

func (d *TraversalSelectorWrapper) WalkTransforming(
	ctx context.Context,
	root cid.Cid,
	sel selector.Selector,
	transform traversal.TransformFn,
) (datamodel.Node, error) {
	node, err := d.GetIPLD(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("load root %s: %w", root, err)
	}
	prog := d.defaultTraversalProgress()
	return prog.WalkTransforming(node, sel, transform)
}
