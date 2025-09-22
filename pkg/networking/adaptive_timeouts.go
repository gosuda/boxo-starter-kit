package networking

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// AdaptiveTimeouts dynamically adjusts timeout values based on network conditions
type AdaptiveTimeouts struct {
	metrics *metrics.ComponentMetrics
	config  TimeoutConfig

	mu        sync.RWMutex
	peerStats map[peer.ID]*peerTimeoutStats
	global    *globalTimeoutStats

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// TimeoutConfig defines adaptive timeout parameters
type TimeoutConfig struct {
	MinTimeout         time.Duration // Minimum timeout value
	MaxTimeout         time.Duration // Maximum timeout value
	InitialTimeout     time.Duration // Initial timeout for new peers
	RTTMultiplier      float64       // Multiplier for RTT-based timeout calculation
	VarianceMultiplier float64       // Multiplier for RTT variance
	AdaptationRate     float64       // How quickly to adapt (0-1)
	DecayRate          float64       // How quickly old samples decay (0-1)
	SampleWindowSize   int           // Number of samples to keep for calculation
	CleanupInterval    time.Duration // How often to clean up old peer stats
	PeerTimeoutTTL     time.Duration // How long to keep peer stats
}

// DefaultTimeoutConfig returns sensible defaults
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		MinTimeout:         100 * time.Millisecond,
		MaxTimeout:         30 * time.Second,
		InitialTimeout:     5 * time.Second,
		RTTMultiplier:      2.0,
		VarianceMultiplier: 4.0,
		AdaptationRate:     0.1,
		DecayRate:          0.95,
		SampleWindowSize:   20,
		CleanupInterval:    10 * time.Minute,
		PeerTimeoutTTL:     30 * time.Minute,
	}
}

// TimeoutStrategy defines different timeout calculation strategies
type TimeoutStrategy int

const (
	StrategyFixed TimeoutStrategy = iota
	StrategyRTTBased
	StrategyAdaptive
	StrategyAggressive
	StrategyConservative
)

// peerTimeoutStats tracks timeout statistics for a specific peer
type peerTimeoutStats struct {
	peer         peer.ID
	rttSamples   []time.Duration
	timeouts     []time.Duration
	successCount int64
	failureCount int64
	lastSeen     time.Time
	currentRTT   time.Duration
	rttVariance  time.Duration
	strategy     TimeoutStrategy
}

// globalTimeoutStats tracks global timeout statistics
type globalTimeoutStats struct {
	averageRTT     time.Duration
	globalVariance time.Duration
	totalRequests  int64
	totalTimeouts  int64
	lastUpdate     time.Time
}

