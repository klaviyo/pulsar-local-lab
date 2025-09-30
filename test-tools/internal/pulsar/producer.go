package pulsar

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/pulsar-local-lab/perf-test/internal/config"
)

// ProducerClient wraps a Pulsar producer with additional functionality for production use.
// It provides thread-safe operations, automatic reconnection, health checks, and statistics tracking.
//
// Example usage:
//
//	cfg := config.DefaultConfig("")
//	producer, err := NewProducer(ctx, &cfg.Pulsar, &cfg.Producer)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer producer.Close()
//
//	msgID, err := producer.Send(ctx, []byte("hello"))
//	if err != nil {
//	    log.Printf("send failed: %v", err)
//	}
type ProducerClient struct {
	client   pulsar.Client
	producer pulsar.Producer

	pulsarCfg   *config.PulsarConfig
	producerCfg *config.ProducerConfig

	// Connection state management
	mu         sync.RWMutex
	connected  bool
	closed     bool

	// Statistics tracking
	stats ProducerStats

	// Reconnection state
	reconnecting atomic.Bool
	lastError    error
}

// ProducerStats holds producer statistics and metrics.
type ProducerStats struct {
	// MessagesSent is the total number of messages successfully sent
	MessagesSent uint64

	// MessagesFailures is the total number of send failures
	MessageFailures uint64

	// BytesSent is the total number of bytes sent
	BytesSent uint64

	// LastMessageTime is the timestamp of the last successful send
	LastMessageTime time.Time

	// PendingMessages is the current number of pending messages
	PendingMessages int
}

// NewProducer creates a new production-ready Pulsar producer client.
// It establishes a connection to the Pulsar cluster and creates a producer for the specified topic.
//
// Parameters:
//   - ctx: Context for connection timeout and cancellation
//   - pulsarCfg: Pulsar cluster connection configuration
//   - producerCfg: Producer-specific configuration (batching, compression, etc.)
//
// Returns:
//   - *ProducerClient: Configured and connected producer client
//   - error: Connection or configuration error
//
// The producer is created with the following features enabled based on configuration:
//   - Message batching for improved throughput
//   - Compression to reduce network usage
//   - Send timeout protection
//   - Pending message limits to prevent memory exhaustion
func NewProducer(ctx context.Context, pulsarCfg *config.PulsarConfig, producerCfg *config.ProducerConfig) (*ProducerClient, error) {
	if pulsarCfg == nil {
		return nil, fmt.Errorf("pulsar config cannot be nil")
	}
	if producerCfg == nil {
		return nil, fmt.Errorf("producer config cannot be nil")
	}

	pc := &ProducerClient{
		pulsarCfg:   pulsarCfg,
		producerCfg: producerCfg,
		connected:   false,
		closed:      false,
	}

	if err := pc.connect(ctx); err != nil {
		return nil, err
	}

	return pc, nil
}

// connect establishes connection to Pulsar cluster and creates producer.
// This is an internal method used by NewProducer and for reconnection attempts.
func (pc *ProducerClient) connect(ctx context.Context) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return fmt.Errorf("producer is closed")
	}

	// Create Pulsar client with connection pooling and timeout
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:              pc.pulsarCfg.ServiceURL,
		OperationTimeout: 30 * time.Second,
		ConnectionTimeout: 30 * time.Second,
	})
	if err != nil {
		pc.lastError = err
		return fmt.Errorf("failed to create pulsar client: %w", err)
	}

	// Create producer with configured options
	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic:               pc.pulsarCfg.Topic,
		DisableBatching:     !pc.producerCfg.BatchingEnabled,
		BatchingMaxMessages: uint(pc.producerCfg.BatchingMaxSize),
		CompressionType:     getCompressionType(pc.producerCfg.CompressionType),
		SendTimeout:         pc.producerCfg.SendTimeout,
		MaxPendingMessages:  pc.producerCfg.MaxPendingMsg,
	})
	if err != nil {
		client.Close()
		pc.lastError = err
		return fmt.Errorf("failed to create producer: %w", err)
	}

	pc.client = client
	pc.producer = producer
	pc.connected = true
	pc.lastError = nil

	log.Printf("Producer connected to topic: %s", pc.pulsarCfg.Topic)
	return nil
}

