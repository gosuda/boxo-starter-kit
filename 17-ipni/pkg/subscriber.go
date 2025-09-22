package ipni

import (
	"context"

	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	dagsync "github.com/ipni/go-libipni/dagsync"
	"github.com/ipni/go-libipni/find/model"
	"github.com/ipni/go-libipni/pcache"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog/log"
)

func GetTopic(topic string) string {
	return "/indexer/ingest/" + topic
}

type SubscriberWrapper struct {
	*dagsync.Subscriber
	pcache *pcache.ProviderCache
	lsys   ipld.LinkSystem

	cancel context.CancelFunc
}

func NewSubscriberWrapper(hostWrapper *network.HostWrapper, ipldWrapper *ipldprime.IpldWrapper, sourceUrl ...string) (*SubscriberWrapper, error) {
	if len(sourceUrl) == 0 {
		sourceUrl = append(sourceUrl, "https://cid.contact")
	}

	subscriber, err := dagsync.NewSubscriber(hostWrapper.Host, ipldWrapper.LinkSystem)
	if err != nil {
		return nil, err
	}

	pc, err := pcache.New(
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

func (s *SubscriberWrapper) Start(fn func(ev *dagsync.SyncFinished, provider *model.ProviderInfo) error) error {
	ch, cancel := s.Subscriber.OnSyncFinished()
	s.cancel = cancel

	go func() {
		for ev := range ch {
			provider, err := s.pcache.Get(context.Background(), ev.PeerID)
			if err != nil {
				log.Error().Err(err).Msg("failed to get provider info from cache")
				continue
			}

			if err := fn(&ev, provider); err != nil {
				log.Error().Err(err).Msg("dagsync sync finished handler error")
				continue // stop on error
			}
		}
	}()

	return nil
}

func (s *SubscriberWrapper) Stop() error {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	return nil
}

func (s *SubscriberWrapper) Announce(ctx context.Context, next cid.Cid, ai peer.AddrInfo) error {
	return s.Subscriber.Announce(ctx, next, ai)
}

func (s *SubscriberWrapper) ProviderInfo(ctx context.Context, pid peer.ID) (*model.ProviderInfo, error) {
	return s.pcache.Get(ctx, pid)
}
