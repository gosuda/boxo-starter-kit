package networking

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// BandwidthManager controls and monitors network bandwidth usage
type BandwidthManager struct {
	metrics *metrics.ComponentMetrics
	config  BandwidthConfig

	// Bandwidth tracking
	uploadUsed   int64 // bytes per second
	downloadUsed int64 // bytes per second

	// Traffic shaping
	uploadTokens   chan struct{}
	downloadTokens chan struct{}

	// QoS queues
	mu           sync.RWMutex
	qosQueues    map[TrafficClass]*trafficQueue
	peerLimits   map[peer.ID]*peerBandwidth
	globalLimits *bandwidthLimits

	// Background workers
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// BandwidthConfig defines bandwidth management parameters
type BandwidthConfig struct {
	MaxUpload       int64         // bytes per second
	MaxDownload     int64         // bytes per second
	QoSEnabled      bool          // Enable Quality of Service
	TokenRefillRate time.Duration // How often to refill token buckets
	BurstSize       int64         // Maximum burst size in bytes
	PeerLimitRatio  float64       // Max bandwidth per peer as ratio of total

	// QoS class configurations
	HighPriorityRatio   float64 // Bandwidth reserved for high priority traffic
	NormalPriorityRatio float64 // Bandwidth reserved for normal priority traffic
	LowPriorityRatio    float64 // Bandwidth reserved for low priority traffic
}

// DefaultBandwidthConfig returns sensible defaults
func DefaultBandwidthConfig() BandwidthConfig {
	return BandwidthConfig{
		MaxUpload:           10 * 1024 * 1024, // 10 MB/s
		MaxDownload:         50 * 1024 * 1024, // 50 MB/s
		QoSEnabled:          true,
		TokenRefillRate:     100 * time.Millisecond,
		BurstSize:           1024 * 1024, // 1 MB
		PeerLimitRatio:      0.1,         // 10% per peer max
		HighPriorityRatio:   0.4,         // 40% for high priority
		NormalPriorityRatio: 0.5,         // 50% for normal priority
		LowPriorityRatio:    0.1,         // 10% for low priority
	}
}

// TrafficClass defines different classes of network traffic
type TrafficClass int

const (
	TrafficClassLow TrafficClass = iota
	TrafficClassNormal
	TrafficClassHigh
	TrafficClassSystem // For critical system messages
)

// Direction specifies traffic direction
type Direction int

const (
	DirectionUpload Direction = iota
	DirectionDownload
	DirectionBoth
)

// trafficQueue manages bandwidth for a specific traffic class
type trafficQueue struct {
	class     TrafficClass
	tokens    chan struct{}
	allocated int64
	used      int64
	requests  chan bandwidthRequest
}

// peerBandwidth tracks bandwidth usage per peer
type peerBandwidth struct {
	peer          peer.ID
	uploadUsed    int64
	downloadUsed  int64
	uploadLimit   int64
	downloadLimit int64
	lastUpdate    time.Time
}

// bandwidthLimits tracks global bandwidth limits
type bandwidthLimits struct {
	uploadLimit   int64
	downloadLimit int64
	uploadUsed    int64
	downloadUsed  int64
	window        time.Duration
	lastReset     time.Time
}

// bandwidthRequest represents a request for bandwidth allocation
type bandwidthRequest struct {
	peer      peer.ID
	class     TrafficClass
	direction Direction
	bytes     int64
	response  chan bool
}

// NewBandwidthManager creates a new bandwidth manager
func NewBandwidthManager(config BandwidthConfig) *BandwidthManager {
	ctx, cancel := context.WithCancel(context.Background())

	bwMetrics := metrics.NewComponentMetrics("bandwidth_manager")
	metrics.RegisterGlobalComponent(bwMetrics)

	bm := &BandwidthManager{
		metrics:        bwMetrics,
		config:         config,
		uploadTokens:   make(chan struct{}, int(config.BurstSize/1024)),
		downloadTokens: make(chan struct{}, int(config.BurstSize/1024)),
		qosQueues:      make(map[TrafficClass]*trafficQueue),
		peerLimits:     make(map[peer.ID]*peerBandwidth),
		ctx:            ctx,
		cancel:         cancel,
		globalLimits: &bandwidthLimits{
			uploadLimit:   config.MaxUpload,
			downloadLimit: config.MaxDownload,
			window:        time.Second,
			lastReset:     time.Now(),
		},
	}

	// Initialize QoS queues if enabled
	if config.QoSEnabled {
		bm.initQoSQueues()
	}

	// Start background workers
	bm.wg.Add(3)
	go bm.tokenRefiller()
	go bm.bandwidthTracker()
	go bm.qosScheduler()

	return bm
}

// RequestBandwidth attempts to allocate bandwidth for a transfer
func (bm *BandwidthManager) RequestBandwidth(peerID peer.ID, class TrafficClass, direction Direction, bytes int64) bool {
	start := time.Now()
	bm.metrics.RecordRequest()

	// Check global limits first
	if !bm.checkGlobalLimits(direction, bytes) {
		bm.metrics.RecordFailure(time.Since(start), "global_limit_exceeded")
		return false
	}

	// Check per-peer limits
	if !bm.checkPeerLimits(peerID, direction, bytes) {
		bm.metrics.RecordFailure(time.Since(start), "peer_limit_exceeded")
		return false
	}

	// Handle QoS if enabled
	if bm.config.QoSEnabled {
		if !bm.requestQoSBandwidth(peerID, class, direction, bytes) {
			bm.metrics.RecordFailure(time.Since(start), "qos_rejected")
			return false
		}
	} else {
		// Simple token bucket for non-QoS
		if !bm.requestTokens(direction, bytes) {
			bm.metrics.RecordFailure(time.Since(start), "tokens_unavailable")
			return false
		}
	}

	// Update usage counters
	bm.updateUsage(peerID, direction, bytes)

	bm.metrics.RecordSuccess(time.Since(start), bytes)
	return true
}

// checkGlobalLimits verifies we haven't exceeded global bandwidth limits
func (bm *BandwidthManager) checkGlobalLimits(direction Direction, bytes int64) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	now := time.Now()
	if now.Sub(bm.globalLimits.lastReset) >= bm.globalLimits.window {
		// Reset counters
		bm.globalLimits.uploadUsed = 0
		bm.globalLimits.downloadUsed = 0
		bm.globalLimits.lastReset = now
	}

	switch direction {
	case DirectionUpload:
		return bm.globalLimits.uploadUsed+bytes <= bm.globalLimits.uploadLimit
	case DirectionDownload:
		return bm.globalLimits.downloadUsed+bytes <= bm.globalLimits.downloadLimit
	case DirectionBoth:
		return (bm.globalLimits.uploadUsed+bytes <= bm.globalLimits.uploadLimit) &&
			(bm.globalLimits.downloadUsed+bytes <= bm.globalLimits.downloadLimit)
	}
	return false
}

