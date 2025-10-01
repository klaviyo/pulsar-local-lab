package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/metrics"
	"github.com/pulsar-local-lab/perf-test/internal/pulsar"
)

// Pool represents a pool of workers
type Pool struct {
	workers   []Worker
	collector *metrics.Collector
	config    *config.Config
	wg        sync.WaitGroup
	mu        sync.RWMutex
	running   bool
}

// Worker interface for producer and consumer workers
type Worker interface {
	Start(ctx context.Context) error
	Stop() error
	ID() int
}

// NewProducerPool creates a new producer worker pool
func NewProducerPool(ctx context.Context, cfg *config.Config) (*Pool, error) {
	// Ensure topic exists with correct partition configuration
	if err := pulsar.EnsureTopic(cfg); err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	collector := metrics.NewCollector(cfg.Metrics.HistogramBuckets)

	pool := &Pool{
		workers:   make([]Worker, 0, cfg.Producer.NumProducers),
		collector: collector,
		config:    cfg,
	}

	// Create producer workers
	for i := 0; i < cfg.Producer.NumProducers; i++ {
		worker, err := NewProducerWorker(i, cfg, collector)
		if err != nil {
			// Clean up any created workers
			pool.Stop()
			return nil, fmt.Errorf("failed to create producer worker %d: %w", i, err)
		}

		// Set per-worker context
		workerCtx, cancelFunc := context.WithCancel(ctx)
		worker.SetContext(workerCtx, cancelFunc)

		pool.workers = append(pool.workers, worker)
	}

	return pool, nil
}

// NewConsumerPool creates a new consumer worker pool
func NewConsumerPool(ctx context.Context, cfg *config.Config) (*Pool, error) {
	// Ensure topic exists with correct partition configuration
	if err := pulsar.EnsureTopic(cfg); err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	collector := metrics.NewCollector(cfg.Metrics.HistogramBuckets)

	pool := &Pool{
		workers:   make([]Worker, 0, cfg.Consumer.NumConsumers),
		collector: collector,
		config:    cfg,
	}

	// Create consumer workers
	for i := 0; i < cfg.Consumer.NumConsumers; i++ {
		worker, err := NewConsumerWorker(i, cfg, collector)
		if err != nil {
			// Clean up any created workers
			pool.Stop()
			return nil, fmt.Errorf("failed to create consumer worker %d: %w", i, err)
		}
		pool.workers = append(pool.workers, worker)
	}

	return pool, nil
}

// Start starts all workers in the pool
func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("pool already running")
	}
	p.running = true
	p.mu.Unlock()

	// Start all workers
	for _, worker := range p.workers {
		p.wg.Add(1)
		go func(w Worker) {
			defer p.wg.Done()
			if err := w.Start(ctx); err != nil {
				// Silently handle error - logging to stdout breaks the TUI
				// In production, would log to file or structured logger
				_ = err
			}
		}(worker)
	}

	return nil
}

// Stop stops all workers in the pool
func (p *Pool) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false
	p.mu.Unlock()

	// Stop all workers
	var errs []error
	for _, worker := range p.workers {
		if err := worker.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers stopped successfully
	case <-time.After(10 * time.Second):
		// Timeout - workers taking too long to stop
		return fmt.Errorf("timeout waiting for workers to stop")
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping workers: %v", errs)
	}

	return nil
}

// GetMetrics returns the metrics collector
func (p *Pool) GetMetrics() *metrics.Collector {
	return p.collector
}

// IsRunning returns whether the pool is running
func (p *Pool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// WorkerCount returns the number of workers
func (p *Pool) WorkerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// AddWorker adds a new worker to the pool dynamically
func (p *Pool) AddWorker(ctx context.Context, workerFactory func(int) (Worker, error)) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	workerID := len(p.workers)
	worker, err := workerFactory(workerID)
	if err != nil {
		return fmt.Errorf("failed to create worker %d: %w", workerID, err)
	}

	// Set per-worker context for ProducerWorker
	if pw, ok := worker.(*ProducerWorker); ok {
		workerCtx, cancelFunc := context.WithCancel(ctx)
		pw.SetContext(workerCtx, cancelFunc)
	}

	p.workers = append(p.workers, worker)

	// Start the worker if pool is running
	if p.running {
		p.wg.Add(1)
		go func(w Worker) {
			defer p.wg.Done()
			if err := w.Start(ctx); err != nil {
				// Silently handle error - logging to stdout breaks the TUI
				_ = err
			}
		}(worker)
	}

	return nil
}

