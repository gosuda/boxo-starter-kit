package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	ipni "github.com/gosuda/boxo-starter-kit/17-ipni/pkg"
)

func TestIPNIPutGet(t *testing.T) {
	ipniWrapper, err := ipni.New("", "", nil, nil, nil)
	require.NoError(t, err)

	providerID := ipniWrapper.Provider.ProviderID()
	data := []byte("hello-ipni")
	c, err := block.ComputeCID(data, nil)
	require.NoError(t, err)

	ctxBitswap := []byte("ctx-bitswap")
	ctxHTTP := []byte("ctx-http")
	ctxGS := []byte("ctx-graphsync-filecoin")
	require.NoError(t, ipniWrapper.PutBitswap(providerID, ctxBitswap, c))
	require.NoError(t, ipniWrapper.PutHTTP(providerID, ctxHTTP, c))
	require.NoError(t, ipniWrapper.PutGraphSyncFilecoin(providerID, c, false, true, ctxGS, c))

	results, found, err := ipniWrapper.GetProvidersByCID(c)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, results, 3)

	require.Equal(t, providerID, results[0].ProviderID)
	require.Equal(t, ctxBitswap, results[0].ContextID)
	require.Equal(t, ipni.TBitswap, ipni.ExportTransportKind(results[0]))

	require.Equal(t, providerID, results[1].ProviderID)
	require.Equal(t, ctxHTTP, results[1].ContextID)
	require.Equal(t, ipni.THTTP, ipni.ExportTransportKind(results[1]))

	require.Equal(t, providerID, results[2].ProviderID)
	require.Equal(t, ctxGS, results[2].ContextID)
	require.Equal(t, ipni.TGraphSync, ipni.ExportTransportKind(results[2]))
}

func TestIPNIPlanner(t *testing.T) {
	ctx := context.Background()

	ipniWrapper, err := ipni.New("", "", nil, nil, nil)
	require.NoError(t, err)
	data := []byte("planner-order-stagger")
	c, err := block.ComputeCID(data, nil)
	require.NoError(t, err)

	pid := ipniWrapper.Provider.ProviderID()
	ctxBitswap := []byte("ctx-bw")
	ctxHTTP := []byte("ctx-http")
	ctxGS := []byte("ctx-gs-filecoin")

	require.NoError(t, ipniWrapper.PutBitswap(pid, ctxBitswap, c))
	require.NoError(t, ipniWrapper.PutHTTP(pid, ctxHTTP, c))
	require.NoError(t, ipniWrapper.PutGraphSyncFilecoin(pid, c, false, true, ctxGS, c))

	vals, found, err := ipniWrapper.GetProvidersByCID(c)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, vals, 3)

	attempts, hit, err := ipniWrapper.PlanByCID(ctx, c, ipni.Intent{
		// no preference, use default
	})
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, attempts, 3)

	// default : HTTP(0.7) > GraphSync(0.6) > Bitswap(0.4)
	require.Equal(t, ipni.THTTP, attempts[0].Proto)
	require.Equal(t, ipni.TGraphSync, attempts[1].Proto)
	require.Equal(t, ipni.TBitswap, attempts[2].Proto)

	// Stagger: 0, 1*default, 2*default …
	require.Equal(t, time.Duration(0), attempts[0].Stagger)

	attempts, hit, err = ipniWrapper.PlanByCID(ctx, c, ipni.Intent{
		Preferred: []ipni.TransportKind{ipni.TGraphSync},
	})
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, attempts, 3)

	// Preferred GraphSync: GraphSync(0.6+0.15) > HTTP(0.7) > Bitswap(0.4)
	require.Equal(t, ipni.TGraphSync, attempts[0].Proto)
	require.Equal(t, ipni.THTTP, attempts[1].Proto)
	require.Equal(t, ipni.TBitswap, attempts[2].Proto)

	attempts, hit, err = ipniWrapper.PlanByCID(ctx, c, ipni.Intent{
		BitswapOnly: true,
	})
	require.NoError(t, err)
	require.True(t, hit)
	require.Len(t, attempts, 1)

	require.Equal(t, ipni.TBitswap, attempts[0].Proto)
}

func TestIPNITransport(t *testing.T) {
	ctx := context.Background()

	host1, err := network.New(nil)
	require.NoError(t, err)
	providerWrapper, err := ipni.New("", "", nil, host1, nil)
	require.NoError(t, err)

	host2, err := network.New(nil)
	require.NoError(t, err)
	subscriberWrapper, err := ipni.New("", "", nil, host2, nil)
	require.NoError(t, err)

	require.NoError(t, host1.ConnectToPeer(ctx, host2.GetFullAddresses()...))
	require.NoError(t, host2.ConnectToPeer(ctx, host1.GetFullAddresses()...))

	require.NoError(t, providerWrapper.Start(ctx))
	require.NoError(t, subscriberWrapper.Start(ctx))

	data := []byte("hello-ipni-transport")
	c, err := block.ComputeCID(data, nil)
	require.NoError(t, err)

	providerID := providerWrapper.Provider.ProviderID()
	ctxBitswap := []byte("ctx-bitswap")
	require.NoError(t, providerWrapper.PutBitswap(providerID, ctxBitswap, c))

	time.Sleep(5 * time.Second)

	results, found, err := subscriberWrapper.GetProvidersByCID(c)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, results, 1)
	require.Equal(t, providerID, results[0].ProviderID)
	require.Equal(t, ctxBitswap, results[0].ContextID)
	require.Equal(t, ipni.TBitswap, ipni.ExportTransportKind(results[0]))
}