// checkPeerLimits verifies per-peer bandwidth limits
func (bm *BandwidthManager) checkPeerLimits(peerID peer.ID, direction Direction, bytes int64) bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	peerBW, exists := bm.peerLimits[peerID]
	if !exists {
		// Create new peer bandwidth tracking
		peerBW = &peerBandwidth{
			peer:          peerID,
			uploadLimit:   int64(float64(bm.config.MaxUpload) * bm.config.PeerLimitRatio),
			downloadLimit: int64(float64(bm.config.MaxDownload) * bm.config.PeerLimitRatio),
			lastUpdate:    time.Now(),
		}
		bm.peerLimits[peerID] = peerBW
	}

	// Reset counters if enough time has passed
	now := time.Now()
	if now.Sub(peerBW.lastUpdate) >= time.Second {
		peerBW.uploadUsed = 0
		peerBW.downloadUsed = 0
		peerBW.lastUpdate = now
	}

	switch direction {
	case DirectionUpload:
		return peerBW.uploadUsed+bytes <= peerBW.uploadLimit
	case DirectionDownload:
		return peerBW.downloadUsed+bytes <= peerBW.downloadLimit
	case DirectionBoth:
		return (peerBW.uploadUsed+bytes <= peerBW.uploadLimit) &&
			(peerBW.downloadUsed+bytes <= peerBW.downloadLimit)
	}
	return false
}

