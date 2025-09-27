# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Pulsar Local Lab - a comprehensive testing environment for Apache Pulsar focused on:
- Multi-broker cluster configurations and load balancing
- Performance testing (throughput, latency, scaling)
- Zero-downtime operations and rolling upgrades
- High availability and disaster recovery scenarios

## Architecture

The project is structured around three deployment environments:

### Docker Compose Environments
- **basic-cluster/**: 2 brokers, 3 bookies, 1 ZooKeeper - for foundational testing
- **ha-cluster/**: 3 brokers, 5 bookies, 3 ZooKeeper - for production-like resilience
- **upgrade-test/**: Version upgrade scenario testing

### Kubernetes Environment
- **manifests/**: Raw Kubernetes YAML files
- **helm-values/**: Helm chart configurations for different cluster sizes

### Testing Framework
- **performance/**: Throughput and latency measurement scripts
- **failover/**: Chaos engineering and disaster recovery scripts
- **upgrade/**: Rolling update and zero-downtime upgrade procedures

## Key Commands

### Quick Start Commands
```bash
# Basic cluster setup
cd docker-compose/basic-cluster
./start-cluster.sh

# HA cluster testing
cd docker-compose/ha-cluster
docker-compose up -d

# Kubernetes setup
cd kubernetes
./setup-k8s.sh

# Complete test suite
./run-all-tests.sh all
```

### Test Categories
```bash
./run-all-tests.sh docker    # Docker Compose tests only
./run-all-tests.sh ha        # HA cluster tests only
./run-all-tests.sh k8s       # Kubernetes tests only
```

### Performance Testing Commands
```bash
# Producer throughput test
bin/pulsar-perf produce --rate 1000 --num-messages 10000 --size 1024

# Consumer latency test
bin/pulsar-perf consume --subscription-name test-sub --num-messages 10000

# Multi-partition scaling test
bin/pulsar-admin topics create --partitions 8 persistent://public/default/scale-test
```

## Critical Configuration Areas

### BookKeeper Durability Settings
- `managedLedgerDefaultEnsembleSize`: Controls replication factor (3-5)
- `managedLedgerDefaultWriteQuorum`: Write acknowledgment requirements (2-3)
- `managedLedgerDefaultAckQuorum`: Read acknowledgment requirements (2)

### Load Balancing Configuration
- `loadManagerClassName`: Choose between ExtensibleLoadManagerImpl and ModularLoadManagerImpl
- Important for broker load distribution and failover behavior

### High Availability Features
- `brokerDeduplicationEnabled`: Enables exactly-once semantics
- `topicLevelPoliciesEnabled`: Allows dynamic topic configuration
- `systemTopicEnabled`: Required for metadata topics

## Monitoring & Observability

The project includes comprehensive monitoring setup:
- **Prometheus**: Metrics collection from all Pulsar components
- **Grafana**: Visualization dashboards in `monitoring/grafana/`
- **Key metrics**: Message throughput, latency (P50/P99), consumer lag, resource utilization

## Operational Procedures

Two critical playbooks are maintained:
- `procedures/failover-playbook.md`: Disaster recovery procedures
- `procedures/upgrade-playbook.md`: Zero-downtime upgrade procedures

## Performance Targets

Success criteria for the lab environment:
- 10,000+ msg/sec sustained throughput
- <10ms P99 latency for 1KB messages
- <5 second consumer lag during normal operations
- 99.9%+ uptime during chaos testing
- <2 minute cluster startup time
- Sub-second broker failover detection

## Test Scenarios

### Chaos Engineering
- Network partitions and component isolation
- Resource exhaustion testing (CPU/memory pressure)
- Storage failures and BookKeeper corruption simulation
- Clock skew and time synchronization issues

### Performance Validation
- Linear scaling verification (throughput vs partition count)
- Latency characteristics under varying loads
- Consumer pattern testing (shared vs exclusive subscriptions)
- Long retention period performance impact

### Operational Testing
- Rolling upgrades with zero downtime
- Dynamic cluster expansion (adding brokers/bookies)
- Topic migration between clusters
- Backup and restore procedures