# Pulsar Local Lab Implementation Plan

## 🎯 Project Objectives

Build a comprehensive local testing environment for Apache Pulsar to experiment with:
- **Cluster Configuration** - Multi-broker setups and load balancing strategies
- **Performance Testing** - Throughput, latency, and scaling characteristics
- **Zero-Downtime Operations** - Rolling upgrades and maintenance procedures
- **High Availability** - Failover scenarios and disaster recovery

## 📁 Project Structure

```
pulsar-local-lab/
├── docker-compose/
│   ├── basic-cluster/          # 2 brokers, 3 bookies, 1 ZK
│   ├── ha-cluster/             # 3 brokers, 5 bookies, 3 ZK
│   └── upgrade-test/           # Version upgrade scenarios
├── kubernetes/
│   ├── manifests/              # Raw K8s YAML files
│   └── helm-values/            # Helm chart configurations
├── test-scripts/
│   ├── performance/            # Throughput and latency tests
│   ├── failover/               # Chaos engineering scripts
│   └── upgrade/                # Rolling update procedures
├── monitoring/
│   ├── grafana/                # Dashboards and provisioning
│   └── prometheus/             # Metrics collection config
└── procedures/
    ├── upgrade-playbook.md     # Step-by-step upgrade guide
    └── failover-playbook.md    # Disaster recovery procedures
```

## 🚀 Implementation Phases

### Phase 1: Basic Multi-Broker Setup
**Goal**: Establish foundational cluster with monitoring

**Components**:
- 3 BookKeeper nodes (minimum quorum)
- 2 Pulsar brokers (load distribution)
- 1 ZooKeeper instance (simplified coordination)
- Prometheus + Grafana (observability)

**Key Deliverables**:
- `docker-compose/basic-cluster/docker-compose.yml`
- `monitoring/prometheus/prometheus.yml`
- `test-scripts/performance/baseline-test.sh`
- `docker-compose/basic-cluster/start-cluster.sh`

**Configuration Focus**:
- `loadManagerClassName` for load balancing strategies
- `managedLedgerDefaultEnsembleSize/WriteQuorum/AckQuorum` for durability
- `brokerDeduplicationEnabled` for exactly-once semantics
- `topicLevelPoliciesEnabled` for dynamic configuration

---

### Phase 2: High Availability Setup
**Goal**: Production-like resilience and fault tolerance

**Components**:
- 3 ZooKeeper ensemble (proper quorum)
- 5 BookKeeper nodes (2-failure tolerance)
- 3 Pulsar brokers (proper load distribution)
- Pulsar Proxy (client connection resilience)

**Key Deliverables**:
- `docker-compose/ha-cluster/docker-compose.yml`
- `test-scripts/failover/broker-failover.sh`
- `test-scripts/failover/bookie-failover.sh`
- `test-scripts/failover/network-chaos.sh`

**Testing Scenarios**:
- Network partition simulation
- Component failure recovery
- Load rebalancing verification
- Client reconnection behavior

---

### Phase 3: Kubernetes Environment
**Goal**: Container orchestration and cloud-native operations

**Components**:
- Kind/minikube local cluster
- Pulsar Helm chart deployment
- Pod disruption budgets
- Node affinity rules

**Key Deliverables**:
- `kubernetes/kind-config.yml`
- `kubernetes/helm-values/values-basic.yaml`
- `kubernetes/helm-values/values-ha.yaml`
- `kubernetes/setup-k8s.sh`
- `test-scripts/upgrade/rolling-upgrade.sh`

**Advanced Features**:
- Blue-green deployment strategies
- Canary releases with traffic splitting
- Automated scaling based on metrics
- Resource limit testing

---

### Phase 4: Testing & Procedures
**Goal**: Comprehensive operational playbooks

**Performance Testing**:
- Baseline throughput measurements
- Latency characteristics under load
- Consumer lag monitoring
- Storage utilization patterns

**Failover Procedures**:
- Broker failure recovery time
- BookKeeper node replacement
- ZooKeeper leader election
- Message ordering guarantees

**Zero-Downtime Upgrades**:
- Rolling broker updates
- BookKeeper ensemble replacement
- Configuration hot-reloading
- Rollback procedures

## 🔧 Key Configuration Parameters

### BookKeeper Settings
```yaml
managedLedgerDefaultEnsembleSize: 3-5    # Replication factor
managedLedgerDefaultWriteQuorum: 2-3     # Write acknowledgments
managedLedgerDefaultAckQuorum: 2         # Read acknowledgments
```

