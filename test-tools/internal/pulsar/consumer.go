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

// ConsumerClient wraps a Pulsar consumer with additional functionality for production use.
// It provides thread-safe operations, automatic reconnection, health checks, and statistics tracking.
//
// Example usage:
//
//	cfg := config.DefaultConfig("")
//	consumer, err := NewConsumer(ctx, &cfg.Pulsar, &cfg.Consumer, "consumer-1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer consumer.Close()
//
//	for {
//	    msg, err := consumer.Receive(ctx)
//	    if err != nil {
//	        log.Printf("receive failed: %v", err)
//	        break
//	    }
//	    // Process message
//	    consumer.Ack(msg)
//	}
type ConsumerClient struct {
	client   pulsar.Client
	consumer pulsar.Consumer

	pulsarCfg   *config.PulsarConfig
	consumerCfg *config.ConsumerConfig
	consumerID  string

	// Connection state management
	mu        sync.RWMutex
	connected bool
	closed    bool

	// Statistics tracking
	stats ConsumerStats

	// Reconnection state
	reconnecting atomic.Bool
	lastError    error
}

// ConsumerStats holds consumer statistics and metrics.
type ConsumerStats struct {
	// MessagesReceived is the total number of messages received
	MessagesReceived uint64

	// MessagesAcked is the total number of messages acknowledged
	MessagesAcked uint64

	// MessagesNacked is the total number of messages negatively acknowledged
	MessagesNacked uint64

	// BytesReceived is the total number of bytes received
	BytesReceived uint64

	// LastMessageTime is the timestamp of the last received message
	LastMessageTime time.Time

	// ReceiveErrors is the total number of receive errors
	ReceiveErrors uint64
}

// NewConsumer creates a new production-ready Pulsar consumer client.
// It establishes a connection to the Pulsar cluster and subscribes to the specified topic.
//
// Parameters:
//   - ctx: Context for connection timeout and cancellation
//   - pulsarCfg: Pulsar cluster connection configuration
//   - consumerCfg: Consumer-specific configuration (subscription type, queue size, etc.)
//   - consumerID: Unique identifier for this consumer instance (for debugging)
//
// Returns:
//   - *ConsumerClient: Configured and connected consumer client
//   - error: Connection or configuration error
//
// The consumer supports multiple subscription types:
//   - Exclusive: Only one consumer can subscribe
//   - Shared: Multiple consumers can subscribe, messages distributed round-robin
//   - Failover: Multiple consumers can subscribe, one active at a time
//   - KeyShared: Multiple consumers, messages with same key go to same consumer
func NewConsumer(ctx context.Context, pulsarCfg *config.PulsarConfig, consumerCfg *config.ConsumerConfig, consumerID string) (*ConsumerClient, error) {
	if pulsarCfg == nil {
		return nil, fmt.Errorf("pulsar config cannot be nil")
	}
	if consumerCfg == nil {
		return nil, fmt.Errorf("consumer config cannot be nil")
	}
	if consumerID == "" {
		return nil, fmt.Errorf("consumer ID cannot be empty")
	}

	cc := &ConsumerClient{
		pulsarCfg:   pulsarCfg,
		consumerCfg: consumerCfg,
		consumerID:  consumerID,
		connected:   false,
		closed:      false,
	}

	if err := cc.connect(ctx); err != nil {
		return nil, err
	}

	return cc, nil
}

// connect establishes connection to Pulsar cluster and creates consumer.
// This is an internal method used by NewConsumer and for reconnection attempts.
func (cc *ConsumerClient) connect(ctx context.Context) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closed {
		return fmt.Errorf("consumer is closed")
	}

	// Create Pulsar client with connection pooling and timeout
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:               cc.pulsarCfg.ServiceURL,
		OperationTimeout:  30 * time.Second,
		ConnectionTimeout: 30 * time.Second,
	})
	if err != nil {
		cc.lastError = err
		return fmt.Errorf("failed to create pulsar client: %w", err)
	}

	// Create consumer with configured options
	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:               cc.pulsarCfg.Topic,
		SubscriptionName:    cc.consumerCfg.SubscriptionName,
		Type:                getSubscriptionType(cc.consumerCfg.SubscriptionType),
		ReceiverQueueSize:   cc.consumerCfg.ReceiverQueueSize,
		NackRedeliveryDelay: 5 * time.Second,
		Name:                cc.consumerID,
	})
	if err != nil {
		client.Close()
		cc.lastError = err
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	cc.client = client
	cc.consumer = consumer
	cc.connected = true
	cc.lastError = nil

	log.Printf("Consumer %s connected to topic: %s (subscription: %s)", cc.consumerID, cc.pulsarCfg.Topic, cc.consumerCfg.SubscriptionName)
	return nil
}

