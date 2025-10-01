# Pulsar Performance Testing Tools

Go-based performance testing tools with interactive terminal UIs for Apache Pulsar.

> **📖 Full Documentation**: See the [main README](../README.md) for complete setup instructions including Pulsar deployment, monitoring, and troubleshooting.

## Features

- **Producer Tool**: High-performance message producer with configurable concurrency and batching
- **Consumer Tool**: Multi-threaded consumer with subscription management
- **Interactive Terminal UI**: Real-time metrics display using tview
- **Metrics Collection**: Latency histograms, throughput tracking, and percentile calculations
- **Rate Limiting**: Token bucket-based rate limiting for controlled load testing
- **Performance Profiles**: Pre-configured profiles for different testing scenarios
- **Dynamic Partitioning**: Create and test topics with different partition sizes
- **Configurable Settings**: JSON-based configuration with CLI overrides

## Quick Start

**Prerequisites**: Go 1.25.1+, Apache Pulsar running (see [main README](../README.md) for deployment)

### Build

```bash
make build
```

### Run

```bash
# Producer with default settings
./bin/producer

# Consumer with high-throughput profile
./bin/consumer --profile high-throughput

# Test with 4 partitions
./bin/producer --partitions 4 --workers 4
```

## Build and Installation

```bash
# Install dependencies
make deps

# Build both tools
make build
```

This creates:
- `bin/producer` - Producer testing tool
- `bin/consumer` - Consumer testing tool

## Usage Examples

### Basic Usage

```bash
# Producer with defaults
./bin/producer

# Consumer with defaults
./bin/consumer

# Or use make targets
make run-producer
make run-consumer
```

### With Profiles

```bash
./bin/producer --profile high-throughput
./bin/consumer --profile low-latency
```

### Testing Partitioned Topics

```bash
# Create and test 4-partition topic
./bin/producer --partitions 4 --workers 4

# Consumer for same topic
./bin/consumer --partitions 4 --workers 4 --subscription-type Shared
```

### Custom Configuration

```bash
./bin/producer --config config.json --profile sustained
./bin/consumer --config config.json --subscription my-sub
```

### Command-Line Overrides

```bash
./bin/producer \
  --service-url pulsar://localhost:6650 \
  --topic persistent://public/default/test \
  --partitions 8 \
  --workers 8

./bin/consumer \
  --topic persistent://public/default/test \
  --subscription-type KeyShared \
  --workers 8
```

## Performance Profiles

Pre-configured profiles for different testing scenarios:

| Profile | Workers | Batching | Target Use Case |
|---------|---------|----------|-----------------|
| `default` | 1 | Enabled | General testing |
| `low-latency` | 3 | Minimal | Real-time applications |
| `high-throughput` | 10 | Large batches | Batch processing |
| `burst` | 5 | Enabled | Peak load testing |
| `sustained` | 8 | Optimized | Endurance testing |

List all profiles: `./bin/producer --list-profiles`

## Configuration

### Configuration Options

See `config.example.json` for a complete configuration template.

**Key settings:**
- `pulsar.topic_partitions` - Number of topic partitions (0 = non-partitioned)
- `producer.num_producers` - Concurrent producer workers
- `consumer.subscription_type` - Exclusive, Shared, Failover, or KeyShared
- `performance.target_throughput` - Messages per second (0 = unlimited)
- `metrics.export_enabled` - Save metrics to JSON files

### Environment Variables

```bash
export PULSAR_SERVICE_URL=pulsar://localhost:6650
export PULSAR_TOPIC=persistent://public/default/test
export PULSAR_TOPIC_PARTITIONS=4
export PRODUCER_NUM_WORKERS=5
export CONSUMER_SUBSCRIPTION_TYPE=Shared
```

### CLI Flags Reference

