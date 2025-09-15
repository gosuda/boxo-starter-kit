package network

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	block "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-varint"
)

type HostWrapper struct {
	host.Host

	// config
	maxPayload uint64
	protoID    protocol.ID
	timeout    time.Duration

	// runtime
	receiver  chan network.Stream
	done      chan struct{}
	onceClose sync.Once
	waitMu    sync.Mutex
	waiters   map[string][]chan message
	buf       map[string]message

	// node info
	id        peer.ID
	addresses []multiaddr.Multiaddr
	peers     []peer.AddrInfo

	// simple stats
	stats struct {
		mutex          sync.RWMutex
		BlocksSent     int64
		BlocksReceived int64
		PeersConnected int
		WantListSize   int
	}
}

// NodeConfig configures a bitswap node
type NodeConfig struct {
	MaxPayload  uint64
	ProtoID     string
	Timeout     time.Duration
	ListenAddrs []string // Addresses to listen on (e.g., "/ip4/0.0.0.0/tcp/0")
}

func New(config *NodeConfig) (*HostWrapper, error) {
	if config == nil {
		config = &NodeConfig{}
	}
	if config.MaxPayload == 0 {
		config.MaxPayload = 1 << 20 // 1 MiB
	}
	if config.ProtoID == "" {
		config.ProtoID = "/custom/xfer/1.0.0"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config == nil {
		config = &NodeConfig{}
	}
	if config.ListenAddrs == nil {
		config.ListenAddrs = []string{"/ip4/0.0.0.0/tcp/0"}
	}

	// Build listen addrs
	var listenAddrs []multiaddr.Multiaddr
	for _, addr := range config.ListenAddrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid listen address %s: %w", addr, err)
		}
		listenAddrs = append(listenAddrs, ma)
	}

	// Create host
	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.EnableRelay(), // NAT traversal aid
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	node := &HostWrapper{
		maxPayload: config.MaxPayload,
		protoID:    protocol.ID(config.ProtoID),
		timeout:    config.Timeout,

		Host:      h,
		id:        h.ID(),
		addresses: h.Addrs(),

		receiver: make(chan network.Stream, 32),
		done:     make(chan struct{}),
		waiters:  make(map[string][]chan message),
		buf:      make(map[string]message),
	}

	// install handler: push streams into receiver channel
	h.SetStreamHandler(node.protoID, func(s network.Stream) {
		select {
		case node.receiver <- s:
		case <-node.done:
			_ = s.Reset()
		}
	})
	go node.runDispatcher()

	return node, nil
}

// GetID returns the peer ID of this node
func (h *HostWrapper) GetID() peer.ID {
	return h.id
}

// GetAddresses returns the multiaddresses this node is listening on
func (h *HostWrapper) GetAddresses() []multiaddr.Multiaddr {
	return h.addresses
}

// GetFullAddresses returns the full multiaddresses including peer ID
func (h *HostWrapper) GetFullAddresses() []multiaddr.Multiaddr {
	var fullAddrs []multiaddr.Multiaddr
	peerPart, _ := multiaddr.NewMultiaddr("/p2p/" + h.id.String())

	for _, addr := range h.addresses {
		fullAddr := addr.Encapsulate(peerPart)
		fullAddrs = append(fullAddrs, fullAddr)
	}

	return fullAddrs
}

// ConnectToPeer connects to another bitswap node
func (h *HostWrapper) ConnectToPeer(ctx context.Context, addr multiaddr.Multiaddr) error {
	// Extract peer info from multiaddr
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("failed to parse peer address: %w", err)
	}

	// Connect to peer
	err = h.Host.Connect(ctx, *info)
	if err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", info.ID, err)
	}

	// Update stats
	h.peers = append(h.peers, *info)

	return nil
}

func (h *HostWrapper) Send(ctx context.Context, payload []byte, id peer.ID) (cid.Cid, error) {
	if id == "" {
		if len(h.peers) == 0 {
			return cid.Undef, fmt.Errorf("no target peer: id is empty and peer list is empty")
		}
		id = h.peers[0].ID
	}
	if len(payload) == 0 {
		return cid.Undef, fmt.Errorf("empty payload")
	}
	if uint64(len(payload)) > h.maxPayload {
		return cid.Undef, fmt.Errorf("payload too large: %d > %d", len(payload), h.maxPayload)
	}

	s, err := h.Host.NewStream(ctx, id, h.protoID)
	if err != nil {
		return cid.Undef, fmt.Errorf("new stream: %w", err)
	}
	defer s.Close()
	_ = s.SetDeadline(time.Now().Add(h.timeout))

	if _, err := s.Write(varint.ToUvarint(uint64(len(payload)))); err != nil {
		return cid.Undef, fmt.Errorf("write length: %w", err)
	}
	if _, err := s.Write(payload); err != nil {
		return cid.Undef, fmt.Errorf("write payload: %w", err)
	}
	_ = s.CloseWrite()

	h.stats.mutex.Lock()
	h.stats.BlocksSent++
	h.stats.mutex.Unlock()

	c, err := block.ComputeCID(payload, nil)
	if err != nil {
		return cid.Undef, err
	}
	return c, nil
}

