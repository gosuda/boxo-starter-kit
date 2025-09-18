package ipldprime

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/storage/bsadapter"
	"github.com/ipld/go-ipld-prime/traversal"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

type IpldWrapper struct {
	Prefix     *cid.Prefix
	LinkSystem linking.LinkSystem
}

func New(prefix *cid.Prefix, linkSystem *linking.LinkSystem) (*IpldWrapper, error) {
	if prefix == nil {
		prefix = block.NewV1Prefix(mc.DagCbor, 0, 0)
	}
	if linkSystem == nil {
		return nil, fmt.Errorf("linkSystem is required")
	}
	return &IpldWrapper{
		Prefix:     prefix,
		LinkSystem: *linkSystem,
	}, nil
}

func NewDefault(prefix *cid.Prefix, persistentWrapper *persistent.PersistentWrapper) (*IpldWrapper, error) {
	var err error
	if prefix == nil {
		prefix = block.NewV1Prefix(mc.DagCbor, 0, 0)
	}
	if persistentWrapper == nil {
		persistentWrapper, err = persistent.New(persistent.Pebbledb, "")
		if err != nil {
			return nil, err
		}
	}

	ad := &bsadapter.Adapter{
		Wrapped: persistentWrapper,
	}
	linkSystem := cidlink.DefaultLinkSystem()
	linkSystem.SetReadStorage(ad)
	linkSystem.SetWriteStorage(ad)

	return New(prefix, &linkSystem)
}

//-------------------------------------------------------------------------------//
// IPLD-prime util methods
//-------------------------------------------------------------------------------//

func (d *IpldWrapper) PutIPLD(ctx context.Context, n datamodel.Node) (cid.Cid, error) {
	lnk, err := d.LinkSystem.Store(
		linking.LinkContext{Ctx: ctx},
		cidlink.LinkPrototype{Prefix: *d.Prefix},
		n,
	)

	if err != nil {
		return cid.Undef, err
	}
	return lnk.(cidlink.Link).Cid, nil
}

func (d *IpldWrapper) PutIPLDAny(ctx context.Context, data any) (cid.Cid, error) {
	node, err := AnyToNode(data)
	if err != nil {
		return cid.Undef, err
	}
	return d.PutIPLD(ctx, node)
}

func (d *IpldWrapper) GetIPLDWith(ctx context.Context, c cid.Cid, proto datamodel.NodePrototype) (datamodel.Node, error) {
	return d.LinkSystem.Load(
		linking.LinkContext{Ctx: ctx},
		cidlink.Link{Cid: c},
		proto,
	)
}

func (d *IpldWrapper) GetIPLD(ctx context.Context, c cid.Cid) (datamodel.Node, error) {
	return d.GetIPLDWith(ctx, c, basicnode.Prototype.Any)
}

func (d *IpldWrapper) GetIPLDAny(ctx context.Context, c cid.Cid) (any, error) {
	node, err := d.GetIPLD(ctx, c)
	if err != nil {
		return nil, err
	}
	return NodeToAny(node)
}

func (d *IpldWrapper) ResolvePath(ctx context.Context, root cid.Cid, path string) (datamodel.Node, cid.Cid, error) {
	start, err := d.GetIPLD(ctx, root)
	if err != nil {
		return nil, cid.Undef, fmt.Errorf("load root %s: %w", root, err)
	}

	ipath := datamodel.ParsePath(path)

	prog := traversal.Progress{
		Cfg: &traversal.Config{
			LinkSystem: d.LinkSystem,
			LinkTargetNodePrototypeChooser: func(lnk datamodel.Link, lc linking.LinkContext) (datamodel.NodePrototype, error) {
				return basicnode.Prototype.Any, nil
			},
		},
	}

	var out datamodel.Node
	outCID := root
	if err := prog.Focus(start, ipath, func(p traversal.Progress, n datamodel.Node) error {
		out = n
		if lb := p.LastBlock; lb.Link != nil {
			if cl, ok := lb.Link.(cidlink.Link); ok {
				outCID = cl.Cid
			}
		}
		return nil
	}); err != nil {
		return nil, cid.Undef, fmt.Errorf("resolve %s with path %q: %w", root, path, err)
	}
	if out == nil {
		return nil, cid.Undef, fmt.Errorf("path %q not found from %s", path, root)
	}
	return out, outCID, nil
}
