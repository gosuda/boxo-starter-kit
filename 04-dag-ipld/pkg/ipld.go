package dag

import (
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/storage/bsadapter"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	bitswap "github.com/gosuda/boxo-starter-kit/03-bitswap-blockservice/pkg"
)

var _ format.DAGService = (*IpldWrapper)(nil)

type IpldWrapper struct {
	*DagServiceWrapper
	Prefix     *cid.Prefix
	linkSystem linking.LinkSystem
}

func NewIpldWrapper(prefix *cid.Prefix, blockserviceWrapper *bitswap.BlockServiceWrapper) (*IpldWrapper, error) {
	var err error
	if prefix == nil {
		prefix = block.NewV1Prefix(mc.DagCbor, 0, 0)
	}
	if blockserviceWrapper == nil {
		blockserviceWrapper, err = bitswap.NewBlockService(nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create BlockService wrapper: %w", err)
		}
	}

	dagServiceWrapper, err := NewDagServiceWrapper(nil, blockserviceWrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to create DAGService wrapper: %w", err)
	}

	ad := &bsadapter.Adapter{
		Wrapped: blockserviceWrapper.Blockstore(),
	}
	linkSystem := cidlink.DefaultLinkSystem()
	linkSystem.SetReadStorage(ad)
	linkSystem.SetWriteStorage(ad)

	return &IpldWrapper{
		DagServiceWrapper: dagServiceWrapper,
		Prefix:            prefix,
		linkSystem:        linkSystem,
	}, nil
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
