# Configuration Variables Reference

## Helm Chart Configuration

### File Location
`helm/values-minikube.yaml` - Minikube-optimized Pulsar deployment configuration

## Component Configuration

### Volumes
```yaml
volumes:
  persistence: false  # Use emptyDir for faster local dev, no persistent storage
```
**Impact**: Data lost on pod restart. Suitable for testing, not production.

### Affinity Rules
```yaml
affinity:
  anti_affinity: false  # Allow multiple pods on same node
```
**Impact**: Enables single-node Minikube deployment. Production should use `true`.

### Component Toggles
```yaml
components:
  autorecovery: false      # Disable BookKeeper auto-recovery
  pulsar_manager: true     # Enable Pulsar Manager UI
```

## ZooKeeper Configuration

```yaml
zookeeper:
  replicaCount: 1  # Single ZK instance (no quorum)
```
**Production Recommendation**: Use 3 or 5 replicas for proper quorum.

## BookKeeper Configuration

### Replica Count
```yaml
bookkeeper:
  replicaCount: 1  # Single bookie instance
```
**Production Recommendation**: Minimum 3 bookies for HA.

### Memory Settings (Minikube Optimized)
```yaml
bookkeeper:
  configData:
    # Write cache: 32MB (default: 256MB)
    dbStorage_writeCacheMaxSizeMb: "32"

    # Read-ahead cache: 32MB (default: 256MB)
    dbStorage_readAheadCacheMaxSizeMb: "32"

    # RocksDB write buffer: 8MB (default: 64MB)
    dbStorage_rocksDB_writeBufferSizeMB: "8"

    # RocksDB block cache: 8MB (default: 268435456 bytes = 256MB)
    dbStorage_rocksDB_blockCacheSize: "8388608"
```

**Trade-offs**:
- ✅ Runs on 8GB RAM laptops
- ✅ Faster startup times
- ⚠️ Lower throughput capacity
- ⚠️ Increased disk I/O (smaller caches)

**Production Settings** (reference):
```yaml
dbStorage_writeCacheMaxSizeMb: "256"
dbStorage_readAheadCacheMaxSizeMb: "256"
dbStorage_rocksDB_writeBufferSizeMB: "64"
dbStorage_rocksDB_blockCacheSize: "268435456"  # 256MB
```

## Broker Configuration

### Replica Count
```yaml
broker:
  replicaCount: 1  # Single broker
```
**Production Recommendation**: Minimum 2-3 brokers for load distribution.

### Broker Settings
```yaml
broker:
  configData:
    # Skip unrecoverable data (for ephemeral storage)
    autoSkipNonRecoverableData: "true"

    # Ensemble configuration (matches single bookie setup)
    managedLedgerDefaultEnsembleSize: "1"
    managedLedgerDefaultWriteQuorum: "1"
    managedLedgerDefaultAckQuorum: "1"
```

### Managed Ledger Parameters

| Parameter | Minikube | Production HA | Purpose |
|-----------|----------|---------------|---------|
| `managedLedgerDefaultEnsembleSize` | 1 | 3-5 | Number of bookies to distribute ledger fragments across |
| `managedLedgerDefaultWriteQuorum` | 1 | 2-3 | Number of bookies to write each entry to |
| `managedLedgerDefaultAckQuorum` | 1 | 2 | Number of ack responses needed before write succeeds |

**Data Durability**:
- Minikube (1/1/1): No redundancy, fastest writes
- Production (3/2/2): Survives 1 bookie failure
- High Durability (5/3/2): Survives 2 bookie failures

### Auto Skip Non-Recoverable Data
```yaml
autoSkipNonRecoverableData: "true"
```
**Purpose**: Allow cluster to start even if some ledger data is lost (acceptable for ephemeral test environment).
**Production**: Should be `"false"` to prevent silent data loss.

## Proxy Configuration

```yaml
proxy:
  replicaCount: 1  # Single proxy instance
```
**Production Recommendation**: 2+ proxies for client connection resilience.

## Load Balancing Configuration (Not Currently Set)

Reference options:
```yaml
# Option 1: Newer extensible load manager
loadManagerClassName: org.apache.pulsar.broker.loadbalance.extensions.ExtensibleLoadManagerImpl

# Option 2: Stable modular load manager
loadManagerClassName: org.apache.pulsar.broker.loadbalance.impl.ModularLoadManagerImpl
```
**Status**: Not currently configured in values-minikube.yaml. Uses Pulsar default.

## High Availability Configuration (Reference)