// RemoveWorker removes the last worker from the pool with graceful shutdown
func (p *Pool) RemoveWorker() error {
	p.mu.Lock()

	if len(p.workers) == 0 {
		p.mu.Unlock()
		return fmt.Errorf("no workers to remove")
	}

	// Don't allow removing the last worker
	if len(p.workers) == 1 {
		p.mu.Unlock()
		return fmt.Errorf("cannot remove last worker")
	}

	// Get the last worker and current target rate
	lastWorker := p.workers[len(p.workers)-1]
	currentTargetRate := p.config.Performance.TargetThroughput

	// Remove from slice immediately to prevent new rate calculations from including it
	p.workers = p.workers[:len(p.workers)-1]
	newWorkerCount := len(p.workers)

	p.mu.Unlock()

	// Step 1: Cancel the worker's context to signal it to stop
	if pw, ok := lastWorker.(*ProducerWorker); ok {
		pw.CancelContext()

		// Step 2: Wait for the goroutine to finish (with timeout)
		done := make(chan struct{})
		go func() {
			pw.WaitForCompletion()
			close(done)
		}()

		select {
		case <-done:
			// Worker stopped gracefully
		case <-time.After(5 * time.Second):
			// Timeout - continue anyway to prevent UI hang
			// The worker will eventually stop but might still try to send to closed client
		}
	}

	// Step 3: Now it's safe to stop (flush and close client)
	if err := lastWorker.Stop(); err != nil {
		// Don't return error - worker is already removed from pool
		// Log would go here if we had proper logging
		_ = err
	}

	// Step 4: Recalculate rate limits for remaining workers
	// This ensures remaining workers get their share of the target rate
	if currentTargetRate > 0 && newWorkerCount > 0 {
		p.mu.Lock()
		ratePerWorker := currentTargetRate / newWorkerCount
		if ratePerWorker == 0 {
			ratePerWorker = 1
		}
		for _, worker := range p.workers {
			if pw, ok := worker.(*ProducerWorker); ok {
				pw.UpdateRateLimiter(ratePerWorker)
			}
		}
		p.mu.Unlock()
	}

	return nil
}

// GetConfig returns the current configuration
func (p *Pool) GetConfig() *config.Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateTargetRate updates the target throughput rate and propagates to all workers
func (p *Pool) UpdateTargetRate(rate int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update config
	p.config.Performance.TargetThroughput = rate
	if rate > 0 {
		p.config.Performance.RateLimitEnabled = true
	} else {
		p.config.Performance.RateLimitEnabled = false
	}

	// Calculate per-worker rate
	numWorkers := len(p.workers)
	if numWorkers == 0 {
		return
	}

	ratePerWorker := 0
	if rate > 0 {
		ratePerWorker = rate / numWorkers
		if ratePerWorker == 0 {
			ratePerWorker = 1 // Ensure at least 1 msg/s per worker
		}
	}

	// Update all producer workers
	for _, worker := range p.workers {
		if pw, ok := worker.(*ProducerWorker); ok {
			pw.UpdateRateLimiter(ratePerWorker)
		}
	}
}

// UpdateBatchSize updates the batching max size
func (p *Pool) UpdateBatchSize(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Producer.BatchingMaxSize = size
}

// UpdateCompression updates the compression type
func (p *Pool) UpdateCompression(compressionType string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Producer.CompressionType = compressionType
}

// UpdateMessageSize updates the message size
func (p *Pool) UpdateMessageSize(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Producer.MessageSize = size
}

// RestartWorkers restarts all workers to apply immutable configuration changes
// This is needed for settings like batch size, compression, and message size
func (p *Pool) RestartWorkers(ctx context.Context) error {
	p.mu.Lock()

	// Store current state
	wasRunning := p.running
	currentWorkerCount := len(p.workers)
	currentConfig := p.config

	// Stop all workers
	oldWorkers := p.workers
	p.workers = make([]Worker, 0, currentWorkerCount)
	p.mu.Unlock()

	// Cancel all worker contexts and wait for them to stop
	for _, worker := range oldWorkers {
		if pw, ok := worker.(*ProducerWorker); ok {
			pw.CancelContext()
		}
	}

	// Wait for all goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		for _, worker := range oldWorkers {
			if pw, ok := worker.(*ProducerWorker); ok {
				pw.WaitForCompletion()
			}
		}
		close(done)
	}()

	select {
	case <-done:
		// All workers stopped
	case <-time.After(10 * time.Second):
		// Timeout - continue anyway
	}

	// Stop (flush and close) all old workers
	for _, worker := range oldWorkers {
		_ = worker.Stop()
	}

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	// Create new workers with updated configuration
	for i := 0; i < currentWorkerCount; i++ {
		worker, err := NewProducerWorker(i, currentConfig, p.collector)
		if err != nil {
			return fmt.Errorf("failed to create worker %d during restart: %w", i, err)
		}

		// Set per-worker context
		workerCtx, cancelFunc := context.WithCancel(ctx)
		worker.SetContext(workerCtx, cancelFunc)

		p.mu.Lock()
		p.workers = append(p.workers, worker)
		p.mu.Unlock()
	}

	// Start workers if pool was running before
	if wasRunning {
		return p.Start(ctx)
	}

	return nil
}