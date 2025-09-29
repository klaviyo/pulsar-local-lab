# Python Pulsar Throughput Testing Suite - Implementation Plan

## Overview
Build Python 3.13+ free-threaded performance testing tools with interactive terminal UIs for Apache Pulsar throughput testing on our Minikube cluster.

## Project Structure
```
test-tools/
├── requirements.txt              # Dependencies (Python 3.13+ required)
├── pyproject.toml               # Build config specifying free-threaded build
├── config.py                    # Shared configuration & connection settings
├── pulsar_producer.py          # Producer controller with TUI
├── pulsar_consumer.py          # Consumer controller with TUI
├── lib/
│   ├── __init__.py
│   ├── pulsar_client.py       # Pulsar client wrapper (GIL-free optimized)
│   ├── metrics.py             # Lock-free metrics collection & aggregation
│   ├── thread_manager.py      # Free-threaded producer/consumer management
│   ├── perf_critical.py       # Performance-critical code (message encoding, batching)
│   └── ui_components.py       # Reusable UI widgets
└── README.md                   # Usage documentation with 3.13 setup
```

## Technology Stack
- **Python**: 3.13+ with free-threaded build (`python3.13t`)
- **Pulsar Client**: `pulsar-client>=3.5.0` (C++ backed, GIL-free friendly)
- **TUI Framework**: `textual>=0.90.0` (modern terminal UI with graphing)
- **Metrics**: `rich>=13.0.0` (bundled with textual)
- **Performance**: Native threading without GIL for CPU-intensive operations

## Python 3.13 Free-Threading Strategy

### Performance-Critical Components (GIL-Free)
These will benefit from true parallelism:

1. **Message Payload Generation** (`lib/perf_critical.py`)
   - Random payload generation
   - Message serialization/compression
   - Batch assembly
   - Runs in parallel threads without GIL contention

2. **Metrics Aggregation** (`lib/metrics.py`)
   - Lock-free data structures where possible
   - Concurrent histogram updates (latency tracking)
   - Percentile calculations
   - Rolling window computations

3. **Message Processing** (`lib/thread_manager.py`)
   - Multiple producer threads sending simultaneously
   - Multiple consumer threads receiving simultaneously
   - True parallel throughput (not limited by GIL)

### Architecture Pattern
```
Main Thread (TUI)
    ↓ (queue communication)
Worker Pool (Free-Threaded)
    ├─ Producer Thread 1 ──┐
    ├─ Producer Thread 2 ──┤ (Truly parallel, no GIL!)
    ├─ Producer Thread N ──┤
    └─ Metrics Thread    ──┘ (Aggregates without blocking)
```

## Core Features

### Producer Controller (`pulsar_producer.py`)

**Dynamic Configuration Panel:**
- Target throughput (msg/sec) - live adjustable
- Message size (bytes) - configurable payload
- Batch size - messages per batch
- Compression type (none, LZ4, ZLIB, ZSTD, SNAPPY)
- Topic name - connect to any topic
- Number of threads - spawn/kill dynamically (leveraging free-threading)
- Producer name prefix
- Payload generation strategy (random, sequential, pattern)

**Live Metrics Dashboard:**
- Real-time throughput graph (msg/sec over time) - 100ms granularity
- Target vs actual rate comparison with visual indicator
- Total messages sent
- Success/failure counts with error categorization
- Latency percentiles (P50, P95, P99, P999)
- Per-thread throughput breakdown
- Active thread count with CPU utilization
- Batch efficiency metrics
- Network throughput (MB/sec)
- GIL contention metrics (show free-threading benefits)

**Interactive Controls:**
- Add/remove producer threads (hotkeys: +/-)
- Pause/resume all producers (spacebar)
- Reset metrics (R key)
- Adjust rate with arrow keys (fine/coarse modes)
- Toggle burst mode (B key)
- Emergency stop (Q key)
- Export metrics snapshot (E key)

### Consumer Controller (`pulsar_consumer.py`)

