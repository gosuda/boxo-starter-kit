package networking

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	networkpkg "github.com/gosuda/boxo-starter-kit/02-network/pkg"
	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// OptimizedNetwork wraps the basic network wrapper with advanced optimizations
type OptimizedNetwork struct {
	baseNetwork *networkpkg.HostWrapper
	metrics     *metrics.ComponentMetrics

	// Optimization components
	connectionPool   *ConnectionPool
	messageBatcher   *MessageBatcher
	bandwidthManager *BandwidthManager
	adaptiveTimeouts *AdaptiveTimeouts

	// Configuration
	config *OptimizationConfig
	mu     sync.RWMutex
}

// OptimizationConfig defines which optimizations to enable
type OptimizationConfig struct {
	EnableConnectionPool   bool
	EnableMessageBatching  bool
	EnableBandwidthManager bool
	EnableAdaptiveTimeouts bool

	// Component configurations
	ConnectionPool   ConnectionPoolConfig
	MessageBatching  BatchingConfig
	BandwidthManager BandwidthConfig
	AdaptiveTimeouts TimeoutConfig
}

// DefaultOptimizationConfig returns sensible defaults for all optimizations
func DefaultOptimizationConfig() *OptimizationConfig {
	return &OptimizationConfig{
		EnableConnectionPool:   true,
		EnableMessageBatching:  true,
		EnableBandwidthManager: true,
		EnableAdaptiveTimeouts: true,

		ConnectionPool:   DefaultConnectionPoolConfig(),
		MessageBatching:  DefaultBatchingConfig(),
		BandwidthManager: DefaultBandwidthConfig(),
		AdaptiveTimeouts: DefaultTimeoutConfig(),
	}
}

// NewOptimizedNetwork creates a new optimized network wrapper
func NewOptimizedNetwork(baseNetwork *networkpkg.HostWrapper, config *OptimizationConfig) (*OptimizedNetwork, error) {
	if config == nil {
		config = DefaultOptimizationConfig()
	}

	// Initialize metrics
	netMetrics := metrics.NewComponentMetrics("optimized_network")
	metrics.RegisterGlobalComponent(netMetrics)

	on := &OptimizedNetwork{
		baseNetwork: baseNetwork,
		metrics:     netMetrics,
		config:      config,
	}

	// Initialize optimization components
	if err := on.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize optimization components: %w", err)
	}

	return on, nil
}

// initializeComponents sets up the optimization components
func (on *OptimizedNetwork) initializeComponents() error {
	// Initialize connection pool
	if on.config.EnableConnectionPool {
		on.connectionPool = NewConnectionPool(on.baseNetwork.Host, on.config.ConnectionPool)
	}

	// Initialize message batcher
	if on.config.EnableMessageBatching {
		on.messageBatcher = NewMessageBatcher(on.config.MessageBatching)
	}

	// Initialize bandwidth manager
	if on.config.EnableBandwidthManager {
		on.bandwidthManager = NewBandwidthManager(on.config.BandwidthManager)
	}

	// Initialize adaptive timeouts
	if on.config.EnableAdaptiveTimeouts {
		on.adaptiveTimeouts = NewAdaptiveTimeouts(on.config.AdaptiveTimeouts)
	}

	return nil
}

// Send sends data to a peer with optimizations applied
func (on *OptimizedNetwork) Send(ctx context.Context, peerID peer.ID, data []byte) error {
	start := time.Now()
	on.metrics.RecordRequest()

	// Check bandwidth availability
	if on.config.EnableBandwidthManager {
		if !on.bandwidthManager.RequestBandwidth(peerID, TrafficClassNormal, DirectionUpload, int64(len(data))) {
			on.metrics.RecordFailure(time.Since(start), "bandwidth_unavailable")
			return fmt.Errorf("bandwidth unavailable")
		}
	}

	// Get appropriate timeout
	var timeout time.Duration
	if on.config.EnableAdaptiveTimeouts {
		timeout = on.adaptiveTimeouts.GetTimeout(peerID, "send")
	} else {
		timeout = 10 * time.Second // Default timeout
	}

	// Create context with timeout
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use message batching if enabled and message is small enough
	if on.config.EnableMessageBatching && len(data) < 32*1024 { // 32KB threshold
		return on.sendWithBatching(sendCtx, peerID, data)
	}

	// Send directly
	return on.sendDirect(sendCtx, peerID, data)
}

