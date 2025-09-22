package network

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-varint"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

type HostWrapper struct {
	host.Host
	protoID    protocol.ID
	maxPayload uint64
	timeout    time.Duration

	inbox chan network.Stream
	done  chan struct{}

	mu      sync.Mutex
	waiters map[string][]chan msg // by cid.String()
	buf     map[string]msg

	// Metrics
	metrics *metrics.ComponentMetrics
}

type Config struct {
	ProtoID     string
	MaxPayload  uint64
	Timeout     time.Duration
	ListenAddrs []string
}

func New(cfg *Config) (*HostWrapper, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.ProtoID == "" {
		cfg.ProtoID = "/custom/xfer/1.0.0"
	}
	if cfg.MaxPayload == 0 {
		cfg.MaxPayload = 1 << 20 // 1MiB
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if len(cfg.ListenAddrs) == 0 {
		cfg.ListenAddrs = []string{"/ip4/0.0.0.0/tcp/0"}
	}

	var las []multiaddr.Multiaddr
	for _, s := range cfg.ListenAddrs {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			return nil, fmt.Errorf("listen addr %q: %w", s, err)
		}
		las = append(las, ma)
	}

	h, err := libp2p.New(libp2p.ListenAddrs(las...))
	if err != nil {
		return nil, err
	}

	// Initialize metrics
	networkMetrics := metrics.NewComponentMetrics("network")
	metrics.RegisterGlobalComponent(networkMetrics)

	n := &HostWrapper{
		Host:       h,
		protoID:    protocol.ID(cfg.ProtoID),
		maxPayload: cfg.MaxPayload,
		timeout:    cfg.Timeout,
		inbox:      make(chan network.Stream, 32),
		done:       make(chan struct{}),
		waiters:    make(map[string][]chan msg),
		buf:        make(map[string]msg),
		metrics:    networkMetrics,
	}

	h.SetStreamHandler(n.protoID, func(s network.Stream) {
		select {
		case n.inbox <- s:
		case <-n.done:
			_ = s.Reset()
		}
	})
	go n.dispatch()

	return n, nil
}

func (n *HostWrapper) ConnectToPeer(ctx context.Context, addrs ...multiaddr.Multiaddr) error {
	start := time.Now()
	n.metrics.RecordRequest()

	for _, a := range addrs {
		info, err := peer.AddrInfoFromP2pAddr(a)
		if err != nil {
			n.metrics.RecordFailure(time.Since(start), "addr_parse_error")
			return fmt.Errorf("parse addr: %w", err)
		}
		if err := n.Host.Connect(ctx, *info); err != nil {
			n.metrics.RecordFailure(time.Since(start), "connection_error")
			return fmt.Errorf("connect %s: %w", info.ID, err)
		}
	}

	n.metrics.RecordSuccess(time.Since(start), 0)
	return nil
}

func (n *HostWrapper) Peers() []peer.ID {
	return n.Host.Network().Peers()
}

func (n *HostWrapper) Send(ctx context.Context, to peer.ID, payload []byte) (cid.Cid, error) {
	start := time.Now()
	n.metrics.RecordRequest()

	if to == "" {
		n.metrics.RecordFailure(time.Since(start), "missing_peer_id")
		return cid.Undef, fmt.Errorf("missing peer id")
	}
	if len(payload) == 0 {
		n.metrics.RecordFailure(time.Since(start), "empty_payload")
		return cid.Undef, fmt.Errorf("empty payload")
	}
	if uint64(len(payload)) > n.maxPayload {
		n.metrics.RecordFailure(time.Since(start), "payload_too_large")
		return cid.Undef, fmt.Errorf("payload too large: %d > %d", len(payload), n.maxPayload)
	}

	s, err := n.NewStream(ctx, to, n.protoID)
	if err != nil {
		n.metrics.RecordFailure(time.Since(start), "stream_creation_error")
		return cid.Undef, err
	}
	defer s.Close()
	_ = s.SetDeadline(time.Now().Add(n.timeout))

	if _, err := s.Write(varint.ToUvarint(uint64(len(payload)))); err != nil {
		n.metrics.RecordFailure(time.Since(start), "write_length_error")
		return cid.Undef, fmt.Errorf("write len: %w", err)
	}
	if _, err := s.Write(payload); err != nil {
		n.metrics.RecordFailure(time.Since(start), "write_payload_error")
		return cid.Undef, fmt.Errorf("write payload: %w", err)
	}
	_ = s.CloseWrite()

	c, err := block.ComputeCID(payload, nil)
	if err != nil {
		n.metrics.RecordFailure(time.Since(start), "cid_computation_error")
		return cid.Undef, err
	}

	n.metrics.RecordSuccess(time.Since(start), int64(len(payload)))
	return c, nil
}