Common flags for both tools:
- `--config <path>` - JSON configuration file
- `--profile <name>` - Performance profile
- `--service-url <url>` - Pulsar broker URL
- `--topic <name>` - Topic name
- `--partitions <n>` - Number of partitions (-1=use config, 0=non-partitioned)
- `--workers <n>` - Number of workers

Producer-specific:
- `--help` - Show all options

Consumer-specific:
- `--subscription <name>` - Subscription name
- `--subscription-type <type>` - Subscription type
- `--help` - Show all options

## Interactive Controls

While running the tools, you can use these keyboard shortcuts:

- `q` or `Ctrl+C` - Quit the application
- `r` - Reset metrics counters
- `p` - Pause/Resume workers

## Metrics

The tools track and display the following metrics:

### Producer Metrics
- Messages sent (total count)
- Messages failed (error count)
- Total bytes sent
- Message rate (msg/s)
- Throughput (MB/s)
- Latency statistics (min, max, mean, P50, P95, P99, P999)

### Consumer Metrics
- Messages received (total count)
- Messages acknowledged
- Messages failed
- Total bytes received
- Receive rate (msg/s)
- Throughput (MB/s)
- Acknowledgment rate (%)

## Development

### Project Structure

```
test-tools/
├── cmd/
│   ├── producer/          # Producer CLI entry point
│   └── consumer/          # Consumer CLI entry point
├── internal/
│   ├── config/            # Configuration management
│   ├── pulsar/            # Pulsar client wrappers
│   ├── metrics/           # Metrics collection and aggregation
│   ├── worker/            # Worker pool management
│   ├── generator/         # Payload generation
│   └── ui/                # Terminal UI components
├── pkg/
│   └── ratelimit/         # Rate limiting utilities
├── Makefile               # Build and development tasks
└── README.md
```

### Make Targets

```bash
make help              # Display available targets
make deps              # Download dependencies
make build             # Build both producer and consumer
make build-producer    # Build only producer
make build-consumer    # Build only consumer
make run-producer      # Build and run producer
make run-consumer      # Build and run consumer
make test              # Run tests
make test-coverage     # Run tests with coverage
make bench             # Run benchmarks
make clean             # Remove build artifacts
make fmt               # Format code
make vet               # Run go vet
make lint              # Run golangci-lint
make tidy              # Tidy go.mod
make all               # Run all checks and build
```

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run benchmarks
make bench
```

### Code Formatting

```bash
# Format code
make fmt

# Run linter
make lint

# Run go vet
make vet
```

## Troubleshooting

### Connection Issues

**Problem**: Cannot connect to Pulsar

**Solutions:**
1. Verify Pulsar is running: `kubectl get pods -n pulsar`
2. Check port forwarding is active: `kubectl port-forward -n pulsar svc/pulsar-broker 6650:6650`
3. Verify service URL in configuration

### Topic Creation Failures

**Problem**: "Failed to ensure topic exists"

**Solutions:**
1. Check Pulsar Manager is running: `kubectl get pods -n pulsar | grep manager`
2. Verify admin API access: `kubectl port-forward -n pulsar svc/pulsar-broker 8080:8080`
3. Check topic permissions and namespace configuration

### Performance Issues

**Problem**: Low throughput

**Solutions:**
1. Use `--profile high-throughput`
2. Increase workers: `--workers 10`
3. For partitioned topics, match workers to partitions
4. Check network latency and broker metrics in Grafana

See the [main README](../README.md) for more troubleshooting guidance.

## Development

### Running Tests

```bash
make test              # Run all tests
make test-coverage     # With coverage report
make bench             # Run benchmarks
```

### Code Quality

```bash
make fmt               # Format code
make vet               # Run go vet
make lint              # Run golangci-lint
```

## Related Documentation

- **[Main README](../README.md)** - Complete project documentation
- **[Apache Pulsar Docs](https://pulsar.apache.org/docs/)** - Official Pulsar documentation
- **[Pulsar Client Go](https://github.com/apache/pulsar-client-go)** - Go client library