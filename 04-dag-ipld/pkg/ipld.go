package dag

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/boxo/ipld/merkledag"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	bitswap "github.com/gosuda/boxo-starter-kit/03-bitswap-blockservice/pkg"
)

var _ format.DAGService = (*IpldWrapper)(nil)

type IpldWrapper struct {
	*bitswap.BlockServiceWrapper
	prefix *cid.Prefix

	proto datamodel.NodePrototype
	enc   codec.Encoder
	dec   codec.Decoder
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

	enc, dec, err := GetEncodeFuncs(prefix.Codec)
	if err != nil {
		return nil, err
	}

	return &IpldWrapper{
		BlockServiceWrapper: blockserviceWrapper,
		prefix:              prefix,
		proto:               basicnode.Prototype.Any,
		enc:                 enc,
		dec:                 dec,
	}, nil
}

func (d *IpldWrapper) Add(ctx context.Context, n format.Node) error {
	if err := d.BlockServiceWrapper.AddBlock(ctx, n); err != nil {
		return err
	}
	return nil
}

func (d *IpldWrapper) AddMany(ctx context.Context, nodes []format.Node) error {
	blks := make([]blocks.Block, len(nodes))
	for i, nd := range nodes {
		blks[i] = nd
	}
	return d.AddBlocks(ctx, blks)
}

// format.NodeGetter
func (d *IpldWrapper) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	blk, err := d.BlockServiceWrapper.GetBlock(ctx, c)
	if err != nil {
		return nil, format.ErrNotFound{Cid: c}
	}

	switch uint64(c.Prefix().Codec) {
	case uint64(mc.DagPb):
		return merkledag.DecodeProtobufBlock(blk)
	case uint64(mc.Raw):
		return merkledag.DecodeRawBlock(blk)
		// intentionally unsupported in DAGService: use GetIPLD() for general IPLD (dag-cbor).
		// case uint64(mc.DagCbor):
	}
	return nil, fmt.Errorf("unsupported codec in DAGService.Get: %s", mc.Code(c.Prefix().Codec).String())
}

func (d *IpldWrapper) GetMany(ctx context.Context, cs []cid.Cid) <-chan *format.NodeOption {
	out := make(chan *format.NodeOption, len(cs))
	blocks := d.BlockServiceWrapper.GetBlocks(ctx, cs)
	var count int

	go func() {
		defer close(out)
		for {
			select {
			case b, ok := <-blocks:
				if !ok {
					if count != len(cs) {
						out <- &format.NodeOption{Err: errors.New("failed to fetch all nodes")}
					}
					return
				}
				var nd format.Node
				var err error
				switch uint64(b.Cid().Prefix().Codec) {
				case uint64(mc.DagPb):
					nd, err = merkledag.DecodeProtobufBlock(b)
				case uint64(mc.Raw):
					nd, err = merkledag.DecodeRawBlock(b)
					// intentionally unsupported in DAGService: use GetIPLD() for general IPLD (dag-cbor).
					// case uint64(mc.DagCbor):
				default:
					err = fmt.Errorf("unsupported codec in DAGService.GetMany: %s", mc.Code(b.Cid().Prefix().Codec).String())
				}
				if err != nil {
					out <- &format.NodeOption{Err: err}
				}

				out <- &format.NodeOption{Node: nd}
				count++

			case <-ctx.Done():
				out <- &format.NodeOption{Err: ctx.Err()}
				return
			}
		}
	}()
	return out
}

func (d *IpldWrapper) Remove(ctx context.Context, c cid.Cid) error {
	return d.BlockService.DeleteBlock(ctx, c)
}

func (d *IpldWrapper) RemoveMany(ctx context.Context, cs []cid.Cid) error {
	for _, c := range cs {
		if err := d.BlockService.DeleteBlock(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

//-------------------------------------------------------------------------------//
// IPLD-prime util methods
//-------------------------------------------------------------------------------//

func (d *IpldWrapper) PutIPLD(ctx context.Context, node datamodel.Node) (cid.Cid, error) {
	var buf bytes.Buffer
	if err := d.enc(node, &buf); err != nil {
		return cid.Undef, err
	}
	return d.BlockServiceWrapper.AddBlockRaw(ctx, buf.Bytes())
}

func (d *IpldWrapper) PutAny(ctx context.Context, data any) (cid.Cid, error) {
	node, err := AnyToNode(data)
	if err != nil {
		return cid.Undef, err
	}
	return d.PutIPLD(ctx, node)
}

func (d *IpldWrapper) GetIPLD(ctx context.Context, c cid.Cid) (datamodel.Node, error) {
	b, err := d.BlockService.GetBlock(ctx, c)
	if err != nil {
		return nil, err
	}
	nb := d.proto.NewBuilder()
	if err := d.dec(nb, bytes.NewReader(b.RawData())); err != nil {
		return nil, err
	}
	return nb.Build(), nil
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
