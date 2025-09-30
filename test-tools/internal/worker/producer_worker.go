package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/generator"
	"github.com/pulsar-local-lab/perf-test/internal/metrics"
	"github.com/pulsar-local-lab/perf-test/internal/pulsar"
	"github.com/pulsar-local-lab/perf-test/pkg/ratelimit"
)

// ProducerWorker represents a producer worker
type ProducerWorker struct {
	id        int
	client    *pulsar.ProducerClient
	pool      *generator.PayloadPool
	collector *metrics.Collector
	limiter   *ratelimit.Limiter
	config    *config.Config
}

// NewProducerWorker creates a new producer worker
func NewProducerWorker(id int, cfg *config.Config, collector *metrics.Collector) (*ProducerWorker, error) {
	// Create Pulsar producer client
	client, err := pulsar.NewProducerClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer client: %w", err)
	}

	// Create payload pool for efficient buffer reuse
	pool := generator.NewPayloadPool(cfg.Producer.MessageSize, 100)

	// Create rate limiter if enabled
	var limiter *ratelimit.Limiter
	if cfg.Performance.RateLimitEnabled && cfg.Performance.TargetThroughput > 0 {
		// Divide target throughput among workers
		ratePerWorker := cfg.Performance.TargetThroughput / cfg.Producer.NumProducers
		limiter = ratelimit.NewLimiter(ratePerWorker)
	}

	return &ProducerWorker{
		id:        id,
		client:    client,
		pool:      pool,
		collector: collector,
		limiter:   limiter,
		config:    cfg,
	}, nil
}

// Start starts the producer worker
func (pw *ProducerWorker) Start(ctx context.Context) error {
	// Warmup period
	if pw.config.Performance.Warmup > 0 {
		time.Sleep(pw.config.Performance.Warmup)
	}

	// Main production loop
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Check duration limit
		if pw.config.Performance.Duration > 0 &&
			time.Since(startTime) >= pw.config.Performance.Duration {
			return nil
		}

		// Apply rate limiting if enabled
		if pw.limiter != nil {
			pw.limiter.Wait(ctx)
		}

		// Get payload buffer from pool and generate random data
		payload := pw.pool.Get()
		generator.GenerateRandomPayloadTo(payload)

		// Send message and measure latency
		sendStart := time.Now()
		_, err := pw.client.Send(ctx, payload)
		sendLatency := time.Since(sendStart)

		// Return buffer to pool
		pw.pool.Put(payload)

		if err != nil {
			pw.collector.RecordFailure()
			continue
		}

		// Record metrics
		pw.collector.RecordSend(len(payload), sendLatency)
	}
}

// Stop stops the producer worker
func (pw *ProducerWorker) Stop() error {
	// Flush any pending messages
	if err := pw.client.Flush(); err != nil {
		return fmt.Errorf("failed to flush producer: %w", err)
	}

	// Close client
	return pw.client.Close()
}

// ID returns the worker ID
func (pw *ProducerWorker) ID() int {
	return pw.id
}