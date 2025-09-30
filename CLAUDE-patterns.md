# Code Patterns and Conventions

## Project Organization Patterns

### Directory Structure
```
pulsar-local-lab/
├── helm/                    # Kubernetes Helm values
├── monitoring/              # Observability configurations
│   ├── grafana/            # Dashboards and provisioning
│   └── prometheus/         # Metrics collection configs
├── .claude/                # Claude Code configuration
└── setup.md                # Master implementation plan
```


## Configuration Patterns

### Minikube/Local Development
- **Single node deployments**: 1 replica for ZK, Bookkeeper, Broker, Proxy
- **Persistence disabled**: Use emptyDir volumes for faster iteration
- **Anti-affinity disabled**: Allow pods on same node
- **Auto-recovery disabled**: Simplified setup
- **Memory optimized**: Reduced cache sizes for local resource constraints

### Prometheus Configuration Variants
Three configuration profiles exist:
1. **prometheus.yml** - Basic cluster monitoring
2. **prometheus-ha.yml** - High availability setup
3. **prometheus-upgrade.yml** - Upgrade scenario monitoring

Pattern: Separate configs for different deployment scenarios rather than single config with conditionals.

### Grafana Dashboards
**Naming Convention**: Sequential numbering with descriptive names
- `01-pulsar-overview.json` - System-wide view
- `02-broker-metrics.json` - Broker-specific
- `03-bookkeeper-metrics.json` - Storage layer
- `04-topic-namespace.json` - Topic management
- `05-consumer-subscription.json` - Consumer tracking
- `06-jvm-system.json` - JVM performance

## Pulsar Configuration Patterns

### Minimal Local Setup (Minikube)
```yaml
managedLedgerDefaultEnsembleSize: "1"
managedLedgerDefaultWriteQuorum: "1"
managedLedgerDefaultAckQuorum: "1"
autoSkipNonRecoverableData: "true"
```

### HA Production Setup Example
```yaml
managedLedgerDefaultEnsembleSize: 3-5
managedLedgerDefaultWriteQuorum: 2-3
managedLedgerDefaultAckQuorum: 2
brokerDeduplicationEnabled: true
topicLevelPoliciesEnabled: true
```

## Git Workflow Patterns

### Commit History
Recent commits show iterative development style:
- Feature commits: "switching to minikube", "parallel tests"
- Maintenance: "more fixes", "attempting fix"
- No strict convention currently enforced

### File Management
- Configuration files committed to git
- .claude/ directory kept local (untracked)
- CLAUDE.md and CLAUDE-*.md excluded from commits (per guidance)

## Documentation Patterns

### Learning-Oriented Design
- Focus on understanding Pulsar operations
- Comprehensive metrics and observability

## Go Project Patterns (test-tools/)

### Project Architecture

#### Clean Architecture Structure
```
test-tools/
├── cmd/                    # Application entry points
│   ├── producer/          # Producer CLI
│   └── consumer/          # Consumer CLI
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── pulsar/           # Pulsar client wrappers
│   ├── metrics/          # Metrics collection
│   ├── worker/           # Worker pool management
│   ├── generator/        # Payload generation
│   └── ui/               # Terminal UI components
└── pkg/                   # Reusable packages
    └── ratelimit/        # Rate limiting utilities
```

**Pattern Rationale**:
- `cmd/` contains only main packages and CLI setup
- `internal/` prevents external imports (Go convention)
- `pkg/` contains reusable, importable components
- Clean separation of concerns

### Concurrency Patterns

#### Lock-Free Atomic Operations
```go
type Collector struct {
    messagesSent atomic.Uint64
    bytesSent    atomic.Uint64
}

func (c *Collector) RecordSend(bytes int) {
    c.messagesSent.Add(1)
    c.bytesSent.Add(uint64(bytes))
}
```

**Pattern Characteristics**:
- No mutex locks needed
- Sub-nanosecond overhead (1.66ns measured)
- Wait-free for readers
- Lock-free for writers using CAS

**Used In**:
- `/internal/metrics/collector.go` - Metrics collection
- `/pkg/ratelimit/limiter.go` - Token bucket rate limiter

#### Worker Pool Pattern
```go
type Pool struct {
    workers   []Worker
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

func (p *Pool) Start() {
    for _, worker := range p.workers {
        p.wg.Add(1)
        go func(w Worker) {
            defer p.wg.Done()
            w.Run(p.ctx)
        }(worker)
    }
}

func (p *Pool) Stop() {
    p.cancel()
    p.wg.Wait()
}
```

**Pattern Benefits**:
- Graceful shutdown with context cancellation
- WaitGroup ensures all workers complete
- Clean lifecycle management

**Used In**:
- `/internal/worker/pool.go` - Worker pool management
- `/internal/worker/producer_worker.go` - Producer workers
- `/internal/worker/consumer_worker.go` - Consumer workers

#### Channel-Based Communication
```go
type Worker struct {
    id       int
    producer pulsar.Producer
    rateLimiter *ratelimit.Limiter
}

func (w *Worker) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            if err := w.rateLimiter.Wait(ctx); err != nil {
                return
            }
            w.sendMessage(ctx)
        }
    }
}
```

**Pattern Characteristics**:
- Context-aware cancellation
- Non-blocking select statements
- Respects backpressure from rate limiter

### Configuration Patterns

