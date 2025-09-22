package traversalselector

import (
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/traversal"
)

type VisitRecord struct {
	Node datamodel.Node
	Cid  cid.Cid
}

type AdvVisitRecord struct {
	Node   datamodel.Node
	Cid    cid.Cid
	Reason traversal.VisitReason
}

func resolvedFromProgress(p traversal.Progress, root cid.Cid) cid.Cid {
	c := root
	if p.LastBlock.Link != nil {
		if cl, ok := p.LastBlock.Link.(cidlink.Link); ok {
			c = cl.Cid
		}
	}
	return c
}

type VisitStream struct{ C chan VisitRecord }

func (s *VisitStream) Close() { close(s.C) }

func NewVisitStream(root cid.Cid, buf int) (traversal.VisitFn, *VisitStream) {
	stream := &VisitStream{C: make(chan VisitRecord, buf)}
	visit := func(p traversal.Progress, n datamodel.Node) error {
		rec := VisitRecord{Node: n, Cid: resolvedFromProgress(p, root)}
		select {
		case stream.C <- rec:
			return nil
		default:
			stream.C <- rec
			return nil
		}
	}
	return visit, stream
}

type VisitCollector struct {
	Records []VisitRecord
}

func NewVisitAll(root cid.Cid) (traversal.VisitFn, *VisitCollector) {
	col := &VisitCollector{Records: make([]VisitRecord, 0)}
	visit := func(p traversal.Progress, n datamodel.Node) error {
		col.Records = append(col.Records, VisitRecord{Node: n, Cid: resolvedFromProgress(p, root)})
		return nil
	}
	return visit, col
}

type VisitOne struct {
	Found bool
	Rec   VisitRecord
}

func NewVisitOne(root cid.Cid) (traversal.VisitFn, *VisitOne) {
	state := &VisitOne{}
	visit := func(p traversal.Progress, n datamodel.Node) error {
		if !state.Found {
			state.Rec = VisitRecord{Node: n, Cid: resolvedFromProgress(p, root)}
			state.Found = true
		}
		return nil
	}
	return visit, state
}

type TransformCollector struct {
	Records []VisitRecord
}

func NewTransformAll(
	root cid.Cid,
	replacer func(p traversal.Progress, n datamodel.Node) (datamodel.Node, error),
) (traversal.TransformFn, *TransformCollector) {
	if replacer == nil {
		replacer = func(_ traversal.Progress, n datamodel.Node) (datamodel.Node, error) { return n, nil }
	}
	col := &TransformCollector{Records: make([]VisitRecord, 0)}
	fn := func(p traversal.Progress, n datamodel.Node) (datamodel.Node, error) {
		col.Records = append(col.Records, VisitRecord{Node: n, Cid: resolvedFromProgress(p, root)})
		return replacer(p, n)
	}
	return fn, col
}

type TransformStream struct{ C chan VisitRecord }

func (s *TransformStream) Close() { close(s.C) }

func NewTransformStream(
	root cid.Cid,
	buf int,
	replacer func(p traversal.Progress, n datamodel.Node) (datamodel.Node, error),
) (traversal.TransformFn, *TransformStream) {
	if replacer == nil {
		replacer = func(_ traversal.Progress, n datamodel.Node) (datamodel.Node, error) { return n, nil }
	}
	stream := &TransformStream{C: make(chan VisitRecord, buf)}
	fn := func(p traversal.Progress, n datamodel.Node) (datamodel.Node, error) {
		rec := VisitRecord{Node: n, Cid: resolvedFromProgress(p, root)}
		select {
		case stream.C <- rec:
		default:
			stream.C <- rec
		}
		return replacer(p, n)
	}
	return fn, stream
}

type AdvVisitOne struct {
	Found bool
	Rec   AdvVisitRecord
}

func NewAdvVisitOne(root cid.Cid) (traversal.AdvVisitFn, *AdvVisitOne) {
	state := &AdvVisitOne{}
	fn := func(p traversal.Progress, n datamodel.Node, r traversal.VisitReason) error {
		if !state.Found {
			state.Rec = AdvVisitRecord{
				Node:   n,
				Cid:    resolvedFromProgress(p, root),
				Reason: r,
			}
			state.Found = true
		}
		return nil
	}
	return fn, state
}

type AdvVisitCollector struct {
	Records []AdvVisitRecord
}

func NewAdvVisitAll(root cid.Cid) (traversal.AdvVisitFn, *AdvVisitCollector) {
	col := &AdvVisitCollector{Records: make([]AdvVisitRecord, 0)}
	fn := func(p traversal.Progress, n datamodel.Node, r traversal.VisitReason) error {
		col.Records = append(col.Records, AdvVisitRecord{
			Node:   n,
			Cid:    resolvedFromProgress(p, root),
			Reason: r,
		})
		return nil
	}
	return fn, col
}

type AdvVisitStream struct{ C chan AdvVisitRecord }

func (s *AdvVisitStream) Close() { close(s.C) }

func NewAdvVisitStream(root cid.Cid, buf int) (traversal.AdvVisitFn, *AdvVisitStream) {
	stream := &AdvVisitStream{C: make(chan AdvVisitRecord, buf)}
	fn := func(p traversal.Progress, n datamodel.Node, r traversal.VisitReason) error {
		rec := AdvVisitRecord{
			Node:   n,
			Cid:    resolvedFromProgress(p, root),
			Reason: r,
		}
		select {
		case stream.C <- rec:
		default:
			stream.C <- rec
		}
		return nil
	}
	return fn, stream
}
