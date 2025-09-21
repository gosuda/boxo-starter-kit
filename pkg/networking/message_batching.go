package networking

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/gosuda/boxo-starter-kit/pkg/metrics"
)

// MessageBatcher groups multiple small messages into larger batches
// to reduce protocol overhead and improve network efficiency
type MessageBatcher struct {
	metrics *metrics.ComponentMetrics
	config  BatchingConfig

	mu       sync.Mutex
	batches  map[peer.ID]*peerBatch
	outgoing chan batchJob

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// BatchingConfig defines message batching parameters
type BatchingConfig struct {
	MaxBatchSize     int           // Maximum messages per batch
	MaxBatchBytes    int           // Maximum bytes per batch
	BatchTimeout     time.Duration // Maximum time to wait for batch completion
	CompressionLevel int           // Gzip compression level (1-9, 0=disabled)
	EnablePriority   bool          // Enable priority message handling
	WorkerCount      int           // Number of batch processing workers
}

// DefaultBatchingConfig returns sensible defaults
func DefaultBatchingConfig() BatchingConfig {
	return BatchingConfig{
		MaxBatchSize:     100,
		MaxBatchBytes:    64 * 1024, // 64KB
		BatchTimeout:     10 * time.Millisecond,
		CompressionLevel: 6,
		EnablePriority:   true,
		WorkerCount:      4,
	}
}

// MessagePriority defines message priority levels
type MessagePriority int

const (
	PriorityLow MessagePriority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// BatchedMessage represents a message to be batched
type BatchedMessage struct {
	ID       string
	Data     []byte
	Priority MessagePriority
	Callback func(error) // Called when message is sent
}

// peerBatch tracks batching state for a specific peer
type peerBatch struct {
	peer     peer.ID
	messages []BatchedMessage
	bytes    int
	timer    *time.Timer
	priority MessagePriority // Highest priority in batch
}

// batchJob represents work to send a completed batch
type batchJob struct {
	peer     peer.ID
	messages []BatchedMessage
	data     []byte
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher(config BatchingConfig) *MessageBatcher {
	ctx, cancel := context.WithCancel(context.Background())

	batchMetrics := metrics.NewComponentMetrics("message_batcher")
	metrics.RegisterGlobalComponent(batchMetrics)

	mb := &MessageBatcher{
		metrics:  batchMetrics,
		config:   config,
		batches:  make(map[peer.ID]*peerBatch),
		outgoing: make(chan batchJob, config.WorkerCount*2),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start batch workers
	for i := 0; i < config.WorkerCount; i++ {
		mb.wg.Add(1)
		go mb.batchWorker()
	}

	return mb
}

// QueueMessage adds a message to the batching queue
func (mb *MessageBatcher) QueueMessage(peerID peer.ID, msg BatchedMessage) error {
	start := time.Now()
	mb.metrics.RecordRequest()

	mb.mu.Lock()
	defer mb.mu.Unlock()

	batch, exists := mb.batches[peerID]
	if !exists {
		batch = &peerBatch{
			peer:     peerID,
			messages: make([]BatchedMessage, 0, mb.config.MaxBatchSize),
			priority: msg.Priority,
		}
		mb.batches[peerID] = batch
	}

	// Update batch priority to highest priority message
	if msg.Priority > batch.priority {
		batch.priority = msg.Priority
	}

	// Add message to batch
	batch.messages = append(batch.messages, msg)
	batch.bytes += len(msg.Data)

	// Check if batch should be sent immediately
	shouldSend := false
	reason := ""

	if len(batch.messages) >= mb.config.MaxBatchSize {
		shouldSend = true
		reason = "max_size"
	} else if batch.bytes >= mb.config.MaxBatchBytes {
		shouldSend = true
		reason = "max_bytes"
	} else if msg.Priority >= PriorityHigh {
		shouldSend = true
		reason = "high_priority"
	}

	if shouldSend {
		mb.sendBatch(batch, reason)
	} else if batch.timer == nil {
		// Set timer for batch timeout
		batch.timer = time.AfterFunc(mb.config.BatchTimeout, func() {
			mb.mu.Lock()
			if b, exists := mb.batches[peerID]; exists && b == batch {
				mb.sendBatch(batch, "timeout")
			}
			mb.mu.Unlock()
		})
	}

	mb.metrics.RecordSuccess(time.Since(start), int64(len(msg.Data)))
	return nil
}

// SendImmediately forces immediate sending of any pending batch for a peer
func (mb *MessageBatcher) SendImmediately(peerID peer.ID) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if batch, exists := mb.batches[peerID]; exists {
		mb.sendBatch(batch, "forced")
	}
}

// sendBatch prepares and queues a batch for sending
// Must be called with mutex held
func (mb *MessageBatcher) sendBatch(batch *peerBatch, reason string) {
	if len(batch.messages) == 0 {
		return
	}

	// Cancel timer if it exists
	if batch.timer != nil {
		batch.timer.Stop()
		batch.timer = nil
	}

	// Serialize batch
	data, err := mb.serializeBatch(batch.messages)
	if err != nil {
		// Call error callbacks
		for _, msg := range batch.messages {
			if msg.Callback != nil {
				msg.Callback(fmt.Errorf("serialization failed: %w", err))
			}
		}
		delete(mb.batches, batch.peer)
		return
	}

	// Queue for sending
	job := batchJob{
		peer:     batch.peer,
		messages: batch.messages,
		data:     data,
	}

	select {
	case mb.outgoing <- job:
		// Successfully queued
	default:
		// Queue full, call error callbacks
		for _, msg := range batch.messages {
			if msg.Callback != nil {
				msg.Callback(fmt.Errorf("batch queue full"))
			}
		}
	}

	// Remove batch from map
	delete(mb.batches, batch.peer)
}

// serializeBatch converts messages to wire format
func (mb *MessageBatcher) serializeBatch(messages []BatchedMessage) ([]byte, error) {
	var buf bytes.Buffer

	// Write batch header
	header := struct {
		Version     uint8
		Compressed  uint8
		MessageCount uint32
	}{
		Version:      1,
		Compressed:   0,
		MessageCount: uint32(len(messages)),
	}

	if mb.config.CompressionLevel > 0 {
		header.Compressed = 1
	}

	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return nil, err
	}

	// Prepare message data
	var msgBuf bytes.Buffer
	for _, msg := range messages {
		// Write message length
		if err := binary.Write(&msgBuf, binary.LittleEndian, uint32(len(msg.Data))); err != nil {
			return nil, err
		}
		// Write message ID length and ID
		idBytes := []byte(msg.ID)
		if err := binary.Write(&msgBuf, binary.LittleEndian, uint8(len(idBytes))); err != nil {
			return nil, err
		}
		if _, err := msgBuf.Write(idBytes); err != nil {
			return nil, err
		}
		// Write message data
		if _, err := msgBuf.Write(msg.Data); err != nil {
			return nil, err
		}
	}

	// Apply compression if enabled
	if mb.config.CompressionLevel > 0 {
		var compressedBuf bytes.Buffer
		writer, err := gzip.NewWriterLevel(&compressedBuf, mb.config.CompressionLevel)
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write(msgBuf.Bytes()); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}

		// Write compressed size then compressed data
		if err := binary.Write(&buf, binary.LittleEndian, uint32(compressedBuf.Len())); err != nil {
			return nil, err
		}
		if _, err := buf.Write(compressedBuf.Bytes()); err != nil {
			return nil, err
		}
	} else {
		// Write uncompressed size then data
		if err := binary.Write(&buf, binary.LittleEndian, uint32(msgBuf.Len())); err != nil {
			return nil, err
		}
		if _, err := buf.Write(msgBuf.Bytes()); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// DeserializeBatch parses a received batch
func (mb *MessageBatcher) DeserializeBatch(data []byte) ([]BatchedMessage, error) {
	buf := bytes.NewReader(data)

	// Read header
	var header struct {
		Version     uint8
		Compressed  uint8
		MessageCount uint32
	}

	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Version != 1 {
		return nil, fmt.Errorf("unsupported batch version: %d", header.Version)
	}

	// Read payload size
	var payloadSize uint32
	if err := binary.Read(buf, binary.LittleEndian, &payloadSize); err != nil {
		return nil, err
	}

	// Read payload
	payload := make([]byte, payloadSize)
	if _, err := io.ReadFull(buf, payload); err != nil {
		return nil, err
	}

	// Decompress if needed
	var msgData []byte
	if header.Compressed == 1 {
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		msgData = decompressed
	} else {
		msgData = payload
	}

	// Parse messages
	msgBuf := bytes.NewReader(msgData)
	messages := make([]BatchedMessage, 0, header.MessageCount)

	for i := uint32(0); i < header.MessageCount; i++ {
		// Read message length
		var msgLen uint32
		if err := binary.Read(msgBuf, binary.LittleEndian, &msgLen); err != nil {
			return nil, err
		}

		// Read message ID
		var idLen uint8
		if err := binary.Read(msgBuf, binary.LittleEndian, &idLen); err != nil {
			return nil, err
		}

		idBytes := make([]byte, idLen)
		if _, err := io.ReadFull(msgBuf, idBytes); err != nil {
			return nil, err
		}

		// Read message data
		data := make([]byte, msgLen)
		if _, err := io.ReadFull(msgBuf, data); err != nil {
			return nil, err
		}

		messages = append(messages, BatchedMessage{
			ID:       string(idBytes),
			Data:     data,
			Priority: PriorityNormal,
		})
	}

	return messages, nil
}

// batchWorker processes outgoing batches
func (mb *MessageBatcher) batchWorker() {
	defer mb.wg.Done()

	for {
		select {
		case job := <-mb.outgoing:
			mb.processBatchJob(job)
		case <-mb.ctx.Done():
			return
		}
	}
}

// processBatchJob sends a batch to its destination
func (mb *MessageBatcher) processBatchJob(job batchJob) {
	start := time.Now()

	// This would integrate with the actual network layer
	// For now, we simulate successful sending
	success := true
	var err error

	// Simulate network delay based on batch size
	time.Sleep(time.Duration(len(job.data)/1024) * time.Microsecond)

	// Call message callbacks
	for _, msg := range job.messages {
		if msg.Callback != nil {
			if success {
				msg.Callback(nil)
			} else {
				msg.Callback(err)
			}
		}
	}

	if success {
		mb.metrics.RecordSuccess(time.Since(start), int64(len(job.data)))
	} else {
		mb.metrics.RecordFailure(time.Since(start), "send_failed")
	}
}

// GetStats returns current batching statistics
func (mb *MessageBatcher) GetStats() BatchingStats {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	stats := BatchingStats{
		PendingBatches:   len(mb.batches),
		PendingMessages:  0,
		QueuedJobs:      len(mb.outgoing),
	}

	for _, batch := range mb.batches {
		stats.PendingMessages += len(batch.messages)
	}

	return stats
}

// BatchingStats provides batching statistics
type BatchingStats struct {
	PendingBatches  int
	PendingMessages int
	QueuedJobs      int
}

// GetMetrics returns the current metrics for this message batcher
func (mb *MessageBatcher) GetMetrics() metrics.MetricsSnapshot {
	return mb.metrics.GetSnapshot()
}

// Flush forces all pending batches to be sent immediately
func (mb *MessageBatcher) Flush() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for peerID, batch := range mb.batches {
		mb.sendBatch(batch, "flush")
		delete(mb.batches, peerID)
	}
}

// Close shuts down the message batcher
func (mb *MessageBatcher) Close() error {
	// Flush pending batches
	mb.Flush()

	// Stop workers
	mb.cancel()
	mb.wg.Wait()

	return nil
}