### Load Balancing
```yaml
loadManagerClassName: 
  - org.apache.pulsar.broker.loadbalance.extensions.ExtensibleLoadManagerImpl
  - org.apache.pulsar.broker.loadbalance.impl.ModularLoadManagerImpl
```

### High Availability
```yaml
brokerDeduplicationEnabled: true         # Exactly-once semantics
topicLevelPoliciesEnabled: true          # Dynamic policies
systemTopicEnabled: true                 # Metadata topics
```

## 📊 Monitoring & Observability

### Metrics Collection
- **Prometheus** scrapes from all Pulsar components
- **Grafana** dashboards for visualization
- **Custom alerts** for critical thresholds

### Key Metrics to Track
- Message throughput (msg/sec)
- End-to-end latency (p50, p99)
- Consumer lag by subscription
- Broker CPU/memory utilization
- BookKeeper write/read latency
- ZooKeeper session counts

### Performance Baselines
```bash
# Producer throughput test
bin/pulsar-perf produce --rate 1000 --num-messages 10000 --size 1024

# Consumer latency test  
bin/pulsar-perf consume --subscription-name test-sub --num-messages 10000

# Multi-partition scaling test
bin/pulsar-admin topics create --partitions 8 persistent://public/default/scale-test
```

## 🧪 Test Scenarios

### Chaos Engineering
1. **Network Partitions**: Isolate components temporarily
2. **Resource Exhaustion**: CPU/memory pressure testing  
3. **Disk Failures**: BookKeeper storage corruption
4. **Clock Skew**: Time synchronization issues

### Performance Validation
1. **Linear Scaling**: Throughput vs partition count
2. **Latency Bounds**: P99 latency under different loads
3. **Consumer Patterns**: Shared vs exclusive subscriptions
4. **Retention Impact**: Performance with long retention

### Operational Procedures
1. **Rolling Upgrades**: Zero-downtime version updates
2. **Cluster Expansion**: Adding brokers/bookies dynamically
3. **Topic Migration**: Moving topics between clusters
4. **Backup/Restore**: Data recovery procedures

## 🚦 Success Criteria

### Functional Requirements
- ✅ Cluster startup in < 2 minutes
- ✅ Sub-second broker failover detection
- ✅ Zero message loss during planned maintenance
- ✅ Automatic load rebalancing within 30 seconds

### Performance Targets
- ✅ 10,000+ msg/sec sustained throughput
- ✅ < 10ms P99 latency for 1KB messages
- ✅ < 5 second consumer lag during normal operations
- ✅ 99.9%+ uptime during chaos testing

### Operational Goals
- ✅ Upgrade procedures tested and documented
- ✅ Monitoring dashboards configured
- ✅ Runbooks for common failure scenarios
- ✅ Automated test suite for regression testing

## 🎛️ Quick Start Commands

### Basic Cluster
```bash
# Setup and start basic cluster
cd docker-compose/basic-cluster
./start-cluster.sh

# Run performance baseline
cd ../../test-scripts/performance  
./baseline-test.sh
```

### HA Cluster Testing
```bash
# Start HA cluster
cd docker-compose/ha-cluster
docker-compose up -d

# Test failover scenarios
cd ../../test-scripts/failover
./broker-failover.sh
./bookie-failover.sh
```

### Kubernetes Deployment
```bash
# Setup K8s cluster
cd kubernetes
./setup-k8s.sh

# Test rolling upgrades
cd ../test-scripts/upgrade
./rolling-upgrade.sh
```

### Complete Test Suite
```bash
# Run all tests
./run-all-tests.sh all

# Run specific test categories
./run-all-tests.sh docker    # Docker Compose tests only
./run-all-tests.sh ha        # HA cluster tests only  
./run-all-tests.sh k8s       # Kubernetes tests only
```

## 📚 Learning Outcomes

By completing this lab, you'll gain hands-on experience with:

- **Pulsar Architecture**: Understanding the separation of serving and storage layers
- **Distributed Systems**: Consensus protocols, replication, and partition tolerance
- **Observability**: Metrics collection, alerting, and performance analysis
- **DevOps Practices**: Infrastructure as code, automated testing, and deployment strategies
- **Operational Excellence**: Incident response, capacity planning, and system optimization

---

*This environment provides a realistic testing ground for understanding Pulsar's operational characteristics before deploying to production.*