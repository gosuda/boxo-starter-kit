package traversalselector

import (
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	sb "github.com/ipld/go-ipld-prime/traversal/selector/builder"
)

func newSSB() sb.SelectorSpecBuilder {
	return sb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
}

func CompileSelector(node ipld.Node) (selector.Selector, error) {
	return selector.CompileSelector(node)
}

func SelectorOne() ipld.Node {
	ssb := newSSB()
	spec := ssb.Matcher()
	return spec.Node()
}

func SelectorAll(match bool) ipld.Node {
	ssb := newSSB()

	var explore sb.SelectorSpec
	if match {
		explore = ssb.ExploreUnion(
			ssb.Matcher(),
			ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
		)
	} else {
		explore = ssb.ExploreAll(ssb.ExploreRecursiveEdge())
	}

	spec := ssb.ExploreRecursive(
		selector.RecursionLimitNone(),
		explore,
	)
	return spec.Node()
}

func SelectorDepth(limit int64, match bool) ipld.Node {
	ssb := newSSB()

	var explore sb.SelectorSpec
	if match {
		explore = ssb.ExploreUnion(
			ssb.Matcher(),
			ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
		)
	} else {
		explore = ssb.ExploreAll(ssb.ExploreRecursiveEdge())
	}

	spec := ssb.ExploreRecursive(
		selector.RecursionLimitDepth(limit),
		explore,
	)
	return spec.Node()
}

func SelectorField(key string) ipld.Node {
	ssb := newSSB()
	spec := ssb.ExploreFields(func(ef sb.ExploreFieldsSpecBuilder) {
		ef.Insert(key, ssb.Matcher())
	})
	return spec.Node()
}

func SelectorIndex(i int64) ipld.Node {
	ssb := newSSB()
	spec := ssb.ExploreIndex(i, ssb.Matcher())
	return spec.Node()
}

func SelectorPath(path datamodel.Path) ipld.Node {
	ssb := newSSB()
	if path.Len() == 0 {
		return SelectorOne()
	}

	segs := path.Segments()
	var spec sb.SelectorSpec = ssb.Matcher()
	for i := len(segs) - 1; i >= 0; i-- {
		seg := segs[i]
		if idx, err := seg.Index(); err == nil {
			spec = ssb.ExploreIndex(idx, spec)
		} else {
			key := seg.String()
			spec = ssb.ExploreFields(func(ef sb.ExploreFieldsSpecBuilder) {
				ef.Insert(key, spec)
			})
		}
	}

	return spec.Node()
}

func SelectorInterpretAs(as string, next sb.SelectorSpec) ipld.Node {
	ssb := newSSB()
	spec := ssb.ExploreInterpretAs(as, next)
	return spec.Node()
}
