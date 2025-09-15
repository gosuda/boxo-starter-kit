package dag

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	mc "github.com/multiformats/go-multicodec"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
)

var _ format.DAGService = (*DagWrapper)(nil)

type DagWrapper struct {
	*persistent.PersistentWrapper
	prefix *cid.Prefix

	proto datamodel.NodePrototype
	enc   codec.Encoder
	dec   codec.Decoder
}

func New(prefix *cid.Prefix, pType persistent.PersistentType) (*DagWrapper, error) {
	if prefix == nil {
		// default to v1, cbor, sha2-256
		prefix = block.NewV1Prefix(mc.DagCbor, 0, 0)
	}
	if pType == "" {
		pType = persistent.Memory
	}

	enc, dec, err := GetEncodeFuncs(prefix.Codec)
	if err != nil {
		return nil, err
	}
	blocks, err := persistent.New(pType, "")
	if err != nil {
		return nil, err
	}

	return &DagWrapper{
		PersistentWrapper: blocks,
		prefix:            prefix,
		proto:             basicnode.Prototype.Any,
		enc:               enc,
		dec:               dec,
	}, nil
}

// format.NodeGetter
func (d *DagWrapper) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	blk, err := d.PersistentWrapper.Get(ctx, c)
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

func (d *DagWrapper) GetMany(ctx context.Context, cs []cid.Cid) <-chan *format.NodeOption {
	out := make(chan *format.NodeOption, len(cs))
	go func() {
		defer close(out)
		for _, c := range cs {
			nd, err := d.Get(ctx, c)
			out <- &format.NodeOption{Node: nd, Err: err}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	return out
}

func (d *DagWrapper) Add(ctx context.Context, n format.Node) error {
	c := n.Cid()
	data := n.RawData()
	if err := d.PersistentWrapper.PutWithCID(ctx, data, c); err != nil {
		return err
	}
	return nil
}

func (d *DagWrapper) AddMany(ctx context.Context, nodes []format.Node) error {
	for _, n := range nodes {
		if err := d.Add(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

func (d *DagWrapper) Remove(ctx context.Context, c cid.Cid) error {
	_ = d.PersistentWrapper.Delete(ctx, c)
	return nil
}

func (d *DagWrapper) RemoveMany(ctx context.Context, cs []cid.Cid) error {
	for _, c := range cs {
		_ = d.PersistentWrapper.Delete(ctx, c)
	}
	return nil
}

//-------------------------------------------------------------------------------//
// IPLD-prime util methods
//-------------------------------------------------------------------------------//

func (d *DagWrapper) PutIPLD(ctx context.Context, node datamodel.Node) (cid.Cid, error) {
	var buf bytes.Buffer
	if err := d.enc(node, &buf); err != nil {
		return cid.Undef, err
	}
	return d.PersistentWrapper.PutV1Cid(ctx, buf.Bytes(), d.prefix)
}

func (d *DagWrapper) PutAny(ctx context.Context, data any) (cid.Cid, error) {
	node, err := AnyToNode(data)
	if err != nil {
		return cid.Undef, err
	}
	return d.PutIPLD(ctx, node)
}

func (d *DagWrapper) GetIPLD(ctx context.Context, c cid.Cid) (datamodel.Node, error) {
	b, err := d.PersistentWrapper.GetRaw(ctx, c)
	if err != nil {
		return nil, err
	}
	nb := d.proto.NewBuilder()
	if err := d.dec(nb, bytes.NewReader(b)); err != nil {
		return nil, err
	}
	return nb.Build(), nil
}

func (d *DagWrapper) GetAny(ctx context.Context, c cid.Cid) (any, error) {
	node, err := d.GetIPLD(ctx, c)
	if err != nil {
		return nil, err
	}
	return NodeToAny(node)
}

func (d *DagWrapper) Delete(ctx context.Context, c cid.Cid) error {
	return d.PersistentWrapper.Delete(ctx, c)
}

func (d *DagWrapper) ResolvePath(ctx context.Context, root cid.Cid, path string) (datamodel.Node, cid.Cid, error) {
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
