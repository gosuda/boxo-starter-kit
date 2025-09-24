package main

import (
	"context"
	"testing"
	"time"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/require"

	traversalselector "github.com/gosuda/boxo-starter-kit/14-traversal-selector/pkg"
	graphsync "github.com/gosuda/boxo-starter-kit/15-graphsync/pkg"
)

func TestGraphSyncPubsub(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gs1, err := graphsync.New(ctx, nil, nil)
	require.NoError(t, err)

	gs2, err := graphsync.New(ctx, nil, nil)
	require.NoError(t, err)

	// connect peers
	require.NoError(t, gs2.Host.ConnectToPeer(ctx, gs1.Host.GetFullAddresses()[0]))
	require.NoError(t, gs1.Host.ConnectToPeer(ctx, gs2.Host.GetFullAddresses()[0]))

	node := "hello graphsync"
	c1, err := gs1.Ipld.PutIPLDAny(ctx, node)
	require.NoError(t, err)

	// fetch with default selector (whole graph)
	progress, err := gs2.Fetch(ctx, gs1.Host.ID(), c1, nil)
	require.NoError(t, err)
	require.True(t, progress)

	// get data
	got, err := gs2.Ipld.GetIPLDAny(ctx, c1)
	require.NoError(t, err)

	require.Equal(t, node, got)
}

func TestGraphSyncPubsubWithSelector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gs1, err := graphsync.New(ctx, nil, nil)
	require.NoError(t, err)

	gs2, err := graphsync.New(ctx, nil, nil)
	require.NoError(t, err)

	// connect peers
	require.NoError(t, gs2.Host.ConnectToPeer(ctx, gs1.Host.GetFullAddresses()[0]))
	require.NoError(t, gs1.Host.ConnectToPeer(ctx, gs2.Host.GetFullAddresses()[0]))

	// build a ipld node
	leftVal := "L"
	leftCID, err := gs1.Ipld.PutIPLDAny(ctx, leftVal)
	require.NoError(t, err)
	rightVal := "R"
	rightCID, err := gs1.Ipld.PutIPLDAny(ctx, rightVal)
	require.NoError(t, err)
	root := map[string]any{
		"left":  cidlink.Link{Cid: leftCID},
		"right": cidlink.Link{Cid: rightCID},
	}
	rootCID, err := gs1.Ipld.PutIPLDAny(ctx, root)
	require.NoError(t, err)

	progress, err := gs2.Fetch(ctx, gs1.Host.ID(), rootCID, traversalselector.SelectorField("left"))
	require.NoError(t, err)
	require.True(t, progress)

	// leftVal should be fetched
	got, err := gs2.Ipld.GetIPLDAny(ctx, leftCID)
	require.NoError(t, err)
	require.Equal(t, leftVal, got)

	// rightVal should NOT be fetched
	got, err = gs2.Ipld.GetIPLDAny(ctx, rightCID)
	require.Error(t, err)
	require.Nil(t, got)

	progress, err = gs2.Fetch(ctx, gs1.Host.ID(), rootCID, nil)
	require.NoError(t, err)
	require.True(t, progress)

	// rightVal should be fetched now
	got, err = gs2.Ipld.GetIPLDAny(ctx, rightCID)
	require.NoError(t, err)
	require.Equal(t, rightVal, got)

	// root should be fetched now
	got, err = gs2.Ipld.GetIPLDAny(ctx, rootCID)
	require.NoError(t, err)
	expected := map[string]any{"left": leftCID, "right": rightCID}
	require.EqualValues(t, expected, got)
}