Example HA configuration:
```yaml
broker:
  configData:
    brokerDeduplicationEnabled: true      # Exactly-once semantics
    topicLevelPoliciesEnabled: true       # Dynamic policy changes
    systemTopicEnabled: true              # Metadata topics for coordination
```

## Environment-Specific Configs

### Minikube (Current)
- **Purpose**: Local development, learning, experimentation
- **Resources**: Minimal (fits 8GB RAM laptop)
- **Durability**: None (ephemeral storage)
- **Replicas**: All components = 1
- **Trade-off**: Functionality over performance


## Monitoring Configuration

### Prometheus Files
- `monitoring/prometheus/prometheus.yml` - Basic cluster
- `monitoring/prometheus/prometheus-ha.yml` - HA cluster
- `monitoring/prometheus/prometheus-upgrade.yml` - Upgrade scenarios
- `monitoring/prometheus/rules/alerts.yml` - Alert definitions

### Grafana Dashboards
Located in `monitoring/grafana/dashboards/`:
1. `01-pulsar-overview.json` - Overall cluster health
2. `02-broker-metrics.json` - Broker performance
3. `03-bookkeeper-metrics.json` - Storage layer metrics
4. `04-topic-namespace.json` - Topic-level stats
5. `05-consumer-subscription.json` - Consumer lag and throughput
6. `06-jvm-system.json` - JVM heap, GC, threads

## Quick Reference: Common Config Changes

### Increase Throughput
```yaml
broker:
  configData:
    managedLedgerMaxEntriesPerLedger: "50000"  # Bigger ledgers
    managedLedgerMinLedgerRolloverTimeMinutes: "10"  # Less frequent rollover
```

### Improve Durability
```yaml
broker:
  configData:
    managedLedgerDefaultEnsembleSize: "3"
    managedLedgerDefaultWriteQuorum: "3"
    managedLedgerDefaultAckQuorum: "2"
```

### Reduce Resource Usage
```yaml
bookkeeper:
  resources:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "1Gi"
      cpu: "1000m"
```

### Enable Deduplication
```yaml
broker:
  configData:
    brokerDeduplicationEnabled: "true"
```

## Test Tools Configuration (test-tools/)

### Location
Configuration managed via:
1. Command-line flags (highest priority)
2. JSON configuration file (`-config` flag)
3. Performance profiles (`-profile` flag)
4. Built-in defaults (lowest priority)

### Configuration Structure

#### Pulsar Connection
```json
{
  "pulsar": {
    "service_url": "pulsar://localhost:6650",
    "topic": "persistent://public/default/perf-test"
  }
}
```

**Environment Variables**:
- `PULSAR_SERVICE_URL` - Pulsar broker URL
- `PULSAR_TOPIC` - Target topic for testing

#### Producer Configuration
```json
{
  "producer": {
    "num_producers": 5,
    "message_size": 1024,
    "batching_enabled": true,
    "batching_max_size": 1000,
    "compression_type": "LZ4",
    "max_pending_msg": 1000
  }
}
```

**Variables**:
- `num_producers` - Number of concurrent producer workers (1-100)
- `message_size` - Message payload size in bytes (1-1048576)
- `batching_enabled` - Enable message batching (true/false)
- `batching_max_size` - Maximum messages per batch (1-10000)
- `compression_type` - Compression algorithm ("None", "LZ4", "ZLIB", "ZSTD", "SNAPPY")
- `max_pending_msg` - Maximum pending messages per producer (1-100000)

#### Consumer Configuration
```json
{
  "consumer": {
    "num_consumers": 5,
    "subscription_name": "perf-test-sub",
    "subscription_type": "Shared",
    "receiver_queue_size": 1000
  }
}
```

**Variables**:
- `num_consumers` - Number of concurrent consumer workers (1-100)
- `subscription_name` - Subscription identifier
- `subscription_type` - Subscription mode ("Exclusive", "Shared", "Failover", "KeyShared")
- `receiver_queue_size` - Consumer receive queue size (1-10000)

#### Performance Configuration
```json
{
  "performance": {
    "target_throughput": 10000,
    "duration": "5m",
    "warmup": "5s",
    "rate_limit_enabled": true
  }
}
```

**Variables**:
- `target_throughput` - Target messages per second (0 = unlimited)
- `duration` - Test duration ("5m", "1h", 0 = unlimited)
- `warmup` - Warmup period before metrics collection
- `rate_limit_enabled` - Enable rate limiting (true/false)

#### Metrics Configuration
```json
{
  "metrics": {
    "collection_interval": "1s",
    "export_enabled": true,
    "export_path": "./metrics",
    "histogram_buckets": [1, 5, 10, 25, 50, 100, 250, 500, 1000]
  }
}
```

