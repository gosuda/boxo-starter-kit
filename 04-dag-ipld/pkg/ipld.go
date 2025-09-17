package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"

	bitswap "github.com/gosuda/boxo-starter-kit/03-bitswap/pkg"
)

var _ format.DAGService = (*IpldWrapper)(nil)

type IpldWrapper struct {
	*DagServiceWrapper
}

func NewIpldWrapper(prefix *cid.Prefix, blockserviceWrapper *bitswap.BlockServiceWrapper) (*IpldWrapper, error) {
	var err error
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

	return &IpldWrapper{
		DagServiceWrapper: dagServiceWrapper,
	}, nil
}

// ------------------------------------------------------------------
// go-ipld-format utils
// ------------------------------------------------------------------

func (d *IpldWrapper) PutNode(ctx context.Context, n format.Node) (cid.Cid, error) {
	if err := d.DagServiceWrapper.Add(ctx, n); err != nil {
		return cid.Undef, err
	}
	return n.Cid(), nil
}

func (d *IpldWrapper) PutAny(ctx context.Context, v any) (cid.Cid, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return cid.Undef, err
	}

	node := merkledag.NewRawNode(data)
	return d.PutNode(ctx, node)
}

func (d *IpldWrapper) GetNode(ctx context.Context, c cid.Cid) (format.Node, error) {
	return d.DagServiceWrapper.Get(ctx, c)
}

func (d *IpldWrapper) GetAny(ctx context.Context, c cid.Cid, v any) error {
	n, err := d.DagServiceWrapper.Get(ctx, c)
	if err != nil {
		return err
	}
	return json.Unmarshal(n.RawData(), v)
}

func (d *IpldWrapper) ResolvePath(ctx context.Context, root cid.Cid, path string) (format.Node, cid.Cid, error) {
	cur, err := d.GetNode(ctx, root)
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
		nextCID, err := findChildCID(cur, p)
		if err != nil {
			return nil, cid.Undef, fmt.Errorf("path %q: %w", p, err)
		}

		nextNode, err := d.GetNode(ctx, nextCID)
		if err != nil {
			return nil, cid.Undef, err
		}

		cur = nextNode
		curCID = nextCID

		if i == len(parts)-1 {
			return cur, curCID, nil
		}
	}
	return cur, curCID, nil
}

func findChildCID(n format.Node, seg string) (cid.Cid, error) {
	links := n.Links()
	if idx, err := strconv.Atoi(seg); err == nil {
		if idx < 0 || idx >= len(links) {
			return cid.Undef, fmt.Errorf("index %d out of range (%d links)", idx, len(links))
		}
		return links[idx].Cid, nil
	}
	for _, l := range links {
		if l.Name == seg {
			return l.Cid, nil
		}
	}
	return cid.Undef, fmt.Errorf("link %q not found", seg)
}
