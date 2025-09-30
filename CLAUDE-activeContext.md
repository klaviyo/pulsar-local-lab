# Active Context

## Current Project Status

### Project: Apache Pulsar Local Lab
**Last Updated**: 2025-09-30
**Current Phase**: Test tools implementation complete - Infrastructure operational

### What's Been Done
1. ✅ Initial project structure created
2. ✅ Minikube Helm values configuration (`helm/values-minikube.yaml`)
3. ✅ Monitoring infrastructure setup:
   - 3 Prometheus configurations (prometheus.yml, prometheus-ha.yml, prometheus-upgrade.yml)
   - 6 Grafana dashboards (overview, broker, bookkeeper, topics, consumers, JVM)
   - Alert rules defined
4. ✅ Git repository initialized
5. ✅ Claude Code configuration (.claude directory with agents/commands)
6. ✅ **Performance Testing Tools (test-tools/) - COMPLETE**:
   - Two CLI applications: `producer` and `consumer`
   - Full Go implementation with tview terminal UI
   - 5 performance profiles (default, low-latency, high-throughput, burst, sustained)
   - Lock-free metrics collection with atomic operations
   - Token bucket rate limiter
   - Comprehensive test suite (88.8% avg coverage on tested packages)
   - Production-ready with graceful shutdown

### Technology Switch Decision
**Python to Go Migration**: Originally planned Python implementation abandoned due to pulsar-client-python lacking Python 3.13 support. Go implementation provides:
- Better concurrency primitives (goroutines, channels)
- Superior performance (2.6M payload/sec generation)
- Native cross-compilation
- Single binary deployment
- Strong ecosystem with pulsar-client-go

### Current State
- **Repository**: Clean with uncommitted CLAUDE memory bank files and .claude/ directory
- **Deployment Target**: Kubernetes via Minikube
- **Test Tools**: Located at `/Users/fabian.haupt/projects/pulsar-local-lab/test-tools/`
- **Recent Commits**:
  - `d8133ea` - Switching to minikube
  - `01305b7` - Parallel tests
  - `61d628a` - Stuff
  - `b49e468` - More fixes

### Active Goals
Performance testing tools complete and operational. System ready for:
- Load testing Pulsar clusters
- Benchmarking different configurations
- Validating infrastructure setup
- Testing various workload patterns

## Test Tools Architecture

### Project Structure
```
test-tools/
├── cmd/
│   ├── producer/main.go          # Producer CLI entry point
│   └── consumer/main.go          # Consumer CLI entry point
├── internal/
│   ├── config/                   # Configuration + profiles
│   ├── pulsar/                   # Pulsar client wrappers
│   ├── metrics/                  # Lock-free metrics collection
│   ├── worker/                   # Worker pool management
│   ├── generator/                # High-performance payload generation
│   └── ui/                       # tview terminal UI components
├── pkg/
│   └── ratelimit/                # Token bucket rate limiter
├── go.mod                        # Go 1.25.1+
├── Makefile                      # Build automation
└── README.md                     # Comprehensive documentation
```

### Key Performance Characteristics
- **Payload Generation**: 2.6M payloads/second (JSON generation)
- **Metrics Overhead**: 1.66ns per operation (atomic operations)
- **Rate Limiter**: 0.87ns overhead (token bucket with atomic CAS)
- **Concurrency**: Goroutine-based worker pools
- **Memory Safety**: Lock-free design with atomic operations

### Configuration Profiles Available
1. **default**: Balanced configuration (5 workers, 1KB messages, batching enabled)
2. **low-latency**: Minimal latency (batching disabled, small queues, single worker)
3. **high-throughput**: Maximum throughput (10 workers, large batches, LZ4 compression)
4. **burst**: Bursty traffic simulation (5 workers, 10K msg/s, ZSTD compression)
5. **sustained**: Long-running load (5 workers, unlimited duration, metrics export)

## Session Notes
- Memory bank system synchronized with completed Go implementation
- Test tools provide comprehensive Pulsar performance testing capabilities
- Architecture emphasizes performance, safety, and operational simplicity
- Ready for real-world Pulsar cluster testing and validation