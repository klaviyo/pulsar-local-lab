# Pulsar Local Lab

Local testing environment for Apache Pulsar with Kubernetes (Minikube) and comprehensive performance testing tools.

## 🎯 Purpose

A development environment for learning and experimenting with Apache Pulsar, featuring:
- Production-like Pulsar cluster on Minikube
- Performance testing tools (producer/consumer in Go)
- Monitoring (Prometheus + Grafana dashboards)
- Web-based admin UI (Pulsar Manager)

## 🚀 Quick Start

### Prerequisites

- Minikube running
- Helm 3.x
- kubectl
- Go 1.25.1+ (for test tools)

### 1. Deploy Pulsar Cluster

```bash
# Download Helm chart dependencies (one-time)
cd helm && helm dependency update && cd ..

# Deploy to Minikube
helm install pulsar ./helm --namespace pulsar --create-namespace
```

### 2. Access Web UIs

```bash
# Start port forwarding and display credentials
./scripts/access-ui.sh
```

This will show:
- **Grafana**: http://localhost:3000 (admin/admin)
- **Pulsar Manager**: http://localhost:9527 (credentials displayed by script)

**Important:** Pulsar Manager credentials are auto-generated during installation. Use the helper script to retrieve them:

```bash
./scripts/get-manager-credentials.sh
```

### 3. Build and Run Test Tools

```bash
cd test-tools

# Build binaries
make build

# Run producer with default settings
./bin/producer

# Run consumer with high-throughput profile
./bin/consumer --profile high-throughput
```

## 📊 Monitoring

### Grafana Dashboards

Access at http://localhost:3000 (after running `./scripts/access-ui.sh`)

Available dashboards:
- Pulsar Cluster Overview
- Topic Metrics
- Broker Performance
- BookKeeper Stats
- Producer/Consumer Metrics

### Pulsar Manager (Admin UI)

Web-based admin console at http://localhost:9527

Features:
- Topic management (create, delete, view stats)
- Partition management
- Namespace and tenant administration
- Subscription monitoring
- Broker and cluster health

**Get credentials:**
```bash
./scripts/get-manager-credentials.sh
```

## 🧪 Performance Testing

### Test Tool Features

- **Multiple profiles**: default, low-latency, high-throughput, burst, sustained
- **Dynamic partitioning**: Test different partition sizes
- **Real-time metrics**: Terminal UI with live stats
- **Configurable**: JSON configs or CLI flags
- **Rate limiting**: Control message throughput

### Quick Examples

```bash
# Test with 4 partitions and 4 workers
./bin/producer --partitions 4 --workers 4

# High-throughput consumer with shared subscription
./bin/consumer --partitions 4 --workers 4 --subscription-type Shared

# Custom message size and rate
./bin/producer --profile sustained --workers 10

# Load from config file
./bin/producer --config myconfig.json
```

### Available Profiles

- **default**: Single worker, no rate limiting, 1KB messages
- **low-latency**: Minimal batching, optimized for speed
- **high-throughput**: Large batches, compression, high throughput
- **burst**: Short bursts of high traffic
- **sustained**: Steady load over time

Run `./bin/producer --list-profiles` or `./bin/consumer --list-profiles` for details.

## 🔧 Configuration

### Helm Values

Main configuration in `helm/values.yaml`:
- Minikube-optimized resource limits
- Single replica for development
- No persistence (uses emptyDir)
- Pulsar Manager enabled

### Test Tool Configuration

#### Environment Variables

```bash
export PULSAR_SERVICE_URL=pulsar://localhost:6650
export PULSAR_TOPIC=persistent://public/default/my-test
export PULSAR_TOPIC_PARTITIONS=4
export PRODUCER_NUM_WORKERS=5
export CONSUMER_SUBSCRIPTION_TYPE=Shared
```

#### JSON Configuration

See `test-tools/config.example.json` for full configuration options.

## 📁 Project Structure