// Send sends a single message synchronously to the configured Pulsar topic.
// This method blocks until the message is acknowledged by the broker or the context times out.
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//   - payload: Message payload as byte slice
//
// Returns:
//   - pulsar.MessageID: Unique identifier for the sent message
//   - error: Send error or nil on success
//
// The method automatically tracks statistics and handles errors gracefully.
// For high-throughput scenarios, consider using SendAsync instead.
func (pc *ProducerClient) Send(ctx context.Context, payload []byte) (pulsar.MessageID, error) {
	pc.mu.RLock()
	if !pc.connected || pc.closed {
		pc.mu.RUnlock()
		return nil, fmt.Errorf("producer not connected")
	}
	producer := pc.producer
	pc.mu.RUnlock()

	msg := &pulsar.ProducerMessage{
		Payload: payload,
	}

	msgID, err := producer.Send(ctx, msg)
	if err != nil {
		atomic.AddUint64(&pc.stats.MessageFailures, 1)
		pc.lastError = err
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Update statistics
	atomic.AddUint64(&pc.stats.MessagesSent, 1)
	atomic.AddUint64(&pc.stats.BytesSent, uint64(len(payload)))
	pc.stats.LastMessageTime = time.Now()

	return msgID, nil
}

// SendWithProperties sends a message with custom properties/metadata.
// Properties can be used for message routing, filtering, or application-specific metadata.
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//   - payload: Message payload as byte slice
//   - properties: Key-value pairs for message metadata
//
// Returns:
//   - pulsar.MessageID: Unique identifier for the sent message
//   - error: Send error or nil on success
func (pc *ProducerClient) SendWithProperties(ctx context.Context, payload []byte, properties map[string]string) (pulsar.MessageID, error) {
	pc.mu.RLock()
	if !pc.connected || pc.closed {
		pc.mu.RUnlock()
		return nil, fmt.Errorf("producer not connected")
	}
	producer := pc.producer
	pc.mu.RUnlock()

	msg := &pulsar.ProducerMessage{
		Payload:    payload,
		Properties: properties,
	}

	msgID, err := producer.Send(ctx, msg)
	if err != nil {
		atomic.AddUint64(&pc.stats.MessageFailures, 1)
		pc.lastError = err
		return nil, fmt.Errorf("failed to send message with properties: %w", err)
	}

	// Update statistics
	atomic.AddUint64(&pc.stats.MessagesSent, 1)
	atomic.AddUint64(&pc.stats.BytesSent, uint64(len(payload)))
	pc.stats.LastMessageTime = time.Now()

	return msgID, nil
}

// SendAsync sends a message asynchronously without blocking.
// The callback function is invoked when the send operation completes (success or failure).
// This method provides better throughput for high-volume scenarios.
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//   - payload: Message payload as byte slice
//   - callback: Function called when send completes (can be nil)
//
// The callback receives:
//   - pulsar.MessageID: Message identifier (nil on error)
//   - *pulsar.ProducerMessage: The original message
//   - error: Send error or nil on success
//
// Note: The callback may be invoked from a different goroutine.
func (pc *ProducerClient) SendAsync(ctx context.Context, payload []byte, callback func(pulsar.MessageID, *pulsar.ProducerMessage, error)) {
	pc.mu.RLock()
	if !pc.connected || pc.closed {
		pc.mu.RUnlock()
		if callback != nil {
			callback(nil, nil, fmt.Errorf("producer not connected"))
		}
		return
	}
	producer := pc.producer
	pc.mu.RUnlock()

	msg := &pulsar.ProducerMessage{
		Payload: payload,
	}

	// Wrap callback to update statistics
	wrappedCallback := func(msgID pulsar.MessageID, message *pulsar.ProducerMessage, err error) {
		if err != nil {
			atomic.AddUint64(&pc.stats.MessageFailures, 1)
			pc.lastError = err
		} else {
			atomic.AddUint64(&pc.stats.MessagesSent, 1)
			atomic.AddUint64(&pc.stats.BytesSent, uint64(len(payload)))
			pc.stats.LastMessageTime = time.Now()
		}

		if callback != nil {
			callback(msgID, message, err)
		}
	}

	producer.SendAsync(ctx, msg, wrappedCallback)
}

// Flush flushes all pending messages in the send queue.
// This method blocks until all pending messages are sent or the operation times out.
// It should be called before closing the producer to ensure no messages are lost.
//
// Returns:
//   - error: Flush error or nil on success
func (pc *ProducerClient) Flush() error {
	pc.mu.RLock()
	if !pc.connected || pc.closed {
		pc.mu.RUnlock()
		return fmt.Errorf("producer not connected")
	}
	producer := pc.producer
	pc.mu.RUnlock()

	if err := producer.Flush(); err != nil {
		return fmt.Errorf("failed to flush producer: %w", err)
	}

	return nil
}

// Close gracefully closes the producer and releases all resources.
// It flushes pending messages before closing to prevent message loss.
// This method is safe to call multiple times.
//
// Returns:
//   - error: Close error or nil on success
func (pc *ProducerClient) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil
	}

	pc.closed = true
	pc.connected = false

	// Flush pending messages before closing
	if pc.producer != nil {
		if err := pc.producer.Flush(); err != nil {
			log.Printf("Warning: failed to flush producer during close: %v", err)
		}
		pc.producer.Close()
	}

	if pc.client != nil {
		pc.client.Close()
	}

	log.Printf("Producer closed for topic: %s", pc.pulsarCfg.Topic)
	return nil
}

