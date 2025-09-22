package networking

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// ConnectionPool manages a pool of reusable connections to peers
type ConnectionPool struct {
	host    host.Host
	metrics *metrics.ComponentMetrics

	mu          sync.RWMutex
	connections map[peer.ID]*pooledConnection
	config      ConnectionPoolConfig

	// Background workers
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ConnectionPoolConfig defines connection pool parameters
type ConnectionPoolConfig struct {
	MaxConnections      int           // Maximum total connections
	MaxPerPeer          int           // Maximum connections per peer
	IdleTimeout         time.Duration // Time before closing idle connections
	HealthCheckInterval time.Duration // How often to check connection health
	ConnectTimeout      time.Duration // Timeout for new connections
	RetryAttempts       int           // Number of retry attempts for failed connections
	RetryBackoff        time.Duration // Backoff between retry attempts
}

// DefaultConnectionPoolConfig returns sensible defaults
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxConnections:      1000,
		MaxPerPeer:          3,
		IdleTimeout:         30 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		ConnectTimeout:      5 * time.Second,
		RetryAttempts:       3,
		RetryBackoff:        time.Second,
	}
}

// pooledConnection represents a connection in the pool
type pooledConnection struct {
	conn     network.Conn
	streams  map[protocol.ID]network.Stream
	lastUsed time.Time
	healthy  bool
	inUse    int
	mu       sync.Mutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(h host.Host, config ConnectionPoolConfig) *ConnectionPool {
	ctx, cancel := context.WithCancel(context.Background())

	poolMetrics := metrics.NewComponentMetrics("connection_pool")
	metrics.RegisterGlobalComponent(poolMetrics)

	cp := &ConnectionPool{
		host:        h,
		metrics:     poolMetrics,
		connections: make(map[peer.ID]*pooledConnection),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start background workers
	cp.wg.Add(2)
	go cp.healthChecker()
	go cp.idleCleanup()

	return cp
}

// GetConnection returns a connection to the specified peer
func (cp *ConnectionPool) GetConnection(ctx context.Context, peerID peer.ID) (network.Conn, error) {
	start := time.Now()
	cp.metrics.RecordRequest()

	cp.mu.RLock()
	if pooled, exists := cp.connections[peerID]; exists && pooled.healthy {
		pooled.mu.Lock()
		pooled.inUse++
		pooled.lastUsed = time.Now()
		conn := pooled.conn
		pooled.mu.Unlock()
		cp.mu.RUnlock()

		cp.metrics.RecordSuccess(time.Since(start), 0)
		return conn, nil
	}
	cp.mu.RUnlock()

	// Need to create new connection
	return cp.createConnection(ctx, peerID)
}

// GetStream returns a stream to the specified peer using the given protocol
func (cp *ConnectionPool) GetStream(ctx context.Context, peerID peer.ID, proto protocol.ID) (network.Stream, error) {
	start := time.Now()
	cp.metrics.RecordRequest()

	cp.mu.RLock()
	pooled, exists := cp.connections[peerID]
	cp.mu.RUnlock()

	if !exists || !pooled.healthy {
		// Create new connection first
		_, err := cp.createConnection(ctx, peerID)
		if err != nil {
			cp.metrics.RecordFailure(time.Since(start), "connection_failed")
			return nil, err
		}

		cp.mu.RLock()
		pooled = cp.connections[peerID]
		cp.mu.RUnlock()
	}

	pooled.mu.Lock()
	defer pooled.mu.Unlock()

	// Check if we have a cached stream for this protocol
	if stream, exists := pooled.streams[proto]; exists {
		// Verify stream is still healthy
		if stream.Stat().Direction != network.DirUnknown {
			pooled.lastUsed = time.Now()
			cp.metrics.RecordSuccess(time.Since(start), 0)
			return stream, nil
		}
		// Stream is dead, remove it
		delete(pooled.streams, proto)
	}

	// Create new stream
	stream, err := cp.host.NewStream(ctx, peerID, proto)
	if err != nil {
		cp.metrics.RecordFailure(time.Since(start), "stream_creation_failed")
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Cache the stream
	if pooled.streams == nil {
		pooled.streams = make(map[protocol.ID]network.Stream)
	}
	pooled.streams[proto] = stream
	pooled.lastUsed = time.Now()

	cp.metrics.RecordSuccess(time.Since(start), 0)
	return stream, nil
}

// ReleaseConnection marks a connection as no longer in use
func (cp *ConnectionPool) ReleaseConnection(peerID peer.ID) {
	cp.mu.RLock()
	pooled, exists := cp.connections[peerID]
	cp.mu.RUnlock()

	if exists {
		pooled.mu.Lock()
		if pooled.inUse > 0 {
			pooled.inUse--
		}
		pooled.mu.Unlock()
	}
}

// ConnectToPeer establishes a connection to a peer given their addresses
func (cp *ConnectionPool) ConnectToPeer(ctx context.Context, addrs ...multiaddr.Multiaddr) error {
	start := time.Now()
	cp.metrics.RecordRequest()

	for _, addr := range addrs {
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			continue
		}

		// Check if we already have a connection
		cp.mu.RLock()
		if pooled, exists := cp.connections[info.ID]; exists && pooled.healthy {
			cp.mu.RUnlock()
			cp.metrics.RecordSuccess(time.Since(start), 0)
			return nil
		}
		cp.mu.RUnlock()

		// Try to connect with retry logic
		err = cp.connectWithRetry(ctx, *info)
		if err == nil {
			cp.metrics.RecordSuccess(time.Since(start), 0)
			return nil
		}
	}

	cp.metrics.RecordFailure(time.Since(start), "all_connections_failed")
	return fmt.Errorf("failed to connect to any of the provided addresses")
}

// createConnection creates a new connection to a peer
func (cp *ConnectionPool) createConnection(ctx context.Context, peerID peer.ID) (network.Conn, error) {
	// Check pool limits
	cp.mu.Lock()
	if len(cp.connections) >= cp.config.MaxConnections {
		cp.mu.Unlock()
		return nil, fmt.Errorf("connection pool full")
	}
	cp.mu.Unlock()

	// Create connection with timeout
	connectCtx, cancel := context.WithTimeout(ctx, cp.config.ConnectTimeout)
	defer cancel()

	// Get peer info from host's peerstore
	addrs := cp.host.Peerstore().Addrs(peerID)
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses found for peer %s", peerID)
	}

	info := peer.AddrInfo{
		ID:    peerID,
		Addrs: addrs,
	}

	err := cp.host.Connect(connectCtx, info)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	// Get the connection
	conn := cp.host.Network().ConnsToPeer(peerID)
	if len(conn) == 0 {
		return nil, fmt.Errorf("no connection found after connect")
	}

	// Add to pool
	pooled := &pooledConnection{
		conn:     conn[0],
		streams:  make(map[protocol.ID]network.Stream),
		lastUsed: time.Now(),
		healthy:  true,
		inUse:    1,
	}

	cp.mu.Lock()
	cp.connections[peerID] = pooled
	cp.mu.Unlock()

	return conn[0], nil
}

// connectWithRetry attempts to connect with exponential backoff
func (cp *ConnectionPool) connectWithRetry(ctx context.Context, info peer.AddrInfo) error {
	var lastErr error

	for attempt := 0; attempt < cp.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(cp.config.RetryBackoff * time.Duration(1<<attempt)):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		connectCtx, cancel := context.WithTimeout(ctx, cp.config.ConnectTimeout)
		err := cp.host.Connect(connectCtx, info)
		cancel()

		if err == nil {
			// Success, add to pool
			conns := cp.host.Network().ConnsToPeer(info.ID)
			if len(conns) > 0 {
				pooled := &pooledConnection{
					conn:     conns[0],
					streams:  make(map[protocol.ID]network.Stream),
					lastUsed: time.Now(),
					healthy:  true,
					inUse:    0,
				}

				cp.mu.Lock()
				cp.connections[info.ID] = pooled
				cp.mu.Unlock()
				return nil
			}
		}
		lastErr = err
	}

	return fmt.Errorf("failed to connect after %d attempts: %w", cp.config.RetryAttempts, lastErr)
}