**Variables**:
- `collection_interval` - How often to update UI metrics
- `export_enabled` - Enable metrics export to files
- `export_path` - Directory for exported metrics
- `histogram_buckets` - Latency histogram bucket boundaries (milliseconds)

### Performance Profiles

Pre-configured profiles with optimized settings for specific scenarios:

#### default Profile
**Use Case**: General testing and experimentation
```
num_producers: 5
message_size: 1024
batching_enabled: true
batching_max_size: 1000
compression_type: LZ4
target_throughput: 5000
rate_limit_enabled: true
```

#### low-latency Profile
**Use Case**: Minimize message latency for real-time applications
```
num_producers: 1
message_size: 512
batching_enabled: false
compression_type: None
receiver_queue_size: 10
target_throughput: 1000
collection_interval: 100ms
histogram_buckets: [0.1, 0.5, 1, 2, 5, 10, 25, 50, 100]
```
**Trade-offs**: Lower throughput, minimal latency, fine-grained metrics

#### high-throughput Profile
**Use Case**: Maximum message throughput for batch processing
```
num_producers: 10
num_consumers: 10
message_size: 4096
batching_enabled: true
batching_max_size: 10000
compression_type: LZ4
max_pending_msg: 10000
receiver_queue_size: 10000
subscription_type: Shared
target_throughput: 0 (unlimited)
rate_limit_enabled: false
```
**Trade-offs**: Higher latency acceptable, maximize throughput

#### burst Profile
**Use Case**: Simulate bursty traffic patterns for peak load testing
```
num_producers: 5
message_size: 2048
batching_enabled: true
batching_max_size: 5000
compression_type: ZSTD
max_pending_msg: 5000
target_throughput: 10000
rate_limit_enabled: true
duration: 5m
collection_interval: 500ms
```
**Characteristics**: Rate limited with burst capacity, time-bounded

#### sustained Profile
**Use Case**: Long-running sustained load tests for endurance testing
```
num_producers: 5
message_size: 1024
batching_enabled: true
batching_max_size: 1000
compression_type: LZ4
target_throughput: 5000
rate_limit_enabled: true
duration: 0 (unlimited)
export_enabled: true
collection_interval: 1s
```
**Characteristics**: Unlimited duration, metrics export for analysis

### CLI Usage Examples

```bash
# Use default profile
./bin/producer

# Use specific profile
./bin/producer -profile high-throughput

# Override with config file
./bin/producer -profile low-latency -config custom.json

# Environment variable override
PULSAR_SERVICE_URL=pulsar://prod-cluster:6650 ./bin/producer -profile sustained
```

### Configuration Hierarchy

Configuration merges in this order (later overrides earlier):
1. Built-in defaults
2. Profile settings (if specified)
3. Config file (if specified)
4. Environment variables
5. CLI flags

### Metrics Tracked

**Producer Metrics**:
- `messages_sent` - Total messages successfully sent
- `messages_failed` - Total failed send attempts
- `bytes_sent` - Total bytes sent
- `message_rate` - Messages per second
- `throughput_mbps` - Throughput in MB/s
- `latency_min/max/mean/p50/p95/p99/p999` - Latency percentiles (ms)

**Consumer Metrics**:
- `messages_received` - Total messages received
- `messages_acked` - Total messages acknowledged
- `messages_failed` - Total failed acknowledgments
- `bytes_received` - Total bytes received
- `receive_rate` - Messages per second
- `throughput_mbps` - Throughput in MB/s
- `ack_rate_percent` - Acknowledgment success rate

### Build Configuration (Makefile)

**Build Targets**:
- `make build` - Build both producer and consumer binaries
- `make build-producer` - Build producer only
- `make build-consumer` - Build consumer only
- `make install` - Install to $GOPATH/bin
- `make test` - Run test suite
- `make test-coverage` - Generate coverage report
- `make bench` - Run benchmarks

**Build Flags**:
```makefile
LDFLAGS=-ldflags "-s -w"  # Strip symbols for smaller binary
GOFLAGS=-v                 # Verbose output
```

**Binary Locations**:
- Producer: `./bin/producer`
- Consumer: `./bin/consumer`

## References
- Apache Pulsar Documentation: https://pulsar.apache.org/docs/
- BookKeeper Configuration: https://bookkeeper.apache.org/docs/reference/config
- Pulsar Helm Chart: https://github.com/apache/pulsar-helm-chart
- Pulsar Go Client: https://github.com/apache/pulsar-client-go
- tview Documentation: https://github.com/rivo/tview