# Go Pulsar Throughput Testing Suite - Implementation Plan

## Overview
Build Go-based performance testing tools with interactive terminal UIs for Apache Pulsar throughput testing on our Minikube cluster. Go's native concurrency (goroutines) provides excellent performance without GIL limitations.

## Why Go Instead of Python?
- **Native concurrency**: Goroutines are lightweight and truly parallel
- **Mature Pulsar client**: `pulsar-client-go` has excellent support
- **Better performance**: No GIL, compiled binary, efficient memory management
- **Built-in profiling**: pprof for CPU/memory profiling
- **Static typing**: Compile-time safety
- **Single binary deployment**: No dependency management issues

## Project Structure
```
test-tools/
├── go.mod                          # Go module definition
├── go.sum                          # Dependency checksums
├── cmd/
│   ├── producer/
│   │   └── main.go                 # Producer application entry point
│   └── consumer/
│       └── main.go                 # Consumer application entry point
├── internal/
│   ├── config/
│   │   ├── config.go              # Configuration management
│   │   └── profiles.go            # Configuration profiles
│   ├── pulsar/
│   │   ├── producer.go            # Pulsar producer wrapper
│   │   └── consumer.go            # Pulsar consumer wrapper
│   ├── metrics/
│   │   ├── collector.go           # Metrics collection
│   │   ├── histogram.go           # Latency histogram
│   │   ├── throughput.go          # Throughput tracking
│   │   └── exporter.go            # Metrics export (JSON/CSV/Prometheus)
│   ├── worker/
│   │   ├── pool.go                # Worker pool management
│   │   ├── producer_worker.go     # Producer worker goroutines
│   │   └── consumer_worker.go     # Consumer worker goroutines
│   ├── generator/
│   │   └── payload.go             # Message payload generation
│   └── ui/
│       ├── producer_ui.go         # Producer TUI
│       ├── consumer_ui.go         # Consumer TUI
│       └── components.go          # Shared UI components
├── pkg/
│   └── ratelimit/
│       └── limiter.go             # Rate limiting utilities
├── Makefile                        # Build and run commands
└── README.md                       # Documentation
```

## Technology Stack
- **Language**: Go 1.23+
- **Pulsar Client**: `github.com/apache/pulsar-client-go/pulsar`
- **TUI Framework**: `github.com/rivo/tview` (mature, feature-rich) or `github.com/charmbracelet/bubbletea` (modern)
- **Charts/Graphs**: `github.com/gizak/termui` or custom ASCII graphs
- **Metrics**: Custom implementation with `sync.atomic` for lock-free counters
- **CLI**: `github.com/spf13/cobra` for command-line interface
- **Config**: `github.com/spf13/viper` for configuration management

## Core Features

### Producer Controller (`cmd/producer/main.go`)

**Dynamic Configuration Panel:**
- Target throughput (msg/sec) - live adjustable
- Message size (bytes) - configurable payload
- Batch size - messages per batch
- Compression type (none, LZ4, ZLIB, ZSTD, SNAPPY)
- Topic name - connect to any topic
- Number of goroutines - spawn/kill dynamically
- Producer name prefix
- Payload generation strategy (random, sequential, pattern)

**Live Metrics Dashboard:**
- Real-time throughput graph (msg/sec over time) - 100ms granularity
- Target vs actual rate comparison with visual indicator
- Total messages sent
- Success/failure counts with error categorization
- Latency percentiles (P50, P95, P99, P999)
- Per-goroutine throughput breakdown
- Active goroutine count with CPU/memory usage
- Batch efficiency metrics
- Network throughput (MB/sec)

**Interactive Controls:**
- Add/remove producer goroutines (hotkeys: +/-)
- Pause/resume all producers (spacebar)
- Reset metrics (R key)
- Adjust rate with arrow keys (fine/coarse modes)
- Toggle burst mode (B key)
- Emergency stop (Q key)
- Export metrics snapshot (E key)

### Consumer Controller (`cmd/consumer/main.go`)

**Dynamic Configuration Panel:**
- Subscription name
- Subscription type (Exclusive, Shared, Failover, KeyShared)
- Topic name/pattern
- Number of goroutines - true parallel consumption
- Consumer name prefix
- Receive queue size
- Acknowledgment strategy (immediate, batched, delayed)