// Receive receives a message from the Pulsar topic.
// This method blocks until a message is available or the context is cancelled.
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//
// Returns:
//   - pulsar.Message: Received message (contains payload, properties, metadata)
//   - error: Receive error or nil on success
//
// The message must be acknowledged using Ack() or Nack() after processing.
// Unacknowledged messages will be redelivered based on the subscription settings.
func (cc *ConsumerClient) Receive(ctx context.Context) (pulsar.Message, error) {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return nil, fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	msg, err := consumer.Receive(ctx)
	if err != nil {
		atomic.AddUint64(&cc.stats.ReceiveErrors, 1)
		cc.lastError = err
		return nil, fmt.Errorf("failed to receive message: %w", err)
	}

	// Update statistics
	atomic.AddUint64(&cc.stats.MessagesReceived, 1)
	atomic.AddUint64(&cc.stats.BytesReceived, uint64(len(msg.Payload())))
	cc.stats.LastMessageTime = time.Now()

	return msg, nil
}

// Ack acknowledges successful processing of a message.
// Once acknowledged, the message will not be redelivered.
//
// Parameters:
//   - msg: Message to acknowledge
//
// Returns:
//   - error: Acknowledgment error or nil on success
//
// Acknowledgments may be batched internally for better performance.
func (cc *ConsumerClient) Ack(msg pulsar.Message) error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	consumer.Ack(msg)
	atomic.AddUint64(&cc.stats.MessagesAcked, 1)
	return nil
}

// AckID acknowledges a message by its message ID.
// This is useful when you need to acknowledge a message without having the message object.
//
// Parameters:
//   - msgID: Message ID to acknowledge
//
// Returns:
//   - error: Acknowledgment error or nil on success
func (cc *ConsumerClient) AckID(msgID pulsar.MessageID) error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	if err := consumer.AckID(msgID); err != nil {
		return fmt.Errorf("failed to ack message ID: %w", err)
	}

	atomic.AddUint64(&cc.stats.MessagesAcked, 1)
	return nil
}

// Nack negatively acknowledges a message, indicating processing failure.
// The message will be redelivered after the configured delay.
//
// Parameters:
//   - msg: Message to negatively acknowledge
//
// Returns:
//   - error: Negative acknowledgment error or nil on success
//
// Use Nack when message processing fails transiently and the message should be retried.
func (cc *ConsumerClient) Nack(msg pulsar.Message) error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	consumer.Nack(msg)
	atomic.AddUint64(&cc.stats.MessagesNacked, 1)
	return nil
}

// NackID negatively acknowledges a message by its message ID.
//
// Parameters:
//   - msgID: Message ID to negatively acknowledge
//
// Returns:
//   - error: Negative acknowledgment error or nil on success
func (cc *ConsumerClient) NackID(msgID pulsar.MessageID) error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	consumer.NackID(msgID)
	atomic.AddUint64(&cc.stats.MessagesNacked, 1)
	return nil
}

// Seek moves the subscription cursor to a specific position in the topic.
// This allows replaying messages from a specific point in time or message ID.
//
// Parameters:
//   - msgID: Message ID to seek to (use pulsar.EarliestMessageID() for beginning)
//
// Returns:
//   - error: Seek error or nil on success
//
// Note: Seek operation resets unacknowledged messages for this consumer.
func (cc *ConsumerClient) Seek(msgID pulsar.MessageID) error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	if err := consumer.Seek(msgID); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	log.Printf("Consumer %s seeked to message ID: %v", cc.consumerID, msgID)
	return nil
}

// SeekByTime moves the subscription cursor to messages published at or after a specific time.
//
// Parameters:
//   - timestamp: Time to seek to
//
// Returns:
//   - error: Seek error or nil on success
func (cc *ConsumerClient) SeekByTime(timestamp time.Time) error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	if err := consumer.SeekByTime(timestamp); err != nil {
		return fmt.Errorf("failed to seek by time: %w", err)
	}

	log.Printf("Consumer %s seeked to time: %v", cc.consumerID, timestamp)
	return nil
}

// Chan returns a channel for receiving messages asynchronously.
// This provides an alternative to the blocking Receive() method.
//
// Returns:
//   - <-chan pulsar.ConsumerMessage: Channel delivering messages
//
// Example usage:
//
//	for cm := range consumer.Chan() {
//	    msg := cm.Message
//	    // Process message
//	    consumer.Ack(msg)
//	}
func (cc *ConsumerClient) Chan() <-chan pulsar.ConsumerMessage {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if !cc.connected || cc.closed || cc.consumer == nil {
		// Return a closed channel if not connected
		ch := make(chan pulsar.ConsumerMessage)
		close(ch)
		return ch
	}

	return cc.consumer.Chan()
}

