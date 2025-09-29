# Architecture Decisions and Rationale

## ADR-001: Switch to Minikube for Kubernetes Testing
**Date**: 2025-09-29
**Status**: Accepted
**Commit**: d8133ea

### Decision
Use Minikube instead of Kind or other Kubernetes distribution for local testing.

### Context
Need a local Kubernetes environment for testing Pulsar deployments and operations.

### Rationale
- Minikube provides good balance of features and simplicity
- Well-documented Apache Pulsar Helm chart integration
- Suitable for single-developer experimentation
- Lower resource requirements than multi-node Kind clusters

### Consequences
- **Positive**: Easier local development with reduced resource usage
- **Positive**: Faster iteration cycles
- **Negative**: Cannot test true multi-node scenarios
- **Mitigation**: Use Docker Compose HA cluster for multi-node testing


## ADR-002: Separate Prometheus Configs for Scenarios
**Date**: Early development
**Status**: Accepted

### Decision
Maintain separate Prometheus configuration files for different deployment scenarios rather than single config with conditionals.

### Context
Need monitoring for basic cluster, HA cluster, and upgrade scenarios with different requirements.

### Rationale
- Clearer separation of concerns
- Easier to understand and maintain
- Simpler to switch between scenarios
- Avoids complex conditional logic in configs

### Consequences
- **Positive**: Explicit configuration for each scenario
- **Positive**: No runtime conditional logic
- **Negative**: Some duplication across configs
- **Negative**: Changes may need to be replicated

## ADR-003: Memory-Optimized Minikube Configuration
**Date**: 2025-09-29
**Status**: Accepted

### Decision
Configure Bookkeeper with minimal memory settings for local development.

```yaml
dbStorage_writeCacheMaxSizeMb: "32"
dbStorage_readAheadCacheMaxSizeMb: "32"
dbStorage_rocksDB_writeBufferSizeMB: "8"
dbStorage_rocksDB_blockCacheSize: "8388608"
```

### Context
Running full Pulsar cluster locally requires significant resources. Default settings designed for production servers.

### Rationale
- Enable development on typical laptop hardware
- Faster startup times
- Sufficient for functional testing and experimentation
- Production configs documented separately for reference

### Consequences
- **Positive**: Runs on consumer hardware (8-16GB RAM machines)
- **Positive**: Faster iteration cycles
- **Negative**: Performance characteristics differ from production
- **Note**: Performance testing should acknowledge these limitations

## ADR-004: Disable Auto-Recovery in Minikube
**Date**: 2025-09-29
**Status**: Accepted

### Decision
Disable BookKeeper auto-recovery component in Minikube setup.

### Context
Auto-recovery adds complexity and resource overhead for single-node local development.

### Rationale
- Simplified deployment for learning purposes
- Reduced resource consumption
- Not critical for basic functional testing
- Can be enabled in HA configurations

### Consequences
- **Positive**: Simpler troubleshooting
- **Positive**: Lower resource requirements
- **Negative**: Manual intervention needed for bookie failures
- **Note**: Document this difference from production setups

## ADR-005: Comprehensive Grafana Dashboard Suite
**Date**: Early development
**Status**: Accepted

### Decision
Create 6 specialized Grafana dashboards covering different aspects of Pulsar operations.

### Context
Need observability into Pulsar cluster behavior for learning and troubleshooting.

### Rationale
- **Overview**: High-level cluster health
- **Broker/Bookkeeper**: Component-specific deep dives
- **Topic/Consumer**: Application-level metrics
- **JVM**: System-level performance

Separation allows focused analysis without overwhelming single view.

### Consequences
- **Positive**: Targeted troubleshooting workflows
- **Positive**: Gradual learning curve (start with overview, drill down)
- **Neutral**: Need to maintain multiple dashboards

## ADR-006: Parallel Test Execution
**Date**: Recent (commit 01305b7)
**Status**: Accepted

### Decision
Implement parallel test execution capabilities in test suite.

### Context
Running multiple test scenarios sequentially is time-consuming for iterative development.

### Rationale
- Faster feedback loops during development
- Better resource utilization
- Mirrors real-world concurrent operations

### Consequences
- **Positive**: Reduced total test time
- **Positive**: Can test concurrent scenarios
- **Challenge**: Need proper test isolation and cleanup