#### Profile-Based Configuration
```go
// Five predefined profiles
const (
    ProfileDefault         = "default"
    ProfileLowLatency     = "low-latency"
    ProfileHighThroughput = "high-throughput"
    ProfileBurst          = "burst"
    ProfileSustained      = "sustained"
)

func GetProfile(name string) (*Config, error) {
    switch name {
    case ProfileLowLatency:
        return LowLatencyProfile(), nil
    // ...
    }
}
```

**Pattern Benefits**:
- Quick start with sensible defaults
- Named scenarios for common use cases
- Overridable via CLI flags or config files

**Configuration Sources Priority**:
1. CLI flags (highest priority)
2. Config file
3. Performance profile
4. Built-in defaults (lowest priority)

**Used In**: `/internal/config/profiles.go`

#### Functional Options Pattern
```go
type Config struct {
    Pulsar      PulsarConfig
    Producer    ProducerConfig
    Consumer    ConsumerConfig
    Performance PerformanceConfig
    Metrics     MetricsConfig
}

func DefaultConfig(profile string) *Config {
    cfg := &Config{ /* defaults */ }
    ApplyProfile(cfg, profile)
    return cfg
}
```

### Performance Patterns

#### Zero-Allocation Payload Generation
```go
type PayloadGenerator struct {
    messagePool sync.Pool
}

func (g *PayloadGenerator) Generate() []byte {
    buf := g.messagePool.Get().(*bytes.Buffer)
    defer g.messagePool.Put(buf)
    buf.Reset()
    // Generate payload
    return buf.Bytes()
}
```

**Performance**: 2.6M payloads/second
**Pattern**: Object pooling to reduce GC pressure

**Used In**: `/internal/generator/payload.go`

#### Token Bucket Rate Limiter
```go
type Limiter struct {
    rate       atomic.Int64
    bucket     atomic.Int64
    ticker     *time.Ticker
}

func (l *Limiter) tryAcquire() bool {
    for {
        current := l.bucket.Load()
        if current <= 0 {
            return false
        }
        if l.bucket.CompareAndSwap(current, current-1) {
            return true
        }
    }
}
```

**Performance**: 0.87ns overhead per operation
**Pattern**: Lock-free CAS loop for token acquisition

**Used In**: `/pkg/ratelimit/limiter.go`

### UI Patterns (tview)

#### Component-Based UI Structure
```go
type ProducerUI struct {
    app        *tview.Application
    statsTable *tview.Table
    logView    *tview.TextView
    layout     *tview.Flex
}

func (ui *ProducerUI) Update(snapshot metrics.Snapshot) {
    ui.app.QueueUpdateDraw(func() {
        ui.updateStats(snapshot)
    })
}
```

**Pattern Characteristics**:
- Thread-safe UI updates via `QueueUpdateDraw`
- Component composition with Flex layouts
- Real-time metric display

**Used In**:
- `/internal/ui/producer_ui.go`
- `/internal/ui/consumer_ui.go`
- `/internal/ui/components.go`

### Testing Patterns

#### Table-Driven Tests
```go
func TestGetProfile(t *testing.T) {
    tests := []struct {
        name        string
        profile     string
        wantErr     bool
        checkConfig func(*Config) bool
    }{
        {
            name:    "default profile",
            profile: "default",
            wantErr: false,
        },
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg, err := GetProfile(tt.profile)
            // assertions
        })
    }
}
```

**Coverage Results**:
- `internal/config`: 88.8%
- `internal/generator`: 100%
- `internal/metrics`: 94.8%
- `pkg/ratelimit`: 98.3%

**Used Throughout**: All `*_test.go` files

#### Benchmark Tests
```go
func BenchmarkCollectorRecordSend(b *testing.B) {
    c := NewCollector(DefaultHistogramBuckets)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        c.RecordSend(1024, 10*time.Millisecond)
    }
}
```

**Used In**:
- `/internal/metrics/collector_bench_test.go`
- `/pkg/ratelimit/limiter_bench_test.go`

### Error Handling Patterns

#### Context-Aware Error Propagation
```go
func (w *Worker) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := w.process(ctx); err != nil {
                if ctx.Err() != nil {
                    return ctx.Err() // Context cancelled
                }
                // Log and continue for transient errors
                continue
            }
        }
    }
}
```

**Pattern**: Distinguish between cancellation and transient errors

#### Graceful Shutdown Pattern
```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())

    // Handle signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigCh
        cancel()
    }()

    // Run application
    pool.Start()
    <-ctx.Done()
    pool.Stop()
}
```

**Used In**:
- `/cmd/producer/main.go`
- `/cmd/consumer/main.go`

### Build and Deployment Patterns

#### Makefile-Based Build System
```makefile
build: deps
    @mkdir -p $(BINARY_DIR)
    $(GO) build $(LDFLAGS) -o $(PRODUCER_BINARY) ./cmd/producer
    $(GO) build $(LDFLAGS) -o $(CONSUMER_BINARY) ./cmd/consumer
```

**Pattern Benefits**:
- Single command builds
- Dependency management
- Multiple targets (build, test, bench, clean)
- Optimized binaries with stripped symbols

**Build Flags**:
- `-ldflags "-s -w"`: Strip symbols and debug info
- Single binary output
- Cross-compilation ready

**Used In**: `/test-tools/Makefile`