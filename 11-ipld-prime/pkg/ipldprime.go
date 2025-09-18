package ipldprime

import (
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/storage/bsadapter"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

type IpldWrapper struct {
	Prefix     *cid.Prefix
	linkSystem linking.LinkSystem
}

func New(prefix *cid.Prefix, linkSystem *linking.LinkSystem) (*IpldWrapper, error) {
	if linkSystem == nil {
		return nil, fmt.Errorf("linkSystem is required")
	}
	return &IpldWrapper{
		Prefix:     prefix,
		linkSystem: *linkSystem,
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
	lnk, err := d.linkSystem.Store(
		linking.LinkContext{Ctx: ctx},
		cidlink.LinkPrototype{Prefix: *d.Prefix},
		n,
	)

	if err != nil {
		return cid.Undef, err
	}
	return lnk.(cidlink.Link).Cid, nil
}

func (d *IpldWrapper) PutAny(ctx context.Context, data any) (cid.Cid, error) {
	node, err := AnyToNode(data)
	if err != nil {
		return cid.Undef, err
	}
	return d.PutIPLD(ctx, node)
}

func (d *IpldWrapper) GetIPLD(ctx context.Context, c cid.Cid) (datamodel.Node, error) {
	n, err := d.linkSystem.Load(
		linking.LinkContext{Ctx: ctx},
		cidlink.Link{Cid: c},
		basicnode.Prototype.Any,
	)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (d *IpldWrapper) GetAny(ctx context.Context, c cid.Cid) (any, error) {
	node, err := d.GetIPLD(ctx, c)
	if err != nil {
		return nil, err
	}
	return NodeToAny(node)
}

func (d *IpldWrapper) ResolvePath(ctx context.Context, root cid.Cid, path string) (datamodel.Node, cid.Cid, error) {
	cur, err := d.GetIPLD(ctx, root)
	if err != nil {
		return nil, cid.Undef, err
	}
	curCID := root

	seg := strings.Trim(path, "/")
	if seg == "" {
		return cur, curCID, nil
	}
	parts := strings.Split(seg, "/")

	for i, p := range parts {
		next, err := cur.LookupByString(p)
		if err != nil {
			next, err = lookupListIndex(cur, p)
			if err != nil {
				return nil, cid.Undef, fmt.Errorf("path %q: %w", p, err)
			}
		}

		if next.Kind() == datamodel.Kind_Link {
			lk, err := next.AsLink()
			if err != nil {
				return nil, cid.Undef, fmt.Errorf("invalid link at %q: %w", p, err)
			}
			cl, ok := lk.(cidlink.Link)
			if !ok {
				return nil, cid.Undef, fmt.Errorf("unsupported link at %q", p)
			}
			cur, err = d.GetIPLD(ctx, cl.Cid)
			if err != nil {
				return nil, cid.Undef, err
			}
			curCID = cl.Cid
		} else {
			cur = next
		}

		if i == len(parts)-1 {
			return cur, curCID, nil
		}
	}
	return cur, curCID, nil
}