// requestQoSBandwidth handles QoS bandwidth allocation
func (bm *BandwidthManager) requestQoSBandwidth(peerID peer.ID, class TrafficClass, direction Direction, bytes int64) bool {
	bm.mu.RLock()
	queue, exists := bm.qosQueues[class]
	bm.mu.RUnlock()

	if !exists {
		return false
	}

	// Send request to QoS queue
	request := bandwidthRequest{
		peer:      peerID,
		class:     class,
		direction: direction,
		bytes:     bytes,
		response:  make(chan bool, 1),
	}

	select {
	case queue.requests <- request:
		// Wait for response
		select {
		case approved := <-request.response:
			return approved
		case <-time.After(100 * time.Millisecond):
			return false // Timeout
		}
	default:
		return false // Queue full
	}
}

// requestTokens attempts to acquire tokens from token buckets
func (bm *BandwidthManager) requestTokens(direction Direction, bytes int64) bool {
	tokensNeeded := int(bytes / 1024) // 1 token per KB
	if tokensNeeded == 0 {
		tokensNeeded = 1
	}

	var tokens chan struct{}
	switch direction {
	case DirectionUpload:
		tokens = bm.uploadTokens
	case DirectionDownload:
		tokens = bm.downloadTokens
	default:
		return false
	}

	// Try to acquire tokens (non-blocking)
	for i := 0; i < tokensNeeded; i++ {
		select {
		case <-tokens:
			// Token acquired
		default:
			// No tokens available
			return false
		}
	}

	return true
}

// updateUsage updates bandwidth usage counters
func (bm *BandwidthManager) updateUsage(peerID peer.ID, direction Direction, bytes int64) {
	// Update global counters
	switch direction {
	case DirectionUpload:
		atomic.AddInt64(&bm.uploadUsed, bytes)
		bm.mu.Lock()
		bm.globalLimits.uploadUsed += bytes
		bm.mu.Unlock()
	case DirectionDownload:
		atomic.AddInt64(&bm.downloadUsed, bytes)
		bm.mu.Lock()
		bm.globalLimits.downloadUsed += bytes
		bm.mu.Unlock()
	}

	// Update per-peer counters
	bm.mu.Lock()
	if peerBW, exists := bm.peerLimits[peerID]; exists {
		switch direction {
		case DirectionUpload:
			peerBW.uploadUsed += bytes
		case DirectionDownload:
			peerBW.downloadUsed += bytes
		}
	}
	bm.mu.Unlock()
}

// initQoSQueues initializes Quality of Service queues
func (bm *BandwidthManager) initQoSQueues() {
	classes := []TrafficClass{TrafficClassLow, TrafficClassNormal, TrafficClassHigh, TrafficClassSystem}
	ratios := []float64{bm.config.LowPriorityRatio, bm.config.NormalPriorityRatio, bm.config.HighPriorityRatio, 0.1}

	for i, class := range classes {
		allocated := int64(float64(bm.config.MaxUpload+bm.config.MaxDownload) * ratios[i])
		queue := &trafficQueue{
			class:     class,
			tokens:    make(chan struct{}, int(allocated/1024)),
			allocated: allocated,
			requests:  make(chan bandwidthRequest, 100),
		}
		bm.qosQueues[class] = queue
	}
}

// tokenRefiller periodically refills token buckets
func (bm *BandwidthManager) tokenRefiller() {
	defer bm.wg.Done()

	ticker := time.NewTicker(bm.config.TokenRefillRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bm.refillTokens()
		case <-bm.ctx.Done():
			return
		}
	}
}

