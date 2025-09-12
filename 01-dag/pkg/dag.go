package dag

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	block "github.com/gosunuts/boxo-starter-kit/00-block-cid/pkg"
	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	mc "github.com/multiformats/go-multicodec"

	"github.com/ipfs/go-cid"
)

type DagWrapper struct {
	blocks *block.BlockWrapper
	prefix *cid.Prefix

	proto datamodel.NodePrototype
	enc   codec.Encoder
	dec   codec.Decoder
}

func New(prefix *cid.Prefix) *DagWrapper {
	if prefix == nil {
		// default to v1, cbor, sha2-256
		prefix = block.NewV1Prefix(mc.DagCbor, 0, 0)
	}

	enc, dec, err := GetEncodeFuncs(prefix.Codec)
	if err != nil {
		panic(err)
	}

	return &DagWrapper{
		blocks: block.NewInMemory(),
		prefix: prefix,
		proto:  basicnode.Prototype.Any,
		enc:    enc,
		dec:    dec,
	}
}

func (d *DagWrapper) Put(ctx context.Context, node datamodel.Node) (cid.Cid, error) {
	var buf bytes.Buffer
	if err := d.enc(node, &buf); err != nil {
		return cid.Undef, err
	}
	return d.blocks.PutV1Cid(ctx, buf.Bytes(), d.prefix)
}

func (d *DagWrapper) PutAny(ctx context.Context, data any) (cid.Cid, error) {
	node, err := AnyToNode(data)
	if err != nil {
		return cid.Undef, err
	}
	return d.Put(ctx, node)
}

func (d *DagWrapper) Get(ctx context.Context, c cid.Cid) (datamodel.Node, error) {
	b, err := d.blocks.GetRaw(ctx, c)
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
	node, err := d.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return NodeToAny(node)
}

// func (d *DagWrapper) Delete(ctx context.Context, c cid.Cid) (bool, error) {
// 	return d.Delete(ctx, c)
// }

func (d *DagWrapper) ResolvePath(ctx context.Context, root cid.Cid, path string) (datamodel.Node, cid.Cid, error) {
	cur, err := d.Get(ctx, root)
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
			cur, err = d.Get(ctx, cl.Cid)
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