func (h *HostWrapper) Receive(ctx context.Context, c cid.Cid) (peer.ID, []byte, error) {
	if !c.Defined() {
		return "", nil, fmt.Errorf("want CID is undefined")
	}
	key := c.String()

	// 1) fast path: buffered
	h.waitMu.Lock()
	if msg, ok := h.buf[key]; ok {
		delete(h.buf, key)
		h.waitMu.Unlock()
		return msg.from, msg.data, nil
	}

	// 2) register waiter
	ch := make(chan message, 1)
	h.waiters[key] = append(h.waiters[key], ch)
	h.waitMu.Unlock()

	// 3) wait
	select {
	case msg := <-ch:
		return msg.from, msg.data, nil
	case <-ctx.Done():
		// remove myself from waiters
		h.waitMu.Lock()
		queue := h.waiters[key]
		for i := range queue {
			if queue[i] == ch {
				h.waiters[key] = append(queue[:i], queue[i+1:]...)
				break
			}
		}
		if len(h.waiters[key]) == 0 {
			delete(h.waiters, key)
		}
		h.waitMu.Unlock()
		return "", nil, ctx.Err()
	case <-h.done:
		return "", nil, io.EOF
	}
}

func (h *HostWrapper) receive(s network.Stream) ([]byte, error) {
	_ = s.SetDeadline(time.Now().Add(h.timeout))
	br := bufio.NewReader(s)

	// read uvarint length safely
	length, err := varint.ReadUvarint(br)
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("read length: %w", err)
	}
	if length == 0 || length > h.maxPayload {
		_ = s.Close()
		return nil, fmt.Errorf("invalid/too-large payload: %d", length)
	}

	// read exact payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(br, payload); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("read payload: %w", err)
	}

	// if single-message-per-stream, close here; if streaming, let caller manage
	_ = s.Close()

	h.stats.mutex.Lock()
	h.stats.BlocksReceived++
	h.stats.mutex.Unlock()

	return payload, nil
}

func (h *HostWrapper) runDispatcher() {
	for {
		select {
		case s := <-h.receiver:
			h.handleIncomingStream(s)
		case <-h.done:
			return
		}
	}
}

type message struct {
	from peer.ID
	cid  cid.Cid
	data []byte
}

func (n *HostWrapper) handleIncomingStream(s network.Stream) {
	data, err := n.receive(s)
	if err != nil {
		return
	}
	c, err := block.ComputeCID(data, nil)
	if err != nil {
		return
	}
	msg := message{
		from: s.Conn().RemotePeer(),
		cid:  c,
		data: data,
	}

	key := c.String()
	// waiter 우선
	n.waitMu.Lock()
	q := n.waiters[key]
	if len(q) > 0 {
		ch := q[0]
		n.waiters[key] = q[1:]
		if len(n.waiters[key]) == 0 {
			delete(n.waiters, key)
		}
		n.waitMu.Unlock()

		select {
		case ch <- msg:
		default:
		}
	} else {
		n.buf[key] = msg
		n.waitMu.Unlock()
	}

	// stats
	n.stats.mutex.Lock()
	n.stats.BlocksReceived++
	n.stats.mutex.Unlock()
}

// WantBlock adds a block to the want list (simplified implementation)
func (h *HostWrapper) WantBlock(ctx context.Context, c cid.Cid) error {
	if !c.Defined() {
		return fmt.Errorf("invalid CID")
	}

	// In a full bitswap implementation, this would:
	// 1. Add CID to want list
	// 2. Announce want to connected peers
	// 3. Wait for providers to respond
	// For this educational version, we simulate the behavior
	h.stats.mutex.Lock()
	h.stats.WantListSize++
	h.stats.mutex.Unlock()

	return nil
}

// GetConnectedPeers returns the list of connected peers
func (h *HostWrapper) GetConnectedPeers() []peer.ID {
	return h.Host.Network().Peers()
}

// GetStats returns current bitswap statistics
func (h *HostWrapper) GetStats() BitswapStats {
	h.stats.mutex.RLock()
	defer h.stats.mutex.RUnlock()

	return BitswapStats{
		BlocksSent:     h.stats.BlocksSent,
		BlocksReceived: h.stats.BlocksReceived,
		PeersConnected: len(h.GetConnectedPeers()),
		WantListSize:   h.stats.WantListSize,
		NodeID:         h.id.String(),
	}
}

// BitswapStats contains bitswap node statistics
type BitswapStats struct {
	BlocksSent     int64  `json:"blocks_sent"`
	BlocksReceived int64  `json:"blocks_received"`
	PeersConnected int    `json:"peers_connected"`
	WantListSize   int    `json:"want_list_size"`
	NodeID         string `json:"node_id"`
}

// Close shuts down the bitswap node
func (h *HostWrapper) Close() error {
	if h.Host != nil {
		return h.Host.Close()
	}
	return nil
}

// updatePeerStats updates peer-related statistics
func (h *HostWrapper) updatePeerStats() {
	h.stats.mutex.Lock()
	defer h.stats.mutex.Unlock()

	h.stats.PeersConnected = len(h.GetConnectedPeers())
}

// Note: This simplified implementation focuses on demonstrating P2P networking concepts
// rather than full bitswap protocol implementation. In a production system,
// you would use the complete boxo bitswap package with proper routing,
// want-list management, and provider discovery.
