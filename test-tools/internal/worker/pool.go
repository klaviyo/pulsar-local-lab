package worker

import (
	"context"
	"fmt"
	"sync"

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
				// Log error but don't stop other workers
				fmt.Printf("Worker %d error: %v\n", w.ID(), err)
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

	// Wait for all workers to finish
	p.wg.Wait()

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
	return len(p.workers)
}