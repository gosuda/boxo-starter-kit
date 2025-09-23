package ipni

import (
	"context"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipni/go-indexer-core"
	"github.com/ipni/go-libipni/find/model"
	"github.com/libp2p/go-libp2p/core/peer"
)

// -----------------------------
// Provider model
// -----------------------------

type TransportKind string

const (
	TLocal     TransportKind = "local"
	THTTP      TransportKind = "http"
	TGraphSync TransportKind = "graphsync"
	TBitswap   TransportKind = "bitswap"
)

type Transport struct {
	Kind       TransportKind
	PartialCAR bool              // http partial CAR(IPIP-402) support
	Auth       bool              // http auth support
	Meta       map[string]string // free-form
}

type ScoringPolicy struct {
	TransportBase  map[TransportKind]float64 // prior per transport
	RegionBonus    float64                   // bonus on region match
	PartialBonus   float64                   // bonus if HTTP supports partial CAR and intent wants it
	LocalBias      float64                   // small bias for local
	DefaultStagger time.Duration             // stagger for racing
}

func (ScoringPolicy) Defaults() ScoringPolicy {
	return ScoringPolicy{
		TransportBase: map[TransportKind]float64{
			TLocal:     0.9,
			THTTP:      0.7,
			TGraphSync: 0.6,
			TBitswap:   0.4,
		},
		RegionBonus:    0.2,
		PartialBonus:   0.5,
		LocalBias:      0.6,
		DefaultStagger: 150 * time.Millisecond,
	}
}

type Prefs struct {
	PreferredTransports []TransportKind           // earlier gets slightly higher bonus
	PerRequestBase      map[TransportKind]float64 // override TransportBase per request
}

// -----------------------------
// Planner inputs (intent)
// -----------------------------

type CachePolicy struct {
	OnlyIfCached bool
	Revalidate   bool
	ProviderTTL  time.Duration // default 30~120s
}

type RouteIntent struct {
	HTTPURL       string
	PeerID        string
	LocalOnly     bool
	BitswapOnly   bool
	GraphSyncOnly bool
	Providers     []string // allowlist

	Root           cid.Cid
	Format         string // "car" | "raw" | "unixfs"
	Scope          string // "block" | "entity" | "all"
	SelCBOR        []byte
	Offset, Length int64

	Deadline     time.Duration
	PreferRegion string
	CachePolicy  CachePolicy
	Prefs        *Prefs
}

type ProviderInfoGetter func(ctx context.Context, pid peer.ID) (*model.ProviderInfo, error)

type ProviderView struct {
	ID         string
	Info       *model.ProviderInfo
	Transports []Transport
	Meta       map[string]string
}

type Providers struct {
	Items  []ProviderView
	Source string // e.g. "local-engine"
}

type Attempt struct {
	ProviderID string
	Proto      TransportKind
	Stagger    time.Duration
	Weight     float64
	Meta       map[string]string
}

type Plan struct {
	Attempts []Attempt
	Policy   struct {
		Parallel          bool
		CancelOnFirstWin  bool
		DefaultStagger    time.Duration
		ObservedProviders int
		Source            string
	}
}

// -----------------------------
// Planner
// -----------------------------

type Planner struct {
	Policy ScoringPolicy
}

func NewPlanner(policy *ScoringPolicy) *Planner {
	p := ScoringPolicy{}.Defaults()
	if policy != nil {
		p = *policy
	}
	return &Planner{Policy: p}
}

// Plan ranks candidates and emits a transport-agnostic attempt list.
// No health scoring.
func (p *Planner) Plan(ctx context.Context, provs Providers, in RouteIntent) Plan {
	items := provs.Items
	wantPartialHTTP := (strings.ToLower(in.Format) == "car" && strings.ToLower(in.Scope) != "block")

	// Build per-request bases and preference nudges
	base := make(map[TransportKind]float64, len(p.Policy.TransportBase))
	for k, v := range p.Policy.TransportBase {
		base[k] = v
	}
	if in.Prefs != nil && in.Prefs.PerRequestBase != nil {
		for k, v := range in.Prefs.PerRequestBase {
			base[k] = v
		}
	}
	prefBonus := map[TransportKind]float64{}
	if in.Prefs != nil && len(in.Prefs.PreferredTransports) > 0 {
		step := 0.05
		score := 0.15
		for _, tk := range in.Prefs.PreferredTransports {
			prefBonus[tk] = score
			score -= step
			if score < 0 {
				break
			}
		}
	}

	regionOf := func(pv ProviderView) string {
		if pv.Info == nil {
			return ""
		}
		return ""
	}

	scoreOf := func(pv ProviderView, tk TransportKind) float64 {
		s := base[tk]

		if in.PreferRegion != "" && regionOf(pv) == in.PreferRegion {
			s += p.Policy.RegionBonus
		}
		if tk == THTTP && wantPartialHTTP && providerSupportsPartial(pv) {
			s += p.Policy.PartialBonus
		}
		if tk == TLocal {
			s += p.Policy.LocalBias
		}
		if b, ok := prefBonus[tk]; ok {
			s += b
		}
		return s
	}

	// Flatten to per-transport candidates with weights
	type cand struct {
		id   string
		tk   TransportKind
		wt   float64
		part bool
	}
	var cs []cand
	for _, pv := range items {
		for _, t := range pv.Transports {
			cs = append(cs, cand{
				id:   pv.ID,
				tk:   t.Kind,
				wt:   scoreOf(pv, t.Kind),
				part: (t.Kind == THTTP && t.PartialCAR),
			})
		}
	}

	// Order by weight desc (stable)
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].wt > cs[j].wt })

	// Emit attempts with simple stagger
	pl := Plan{}
	pl.Policy.Parallel = true
	pl.Policy.CancelOnFirstWin = true
	pl.Policy.DefaultStagger = p.Policy.DefaultStagger
	pl.Policy.ObservedProviders = len(items)
	pl.Policy.Source = provs.Source

	for i, c := range cs {
		meta := map[string]string{}
		if c.tk == THTTP && c.part {
			meta["partial_car"] = "true"
		}
		pl.Attempts = append(pl.Attempts, Attempt{
			ProviderID: c.id,
			Proto:      c.tk,
			Stagger:    pl.Policy.DefaultStagger * time.Duration(i),
			Weight:     c.wt,
			Meta:       meta,
		})
	}
	return pl
}

func providerSupportsPartial(pv ProviderView) bool {
	for _, t := range pv.Transports {
		if t.Kind == THTTP && t.PartialCAR {
			return true
		}
	}
	return false
}
func NormalizeFromEngine(ctx context.Context, vals []indexer.Value, get ProviderInfoGetter) Providers {
	out := make([]ProviderView, 0, len(vals))
	for _, v := range vals {
		pv := ProviderView{
			ID:         v.ProviderID.String(),
			Info:       nil,
			Transports: []Transport{},
			Meta:       map[string]string{},
		}
		if get != nil {
			if pi, err := get(ctx, v.ProviderID); err == nil {
				pv.Info = pi
			}
		}
		if len(v.ContextID) > 0 {
			pv.Meta["context_id"] = hex.EncodeToString(v.ContextID)
		}
		if len(v.MetadataBytes) > 0 {
			pv.Meta["metadata_hex"] = hex.EncodeToString(v.MetadataBytes)
			pv.Transports = append(pv.Transports, Transport{Kind: TGraphSync})
		} else {
			pv.Transports = append(pv.Transports, Transport{Kind: TBitswap})
		}
		out = append(out, pv)
	}
	return Providers{Items: out, Source: "engine"}
}