```
.
├── helm/                      # Helm chart for Pulsar
│   ├── Chart.yaml            # Chart definition with dependencies
│   └── values.yaml           # Minikube-optimized values
├── monitoring/               # Grafana dashboards
├── scripts/                  # Helper scripts
│   ├── access-ui.sh          # Start port forwarding for UIs
│   └── get-manager-credentials.sh  # Retrieve Pulsar Manager credentials
└── test-tools/               # Performance testing tools
    ├── cmd/                  # CLI applications
    │   ├── producer/         # Producer binary
    │   └── consumer/         # Consumer binary
    ├── internal/             # Core implementation
    │   ├── config/           # Configuration management
    │   ├── metrics/          # Metrics collection
    │   ├── pulsar/           # Admin API integration
    │   ├── ui/               # Terminal UI
    │   └── worker/           # Producer/consumer workers
    └── pkg/                  # Reusable packages
        └── ratelimit/        # Token bucket rate limiter
```

## 🛠️ Common Tasks

### Managing Topics

#### Via CLI (inside broker pod)
```bash
# Delete partitioned topic
kubectl exec -n pulsar pulsar-broker-0 -- \
  bin/pulsar-admin topics delete-partitioned-topic \
  persistent://public/default/perf-test --force

# List topics
kubectl exec -n pulsar pulsar-broker-0 -- \
  bin/pulsar-admin topics list public/default
```

#### Via Pulsar Manager UI
1. Access http://localhost:9527 (run `./scripts/access-ui.sh`)
2. Navigate to Topics → Manage
3. Use web interface for create/delete/stats

### Port Forwarding Services

```bash
# Pulsar broker (client connections)
kubectl port-forward -n pulsar svc/pulsar-broker 6650:6650

# Pulsar admin API
kubectl port-forward -n pulsar svc/pulsar-broker 8080:8080

# Grafana
kubectl port-forward -n pulsar svc/grafana 3000:3000

# Pulsar Manager
kubectl port-forward -n pulsar svc/pulsar-pulsar-manager 9527:9527
```

Or use the helper script:
```bash
./scripts/access-ui.sh
```

### Viewing Logs

```bash
# Broker logs
kubectl logs -n pulsar pulsar-broker-0 -f

# Pulsar Manager logs
kubectl logs -n pulsar pulsar-pulsar-manager-0 -f

# Test tool logs (shown in terminal UI)
# Press 'L' to view log panel in the TUI
```

## 🧪 Development

### Test Tools

```bash
cd test-tools

# Run tests
make test

# Run benchmarks
make bench

# Format code
make fmt

# View coverage
make coverage
```

### Adding Custom Profiles

Edit `test-tools/internal/config/profiles.go` to add new performance profiles.

## 📚 Documentation

- **CLAUDE.md** - AI assistant guidance and quick reference
- **CLAUDE-decisions.md** - Architecture decisions and rationale
- **CLAUDE-patterns.md** - Code patterns and conventions
- **CLAUDE-config-variables.md** - Configuration reference

## 🔍 Troubleshooting

### Pulsar Manager Login Issues

If login fails, retrieve the actual credentials:
```bash
./scripts/get-manager-credentials.sh
```

Credentials are randomly generated during Helm installation and stored in a Kubernetes secret.

### Port Already in Use

```bash
# Kill processes using ports
lsof -ti:9527 | xargs kill -9  # Pulsar Manager
lsof -ti:3000 | xargs kill -9  # Grafana
```

The `access-ui.sh` script automatically handles this.

### Test Tools Connection Failed

Ensure port forwarding is active:
```bash
kubectl port-forward -n pulsar svc/pulsar-broker 6650:6650
```

Or update config to use the service URL directly if running inside Kubernetes.

### Topics Won't Delete

Force delete with subscriptions:
```bash
kubectl exec -n pulsar pulsar-broker-0 -- \
  bin/pulsar-admin topics delete-partitioned-topic \
  persistent://public/default/your-topic --force
```

## 🤝 Contributing

This is a learning environment. Feel free to experiment and modify as needed.

## 📄 License

Apache License 2.0
