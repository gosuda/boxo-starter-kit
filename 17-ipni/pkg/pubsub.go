package ipni

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PubSubManager handles real-time synchronization via PubSub
type PubSubManager struct {
	host         host.Host
	topics       map[string]*Topic
	subscribers  map[string][]MessageHandler
	messagePool  *MessagePool
	config       *PubSubConfig
	running      bool
	stopCh       chan struct{}
	mutex        sync.RWMutex
}

// PubSubConfig holds PubSub configuration
type PubSubConfig struct {
	BufferSize       int           `json:"buffer_size"`
	MessageTimeout   time.Duration `json:"message_timeout"`
	MaxMessageSize   int           `json:"max_message_size"`
	ValidationTimeout time.Duration `json:"validation_timeout"`
}

// DefaultPubSubConfig returns default PubSub configuration
func DefaultPubSubConfig() *PubSubConfig {
	return &PubSubConfig{
		BufferSize:        1000,
		MessageTimeout:    30 * time.Second,
		MaxMessageSize:    1024 * 1024, // 1MB
		ValidationTimeout: 5 * time.Second,
	}
}

// Topic represents a PubSub topic
type Topic struct {
	name       string
	handlers   []MessageHandler
	messages   chan *Message
	stopCh     chan struct{}
	running    bool
}

// Message represents a PubSub message
type Message struct {
	Type      string    `json:"type"`
	Topic     string    `json:"topic"`
	Data      []byte    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Sender    peer.ID   `json:"sender"`
	Signature []byte    `json:"signature,omitempty"`
}

// MessageHandler interface for handling PubSub messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *Message) error
	GetMessageTypes() []string
}

// MessagePool manages message routing and validation
type MessagePool struct {
	validators map[string]MessageValidator
	filters    []MessageFilter
	metrics    *PubSubMetrics
}

// MessageValidator validates messages
type MessageValidator interface {
	Validate(ctx context.Context, msg *Message) error
}

// MessageFilter filters messages
type MessageFilter interface {
	Filter(msg *Message) bool
}

// PubSubMetrics tracks PubSub performance
type PubSubMetrics struct {
	MessagesReceived  int64 `json:"messages_received"`
	MessagesSent      int64 `json:"messages_sent"`
	MessagesValidated int64 `json:"messages_validated"`
	MessagesRejected  int64 `json:"messages_rejected"`
	TopicCount        int   `json:"topic_count"`
	SubscriberCount   int   `json:"subscriber_count"`
}

// NewPubSubManager creates a new PubSub manager
func NewPubSubManager(h host.Host, messageHandler MessageHandler) (*PubSubManager, error) {
	// Allow nil host for demo mode
	if h == nil {
		fmt.Println("📢 PubSub running in demo mode (no network host)")
	}

	config := DefaultPubSubConfig()
	messagePool := &MessagePool{
		validators: make(map[string]MessageValidator),
		filters:    []MessageFilter{},
		metrics:    &PubSubMetrics{},
	}

	manager := &PubSubManager{
		host:        h,
		topics:      make(map[string]*Topic),
		subscribers: make(map[string][]MessageHandler),
		messagePool: messagePool,
		config:      config,
		stopCh:      make(chan struct{}),
	}

	// Register the main message handler if provided
	if messageHandler != nil {
		for _, msgType := range messageHandler.GetMessageTypes() {
			manager.Subscribe("ipni", msgType, messageHandler)
		}
	}

	return manager, nil
}

// Start initializes the PubSub manager
func (pm *PubSubManager) Start(ctx context.Context) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.running {
		return fmt.Errorf("PubSub manager already running")
	}

	pm.running = true

	// Start message processing loop
	go pm.messageProcessingLoop(ctx)

	fmt.Println("🔊 PubSub manager started")
	return nil
}

// Stop gracefully shuts down the PubSub manager
func (pm *PubSubManager) Stop() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.running {
		return nil
	}

	pm.running = false
	close(pm.stopCh)

	// Stop all topics
	for _, topic := range pm.topics {
		topic.stop()
	}

	fmt.Println("🔊 PubSub manager stopped")
	return nil
}

