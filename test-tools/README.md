# Pulsar Performance Testing Tools

Go-based performance testing tools with interactive terminal UIs for Apache Pulsar.

## Features

- **Producer Tool**: High-performance message producer with configurable concurrency and batching
- **Consumer Tool**: Multi-threaded consumer with subscription management
- **Interactive Terminal UI**: Real-time metrics display using tview
- **Metrics Collection**: Latency histograms, throughput tracking, and percentile calculations
- **Rate Limiting**: Token bucket-based rate limiting for controlled load testing
- **Performance Profiles**: Pre-configured profiles for different testing scenarios
- **Configurable Settings**: JSON-based configuration with CLI overrides

## Prerequisites

- Go 1.25.1 or higher
- Apache Pulsar cluster (local or remote)
- Terminal with ANSI color support
- For local deployment:
  - [Minikube](https://minikube.sigs.k8s.io/docs/start/) 1.30+
  - [kubectl](https://kubernetes.io/docs/tasks/tools/) 1.27+
  - [Helm](https://helm.sh/docs/intro/install/) 3.12+

## Setting Up Local Pulsar with Minikube

This section covers deploying Apache Pulsar to a local Minikube cluster for testing.

### 1. Start Minikube

Start Minikube with sufficient resources for Pulsar:

```bash
# Start Minikube with recommended resources
minikube start --cpus=4 --memory=8192 --disk-size=20g

# Verify Minikube is running
minikube status
```

**Note**: Pulsar requires significant resources. Adjust `--cpus` and `--memory` based on your system capabilities. Minimum recommended: 4 CPUs, 8GB RAM.

### 2. Build Helm Chart Dependencies

The chart uses the Apache Pulsar chart as a dependency. Download it first:

```bash
# From the project root directory (pulsar-local-lab/)
cd helm

# Download chart dependencies
helm dependency update

# Return to project root
cd ..
```

This creates a `helm/charts/` directory with the Pulsar chart. You only need to run this once (or when updating Pulsar versions).

### 3. Deploy Pulsar

Deploy Pulsar using the local Helm chart:

```bash
# From the project root directory (pulsar-local-lab/)
helm install pulsar ./helm \
  --namespace pulsar \
  --create-namespace

# Wait for all pods to be ready (this may take 3-5 minutes)
kubectl wait --for=condition=ready pod -l app=pulsar --namespace pulsar --timeout=300s
```

**Chart structure:**
- `helm/Chart.yaml` - Chart definition with Pulsar dependency
- `helm/values.yaml` - Minikube-optimized configuration:
  - Single replica deployments (zookeeper, bookkeeper, broker, proxy)
  - Disabled persistence (uses emptyDir for faster local testing)
  - Disabled pod anti-affinity (allows running on single-node Minikube)
  - Reduced memory settings for local development
  - Pulsar Manager UI enabled

### 4. Verify Deployment

Check that all Pulsar components are running:

```bash
# Check pod status
kubectl get pods --namespace pulsar

# Check services
kubectl get svc --namespace pulsar

# View logs if needed
kubectl logs -l component=broker --namespace pulsar --tail=50
```

All pods should show `Running` status with `1/1` or `2/2` ready containers.

### 5. Applying Configuration Changes

When you need to modify Pulsar configuration:

```bash
# Edit the values file
vim helm/values.yaml

# Apply changes with Helm upgrade (from project root)
helm upgrade pulsar ./helm \
  --namespace pulsar

# Watch pods restart with new configuration
kubectl get pods --namespace pulsar -w
```

**Common changes in `values.yaml`:**
- Adjust replica counts: `pulsar.broker.replicaCount`, `pulsar.bookkeeper.replicaCount`
- Modify resource limits: Add `resources` sections under component configs
- Change configuration: Update `configData` under each component
- Enable/disable components: Modify `pulsar.components` section

**Note:** All values must be under the `pulsar:` key since it's a subchart dependency.

### 6. Uninstall Pulsar

To remove the Pulsar deployment:

```bash
# Uninstall Pulsar
helm uninstall pulsar --namespace pulsar

# Optional: Delete the namespace
kubectl delete namespace pulsar
```

## Connecting to Pulsar in Minikube

If you're running Pulsar in Minikube, use port-forwarding to access it from your local machine:

```bash
# Forward Pulsar broker port (run in separate terminal and keep running)
kubectl port-forward svc/pulsar-proxy 6650:6650

# Forward admin API port (optional, for management - maps service port 80 to localhost:8080)
kubectl port-forward svc/pulsar-proxy 8080:80
```

**Important**: Keep the port-forward command running in a separate terminal window.

Then connect using localhost:
```bash
./bin/producer --service-url pulsar://localhost:6650
./bin/consumer --service-url pulsar://localhost:6650
```

## Installation

### Clone the repository

```bash
git clone https://github.com/pulsar-local-lab/perf-test.git
cd perf-test/test-tools
```

### Install dependencies

```bash
make deps
```

### Build binaries

```bash
make build
```

This will create two binaries in the `bin/` directory:
- `bin/producer` - Producer testing tool
- `bin/consumer` - Consumer testing tool

## Usage

### Producer

Run the producer with default settings:

```bash
make run-producer
```

Or run the binary directly:

```bash
./bin/producer
```

With custom configuration:

```bash
./bin/producer -config config.json -profile high-throughput
```

### Consumer

Run the consumer with default settings:

```bash
make run-consumer
```

Or run the binary directly:

```bash
./bin/consumer
```

With custom configuration:

```bash
./bin/consumer -config config.json -profile low-latency
```

## Performance Profiles

The tools include several pre-configured performance profiles:

| Profile | Description | Use Case |
|---------|-------------|----------|
| `default` | Balanced configuration | General testing |
| `low-latency` | Optimized for minimal latency | Real-time applications |
| `high-throughput` | Optimized for maximum throughput | Batch processing |
| `burst` | Simulates bursty traffic | Peak load testing |
| `sustained` | Long-running sustained load | Endurance testing |

## Configuration

### Configuration File

Create a `config.json` file to customize settings:

```json
{
  "pulsar": {
    "service_url": "pulsar://localhost:6650",
    "topic": "persistent://public/default/perf-test"
  },
  "producer": {
    "num_producers": 5,
    "message_size": 1024,
    "batching_enabled": true,
    "batching_max_size": 1000,
    "compression_type": "LZ4"
  },
  "consumer": {
    "num_consumers": 5,
    "subscription_name": "perf-test-sub",
    "subscription_type": "Shared",
    "receiver_queue_size": 1000
  },
  "performance": {
    "target_throughput": 10000,
    "duration": "5m",
    "warmup": "5s",
    "rate_limit_enabled": true
  },
  "metrics": {
    "collection_interval": "1s",
    "export_enabled": true,
    "export_path": "./metrics"
  }
}
```

### Command-Line Flags

- `-config <path>` - Path to configuration file
- `-profile <name>` - Performance profile to use (default, low-latency, high-throughput, burst, sustained)

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

If you cannot connect to Pulsar:

1. Verify Pulsar is running: `docker ps` or check your Pulsar cluster status
2. Check the service URL in your configuration
3. Ensure the topic exists or that you have permissions to create it

### Performance Issues

If you're not achieving expected throughput:

1. Try the `high-throughput` profile
2. Increase the number of workers (`num_producers` or `num_consumers`)
3. Enable batching and adjust batch size
4. Check your network latency to the Pulsar cluster
5. Monitor Pulsar broker metrics

### Memory Issues

If you encounter memory issues:

1. Reduce the number of concurrent workers
2. Decrease the receiver queue size
3. Reduce the message size
4. Disable metrics export if not needed

## License

This project is part of the Pulsar Local Lab toolkit.

## Contributing

Contributions are welcome! Please ensure:

1. Code is properly formatted (`make fmt`)
2. All tests pass (`make test`)
3. Linter checks pass (`make lint`)
4. Documentation is updated

## Related Tools

- [Apache Pulsar](https://pulsar.apache.org/)
- [Pulsar Client Go](https://github.com/apache/pulsar-client-go)
- [tview](https://github.com/rivo/tview) - Terminal UI library