// healthChecker periodically checks connection health
func (cp *ConnectionPool) healthChecker() {
	defer cp.wg.Done()

	ticker := time.NewTicker(cp.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.checkHealth()
		case <-cp.ctx.Done():
			return
		}
	}
}

// checkHealth verifies all connections are healthy
func (cp *ConnectionPool) checkHealth() {
	cp.mu.Lock()
	toRemove := make([]peer.ID, 0)

	for peerID, pooled := range cp.connections {
		pooled.mu.Lock()

		// Check if connection is still active
		if pooled.conn.IsClosed() {
			pooled.healthy = false
			toRemove = append(toRemove, peerID)
		}

		pooled.mu.Unlock()
	}

	// Remove unhealthy connections
	for _, peerID := range toRemove {
		delete(cp.connections, peerID)
	}

	cp.mu.Unlock()
}

// idleCleanup removes idle connections
func (cp *ConnectionPool) idleCleanup() {
	defer cp.wg.Done()

	ticker := time.NewTicker(cp.config.IdleTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.cleanupIdle()
		case <-cp.ctx.Done():
			return
		}
	}
}

// cleanupIdle removes connections that have been idle too long
func (cp *ConnectionPool) cleanupIdle() {
	now := time.Now()
	cp.mu.Lock()

	toRemove := make([]peer.ID, 0)
	for peerID, pooled := range cp.connections {
		pooled.mu.Lock()

		if pooled.inUse == 0 && now.Sub(pooled.lastUsed) > cp.config.IdleTimeout {
			toRemove = append(toRemove, peerID)
			// Close all streams
			for _, stream := range pooled.streams {
				stream.Close()
			}
			// Close connection
			pooled.conn.Close()
		}

		pooled.mu.Unlock()
	}

	// Remove idle connections
	for _, peerID := range toRemove {
		delete(cp.connections, peerID)
	}

	cp.mu.Unlock()
}

// GetStats returns current pool statistics
func (cp *ConnectionPool) GetStats() ConnectionPoolStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := ConnectionPoolStats{
		TotalConnections: len(cp.connections),
		ActiveStreams:    0,
		IdleConnections:  0,
	}

	for _, pooled := range cp.connections {
		pooled.mu.Lock()
		stats.ActiveStreams += len(pooled.streams)
		if pooled.inUse == 0 {
			stats.IdleConnections++
		}
		pooled.mu.Unlock()
	}

	return stats
}

// ConnectionPoolStats provides pool statistics
type ConnectionPoolStats struct {
	TotalConnections int
	ActiveStreams    int
	IdleConnections  int
}

// GetMetrics returns the current metrics for this connection pool
func (cp *ConnectionPool) GetMetrics() metrics.MetricsSnapshot {
	return cp.metrics.GetSnapshot()
}

// Close shuts down the connection pool
func (cp *ConnectionPool) Close() error {
	cp.cancel()
	cp.wg.Wait()

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Close all connections and streams
	for _, pooled := range cp.connections {
		pooled.mu.Lock()
		for _, stream := range pooled.streams {
			stream.Close()
		}
		pooled.conn.Close()
		pooled.mu.Unlock()
	}

	cp.connections = make(map[peer.ID]*pooledConnection)
	return nil
}
