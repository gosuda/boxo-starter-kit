package graphsync

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	igs "github.com/ipfs/go-graphsync"
	grphsync "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/peer"

	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	ipldprime "github.com/gosuda/boxo-starter-kit/11-ipld-prime/pkg"
	ts "github.com/gosuda/boxo-starter-kit/13-traversal-selector/pkg"
)

type GraphSyncWrapper struct {
	Host *network.HostWrapper
	Ipld *ipldprime.IpldWrapper
	igs.GraphExchange
}

func New(ctx context.Context, host *network.HostWrapper, ipld *ipldprime.IpldWrapper) (*GraphSyncWrapper, error) {
	var err error
	if host == nil {
		host, err = network.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p host: %w", err)
		}
	}
	if ipld == nil {
		ipld, err = ipldprime.NewDefault(nil, nil)
		if err != nil {
			return nil, err
		}
	}

	gsnet := gsnet.NewFromLibp2pHost(host)
	gs := grphsync.New(ctx, gsnet, ipld.LinkSystem)
	gs.RegisterIncomingRequestHook(func(p peer.ID, request graphsync.RequestData, hookActions graphsync.IncomingRequestHookActions) {
		hookActions.ValidateRequest()
	})

	return &GraphSyncWrapper{
		Host:          host,
		Ipld:          ipld,
		GraphExchange: gs,
	}, nil
}

func defaultSelector() ipld.Node {
	return ts.SelectorAll(true)
}

func (g *GraphSyncWrapper) Fetch(
	ctx context.Context,
	pid peer.ID,
	root cid.Cid,
	sel ipld.Node,
	exts ...igs.ExtensionData,
) (progress bool, err error) {
	respCh, errCh, err := g.Request(ctx, pid, root, sel, exts...)
	if err != nil {
		return false, err
	}
	for respCh != nil || errCh != nil {
		select {
		case _, ok := <-respCh:
			if !ok {
				respCh = nil
				continue
			}
			progress = true
		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if e != nil {
				return progress, e
			}
		case <-ctx.Done():
			return progress, ctx.Err()
		}
	}
	if !progress { // no data received
		return false, nil
	}
	return true, nil
}

func (g *GraphSyncWrapper) Request(
	ctx context.Context,
	pid peer.ID,
	root cid.Cid,
	sel ipld.Node,
	exts ...igs.ExtensionData,
) (<-chan igs.ResponseProgress, <-chan error, error) {
	if sel == nil {
		sel = defaultSelector()
	}

	respCh, errCh := g.GraphExchange.Request(
		ctx,
		pid,
		cidlink.Link{Cid: root},
		sel,
		exts...,
	)
	return respCh, errCh, nil
}