**Live Metrics Dashboard:**
- Real-time consumption rate graph (msg/sec over time)
- End-to-end latency distribution (histogram)
- Processing latency (receive to ack)
- Total messages received
- Consumer lag per partition
- Messages in flight
- Per-goroutine consumption breakdown
- Active goroutine count with CPU/memory usage
- Network throughput (MB/sec)
- Redelivery count with reasons

**Interactive Controls:**
- Add/remove consumer goroutines (hotkeys: +/-)
- Pause/resume consumption (spacebar)
- Reset metrics (R key)
- Acknowledge mode toggle (auto/manual/batched)
- Seek to timestamp/message ID (S key)
- Emergency stop (Q key)
- Export metrics snapshot (E key)

## Go-Specific Advantages

### Concurrency Model
```go
// Goroutines are lightweight (2KB stack vs MB for threads)
for i := 0; i < numWorkers; i++ {
    go producerWorker(ctx, client, config, metrics)
}

// Channels for communication (type-safe, built-in)
messageChan := make(chan *Message, 1000)
metricsChan := make(chan *MetricUpdate, 100)

// Select for multiplexing
select {
case msg := <-messageChan:
    // Process message
case <-ctx.Done():
    // Shutdown
case <-ticker.C:
    // Periodic task
}
```

### Lock-Free Metrics
```go
// Atomic operations for lock-free counters
atomic.AddUint64(&metrics.messagesSent, 1)
atomic.AddUint64(&metrics.bytesSent, uint64(len(payload)))

// Use sync.Map for concurrent map access
metrics.errorCounts.Store(errorType, count)
```

### Performance Profiling
```go
// Built-in CPU profiling
import _ "net/http/pprof"
go http.ListenAndServe("localhost:6060", nil)

// Memory profiling
runtime.MemProfileRate = 1
```

## Implementation Plan with Subagents

### Phase 1: Project Setup & Go Module
**Subagent**: `@general-purpose`
- Create directory structure
- Initialize Go module: `go mod init github.com/yourusername/pulsar-perf-test`
- Create `Makefile` with build, run, test targets
- Create stub files for all packages
- Create `README.md` with setup instructions

### Phase 2: Configuration Management
**Subagent**: `@go-backend-specialist`
- Implement `internal/config/config.go` with:
  - Configuration structs (ProducerConfig, ConsumerConfig, MetricsConfig)
  - Environment variable support using Viper
  - Profile loading/saving (JSON format)
  - Default profiles (low/medium/high/stress)
  - Validation logic
- Implement `internal/config/profiles.go` for preset configurations

### Phase 3: Pulsar Client Wrappers
**Subagent**: `@go-backend-specialist`
- Implement `internal/pulsar/producer.go`:
  - Wrapper around pulsar-client-go Producer
  - Connection pooling and auto-reconnect
  - All producer options (batch, compression, routing)
  - Error handling and retry logic
  - Health checks
- Implement `internal/pulsar/consumer.go`:
  - Wrapper around pulsar-client-go Consumer
  - All subscription types
  - Acknowledgment strategies
  - Seek operations
  - Pattern subscriptions

### Phase 4: Payload Generation
**Subagent**: `@go-backend-specialist`
- Implement `internal/generator/payload.go`:
  - Random payload generation (crypto/rand for speed)
  - Sequential payloads with sequence numbers
  - Pattern-based payloads
  - Benchmark functions
  - Memory pooling for payload reuse

### Phase 5: Metrics Collection System
**Subagent**: `@go-backend-specialist`
- Implement `internal/metrics/collector.go`:
  - Thread-safe metrics collector using atomic operations
  - Lock-free counters where possible
  - Metrics aggregation from multiple goroutines
- Implement `internal/metrics/histogram.go`:
  - HDR histogram for latency tracking
  - Fast percentile calculation
  - Concurrent updates
- Implement `internal/metrics/throughput.go`:
  - Rolling window throughput tracking
  - Time-series data with ring buffer
  - Peak rate tracking
