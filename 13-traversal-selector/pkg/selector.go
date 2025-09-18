package traversalselector

import (
	"github.com/ipld/go-ipld-prime/datamodel"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	sb "github.com/ipld/go-ipld-prime/traversal/selector/builder"
)

func newSSB() sb.SelectorSpecBuilder {
	return sb.NewSelectorSpecBuilder(basicnode.Prototype.Any)
}

func SelectorOne() (selector.Selector, error) {
	ssb := newSSB()
	spec := ssb.Matcher()
	return selector.CompileSelector(spec.Node())
}

func SelectorAll(match bool) (selector.Selector, error) {
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
	return selector.CompileSelector(spec.Node())
}

func SelectorDepth(limit int64, match bool) (selector.Selector, error) {
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
	return selector.CompileSelector(spec.Node())
}

func SelectorField(key string) (selector.Selector, error) {
	ssb := newSSB()
	spec := ssb.ExploreFields(func(ef sb.ExploreFieldsSpecBuilder) {
		ef.Insert(key, ssb.Matcher())
	})
	return selector.CompileSelector(spec.Node())
}

func SelectorIndex(i int64) (selector.Selector, error) {
	ssb := newSSB()
	spec := ssb.ExploreIndex(i, ssb.Matcher())
	return selector.CompileSelector(spec.Node())
}

func SelectorPath(path datamodel.Path) (selector.Selector, error) {
	ssb := newSSB()
	if path.Len() == 0 {
		return SelectorOne()
	}

	segs := path.Segments()
	var currentSpec sb.SelectorSpec = ssb.Matcher()
	for i := len(segs) - 1; i >= 0; i-- {
		seg := segs[i]
		if idx, err := seg.Index(); err == nil {
			currentSpec = ssb.ExploreIndex(idx, currentSpec)
		} else {
			key := seg.String()
			currentSpec = ssb.ExploreFields(func(ef sb.ExploreFieldsSpecBuilder) {
				ef.Insert(key, currentSpec)
			})
		}
	}

	return selector.CompileSelector(currentSpec.Node())
}

func SelectorInterpretAs(as string, next sb.SelectorSpec) (selector.Selector, error) {
	ssb := newSSB()
	spec := ssb.ExploreInterpretAs(as, next)
	return selector.CompileSelector(spec.Node())
}
