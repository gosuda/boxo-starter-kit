package ipni

import (
	"sort"
	"strings"
	"time"

	"github.com/ipni/go-indexer-core"
)

type Attempt struct {
	ProviderID string
	Proto      TransportKind
	Weight     float64
	Stagger    time.Duration
	Meta       map[string]string
}

type Intent struct {
	Providers     []string
	LocalOnly     bool
	BitswapOnly   bool
	GraphSyncOnly bool

	Format string // "car" | "raw" | ...
	Scope  string // "block" | "entity" | "all"

	Preferred []TransportKind
}

const (
	defaultStagger  = 150 * time.Millisecond
	partialCarBonus = 0.5
	prefTopBonus    = 0.15
	prefStep        = 0.05
)

var baseWeight = map[TransportKind]float64{
	THTTP:      0.7,
	TGraphSync: 0.6,
	TBitswap:   0.4,
	TLocal:     0.9,
}

type GetMeta func(indexer.Value) map[string]string

func Plan(vals []indexer.Value, in Intent, getMeta GetMeta) []Attempt {
	wantPartial := strings.EqualFold(in.Format, "car") && !strings.EqualFold(in.Scope, "block")

	prefBonus := map[TransportKind]float64{}
	if n := len(in.Preferred); n > 0 {
		score := prefTopBonus
		for _, tk := range in.Preferred {
			if score <= 0 {
				break
			}
			prefBonus[tk] = score
			score -= prefStep
		}
	}

	type cand struct {
		id   string
		tk   TransportKind
		wt   float64
		meta map[string]string
	}
	cs := make([]cand, 0, len(vals))

	for _, v := range vals {
		pid := v.ProviderID.String()
		if len(in.Providers) > 0 && !contains(in.Providers, pid) {
			continue
		}

		tk := ExportTransportKind(v)
		if in.LocalOnly && tk != TLocal {
			continue
		}
		if in.BitswapOnly && tk != TBitswap {
			continue
		}
		if in.GraphSyncOnly && tk != TGraphSync {
			continue
		}

		meta := map[string]string{}
		if getMeta != nil {
			if m := getMeta(v); m != nil {
				meta = m
			}
		}

		wt := baseWeight[tk]
		if tk == THTTP && wantPartial && strings.EqualFold(meta["partial_car"], "true") {
			wt += partialCarBonus
		}
		if b, ok := prefBonus[tk]; ok {
			wt += b
		}

		cs = append(cs, cand{id: pid, tk: tk, wt: wt, meta: meta})
	}

	sort.SliceStable(cs, func(i, j int) bool { return cs[i].wt > cs[j].wt })

	out := make([]Attempt, 0, len(cs))
	for i, c := range cs {
		out = append(out, Attempt{
			ProviderID: c.id,
			Proto:      c.tk,
			Weight:     c.wt,
			Stagger:    defaultStagger * time.Duration(i),
			Meta:       c.meta,
		})
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}