- Implement `internal/metrics/exporter.go`:
  - JSON export
  - CSV export
  - Prometheus format
  - Real-time streaming

### Phase 6: Worker Pool Management
**Subagent**: `@go-backend-specialist`
- Implement `internal/worker/pool.go`:
  - Goroutine pool management
  - Dynamic scaling (add/remove workers)
  - Graceful shutdown with context cancellation
  - Per-worker metrics tracking
- Implement `internal/worker/producer_worker.go`:
  - Producer worker goroutine logic
  - Rate limiting per worker
  - Error handling and retry
  - Metrics reporting
- Implement `internal/worker/consumer_worker.go`:
  - Consumer worker goroutine logic
  - Message processing
  - Acknowledgment handling
  - Metrics reporting

### Phase 7: Rate Limiting
**Subagent**: `@go-backend-specialist`
- Implement `pkg/ratelimit/limiter.go`:
  - Token bucket rate limiter
  - Thread-safe using atomic operations
  - Configurable burst size
  - Minimal overhead (<100ns per token)

### Phase 8: UI Components Library
**Subagent**: `@ux-design-expert`
- Implement `internal/ui/components.go`:
  - Reusable tview widgets
  - Configuration panel component
  - Metrics display component
  - Real-time graph widget (ASCII art)
  - Status bar component
  - Help/keybinding overlay
  - Consistent styling

### Phase 9: Producer Application
**Subagent**: `@go-backend-specialist`
- Implement `cmd/producer/main.go`:
  - Main application using Cobra CLI
  - Initialize Pulsar producer client
  - Start worker pool
  - Initialize tview UI
  - Wire up controls and metrics display
  - Implement hot-reload for config changes
  - Graceful shutdown handling

### Phase 10: Consumer Application
**Subagent**: `@go-backend-specialist`
- Implement `cmd/consumer/main.go`:
  - Main application using Cobra CLI
  - Initialize Pulsar consumer client
  - Start worker pool
  - Initialize tview UI
  - Wire up controls and metrics display
  - Implement subscription management
  - Graceful shutdown handling

### Phase 11: Build System & Makefile
**Subagent**: `@general-purpose`
- Create comprehensive Makefile:
  - `make build` - Build binaries
  - `make run-producer` - Run producer
  - `make run-consumer` - Run consumer
  - `make test` - Run tests
  - `make bench` - Run benchmarks
  - `make profile` - Run with profiling
  - `make install` - Install binaries

### Phase 12: Testing & Validation
**Subagent**: `@qa-requirements-validator`
- Verify all features work as specified
- Test with Minikube Pulsar cluster
- Validate metrics accuracy
- Test goroutine scaling (1 to 1000 goroutines)
- Verify performance characteristics
- Test all interactive controls
- Load test with high throughput
- Memory leak detection

### Phase 13: Documentation
**Subagent**: `@general-purpose`
- Complete README.md with:
  - Installation instructions
  - Quick start guide
  - Configuration reference
  - Usage examples
  - Performance tuning guide
  - Troubleshooting
  - Architecture overview
- Add inline documentation (godoc)
- Create example configurations

### Phase 14: Memory Bank Update
**Subagent**: `@memory-bank-synchronizer`
- Update `CLAUDE-patterns.md` with Go testing patterns
- Update `CLAUDE-decisions.md` with Go architecture decisions
- Update `CLAUDE-activeContext.md` with implementation status
- Document Go-specific performance optimizations

### Phase 15: Design Review (Optional)
**Subagent**: `@design-review`
- Review TUI design for usability
- Validate color schemes and visual hierarchy
- Check keyboard navigation
- Verify responsive layout
- Ensure consistent UX

## Key Dependencies

```go
// go.mod
module github.com/yourusername/pulsar-perf-test

go 1.23

require (
    github.com/apache/pulsar-client-go v0.13.0
    github.com/rivo/tview v0.0.0-20240101183219-...
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2
    github.com/stretchr/testify v1.9.0  // for testing
    go.uber.org/atomic v1.11.0         // lock-free atomics
)
```

## Configuration Example