**Dynamic Configuration Panel:**
- Subscription name
- Subscription type (Exclusive, Shared, Failover, Key_Shared)
- Topic name/pattern
- Number of threads - true parallel consumption
- Consumer name prefix
- Receive queue size
- Message listener or polling mode
- Acknowledgment strategy (immediate, batched, delayed)

**Live Metrics Dashboard:**
- Real-time consumption rate graph (msg/sec over time)
- End-to-end latency distribution (histogram)
- Processing latency (receive to ack)
- Total messages received
- Consumer lag per partition (broker metrics)
- Messages in flight
- Per-thread consumption breakdown
- Active thread count with CPU utilization
- Network throughput (MB/sec)
- Redelivery count with reasons
- GIL-free performance indicator

**Interactive Controls:**
- Add/remove consumer threads (hotkeys: +/-)
- Pause/resume consumption (spacebar)
- Reset metrics (R key)
- Acknowledge mode toggle (auto/manual/batched)
- Seek to timestamp/message ID (S key)
- Emergency stop (Q key)
- Export metrics snapshot (E key)

## Advanced Features

### Connection Management
- Connection string from environment or config
- Port-forward helper command for minikube
- Auto-reconnect with exponential backoff
- Connection pool status with health checks
- TLS/Auth support (future-ready)

### Producer Advanced Features
- **Routing modes**: SinglePartition, RoundRobinPartition, CustomPartition
- **Send timeout**: Per-message configurable
- **Async callbacks**: Track individual message fates
- **Producer sequence IDs**: Deduplication support
- **Message properties**: Custom metadata injection
- **Partitioned topics**: Auto-discovery and balancing
- **Schema support**: JSON, Avro, Protobuf (future)

### Consumer Advanced Features
- **Negative acknowledgment**: Explicit rejection
- **Dead letter topic**: DLQ configuration
- **Seek operations**: By timestamp, message ID, or beginning/end
- **Pattern subscriptions**: Regex topic matching
- **Chunked messages**: Automatic assembly
- **Priority levels**: Priority queue consumption
- **Replay mode**: Re-consume historical messages

### Performance Monitoring
- **CPU utilization**: Per-thread tracking
- **Memory usage**: RSS and heap tracking
- **Network I/O**: Bytes sent/received per second
- **GIL metrics**: Show free-threading effectiveness
- **Thread efficiency**: Work done vs idle time
- **Backpressure detection**: Queue depth monitoring

### Metrics & Visualization
- **Time-series graphs**: 60-second rolling window
- **Latency histograms**: HDR histogram for accuracy
- **Heatmaps**: Latency over time visualization
- **Percentile trends**: P50/P95/P99 over time
- **Error categorization**: By type and frequency
- **Export formats**: JSON, CSV, Prometheus metrics

### Configuration Profiles
- **Presets**: Low/medium/high throughput, stress test, sustained
- **Save/load**: Named profiles to disk
- **CLI overrides**: All settings via command-line args
- **Environment variables**: Docker/k8s friendly config

### Test Scenarios (Automation)
- **Ramp test**: Linear increase from 0 to target rate
- **Spike test**: Sudden bursts at intervals
- **Sustained load**: Constant rate for duration
- **Chaos mode**: Random rate/size variation
- **Backpressure test**: Overwhelm consumer capacity

## Implementation Plan with Subagents

### Phase 1: Project Setup & Infrastructure
**Subagent**: `@general-purpose`
- Create directory structure `test-tools/` with subdirectories
- Create `requirements.txt` with Python 3.13+ dependencies
- Create `pyproject.toml` specifying free-threaded Python requirement
- Create stub files for all modules
- Create `README.md` with Python 3.13t installation instructions

### Phase 2: Configuration & Connection Module
**Subagent**: `@nodejs-game-developer` (full-stack expertise applicable to config management)
- Implement `config.py` with environment variable support
- Add connection string management
- Add profile loading/saving functionality
- Add validation for Python 3.13+ requirement
- Document configuration options

