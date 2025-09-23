package ipni

import (
	"context"
	"fmt"
	"os"

	"github.com/ipfs/go-cid"
	"github.com/ipni/go-libipni/find/model"
	md "github.com/ipni/go-libipni/metadata"
	"github.com/ipni/go-libipni/pcache"
	provengine "github.com/ipni/index-provider/engine"
	carsupplier "github.com/ipni/index-provider/supplier"
	"github.com/libp2p/go-libp2p/core/peer"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
)

var _ pcache.ProviderSource = (*ProviderWrapper)(nil)

type ProviderWrapper struct {
	path  string
	topic string

	provider *peer.AddrInfo
	engine   *provengine.Engine

	*carsupplier.CarSupplier
}

func NewProviderWrapper(path, topic string, persistentWrapper *persistent.PersistentWrapper, hostWrapper *network.HostWrapper) (*ProviderWrapper, error) {
	if path == "" {
		path = os.TempDir()
	}
	path += "/ipni-provider"

	if topic == "" {
		topic = MakeTopic("index")
	}

	var err error
	if persistentWrapper == nil {
		persistentWrapper, err = persistent.New(persistent.Memory, "")
		if err != nil {
			return nil, fmt.Errorf("failed to create persistent wrapper: %w", err)
		}
	}
	if hostWrapper == nil {
		hostWrapper, err = network.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p host: %w", err)
		}
	}

	provider := &peer.AddrInfo{
		ID:    hostWrapper.ID(),
		Addrs: hostWrapper.Addrs(),
	}

	eng, err := provengine.New(
		provengine.WithDatastore(persistentWrapper.Batching),
		provengine.WithHost(hostWrapper.Host),
		provengine.WithTopicName(topic),
		provengine.WithPublisherKind(provengine.Libp2pHttpPublisher),
		provengine.WithPubsubAnnounce(true),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index provider: %w", err)
	}

	carSup := carsupplier.NewCarSupplier(eng, persistentWrapper.Batching)

	return &ProviderWrapper{
		path:        path,
		topic:       topic,
		provider:    provider,
		engine:      eng,
		CarSupplier: carSup,
	}, nil
}

func (p *ProviderWrapper) ProviderID() peer.ID {
	return p.provider.ID
}

func (p *ProviderWrapper) Start(ctx context.Context) error {
	err := p.engine.Start(ctx)
	if err != nil {
		return fmt.Errorf("provider engine start: %w", err)
	}
	return nil
}

func (p *ProviderWrapper) PutCAR(ctx context.Context, contextID []byte, md md.Metadata) (cid.Cid, error) {
	if p.CarSupplier == nil {
		return cid.Undef, fmt.Errorf("car supplier not initialized")
	}
	return p.CarSupplier.Put(ctx, contextID, p.path, md)
}

func (p *ProviderWrapper) RemoveCAR(ctx context.Context, contextID []byte) (cid.Cid, error) {
	if p.CarSupplier == nil {
		return cid.Undef, fmt.Errorf("car supplier not initialized")
	}
	return p.CarSupplier.Remove(ctx, contextID)
}

func (p *ProviderWrapper) Fetch(ctx context.Context, pid peer.ID) (*model.ProviderInfo, error) {
	if pid != "" && pid != p.provider.ID {
		return nil, nil
	}
	return p.buildProviderInfo(), nil
}

func (p *ProviderWrapper) FetchAll(ctx context.Context) ([]*model.ProviderInfo, error) {
	return []*model.ProviderInfo{p.buildProviderInfo()}, nil
}

func (p *ProviderWrapper) String() string { return "local-provider" }

func (p *ProviderWrapper) buildProviderInfo() *model.ProviderInfo {
	lastCid, _, err := p.engine.GetLatestAdv(context.Background())
	if err != nil {
		return nil
	}

	pi := &model.ProviderInfo{
		AddrInfo:          *p.provider,
		Publisher:         p.provider,
		LastAdvertisement: lastCid,
	}
	return pi
}
