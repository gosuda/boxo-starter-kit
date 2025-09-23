package dht

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"

	persistent "github.com/gosuda/boxo-starter-kit/01-persistent/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
)

type DHTWrapper struct {
	routing.Routing
}

func NewWithRouting(ctx context.Context, r routing.Routing) (*DHTWrapper, error) {
	return &DHTWrapper{
		Routing: r,
	}, nil
}

func New(ctx context.Context, host *network.HostWrapper, persistentWrapper *persistent.PersistentWrapper) (*DHTWrapper, error) {
	var err error
	if host == nil {
		host, err = network.New(nil)
		if err != nil {
			return nil, err
		}
	}
	if persistentWrapper == nil {
		persistentWrapper, err = persistent.New(persistent.Memory, "")
		if err != nil {
			return nil, err
		}
	}

	ipfsdht, err := dht.New(ctx, host,
		dht.Mode(dht.ModeAutoServer),
		dht.Datastore(persistentWrapper.Batching))
	if err != nil {
		return nil, err
	}
	return NewWithRouting(ctx, ipfsdht)
}

func (w *DHTWrapper) FindProviders(ctx context.Context, c cid.Cid, max int) ([]peer.AddrInfo, error) {
	if !c.Defined() {
		return nil, fmt.Errorf("undefined cid")
	}

	ch := w.Routing.FindProvidersAsync(ctx, c, 0)
	var out []peer.AddrInfo
	for pi := range ch {
		out = append(out, pi)
	}
	return out, nil
}

func (w *DHTWrapper) RoutingTableSize() int {
	if ipfsdht, ok := w.Routing.(*dht.IpfsDHT); ok {
		return ipfsdht.RoutingTable().Size()
	}
	return 0
}
