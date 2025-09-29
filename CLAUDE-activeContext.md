# Active Context

## Current Project Status

### Project: Apache Pulsar Local Lab
**Last Updated**: 2025-09-29
**Current Phase**: Early development - Kubernetes/Minikube

### What's Been Done
1. ✅ Initial project structure created
2. ✅ Minikube Helm values configuration (`helm/values-minikube.yaml`)
3. ✅ Monitoring infrastructure setup:
   - 3 Prometheus configurations (prometheus.yml, prometheus-ha.yml, prometheus-upgrade.yml)
   - 6 Grafana dashboards (overview, broker, bookkeeper, topics, consumers, JVM)
   - Alert rules defined
4. ✅ Git repository initialized
5. ✅ Claude Code configuration (.claude directory with agents/commands)

### Current State
- **Repository**: Clean with uncommitted CLAUDE memory bank files and .claude/ directory
- **Deployment Target**: Kubernetes via Minikube
- **Recent Commits**:
  - `d8133ea` - Switching to minikube
  - `01305b7` - Parallel tests
  - `61d628a` - Stuff
  - `b49e468` - More fixes

### Active Goals
Memory bank system initialized/updated.

## Session Notes
- Memory bank system created with 5 core context files
- Project focused on Minikube-based Pulsar testing environment
- Optimized for local development and experimentation