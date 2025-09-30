package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/metrics"
	"github.com/pulsar-local-lab/perf-test/internal/pulsar"
)

// ConsumerWorker represents a consumer worker
type ConsumerWorker struct {
	id        int
	client    *pulsar.ConsumerClient
	collector *metrics.Collector
	config    *config.Config
}

// NewConsumerWorker creates a new consumer worker
func NewConsumerWorker(id int, cfg *config.Config, collector *metrics.Collector) (*ConsumerWorker, error) {
	// Create Pulsar consumer client
	client, err := pulsar.NewConsumerClient(cfg, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer client: %w", err)
	}

	return &ConsumerWorker{
		id:        id,
		client:    client,
		collector: collector,
		config:    cfg,
	}, nil
}

// Start starts the consumer worker
func (cw *ConsumerWorker) Start(ctx context.Context) error {
	// Warmup period
	if cw.config.Performance.Warmup > 0 {
		time.Sleep(cw.config.Performance.Warmup)
	}

	// Main consumption loop
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Check duration limit
		if cw.config.Performance.Duration > 0 &&
			time.Since(startTime) >= cw.config.Performance.Duration {
			return nil
		}

		// Receive message with timeout
		receiveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		msg, err := cw.client.Receive(receiveCtx)
		cancel()

		if err != nil {
			// Timeout is expected when no messages are available
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		// Record metrics
		cw.collector.RecordReceive(len(msg.Payload()))

		// Acknowledge message
		if err := cw.client.Ack(msg); err != nil {
			cw.collector.RecordFailure()
			continue
		}

		cw.collector.RecordAck()
	}
}

// Stop stops the consumer worker
func (cw *ConsumerWorker) Stop() error {
	return cw.client.Close()
}

// ID returns the worker ID
func (cw *ConsumerWorker) ID() int {
	return cw.id
}