### Phase 3: Pulsar Client Wrapper
**Subagent**: `@nodejs-game-developer` (backend architecture expertise)
- Implement `lib/pulsar_client.py` wrapper around pulsar-client
- Add connection pooling and auto-reconnect logic
- Implement producer wrapper with all send options
- Implement consumer wrapper with all subscription types
- Add error handling and retry logic
- Add health check functionality

### Phase 4: Performance-Critical Components
**Subagent**: `@general-purpose` (for Python performance optimization)
- Implement `lib/perf_critical.py` with GIL-free message generation
- Optimize payload generation (random, sequential, pattern)
- Implement efficient batch assembly
- Add compression handling
- Benchmark and verify free-threading benefits

### Phase 5: Metrics Collection System
**Subagent**: `@general-purpose`
- Implement `lib/metrics.py` with lock-free data structures
- Create thread-safe metrics aggregator
- Implement HDR histogram for latency tracking
- Add percentile calculations (P50, P95, P99, P999)
- Implement rolling window for time-series data
- Add export functionality (JSON, CSV, Prometheus)

### Phase 6: Thread Manager
**Subagent**: `@general-purpose`
- Implement `lib/thread_manager.py` for worker pool management
- Add dynamic thread spawning/killing
- Implement graceful shutdown
- Add CPU pinning support (optional)
- Add per-thread metrics tracking
- Implement thread health monitoring

### Phase 7: UI Components Library
**Subagent**: `@ux-design-expert` (for UI/UX design)
- Design reusable Textual widgets in `lib/ui_components.py`
- Create configuration panel component
- Create metrics dashboard component
- Create real-time graph widget
- Create status bar component
- Create help/keybinding overlay
- Ensure consistent styling and layout

### Phase 8: Producer Application
**Subagent**: `@nodejs-game-developer` (full-stack development)
- Implement `pulsar_producer.py` main application
- Integrate Textual UI framework
- Wire up configuration panel with live updates
- Implement producer worker threads
- Add interactive controls (hotkeys)
- Integrate real-time metrics and graphs
- Add rate control logic (target vs actual)
- Implement pause/resume functionality
- Add metrics export on demand

### Phase 9: Consumer Application
**Subagent**: `@nodejs-game-developer`
- Implement `pulsar_consumer.py` main application
- Mirror producer UI layout for consistency
- Wire up consumer configuration panel
- Implement consumer worker threads
- Add subscription management
- Integrate real-time metrics and graphs
- Add acknowledgment controls
- Implement seek functionality
- Add lag monitoring

### Phase 10: Advanced Features
**Subagent**: `@general-purpose`
- Implement configuration profile system (save/load presets)
- Add CLI argument parsing for all options
- Implement test scenario automation framework
- Add advanced Pulsar features (DLQ, routing, patterns)
- Add performance comparison mode (with/without free-threading)

### Phase 11: Testing & Validation
**Subagent**: `@qa-requirements-validator`
- Verify all features work as specified
- Test with Minikube Pulsar cluster
- Validate metrics accuracy
- Test thread scaling (1 to 32 threads)
- Verify free-threading performance gains
- Test all interactive controls
- Validate configuration profiles
- Test error handling and reconnection

### Phase 12: Documentation & Integration
**Subagent**: `@general-purpose`
- Complete README.md with comprehensive setup guide
- Add Python 3.13t installation instructions
- Document Minikube port-forwarding setup
- Create usage examples for common scenarios
- Add performance tuning guide
- Document configuration options
- Create troubleshooting section

### Phase 13: Memory Bank Update
**Subagent**: `@memory-bank-synchronizer`
- Update `CLAUDE-patterns.md` with testing tool patterns
- Update `CLAUDE-decisions.md` with Python 3.13 free-threading decision
- Update `CLAUDE-activeContext.md` with current implementation status
- Add testing approach to documentation