// Subscribe subscribes to a topic with a message handler
func (pm *PubSubManager) Subscribe(topicName, messageType string, handler MessageHandler) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Create topic if it doesn't exist
	if _, exists := pm.topics[topicName]; !exists {
		topic := &Topic{
			name:     topicName,
			handlers: []MessageHandler{},
			messages: make(chan *Message, pm.config.BufferSize),
			stopCh:   make(chan struct{}),
		}
		pm.topics[topicName] = topic
		go topic.start()
	}

	// Add handler to topic
	pm.topics[topicName].handlers = append(pm.topics[topicName].handlers, handler)

	// Add to subscribers map
	key := topicName + ":" + messageType
	pm.subscribers[key] = append(pm.subscribers[key], handler)

	pm.messagePool.metrics.SubscriberCount++

	fmt.Printf("📡 Subscribed to topic '%s' for message type '%s'\n", topicName, messageType)
	return nil
}

// Publish publishes a message to a topic
func (pm *PubSubManager) Publish(ctx context.Context, topicName, messageType string, data interface{}) error {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if !pm.running {
		return fmt.Errorf("PubSub manager not running")
	}

	// Serialize data
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize data: %w", err)
	}

	// Create message
	var senderID peer.ID
	if pm.host != nil {
		senderID = pm.host.ID()
	} else {
		senderID = peer.ID("demo-sender")
	}

	msg := &Message{
		Type:      messageType,
		Topic:     topicName,
		Data:      dataBytes,
		Timestamp: time.Now(),
		Sender:    senderID,
	}

	// Check message size
	if len(dataBytes) > pm.config.MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes", len(dataBytes))
	}

	// Send to topic
	if topic, exists := pm.topics[topicName]; exists {
		select {
		case topic.messages <- msg:
			pm.messagePool.metrics.MessagesSent++
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pm.config.MessageTimeout):
			return fmt.Errorf("message send timeout")
		}
	}

	return fmt.Errorf("topic '%s' not found", topicName)
}

// PublishProviderAnnouncement publishes a provider announcement
func (pm *PubSubManager) PublishProviderAnnouncement(ctx context.Context, announcement *PubSubProviderAnnouncement) error {
	return pm.Publish(ctx, "ipni", "provider_announcement", announcement)
}

// messageProcessingLoop processes incoming messages
func (pm *PubSubManager) messageProcessingLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.processMessages(ctx)
		}
	}
}

// processMessages processes pending messages
func (pm *PubSubManager) processMessages(ctx context.Context) {
	// In a real implementation, this would process messages from the network
	// For demo purposes, we'll simulate message processing
	pm.messagePool.metrics.MessagesReceived++
}

// GetMetrics returns PubSub metrics
func (pm *PubSubManager) GetMetrics() *PubSubMetrics {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	metrics := *pm.messagePool.metrics
	metrics.TopicCount = len(pm.topics)
	return &metrics
}

// GetTopics returns list of active topics
func (pm *PubSubManager) GetTopics() []string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var topics []string
	for name := range pm.topics {
		topics = append(topics, name)
	}
	return topics
}

// Topic methods

// start starts the topic message handler
func (t *Topic) start() {
	t.running = true

	for {
		select {
		case msg := <-t.messages:
			t.handleMessage(msg)
		case <-t.stopCh:
			return
		}
	}
}

// stop stops the topic
func (t *Topic) stop() {
	if t.running {
		t.running = false
		close(t.stopCh)
	}
}

// handleMessage handles a message for this topic
func (t *Topic) handleMessage(msg *Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send message to all handlers
	for _, handler := range t.handlers {
		go func(h MessageHandler) {
			if err := h.HandleMessage(ctx, msg); err != nil {
				fmt.Printf("❌ Handler error for topic '%s': %v\n", t.name, err)
			}
		}(handler)
	}
}

// Simple message validator
type SimpleMessageValidator struct{}

// Validate validates a message
func (v *SimpleMessageValidator) Validate(ctx context.Context, msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	if msg.Type == "" {
		return fmt.Errorf("message type is empty")
	}

	if len(msg.Data) == 0 {
		return fmt.Errorf("message data is empty")
	}

	if time.Since(msg.Timestamp) > 5*time.Minute {
		return fmt.Errorf("message too old")
	}

	return nil
}

// Size filter
type SizeMessageFilter struct {
	maxSize int
}

// NewSizeMessageFilter creates a new size filter
func NewSizeMessageFilter(maxSize int) *SizeMessageFilter {
	return &SizeMessageFilter{maxSize: maxSize}
}

// Filter filters messages by size
func (f *SizeMessageFilter) Filter(msg *Message) bool {
	return len(msg.Data) <= f.maxSize
}