// refillTokens adds tokens to all buckets
func (bm *BandwidthManager) refillTokens() {
	// Calculate tokens to add based on rate and interval
	interval := bm.config.TokenRefillRate.Seconds()
	uploadTokens := int(float64(bm.config.MaxUpload) * interval / 1024)
	downloadTokens := int(float64(bm.config.MaxDownload) * interval / 1024)

	// Refill upload tokens
	for i := 0; i < uploadTokens; i++ {
		select {
		case bm.uploadTokens <- struct{}{}:
		default:
			// Bucket full
			break
		}
	}

	// Refill download tokens
	for i := 0; i < downloadTokens; i++ {
		select {
		case bm.downloadTokens <- struct{}{}:
		default:
			// Bucket full
			break
		}
	}

	// Refill QoS queue tokens
	if bm.config.QoSEnabled {
		bm.mu.RLock()
		for _, queue := range bm.qosQueues {
			queueTokens := int(float64(queue.allocated) * interval / 1024)
			for i := 0; i < queueTokens; i++ {
				select {
				case queue.tokens <- struct{}{}:
				default:
					// Queue full
					break
				}
			}
		}
		bm.mu.RUnlock()
	}
}

// bandwidthTracker monitors bandwidth usage and resets counters
func (bm *BandwidthManager) bandwidthTracker() {
	defer bm.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Reset per-second counters
			atomic.StoreInt64(&bm.uploadUsed, 0)
			atomic.StoreInt64(&bm.downloadUsed, 0)
		case <-bm.ctx.Done():
			return
		}
	}
}

// qosScheduler handles QoS bandwidth requests
func (bm *BandwidthManager) qosScheduler() {
	defer bm.wg.Done()

	for {
		select {
		case <-bm.ctx.Done():
			return
		default:
			// Process requests from all QoS queues in priority order
			bm.processQoSRequests()
			time.Sleep(time.Millisecond) // Small delay to prevent busy waiting
		}
	}
}

// processQoSRequests handles pending QoS requests
func (bm *BandwidthManager) processQoSRequests() {
	classes := []TrafficClass{TrafficClassSystem, TrafficClassHigh, TrafficClassNormal, TrafficClassLow}

	bm.mu.RLock()
	defer bm.mu.RUnlock()

	for _, class := range classes {
		queue, exists := bm.qosQueues[class]
		if !exists {
			continue
		}

		// Process one request from this queue
		select {
		case request := <-queue.requests:
			// Try to allocate bandwidth
			tokensNeeded := int(request.bytes / 1024)
			if tokensNeeded == 0 {
				tokensNeeded = 1
			}

			approved := true
			for i := 0; i < tokensNeeded && approved; i++ {
				select {
				case <-queue.tokens:
					// Token acquired
				default:
					approved = false
				}
			}

			// Send response
			select {
			case request.response <- approved:
			default:
				// Response channel blocked
			}
		default:
			// No requests in this queue
		}
	}
}

// GetStats returns current bandwidth statistics
func (bm *BandwidthManager) GetStats() BandwidthStats {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	stats := BandwidthStats{
		UploadUsed:    atomic.LoadInt64(&bm.uploadUsed),
		DownloadUsed:  atomic.LoadInt64(&bm.downloadUsed),
		UploadLimit:   bm.config.MaxUpload,
		DownloadLimit: bm.config.MaxDownload,
		ActivePeers:   len(bm.peerLimits),
		QoSEnabled:    bm.config.QoSEnabled,
	}

	if bm.config.QoSEnabled {
		stats.QoSQueues = make(map[TrafficClass]QoSQueueStats)
		for class, queue := range bm.qosQueues {
			stats.QoSQueues[class] = QoSQueueStats{
				Allocated:       queue.allocated,
				Used:            queue.used,
				PendingRequests: len(queue.requests),
				AvailableTokens: len(queue.tokens),
			}
		}
	}

	return stats
}

// BandwidthStats provides bandwidth usage statistics
type BandwidthStats struct {
	UploadUsed    int64
	DownloadUsed  int64
	UploadLimit   int64
	DownloadLimit int64
	ActivePeers   int
	QoSEnabled    bool
	QoSQueues     map[TrafficClass]QoSQueueStats
}

// QoSQueueStats provides per-queue statistics
type QoSQueueStats struct {
	Allocated       int64
	Used            int64
	PendingRequests int
	AvailableTokens int
}

// GetMetrics returns the current metrics for this bandwidth manager
func (bm *BandwidthManager) GetMetrics() metrics.MetricsSnapshot {
	return bm.metrics.GetSnapshot()
}

// Close shuts down the bandwidth manager
func (bm *BandwidthManager) Close() error {
	bm.cancel()
	bm.wg.Wait()
	return nil
}