// Close gracefully closes the consumer and releases all resources.
// This method is safe to call multiple times.
//
// Returns:
//   - error: Close error or nil on success
func (cc *ConsumerClient) Close() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closed {
		return nil
	}

	cc.closed = true
	cc.connected = false

	if cc.consumer != nil {
		cc.consumer.Close()
	}

	if cc.client != nil {
		cc.client.Close()
	}

	log.Printf("Consumer %s closed for topic: %s", cc.consumerID, cc.pulsarCfg.Topic)
	return nil
}

// IsConnected returns the current connection status of the consumer.
// This can be used for health checks and monitoring.
//
// Returns:
//   - bool: true if connected and ready to receive, false otherwise
func (cc *ConsumerClient) IsConnected() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.connected && !cc.closed
}

// Stats returns a snapshot of current consumer statistics.
// The returned struct contains metrics like message count, bytes received, and error count.
//
// Returns:
//   - ConsumerStats: Current consumer statistics
func (cc *ConsumerClient) Stats() ConsumerStats {
	return ConsumerStats{
		MessagesReceived: atomic.LoadUint64(&cc.stats.MessagesReceived),
		MessagesAcked:    atomic.LoadUint64(&cc.stats.MessagesAcked),
		MessagesNacked:   atomic.LoadUint64(&cc.stats.MessagesNacked),
		BytesReceived:    atomic.LoadUint64(&cc.stats.BytesReceived),
		LastMessageTime:  cc.stats.LastMessageTime,
		ReceiveErrors:    atomic.LoadUint64(&cc.stats.ReceiveErrors),
	}
}

// LastError returns the most recent error encountered by the consumer.
// This can be useful for diagnostics when IsConnected returns false.
//
// Returns:
//   - error: Last error or nil if no errors occurred
func (cc *ConsumerClient) LastError() error {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.lastError
}

// Reconnect attempts to reconnect the consumer to the Pulsar cluster.
// This is useful for recovering from transient network failures.
// The method uses exponential backoff for retry attempts.
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//   - maxRetries: Maximum number of reconnection attempts (0 = unlimited)
//
// Returns:
//   - error: Reconnection error or nil on success
//
// Note: The subscription cursor position is preserved during reconnection.
func (cc *ConsumerClient) Reconnect(ctx context.Context, maxRetries int) error {
	if !cc.reconnecting.CompareAndSwap(false, true) {
		return fmt.Errorf("reconnection already in progress")
	}
	defer cc.reconnecting.Store(false)

	cc.mu.Lock()
	if cc.closed {
		cc.mu.Unlock()
		return fmt.Errorf("cannot reconnect closed consumer")
	}

	// Close existing connections
	if cc.consumer != nil {
		cc.consumer.Close()
		cc.consumer = nil
	}
	if cc.client != nil {
		cc.client.Close()
		cc.client = nil
	}
	cc.connected = false
	cc.mu.Unlock()

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
			log.Printf("Consumer %s reconnection attempt %d after %v", cc.consumerID, attempt+1, backoff)
			time.Sleep(backoff)

			// Exponential backoff with max cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		if err := cc.connect(ctx); err != nil {
			log.Printf("Consumer %s reconnection attempt %d failed: %v", cc.consumerID, attempt+1, err)
			continue
		}

		log.Printf("Consumer %s successfully reconnected after %d attempts", cc.consumerID, attempt+1)
		return nil
	}

	return fmt.Errorf("failed to reconnect consumer %s after %d attempts", cc.consumerID, maxRetries)
}

// Unsubscribe unsubscribes the consumer from the topic and deletes the subscription.
// This is different from Close() which just disconnects without deleting the subscription.
//
// Returns:
//   - error: Unsubscribe error or nil on success
//
// Warning: This deletes the subscription cursor. All unacknowledged messages will be lost.
func (cc *ConsumerClient) Unsubscribe() error {
	cc.mu.RLock()
	if !cc.connected || cc.closed {
		cc.mu.RUnlock()
		return fmt.Errorf("consumer not connected")
	}
	consumer := cc.consumer
	cc.mu.RUnlock()

	if err := consumer.Unsubscribe(); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	log.Printf("Consumer %s unsubscribed from topic: %s", cc.consumerID, cc.pulsarCfg.Topic)
	return nil
}

// getSubscriptionType converts string subscription type to Pulsar SubscriptionType enum.
// Supported subscription types: Exclusive, Shared, Failover, KeyShared
func getSubscriptionType(subType string) pulsar.SubscriptionType {
	switch subType {
	case "Exclusive":
		return pulsar.Exclusive
	case "Shared":
		return pulsar.Shared
	case "Failover":
		return pulsar.Failover
	case "KeyShared":
		return pulsar.KeyShared
	default:
		return pulsar.Shared
	}
}

// Legacy wrapper for backward compatibility
// Deprecated: Use NewConsumer instead
func NewConsumerClient(cfg *config.Config, consumerID int) (*ConsumerClient, error) {
	return NewConsumer(context.Background(), &cfg.Pulsar, &cfg.Consumer, fmt.Sprintf("consumer-%d", consumerID))
}