// NewAdaptiveTimeouts creates a new adaptive timeout manager
func NewAdaptiveTimeouts(config TimeoutConfig) *AdaptiveTimeouts {
	ctx, cancel := context.WithCancel(context.Background())

	timeoutMetrics := metrics.NewComponentMetrics("adaptive_timeouts")
	metrics.RegisterGlobalComponent(timeoutMetrics)

	at := &AdaptiveTimeouts{
		metrics:   timeoutMetrics,
		config:    config,
		peerStats: make(map[peer.ID]*peerTimeoutStats),
		global: &globalTimeoutStats{
			averageRTT: config.InitialTimeout / 2,
			lastUpdate: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Start cleanup worker
	at.wg.Add(1)
	go at.cleanupWorker()

	return at
}

// GetTimeout returns the appropriate timeout for a peer and operation
func (at *AdaptiveTimeouts) GetTimeout(peerID peer.ID, operation string) time.Duration {
	start := time.Now()
	at.metrics.RecordRequest()

	at.mu.RLock()
	stats, exists := at.peerStats[peerID]
	at.mu.RUnlock()

	var timeout time.Duration

	if !exists {
		// New peer, use initial timeout
		timeout = at.config.InitialTimeout
		at.initializePeer(peerID)
	} else {
		// Calculate timeout based on peer's history
		timeout = at.calculateTimeout(stats)
	}

	// Ensure timeout is within bounds
	if timeout < at.config.MinTimeout {
		timeout = at.config.MinTimeout
	} else if timeout > at.config.MaxTimeout {
		timeout = at.config.MaxTimeout
	}

	at.metrics.RecordSuccess(time.Since(start), int64(timeout))
	return timeout
}

// RecordRTT records a round-trip time measurement for a peer
func (at *AdaptiveTimeouts) RecordRTT(peerID peer.ID, rtt time.Duration) {
	start := time.Now()
	at.metrics.RecordRequest()

	at.mu.Lock()
	defer at.mu.Unlock()

	stats, exists := at.peerStats[peerID]
	if !exists {
		stats = at.createPeerStats(peerID)
		at.peerStats[peerID] = stats
	}

	// Add RTT sample
	stats.rttSamples = append(stats.rttSamples, rtt)
	if len(stats.rttSamples) > at.config.SampleWindowSize {
		stats.rttSamples = stats.rttSamples[1:]
	}

	// Update current RTT using exponential moving average
	if stats.currentRTT == 0 {
		stats.currentRTT = rtt
	} else {
		alpha := at.config.AdaptationRate
		stats.currentRTT = time.Duration(float64(stats.currentRTT)*(1-alpha) + float64(rtt)*alpha)
	}

	// Calculate RTT variance
	stats.rttVariance = at.calculateRTTVariance(stats.rttSamples)
	stats.lastSeen = time.Now()

	// Update global statistics
	at.updateGlobalStats(rtt)

	at.metrics.RecordSuccess(time.Since(start), int64(rtt))
}

// RecordSuccess records a successful operation for a peer
func (at *AdaptiveTimeouts) RecordSuccess(peerID peer.ID, duration time.Duration) {
	at.mu.Lock()
	defer at.mu.Unlock()

	stats, exists := at.peerStats[peerID]
	if !exists {
		stats = at.createPeerStats(peerID)
		at.peerStats[peerID] = stats
	}

	stats.successCount++
	stats.lastSeen = time.Now()

	// If operation completed faster than expected, consider it for RTT
	if duration < stats.currentRTT*2 {
		at.RecordRTT(peerID, duration)
	}
}

// RecordTimeout records a timeout for a peer
func (at *AdaptiveTimeouts) RecordTimeout(peerID peer.ID, timeoutValue time.Duration) {
	start := time.Now()
	at.metrics.RecordRequest()

	at.mu.Lock()
	defer at.mu.Unlock()

	stats, exists := at.peerStats[peerID]
	if !exists {
		stats = at.createPeerStats(peerID)
		at.peerStats[peerID] = stats
	}

	stats.failureCount++
	stats.timeouts = append(stats.timeouts, timeoutValue)
	if len(stats.timeouts) > at.config.SampleWindowSize {
		stats.timeouts = stats.timeouts[1:]
	}
	stats.lastSeen = time.Now()

	// Adapt strategy based on failure rate
	failureRate := float64(stats.failureCount) / float64(stats.successCount+stats.failureCount)
	if failureRate > 0.3 {
		// High failure rate, switch to conservative strategy
		stats.strategy = StrategyConservative
	} else if failureRate > 0.1 {
		// Moderate failure rate, use adaptive strategy
		stats.strategy = StrategyAdaptive
	}

	// Update global timeout statistics
	at.global.totalTimeouts++

	at.metrics.RecordFailure(time.Since(start), "timeout_recorded")
}

// SetStrategy sets the timeout strategy for a specific peer
func (at *AdaptiveTimeouts) SetStrategy(peerID peer.ID, strategy TimeoutStrategy) {
	at.mu.Lock()
	defer at.mu.Unlock()

	stats, exists := at.peerStats[peerID]
	if !exists {
		stats = at.createPeerStats(peerID)
		at.peerStats[peerID] = stats
	}

	stats.strategy = strategy
}

// initializePeer creates initial stats for a new peer
func (at *AdaptiveTimeouts) initializePeer(peerID peer.ID) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if _, exists := at.peerStats[peerID]; !exists {
		at.peerStats[peerID] = at.createPeerStats(peerID)
	}
}

// createPeerStats creates new peer statistics
func (at *AdaptiveTimeouts) createPeerStats(peerID peer.ID) *peerTimeoutStats {
	return &peerTimeoutStats{
		peer:       peerID,
		rttSamples: make([]time.Duration, 0, at.config.SampleWindowSize),
		timeouts:   make([]time.Duration, 0, at.config.SampleWindowSize),
		lastSeen:   time.Now(),
		currentRTT: at.config.InitialTimeout / 2,
		strategy:   StrategyAdaptive,
	}
}

// calculateTimeout computes the appropriate timeout for a peer
func (at *AdaptiveTimeouts) calculateTimeout(stats *peerTimeoutStats) time.Duration {
	switch stats.strategy {
	case StrategyFixed:
		return at.config.InitialTimeout

	case StrategyRTTBased:
		if stats.currentRTT > 0 {
			return time.Duration(float64(stats.currentRTT) * at.config.RTTMultiplier)
		}
		return at.config.InitialTimeout

	case StrategyAdaptive:
		return at.calculateAdaptiveTimeout(stats)

	case StrategyAggressive:
		// Aggressive: Use minimum viable timeout
		if stats.currentRTT > 0 {
			return time.Duration(float64(stats.currentRTT) * 1.5)
		}
		return at.config.MinTimeout * 2

	case StrategyConservative:
		// Conservative: Use larger timeout to avoid failures
		if stats.currentRTT > 0 {
			timeout := time.Duration(float64(stats.currentRTT) * at.config.RTTMultiplier * 2)
			if stats.rttVariance > 0 {
				timeout += time.Duration(float64(stats.rttVariance) * at.config.VarianceMultiplier)
			}
			return timeout
		}
		return at.config.InitialTimeout * 2

	default:
		return at.config.InitialTimeout
	}
}

// calculateAdaptiveTimeout uses RTT and variance for adaptive timeout calculation
func (at *AdaptiveTimeouts) calculateAdaptiveTimeout(stats *peerTimeoutStats) time.Duration {
	if stats.currentRTT == 0 {
		return at.config.InitialTimeout
	}

	// Base timeout from RTT
	timeout := time.Duration(float64(stats.currentRTT) * at.config.RTTMultiplier)

	// Add variance component
	if stats.rttVariance > 0 {
		varianceComponent := time.Duration(float64(stats.rttVariance) * at.config.VarianceMultiplier)
		timeout += varianceComponent
	}

	// Adjust based on success/failure ratio
	if stats.successCount+stats.failureCount > 10 {
		successRate := float64(stats.successCount) / float64(stats.successCount+stats.failureCount)
		if successRate < 0.8 {
			// Low success rate, increase timeout
			multiplier := 1.0 + (0.8-successRate)*2.0
			timeout = time.Duration(float64(timeout) * multiplier)
		} else if successRate > 0.95 {
			// High success rate, can be more aggressive
			multiplier := 0.8 + successRate*0.2
			timeout = time.Duration(float64(timeout) * multiplier)
		}
	}

	return timeout
}

// calculateRTTVariance computes the variance of RTT samples
func (at *AdaptiveTimeouts) calculateRTTVariance(samples []time.Duration) time.Duration {
	if len(samples) < 2 {
		return 0
	}

	// Calculate mean
	var sum time.Duration
	for _, sample := range samples {
		sum += sample
	}
	mean := sum / time.Duration(len(samples))

	// Calculate variance
	var variance float64
	for _, sample := range samples {
		diff := float64(sample - mean)
		variance += diff * diff
	}
	variance /= float64(len(samples))

	return time.Duration(math.Sqrt(variance))
}

// updateGlobalStats updates global timeout statistics
func (at *AdaptiveTimeouts) updateGlobalStats(rtt time.Duration) {
	// Update global average RTT using exponential moving average
	if at.global.averageRTT == 0 {
		at.global.averageRTT = rtt
	} else {
		alpha := at.config.AdaptationRate / 10 // Slower adaptation for global stats
		at.global.averageRTT = time.Duration(float64(at.global.averageRTT)*(1-alpha) + float64(rtt)*alpha)
	}

	at.global.totalRequests++
	at.global.lastUpdate = time.Now()
}

// cleanupWorker periodically removes old peer statistics
func (at *AdaptiveTimeouts) cleanupWorker() {
	defer at.wg.Done()

	ticker := time.NewTicker(at.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			at.cleanup()
		case <-at.ctx.Done():
			return
		}
	}
}