// Receive blocks until a message whose CID == want arrives.
// Returns (fromPeer, payload, error).
func (n *HostWrapper) Receive(ctx context.Context, want cid.Cid) (peer.ID, []byte, error) {
	start := time.Now()
	n.metrics.RecordRequest()

	if !want.Defined() {
		n.metrics.RecordFailure(time.Since(start), "undefined_cid")
		return "", nil, fmt.Errorf("undefined CID")
	}
	key := want.String()

	n.mu.Lock()
	if m, ok := n.buf[key]; ok {
		delete(n.buf, key)
		n.mu.Unlock()
		n.metrics.RecordSuccess(time.Since(start), int64(len(m.data)))
		return m.from, m.data, nil
	}
	ch := make(chan msg, 1)
	n.waiters[key] = append(n.waiters[key], ch)
	n.mu.Unlock()

	select {
	case m := <-ch:
		n.metrics.RecordSuccess(time.Since(start), int64(len(m.data)))
		return m.from, m.data, nil
	case <-ctx.Done():
		n.mu.Lock()
		q := n.waiters[key]
		for i := range q {
			if q[i] == ch {
				n.waiters[key] = append(q[:i], q[i+1:]...)
				break
			}
		}
		if len(n.waiters[key]) == 0 {
			delete(n.waiters, key)
		}
		n.mu.Unlock()
		n.metrics.RecordFailure(time.Since(start), "context_cancelled")
		return "", nil, ctx.Err()
	case <-n.done:
		n.metrics.RecordFailure(time.Since(start), "host_shutdown")
		return "", nil, io.EOF
	}
}

func (n *HostWrapper) GetFullAddresses() []multiaddr.Multiaddr {
	peerPart, _ := multiaddr.NewMultiaddr("/p2p/" + n.ID().String())
	var out []multiaddr.Multiaddr
	for _, a := range n.Addrs() {
		out = append(out, a.Encapsulate(peerPart))
	}
	return out
}

func (n *HostWrapper) Close() error {
	close(n.done)
	if n.Host != nil {
		return n.Host.Close()
	}
	return nil
}

// GetMetrics returns the current metrics for this network wrapper
func (n *HostWrapper) GetMetrics() metrics.MetricsSnapshot {
	return n.metrics.GetSnapshot()
}

// --- internal ---

type msg struct {
	from peer.ID
	cid  cid.Cid
	data []byte
}

func (n *HostWrapper) dispatch() {
	for {
		select {
		case s := <-n.inbox:
			n.handle(s)
		case <-n.done:
			return
		}
	}
}

func (n *HostWrapper) handle(s network.Stream) {
	_ = s.SetDeadline(time.Now().Add(n.timeout))
	br := bufio.NewReader(s)

	length, err := varint.ReadUvarint(br)
	if err != nil || length == 0 || length > n.maxPayload {
		_ = s.Close()
		return
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(br, data); err != nil {
		_ = s.Close()
		return
	}
	_ = s.Close()

	c, err := block.ComputeCID(data, nil)
	if err != nil {
		return
	}
	m := msg{from: s.Conn().RemotePeer(), cid: c, data: data}
	key := c.String()

	n.mu.Lock()
	defer n.mu.Unlock()
	if wait := n.waiters[key]; len(wait) > 0 {
		ch := wait[0]
		n.waiters[key] = wait[1:]
		if len(n.waiters[key]) == 0 {
			delete(n.waiters, key)
		}
		select {
		case ch <- m:
		default:
		}
	} else {
		n.buf[key] = m
	}
}
