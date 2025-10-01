# Pulsar Local Lab Helm Chart

Self-contained Helm chart for deploying Apache Pulsar to Kubernetes.

## Structure

```
helm/
├── Chart.yaml              # Chart metadata and dependencies
├── values.yaml             # Minikube-optimized values (local lab defaults)
├── .helmignore            # Files to exclude from packaging
└── charts/                # Dependencies (gitignored, auto-downloaded)
```

## Quick Start

### 1. Download Dependencies

```bash
cd helm
helm dependency update
```

This downloads the Apache Pulsar chart (v3.3.0) to `charts/pulsar-3.3.0.tgz`.

### 2. Deploy to Minikube

```bash
# From project root
helm install pulsar ./helm \
  --namespace pulsar \
  --create-namespace
```

## Configuration

### values.yaml
Minikube-optimized defaults for local development:
- 1 replica for all components
- Persistence disabled (emptyDir)
- Pod anti-affinity disabled
- Reduced memory settings
- Pulsar Manager UI enabled

### Custom Overrides
Create additional values files for different scenarios:

```bash
# Example: Production-like testing
helm install pulsar ./helm -f my-production-values.yaml

# Example: High-load testing
helm install pulsar ./helm -f my-performance-values.yaml
```

## Customization

All Pulsar configuration must be under the `pulsar:` key:

```yaml
pulsar:
  broker:
    replicaCount: 3
    configData:
      managedLedgerDefaultEnsembleSize: "3"
```

See [Apache Pulsar Helm Chart](https://github.com/apache/pulsar-helm-chart) for all available options.

## Updating Pulsar Version

1. Edit `Chart.yaml` and change the Pulsar dependency version
2. Run `helm dependency update`
3. Test the new version

## Troubleshooting

### Dependencies not found
```bash
# Re-download dependencies
helm dependency update

# Verify charts/ directory exists
ls -la charts/
```

### Values not applying
Remember that all values must be under `pulsar:` key when using this chart since Pulsar is a subchart dependency.

