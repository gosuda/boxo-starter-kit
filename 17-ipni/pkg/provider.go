package ipni

import (
	"context"
	"fmt"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"

	"github.com/ipfs/go-cid"
	md "github.com/ipni/go-libipni/metadata"
	iprov "github.com/ipni/index-provider"
	provengine "github.com/ipni/index-provider/engine"
	carsupplier "github.com/ipni/index-provider/supplier"
	"github.com/libp2p/go-libp2p/core/peer"
)

type ProviderWrapper struct {
	provider *peer.AddrInfo
	iprov.Interface
	topic  string
	carSup *carsupplier.CarSupplier
}

func NewProviderWrapper(topic string, persistentWrapper *persistent.PersistentWrapper, hostWrapper *network.HostWrapper) (*ProviderWrapper, error) {
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
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create index provider: %w", err)
	}

	carSup := carsupplier.NewCarSupplier(eng, persistentWrapper.Batching)

	return &ProviderWrapper{
		topic:     topic,
		provider:  provider,
		Interface: eng,
		carSup:    carSup,
	}, nil
}

func (p *ProviderWrapper) PutCAR(ctx context.Context, contextID []byte, carPath string, md md.Metadata) (cid.Cid, error) {
	if p.carSup == nil {
		return cid.Undef, fmt.Errorf("car supplier not initialized")
	}
	return p.carSup.Put(ctx, contextID, carPath, md)
}

func (p *ProviderWrapper) RemoveCAR(ctx context.Context, contextID []byte) (cid.Cid, error) {
	if p.carSup == nil {
		return cid.Undef, fmt.Errorf("car supplier not initialized")
	}
	return p.carSup.Remove(ctx, contextID)
}
