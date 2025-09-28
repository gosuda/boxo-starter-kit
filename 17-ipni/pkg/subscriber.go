package ipni

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
)

// Subscriber handles content subscription and updates
type Subscriber struct {
	provider *Provider

	// State management
	running    bool
	stopCh     chan struct{}
	stateMutex sync.RWMutex
}

// NewSubscriber creates a new subscriber component
func NewSubscriber(provider *Provider) *Subscriber {
	return &Subscriber{
		provider: provider,
		stopCh:   make(chan struct{}),
	}
}

// Start initializes and starts the subscriber
func (s *Subscriber) Start(ctx context.Context) error {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if s.running {
		return fmt.Errorf("subscriber already running")
	}

	s.running = true

	// Start subscription loop in goroutine
	go s.subscriptionLoop(ctx)

	return nil
}

// Close gracefully shuts down the subscriber
func (s *Subscriber) Close() error {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopCh)

	return nil
}

// IsRunning returns whether the subscriber is currently running
func (s *Subscriber) IsRunning() bool {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.running
}

// subscriptionLoop handles the main subscription logic
func (s *Subscriber) subscriptionLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processUpdates(ctx)
		}
	}
}

// processUpdates handles processing of provider updates
func (s *Subscriber) processUpdates(ctx context.Context) {
	// Simulate processing provider updates
	// In a real implementation, this would handle network updates
	fmt.Println("🔄 Processing provider updates...")
}

// GetProvidersByCID finds providers for a given CID
func (s *Subscriber) GetProvidersByCID(c cid.Cid) ([]ProviderInfo, bool, error) {
	if s.provider != nil {
		return s.provider.GetProvidersByCID(c)
	}
	return nil, false, fmt.Errorf("no provider available")
}

// GetSubscriptionStats returns statistics about current subscriptions
func (s *Subscriber) GetSubscriptionStats() map[string]interface{} {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()

	stats := map[string]interface{}{
		"running":            s.running,
		"subscriber_type":    "simple-demo",
		"provider_connected": s.provider != nil,
	}

	return stats
}