```go
// internal/config/config.go
type Config struct {
    Pulsar    PulsarConfig    `json:"pulsar"`
    Producer  ProducerConfig  `json:"producer"`
    Consumer  ConsumerConfig  `json:"consumer"`
    Metrics   MetricsConfig   `json:"metrics"`
}

type PulsarConfig struct {
    ServiceURL string `json:"service_url" default:"pulsar://localhost:6650"`
    AdminURL   string `json:"admin_url" default:"http://localhost:8080"`
}

type ProducerConfig struct {
    Topic           string `json:"topic" default:"persistent://public/default/perf-test"`
    TargetRate      int    `json:"target_rate" default:"10000"`
    MessageSize     int    `json:"message_size" default:"1024"`
    BatchSize       int    `json:"batch_size" default:"100"`
    NumWorkers      int    `json:"num_workers" default:"4"`
    CompressionType string `json:"compression_type" default:"lz4"`
}

type ConsumerConfig struct {
    Topic            string `json:"topic" default:"persistent://public/default/perf-test"`
    Subscription     string `json:"subscription" default:"perf-test-sub"`
    SubscriptionType string `json:"subscription_type" default:"Shared"`
    NumWorkers       int    `json:"num_workers" default:"4"`
}
```

## Performance Expectations with Go

- **10-100x better throughput** than Python (no GIL)
- **Sub-microsecond latency** for metrics recording
- **100K+ msg/sec** on single core
- **1M+ msg/sec** with multiple cores
- **Linear scaling** with goroutine count
- **Low memory footprint** (~50MB base + ~2KB per goroutine)
- **Instant startup** (compiled binary)

## Build & Run

```bash
# Build
make build

# Run producer
./bin/producer --config producer.json

# Run consumer
./bin/consumer --config consumer.json

# With Minikube port-forward
kubectl port-forward svc/pulsar-broker 6650:6650 &
./bin/producer --service-url pulsar://localhost:6650

# Profile CPU usage
./bin/producer --profile-cpu
# Access http://localhost:6060/debug/pprof/
```

## Deliverables

1. Two standalone Go applications (producer + consumer)
2. Shared internal packages for common functionality
3. Configuration system with profile support
4. Comprehensive Makefile for building and running
5. Complete documentation
6. Unit tests for all packages
7. Benchmark tests for performance validation
8. Updated memory bank files

## Success Criteria

- ✅ Both applications compile to single binaries
- ✅ Interactive TUI with real-time graphs at 100ms intervals
- ✅ Dynamic goroutine scaling without restart
- ✅ Configuration changes apply immediately
- ✅ Accurate metrics (throughput, latency, success rates)
- ✅ Export metrics to JSON/CSV/Prometheus
- ✅ Connect to Minikube Pulsar cluster successfully
- ✅ Handle 100K+ msg/sec per application
- ✅ Graceful error handling and reconnection
- ✅ Memory usage stays stable under load
- ✅ No goroutine leaks
- ✅ Comprehensive documentation

## Timeline Estimate

- Phase 1: Setup (30 min)
- Phase 2-3: Config & Pulsar clients (2-3 hours)
- Phase 4-7: Core functionality (4-5 hours)
- Phase 8: UI components (2-3 hours)
- Phase 9-10: Main applications (3-4 hours)
- Phase 11: Build system (30 min)
- Phase 12: Testing (2-3 hours)
- Phase 13-15: Documentation & polish (2-3 hours)

**Total**: ~18-25 hours of development time

## Advantages of Go Approach

1. **Better Performance**: 10-100x faster than Python
2. **True Concurrency**: No GIL limitations
3. **Simpler Deployment**: Single binary, no dependencies
4. **Better Tooling**: Built-in profiling, testing, benchmarking
5. **Type Safety**: Compile-time error detection
6. **Standard Library**: Excellent built-in packages
7. **Goroutines**: Lightweight, efficient concurrency
8. **Mature Pulsar Client**: Well-maintained, feature-complete

## Future Enhancements

- gRPC API for remote control
- Web UI mode (in addition to TUI)
- Distributed testing coordinator
- Schema support (Avro, Protobuf, JSON)
- Multi-cluster testing
- Built-in chaos testing scenarios
- Grafana/Prometheus integration
- Docker container deployment