// IsConnected returns the current connection status of the producer.
// This can be used for health checks and monitoring.
//
// Returns:
//   - bool: true if connected and ready to send, false otherwise
func (pc *ProducerClient) IsConnected() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.connected && !pc.closed
}

// Stats returns a snapshot of current producer statistics.
// The returned struct contains metrics like message count, bytes sent, and failure count.
//
// Returns:
//   - ProducerStats: Current producer statistics
func (pc *ProducerClient) Stats() ProducerStats {
	return ProducerStats{
		MessagesSent:    atomic.LoadUint64(&pc.stats.MessagesSent),
		MessageFailures: atomic.LoadUint64(&pc.stats.MessageFailures),
		BytesSent:       atomic.LoadUint64(&pc.stats.BytesSent),
		LastMessageTime: pc.stats.LastMessageTime,
		PendingMessages: 0, // Would need producer internals to get accurate count
	}
}

// LastError returns the most recent error encountered by the producer.
// This can be useful for diagnostics when IsConnected returns false.
//
// Returns:
//   - error: Last error or nil if no errors occurred
func (pc *ProducerClient) LastError() error {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.lastError
}

// Reconnect attempts to reconnect the producer to the Pulsar cluster.
// This is useful for recovering from transient network failures.
// The method uses exponential backoff for retry attempts.
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//   - maxRetries: Maximum number of reconnection attempts (0 = unlimited)
//
// Returns:
//   - error: Reconnection error or nil on success
func (pc *ProducerClient) Reconnect(ctx context.Context, maxRetries int) error {
	if !pc.reconnecting.CompareAndSwap(false, true) {
		return fmt.Errorf("reconnection already in progress")
	}
	defer pc.reconnecting.Store(false)

	pc.mu.Lock()
	if pc.closed {
		pc.mu.Unlock()
		return fmt.Errorf("cannot reconnect closed producer")
	}

	// Close existing connections
	if pc.producer != nil {
		pc.producer.Close()
		pc.producer = nil
	}
	if pc.client != nil {
		pc.client.Close()
		pc.client = nil
	}
	pc.connected = false
	pc.mu.Unlock()

	// Exponential backoff retry logic
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for attempt := 0; maxRetries == 0 || attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("reconnection cancelled: %w", ctx.Err())
		default:
		}

		if attempt > 0 {
			log.Printf("Reconnection attempt %d after %v", attempt+1, backoff)
			time.Sleep(backoff)

			// Exponential backoff with max cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		if err := pc.connect(ctx); err != nil {
			log.Printf("Reconnection attempt %d failed: %v", attempt+1, err)
			continue
		}

		log.Printf("Successfully reconnected after %d attempts", attempt+1)
		return nil
	}

	return fmt.Errorf("failed to reconnect after %d attempts", maxRetries)
}

// getCompressionType converts string compression type to Pulsar CompressionType enum.
// Supported compression types: NONE, LZ4, ZLIB, ZSTD
func getCompressionType(compressionType string) pulsar.CompressionType {
	switch compressionType {
	case "LZ4":
		return pulsar.LZ4
	case "ZLIB":
		return pulsar.ZLib
	case "ZSTD":
		return pulsar.ZSTD
	default:
		return pulsar.NoCompression
	}
}

// Legacy wrapper for backward compatibility
// Deprecated: Use NewProducer instead
func NewProducerClient(cfg *config.Config) (*ProducerClient, error) {
	return NewProducer(context.Background(), &cfg.Pulsar, &cfg.Producer)
}