// cleanup removes stale peer statistics
func (at *AdaptiveTimeouts) cleanup() {
	at.mu.Lock()
	defer at.mu.Unlock()

	now := time.Now()
	for peerID, stats := range at.peerStats {
		if now.Sub(stats.lastSeen) > at.config.PeerTimeoutTTL {
			delete(at.peerStats, peerID)
		}
	}
}

// GetStats returns current adaptive timeout statistics
func (at *AdaptiveTimeouts) GetStats() AdaptiveTimeoutStats {
	at.mu.RLock()
	defer at.mu.RUnlock()

	stats := AdaptiveTimeoutStats{
		TrackedPeers:     len(at.peerStats),
		GlobalAverageRTT: at.global.averageRTT,
		TotalRequests:    at.global.totalRequests,
		TotalTimeouts:    at.global.totalTimeouts,
		PeerStats:        make(map[peer.ID]PeerTimeoutSummary),
	}

	// Calculate timeout rate
	if at.global.totalRequests > 0 {
		stats.TimeoutRate = float64(at.global.totalTimeouts) / float64(at.global.totalRequests)
	}

	// Collect per-peer summaries
	for peerID, peerStats := range at.peerStats {
		summary := PeerTimeoutSummary{
			CurrentRTT:   peerStats.currentRTT,
			RTTVariance:  peerStats.rttVariance,
			SuccessCount: peerStats.successCount,
			FailureCount: peerStats.failureCount,
			Strategy:     peerStats.strategy,
			LastSeen:     peerStats.lastSeen,
		}

		if peerStats.successCount+peerStats.failureCount > 0 {
			summary.SuccessRate = float64(peerStats.successCount) / float64(peerStats.successCount+peerStats.failureCount)
		}

		stats.PeerStats[peerID] = summary
	}

	return stats
}

// AdaptiveTimeoutStats provides timeout statistics
type AdaptiveTimeoutStats struct {
	TrackedPeers     int
	GlobalAverageRTT time.Duration
	TotalRequests    int64
	TotalTimeouts    int64
	TimeoutRate      float64
	PeerStats        map[peer.ID]PeerTimeoutSummary
}

// PeerTimeoutSummary provides per-peer timeout statistics
type PeerTimeoutSummary struct {
	CurrentRTT   time.Duration
	RTTVariance  time.Duration
	SuccessCount int64
	FailureCount int64
	SuccessRate  float64
	Strategy     TimeoutStrategy
	LastSeen     time.Time
}

// GetMetrics returns the current metrics for this adaptive timeout manager
func (at *AdaptiveTimeouts) GetMetrics() metrics.MetricsSnapshot {
	return at.metrics.GetSnapshot()
}

// Close shuts down the adaptive timeout manager
func (at *AdaptiveTimeouts) Close() error {
	at.cancel()
	at.wg.Wait()
	return nil
}
