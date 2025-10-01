package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/generator"
	"github.com/pulsar-local-lab/perf-test/internal/metrics"
	"github.com/pulsar-local-lab/perf-test/internal/pulsar"
	"github.com/pulsar-local-lab/perf-test/pkg/ratelimit"
)

// ProducerWorker represents a producer worker
type ProducerWorker struct {
	id         int
	client     *pulsar.ProducerClient
	pool       *generator.PayloadPool
	collector  *metrics.Collector
	limiter    *ratelimit.Limiter
	config     *config.Config
	workerCtx  context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
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
// The context passed here is ignored - worker uses its own workerCtx set during initialization
func (pw *ProducerWorker) Start(ctx context.Context) error {
	// Use worker's own context if set, otherwise fall back to provided context
	workCtx := pw.workerCtx
	if workCtx == nil {
		workCtx = ctx
	}

	// Mark work group as started
	pw.wg.Add(1)
	defer pw.wg.Done()

	// Warmup period
	if pw.config.Performance.Warmup > 0 {
		time.Sleep(pw.config.Performance.Warmup)
	}

	// Main production loop
	startTime := time.Now()
	for {
		select {
		case <-workCtx.Done():
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
			if err := pw.limiter.Wait(workCtx); err != nil {
				// Context cancelled during wait
				return nil
			}
		}

		// Get payload buffer from pool and generate random data
		payload := pw.pool.Get()
		generator.GenerateRandomPayloadTo(payload)

		// Send message and measure latency
		sendStart := time.Now()
		_, err := pw.client.Send(workCtx, payload)
		sendLatency := time.Since(sendStart)

		// Return buffer to pool
		pw.pool.Put(payload)

		if err != nil {
			// Check if context was cancelled (not a real failure)
			if workCtx.Err() != nil {
				return nil
			}
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

// UpdateRateLimiter updates the rate limiter with a new rate per second
// If rate is 0, rate limiting is disabled by setting limiter to nil
// If limiter doesn't exist and rate > 0, a new limiter is created
func (pw *ProducerWorker) UpdateRateLimiter(ratePerSecond int) {
	if ratePerSecond <= 0 {
		// Disable rate limiting
		if pw.limiter != nil {
			pw.limiter.Stop()
			pw.limiter = nil
		}
		return
	}

	// Update or create limiter
	if pw.limiter != nil {
		// Update existing limiter
		pw.limiter.SetRate(ratePerSecond)
	} else {
		// Create new limiter
		pw.limiter = ratelimit.NewLimiter(ratePerSecond)
	}
}

// SetContext sets the worker's context and cancel function
// This must be called before Start() to enable proper shutdown
func (pw *ProducerWorker) SetContext(ctx context.Context, cancel context.CancelFunc) {
	pw.workerCtx = ctx
	pw.cancelFunc = cancel
}

// CancelContext cancels the worker's context, signaling it to stop
func (pw *ProducerWorker) CancelContext() {
	if pw.cancelFunc != nil {
		pw.cancelFunc()
	}
}

// WaitForCompletion waits for the worker's goroutine to finish
func (pw *ProducerWorker) WaitForCompletion() {
	pw.wg.Wait()
}