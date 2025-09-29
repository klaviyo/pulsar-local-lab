# Code Patterns and Conventions

## Project Organization Patterns

### Directory Structure
```
pulsar-local-lab/
├── helm/                    # Kubernetes Helm values
├── monitoring/              # Observability configurations
│   ├── grafana/            # Dashboards and provisioning
│   └── prometheus/         # Metrics collection configs
├── .claude/                # Claude Code configuration
└── setup.md                # Master implementation plan
```


## Configuration Patterns

### Minikube/Local Development
- **Single node deployments**: 1 replica for ZK, Bookkeeper, Broker, Proxy
- **Persistence disabled**: Use emptyDir volumes for faster iteration
- **Anti-affinity disabled**: Allow pods on same node
- **Auto-recovery disabled**: Simplified setup
- **Memory optimized**: Reduced cache sizes for local resource constraints

### Prometheus Configuration Variants
Three configuration profiles exist:
1. **prometheus.yml** - Basic cluster monitoring
2. **prometheus-ha.yml** - High availability setup
3. **prometheus-upgrade.yml** - Upgrade scenario monitoring

Pattern: Separate configs for different deployment scenarios rather than single config with conditionals.

### Grafana Dashboards
**Naming Convention**: Sequential numbering with descriptive names
- `01-pulsar-overview.json` - System-wide view
- `02-broker-metrics.json` - Broker-specific
- `03-bookkeeper-metrics.json` - Storage layer
- `04-topic-namespace.json` - Topic management
- `05-consumer-subscription.json` - Consumer tracking
- `06-jvm-system.json` - JVM performance

## Pulsar Configuration Patterns

### Minimal Local Setup (Minikube)
```yaml
managedLedgerDefaultEnsembleSize: "1"
managedLedgerDefaultWriteQuorum: "1"
managedLedgerDefaultAckQuorum: "1"
autoSkipNonRecoverableData: "true"
```

### HA Production Setup Example
```yaml
managedLedgerDefaultEnsembleSize: 3-5
managedLedgerDefaultWriteQuorum: 2-3
managedLedgerDefaultAckQuorum: 2
brokerDeduplicationEnabled: true
topicLevelPoliciesEnabled: true
```

## Git Workflow Patterns

### Commit History
Recent commits show iterative development style:
- Feature commits: "switching to minikube", "parallel tests"
- Maintenance: "more fixes", "attempting fix"
- No strict convention currently enforced

### File Management
- Configuration files committed to git
- .claude/ directory kept local (untracked)
- CLAUDE.md and CLAUDE-*.md excluded from commits (per guidance)

## Documentation Patterns

### Learning-Oriented Design
- Focus on understanding Pulsar operations
- Comprehensive metrics and observability