### Phase 14: Design Review (Optional)
**Subagent**: `@design-review`
- Review TUI design for usability
- Validate color schemes and visual hierarchy
- Check accessibility (keyboard navigation)
- Verify responsive layout for different terminal sizes
- Ensure consistent UX between producer and consumer apps

## Configuration Example

```python
# config.py
import os
import sys

# Python 3.13 free-threaded verification
if not sys.version_info >= (3, 13):
    raise RuntimeError("Python 3.13+ required for free-threading support")

# Connection
PULSAR_BROKER_URL = os.getenv("PULSAR_BROKER_URL", "pulsar://localhost:6650")
PULSAR_ADMIN_URL = os.getenv("PULSAR_ADMIN_URL", "http://localhost:8080")

# Topic configuration
DEFAULT_TOPIC = "persistent://public/default/perf-test"
DEFAULT_SUBSCRIPTION = "perf-test-sub"

# Performance settings
DEFAULT_MESSAGE_SIZE = 1024  # bytes
DEFAULT_TARGET_RATE = 10000  # msg/sec (higher with free-threading!)
DEFAULT_THREADS = 4  # Optimal: num_cores or num_cores - 1
DEFAULT_BATCH_SIZE = 100
MAX_THREADS = 32

# Metrics
METRICS_UPDATE_INTERVAL = 0.1  # 100ms for high-frequency updates
GRAPH_WINDOW_SECONDS = 60
HISTOGRAM_BUCKETS = 100

# Free-threading optimization
CPU_PINNING_ENABLED = True  # Pin threads to specific cores
NUMA_AWARE = True  # NUMA-aware memory allocation
```

## Key Dependencies

```txt
# requirements.txt
pulsar-client>=3.5.0
textual>=0.90.0
rich>=13.0.0
numpy>=2.0.0  # For efficient metrics computation
psutil>=6.0.0  # For CPU/memory monitoring

# Python 3.13t required (free-threaded build)
# Installation: python3.13t -m pip install -r requirements.txt
```

## Performance Expectations with Free-Threading

- **2-4x throughput** on multi-core systems (4+ cores)
- **Lower latency** due to reduced GIL contention
- **Better CPU utilization** (80%+ vs 25% single-threaded)
- **Linear scaling** with thread count (up to core count)

## Deliverables

1. Two standalone Python 3.13t applications (producer + consumer)
2. Shared performance-optimized library modules
3. Configuration system with Minikube defaults
4. Python 3.13 free-threaded setup guide in README
5. Performance comparison documentation
6. Integration notes for existing monitoring stack
7. Updated memory bank files documenting testing patterns

## Success Criteria

- ✅ Both applications run on Python 3.13t free-threaded build
- ✅ Interactive TUI with real-time graphs updates at 100ms intervals
- ✅ Dynamic thread scaling (add/remove without restart)
- ✅ Configuration changes apply immediately without restart
- ✅ Accurate metrics (throughput, latency, success rates)
- ✅ Export metrics to JSON/CSV
- ✅ Connect to Minikube Pulsar cluster successfully
- ✅ Demonstrate 2x+ performance improvement with free-threading
- ✅ Handle 10,000+ msg/sec per application
- ✅ Graceful error handling and reconnection
- ✅ Comprehensive documentation

## Timeline Estimate

- Phase 1-3: Setup & infrastructure (2-3 hours)
- Phase 4-6: Core performance components (4-5 hours)
- Phase 7: UI components (3-4 hours)
- Phase 8-9: Main applications (6-8 hours)
- Phase 10: Advanced features (3-4 hours)
- Phase 11: Testing (2-3 hours)
- Phase 12-14: Documentation & polish (2-3 hours)

**Total**: ~25-35 hours of development time

## Future Enhancements

- Schema support (Avro, Protobuf, JSON Schema)
- Multi-cluster testing (produce to one, consume from another)
- Message validation and content inspection
- Built-in load testing scenarios with assertions
- Integration with Grafana for long-term metric storage
- Web UI mode (Textual supports web deployment)
- Distributed testing (coordinate multiple producers/consumers)