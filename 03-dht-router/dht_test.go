package main

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	dht "github.com/gosuda/boxo-starter-kit/03-dht-router/pkg"
)

func TestDHTBootstrap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	const num = 5
	var dhts []*dht.DHTWrapper
	var hosts []*network.HostWrapper

	for range num {
		h, err := network.New(nil)
		require.NoError(t, err)
		hosts = append(hosts, h)

		w, err := dht.New(ctx, h, nil)
		require.NoError(t, err)
		dhts = append(dhts, w)
	}
	defer func() {
		for _, h := range hosts {
			h.Close()
		}
	}()

	for i := 1; i < num; i++ {
		err := hosts[i].ConnectToPeer(ctx, hosts[0].GetFullAddresses()...)
		require.NoErrorf(t, err, "connect host[%d] -> host[0]", i)
	}

	for i, w := range dhts {
		err := w.Bootstrap(ctx)
		require.NoErrorf(t, err, "dht[%d] bootstrap", i)
	}

	// wait for routing table update
	time.Sleep(time.Second)

	for i, w := range dhts {
		size := w.RoutingTableSize()
		require.Equal(t, size, num-1, "dht[%d] routing table is not full: %d", i, size)
	}
}

func TestProvideFindProvidersCID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	hA, err := network.New(nil)
	require.NoError(t, err)
	defer hA.Close()
	hB, err := network.New(nil)
	require.NoError(t, err)
	defer hB.Close()

	dA, err := dht.New(ctx, hA, nil)
	require.NoError(t, err)
	dB, err := dht.New(ctx, hB, nil)
	require.NoError(t, err)

	require.NoError(t, hB.ConnectToPeer(ctx, hA.GetFullAddresses()...))
	require.NoError(t, dA.Bootstrap(ctx))
	require.NoError(t, dB.Bootstrap(ctx))
	time.Sleep(time.Second) // wait for routing table update

	// advertisement
	c, err := block.ComputeCID([]byte("hello"), nil)
	require.NoError(t, err)
	require.NoError(t, dA.Provide(ctx, c, true))

	var provs []peer.AddrInfo
	deadline := time.Now().Add(3 * time.Second)
	for {
		var err error
		provs, err = dB.FindProviders(ctx, c, 10)
		require.NoError(t, err)
		if len(provs) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NotEmpty(t, provs)
	foundA := false
	for _, pi := range provs {
		if pi.ID == hA.ID() {
			foundA = true
			break
		}
	}
	require.True(t, foundA, "provider A not found")
}
