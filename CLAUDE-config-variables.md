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

## References
- Apache Pulsar Documentation: https://pulsar.apache.org/docs/
- BookKeeper Configuration: https://bookkeeper.apache.org/docs/reference/config
- Pulsar Helm Chart: https://github.com/apache/pulsar-helm-chart