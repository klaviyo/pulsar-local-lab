package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	"github.com/pulsar-local-lab/perf-test/internal/metrics"
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
		pool.workers = append(pool.workers, worker)
	}

	return pool, nil
}

// NewConsumerPool creates a new consumer worker pool
func NewConsumerPool(ctx context.Context, cfg *config.Config) (*Pool, error) {
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

// RemoveWorker removes the last worker from the pool
func (p *Pool) RemoveWorker() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) == 0 {
		return fmt.Errorf("no workers to remove")
	}

	// Don't allow removing the last worker
	if len(p.workers) == 1 {
		return fmt.Errorf("cannot remove last worker")
	}

	// Get the last worker
	lastWorker := p.workers[len(p.workers)-1]

	// Stop the worker
	if err := lastWorker.Stop(); err != nil {
		return fmt.Errorf("failed to stop worker: %w", err)
	}

	// Remove from slice
	p.workers = p.workers[:len(p.workers)-1]

	return nil
}