// sendWithBatching sends data using message batching
func (on *OptimizedNetwork) sendWithBatching(ctx context.Context, peerID peer.ID, data []byte) error {
	done := make(chan error, 1)

	msg := BatchedMessage{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Data:     data,
		Priority: PriorityNormal,
		Callback: func(err error) {
			done <- err
		},
	}

	if err := on.messageBatcher.QueueMessage(peerID, msg); err != nil {
		return fmt.Errorf("failed to queue message: %w", err)
	}

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			// Record timeout if applicable
			if on.config.EnableAdaptiveTimeouts {
				on.adaptiveTimeouts.RecordTimeout(peerID, on.adaptiveTimeouts.GetTimeout(peerID, "send"))
			}
			return err
		}
		// Record success
		if on.config.EnableAdaptiveTimeouts {
			on.adaptiveTimeouts.RecordSuccess(peerID, time.Since(time.Now()))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendDirect sends data directly without batching
func (on *OptimizedNetwork) sendDirect(ctx context.Context, peerID peer.ID, data []byte) error {
	start := time.Now()

	// Get stream (using connection pool if available)
	var stream network.Stream
	var err error

	if on.config.EnableConnectionPool {
		stream, err = on.connectionPool.GetStream(ctx, peerID, protocol.ID("/optimized/send/1.0.0"))
	} else {
		stream, err = on.baseNetwork.NewStream(ctx, peerID, protocol.ID("/optimized/send/1.0.0"))
	}

	if err != nil {
		if on.config.EnableAdaptiveTimeouts {
			on.adaptiveTimeouts.RecordTimeout(peerID, on.adaptiveTimeouts.GetTimeout(peerID, "send"))
		}
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Send data
	_, err = stream.Write(data)
	if err != nil {
		if on.config.EnableAdaptiveTimeouts {
			on.adaptiveTimeouts.RecordTimeout(peerID, on.adaptiveTimeouts.GetTimeout(peerID, "send"))
		}
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Record successful operation
	if on.config.EnableAdaptiveTimeouts {
		on.adaptiveTimeouts.RecordSuccess(peerID, time.Since(start))
		on.adaptiveTimeouts.RecordRTT(peerID, time.Since(start))
	}

	return nil
}

// ConnectToPeer connects to a peer with optimizations
func (on *OptimizedNetwork) ConnectToPeer(ctx context.Context, addrs ...multiaddr.Multiaddr) error {
	start := time.Now()
	on.metrics.RecordRequest()

	if on.config.EnableConnectionPool {
		err := on.connectionPool.ConnectToPeer(ctx, addrs...)
		if err != nil {
			on.metrics.RecordFailure(time.Since(start), "connection_failed")
			return err
		}
	} else {
		err := on.baseNetwork.ConnectToPeer(ctx, addrs...)
		if err != nil {
			on.metrics.RecordFailure(time.Since(start), "connection_failed")
			return err
		}
	}

	on.metrics.RecordSuccess(time.Since(start), 0)
	return nil
}

// GetStream returns an optimized stream to a peer
func (on *OptimizedNetwork) GetStream(ctx context.Context, peerID peer.ID, proto protocol.ID) (network.Stream, error) {
	start := time.Now()
	on.metrics.RecordRequest()

	if on.config.EnableConnectionPool {
		stream, err := on.connectionPool.GetStream(ctx, peerID, proto)
		if err != nil {
			on.metrics.RecordFailure(time.Since(start), "stream_creation_failed")
			return nil, err
		}
		on.metrics.RecordSuccess(time.Since(start), 0)
		return stream, nil
	}

	// Fallback to base network
	stream, err := on.baseNetwork.NewStream(ctx, peerID, proto)
	if err != nil {
		on.metrics.RecordFailure(time.Since(start), "stream_creation_failed")
		return nil, err
	}

	on.metrics.RecordSuccess(time.Since(start), 0)
	return stream, nil
}

// Flush forces all pending optimizations to complete
func (on *OptimizedNetwork) Flush() {
	if on.config.EnableMessageBatching && on.messageBatcher != nil {
		on.messageBatcher.Flush()
	}
}

// GetOptimizationStats returns statistics for all optimization components
func (on *OptimizedNetwork) GetOptimizationStats() OptimizationStats {
	stats := OptimizationStats{
		Enabled: OptimizationStatus{
			ConnectionPool:   on.config.EnableConnectionPool,
			MessageBatching:  on.config.EnableMessageBatching,
			BandwidthManager: on.config.EnableBandwidthManager,
			AdaptiveTimeouts: on.config.EnableAdaptiveTimeouts,
		},
	}

	if on.config.EnableConnectionPool && on.connectionPool != nil {
		stats.ConnectionPool = on.connectionPool.GetStats()
	}

	if on.config.EnableMessageBatching && on.messageBatcher != nil {
		stats.MessageBatching = on.messageBatcher.GetStats()
	}

	if on.config.EnableBandwidthManager && on.bandwidthManager != nil {
		stats.BandwidthManager = on.bandwidthManager.GetStats()
	}

	if on.config.EnableAdaptiveTimeouts && on.adaptiveTimeouts != nil {
		stats.AdaptiveTimeouts = on.adaptiveTimeouts.GetStats()
	}

	return stats
}

// OptimizationStats provides comprehensive optimization statistics
type OptimizationStats struct {
	Enabled          OptimizationStatus
	ConnectionPool   ConnectionPoolStats
	MessageBatching  BatchingStats
	BandwidthManager BandwidthStats
	AdaptiveTimeouts AdaptiveTimeoutStats
}

// OptimizationStatus shows which optimizations are enabled
type OptimizationStatus struct {
	ConnectionPool   bool
	MessageBatching  bool
	BandwidthManager bool
	AdaptiveTimeouts bool
}

// GetMetrics returns combined metrics from all components
func (on *OptimizedNetwork) GetMetrics() map[string]metrics.MetricsSnapshot {
	result := make(map[string]metrics.MetricsSnapshot)

	// Add base network metrics
	result["optimized_network"] = on.metrics.GetSnapshot()

	// Add component metrics
	if on.config.EnableConnectionPool && on.connectionPool != nil {
		result["connection_pool"] = on.connectionPool.GetMetrics()
	}

	if on.config.EnableMessageBatching && on.messageBatcher != nil {
		result["message_batching"] = on.messageBatcher.GetMetrics()
	}

	if on.config.EnableBandwidthManager && on.bandwidthManager != nil {
		result["bandwidth_manager"] = on.bandwidthManager.GetMetrics()
	}

	if on.config.EnableAdaptiveTimeouts && on.adaptiveTimeouts != nil {
		result["adaptive_timeouts"] = on.adaptiveTimeouts.GetMetrics()
	}

	return result
}

// UpdateConfiguration updates the optimization configuration
func (on *OptimizedNetwork) UpdateConfiguration(config *OptimizationConfig) error {
	on.mu.Lock()
	defer on.mu.Unlock()

	// Store old config for rollback if needed
	oldConfig := on.config

	// Update configuration
	on.config = config

	// Reinitialize components with new config
	if err := on.initializeComponents(); err != nil {
		// Rollback on error
		on.config = oldConfig
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	return nil
}

// ID returns the peer ID of the underlying host
func (on *OptimizedNetwork) ID() peer.ID {
	return on.baseNetwork.ID()
}

// Addrs returns the addresses of the underlying host
func (on *OptimizedNetwork) Addrs() []multiaddr.Multiaddr {
	return on.baseNetwork.Addrs()
}

// Peers returns the connected peers
func (on *OptimizedNetwork) Peers() []peer.ID {
	return on.baseNetwork.Peers()
}

// Close shuts down the optimized network and all its components
func (on *OptimizedNetwork) Close() error {
	var errors []error

	// Close optimization components
	if on.connectionPool != nil {
		if err := on.connectionPool.Close(); err != nil {
			errors = append(errors, fmt.Errorf("connection pool close: %w", err))
		}
	}

	if on.messageBatcher != nil {
		if err := on.messageBatcher.Close(); err != nil {
			errors = append(errors, fmt.Errorf("message batcher close: %w", err))
		}
	}

	if on.bandwidthManager != nil {
		if err := on.bandwidthManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("bandwidth manager close: %w", err))
		}
	}

	if on.adaptiveTimeouts != nil {
		if err := on.adaptiveTimeouts.Close(); err != nil {
			errors = append(errors, fmt.Errorf("adaptive timeouts close: %w", err))
		}
	}

	// Close base network
	if err := on.baseNetwork.Close(); err != nil {
		errors = append(errors, fmt.Errorf("base network close: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple close errors: %v", errors)
	}

	return nil
}