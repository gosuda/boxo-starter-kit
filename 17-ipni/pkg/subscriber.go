package ipni

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	dagsync "github.com/ipni/go-libipni/dagsync"
	"github.com/ipni/go-libipni/find/model"
	"github.com/ipni/go-libipni/ingest/schema"
	md "github.com/ipni/go-libipni/metadata"
	"github.com/ipni/go-libipni/pcache"
	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"
	"github.com/rs/zerolog/log"

	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
)

func MakeTopic(topic string) string {
	return "/indexer/ingest/" + topic
}

type SubscriberWrapper struct {
	*dagsync.Subscriber
	pcache *pcache.ProviderCache
	lsys   ipld.LinkSystem

	cancel context.CancelFunc
}

func NewSubscriberWrapper(hostWrapper *network.HostWrapper, ipldWrapper *ipldprime.IpldWrapper, providerWrapper *ProviderWrapper, sourceUrl ...string) (*SubscriberWrapper, error) {
	if len(sourceUrl) == 0 {
		sourceUrl = append(sourceUrl, "https://cid.contact")
	}

	subscriber, err := dagsync.NewSubscriber(hostWrapper.Host, ipldWrapper.LinkSystem)
	if err != nil {
		return nil, err
	}

	pc, err := pcache.New(
		pcache.WithSource(providerWrapper),
		pcache.WithSourceURL(sourceUrl...),
	)
	if err != nil {
		return nil, err

	}

	return &SubscriberWrapper{
		Subscriber: subscriber,
		pcache:     pc,
		lsys:       ipldWrapper.LinkSystem,
	}, nil
}

type OnPutFn func(providerID peer.ID, contextID []byte, metadataBytes []byte, mhs []mh.Multihash) error
type OnRemoveFn func(providerID peer.ID, contextID []byte) error

func (s *SubscriberWrapper) Start(ctx context.Context, onPut OnPutFn, onRemove OnRemoveFn) error {
	if onPut == nil || onRemove == nil {
		return fmt.Errorf("subscriber: onPut/onRemove must be non-nil")
	}

	ch, cancel := s.Subscriber.OnSyncFinished()
	s.cancel = cancel

	go func() {
		for ev := range ch {
			if ev.Err != nil {
				log.Error().Err(ev.Err).Msg("dagsync error")
				continue
			}

			if _, err := s.pcache.Get(ctx, ev.PeerID); err != nil {
				log.Debug().Err(err).Msg("provider cache get failed (non-fatal)")
			}

			evCopy := ev
			if err := s.handleAd(ctx, evCopy.Cid, onPut, onRemove); err != nil {
				log.Error().Err(err).Str("adCid", evCopy.Cid.String()).Msg("ingest failed")
				continue
			}
		}
	}()
	return nil
}

func (s *SubscriberWrapper) handleAd(ctx context.Context, adCid cid.Cid, onPut OnPutFn, onRemove OnRemoveFn) error {
	n, err := s.lsys.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: adCid}, schema.AdvertisementPrototype)
	if err != nil {
		return fmt.Errorf("load advertisement: %w", err)
	}
	ad, err := schema.UnwrapAdvertisement(n)
	if err != nil {
		return fmt.Errorf("unwrap advertisement: %w", err)
	}

	pid, err := peer.Decode(ad.Provider)
	if err != nil {
		return fmt.Errorf("decode provider id: %w", err)
	}

	if ad.IsRm {
		return onRemove(pid, ad.ContextID)
	}

	meta := md.Default.New()
	if len(ad.Metadata) > 0 {
		if err := meta.UnmarshalBinary(ad.Metadata); err != nil {
			return fmt.Errorf("metadata unmarshal: %w", err)
		}
	}
	mdBytes, err := meta.MarshalBinary()
	if err != nil {
		return fmt.Errorf("metadata marshal: %w", err)
	}

	mhs, err := s.collectMultihashes(ctx, ad.Entries)
	if err != nil {
		return fmt.Errorf("collect entries: %w", err)
	}
	if len(mhs) == 0 {
		return nil
	}
	return onPut(pid, ad.ContextID, mdBytes, mhs)
}

func (s *SubscriberWrapper) collectMultihashes(ctx context.Context, entries ipld.Link) ([]mh.Multihash, error) {
	if entries == nil {
		return nil, nil
	}
	lnk, ok := entries.(cidlink.Link)
	if !ok || lnk.Cid == schema.NoEntries.Cid {
		return nil, nil
	}

	out := make([]mh.Multihash, 0, 2048)
	curr := lnk
	for {
		chunkNode, err := s.lsys.Load(ipld.LinkContext{Ctx: ctx}, curr, schema.EntryChunkPrototype)
		if err != nil {
			return nil, fmt.Errorf("load entry chunk %s: %w", curr.Cid, err)
		}
		chunk, err := schema.UnwrapEntryChunk(chunkNode)
		if err != nil {
			return nil, fmt.Errorf("unwrap entry chunk: %w", err)
		}

		if len(chunk.Entries) > 0 {
			out = append(out, chunk.Entries...)
		}
		if chunk.Next == nil {
			break
		}
		nl, ok := chunk.Next.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("unexpected next link type")
		}
		curr = nl
	}
	return out, nil
}
func (s *SubscriberWrapper) Stop() error {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	return nil
}

func (s *SubscriberWrapper) ProviderInfo(ctx context.Context, pid peer.ID) (*model.ProviderInfo, error) {
	return s.pcache.Get(ctx, pid)
}
