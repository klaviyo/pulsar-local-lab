# Troubleshooting Guide

## Common Issues and Solutions

### Minikube and Kubernetes Issues

#### Issue: Minikube Won't Start
**Symptoms**: `minikube start` fails or hangs

**Common Causes**:
1. Insufficient resources allocated
2. Docker/driver conflicts
3. Stale minikube state

**Solutions**:
```bash
# Check minikube status
minikube status

# Delete and recreate cluster
minikube delete
minikube start --memory=8192 --cpus=4

# Use specific driver
minikube start --driver=docker

# Check logs
minikube logs
```

#### Issue: Pods Stuck in Pending State
**Symptoms**: Pulsar pods won't schedule

**Diagnosis**:
```bash
kubectl get pods -o wide
kubectl describe pod <pod-name>
kubectl get events --sort-by='.lastTimestamp'
```

**Common Causes**:
1. Insufficient cluster resources
2. PVC not bound (if persistence enabled)
3. Image pull failures

**Solutions**:
```bash
# Check node resources
kubectl top nodes
kubectl describe nodes

# For PVC issues (shouldn't occur with emptyDir config)
kubectl get pvc
kubectl describe pvc <pvc-name>

# For image issues
kubectl describe pod <pod-name> | grep -A 10 Events
```

### Pulsar Cluster Issues

#### Issue: Broker Won't Start
**Symptoms**: Broker pod crash loops or stays in pending

**Diagnosis**:
```bash
kubectl logs <broker-pod-name>
kubectl logs <broker-pod-name> --previous  # For crash loops
```

**Common Causes**:
1. ZooKeeper not ready
2. BookKeeper ensemble not available
3. Configuration errors
4. Memory limits too low

**Solutions**:
```bash
# Check ZooKeeper
kubectl exec -it <zookeeper-pod> -- bin/zkCli.sh
ls /
ls /admin
ls /ledgers

# Check BookKeeper
kubectl logs <bookkeeper-pod-name>

# Verify broker config
kubectl get cm <broker-config-map> -o yaml

# Increase memory if needed
kubectl edit deployment <broker-deployment>
```

#### Issue: Can't Create Topics
**Symptoms**: Topic creation fails, timeouts

**Diagnosis**:
```bash
# From inside broker pod
kubectl exec -it <broker-pod> -- bin/pulsar-admin topics list public/default
kubectl exec -it <broker-pod> -- bin/pulsar-admin brokers list <cluster-name>
kubectl exec -it <broker-pod> -- bin/pulsar-admin clusters list
```

**Common Causes**:
1. Broker not connected to cluster
2. Namespace/tenant doesn't exist
3. Insufficient bookies for ensemble size

**Solutions**:
```bash
# Check cluster metadata
kubectl exec -it <broker-pod> -- bin/pulsar-admin clusters get <cluster-name>

# Create tenant/namespace if needed
kubectl exec -it <broker-pod> -- bin/pulsar-admin tenants create public
kubectl exec -it <broker-pod> -- bin/pulsar-admin namespaces create public/default

# Verify bookie availability
kubectl exec -it <broker-pod> -- bin/bookkeeper shell listbookies -rw
kubectl exec -it <broker-pod> -- bin/bookkeeper shell listbookies -ro
```

### BookKeeper Issues

#### Issue: Bookies Not Registering
**Symptoms**: No bookies available for writes

**Diagnosis**:
```bash
kubectl logs <bookie-pod-name>
kubectl exec -it <zookeeper-pod> -- bin/zkCli.sh
ls /ledgers/available
```

**Common Causes**:
1. ZooKeeper connection issues
2. Journal/ledger directory permissions
3. Configuration mismatch

**Solutions**:
```bash
# Check bookie logs for errors
kubectl logs <bookie-pod-name> | grep ERROR

# Verify ZK connectivity from bookie
kubectl exec -it <bookie-pod> -- nc -zv <zookeeper-service> 2181

# Check bookie configuration
kubectl get cm <bookie-config-map> -o yaml
```

#### Issue: Write/Read Failures
**Symptoms**: "Not enough bookies" or "BKLedgerNoSuchLedgerExists"

**Diagnosis**:
```bash
# Check ensemble size vs available bookies
kubectl exec -it <broker-pod> -- bin/pulsar-admin brokers get-runtime-configuration | grep managedLedger

# Count available bookies
kubectl get pods -l component=bookkeeper
```

**Solutions**:
For Minikube setup with 1 bookie, ensure:
```yaml
managedLedgerDefaultEnsembleSize: "1"
managedLedgerDefaultWriteQuorum: "1"
managedLedgerDefaultAckQuorum: "1"
```

For HA setup:
- Minimum 3 bookies for ensemble size 3
- Adjust quorum settings accordingly

### Monitoring Issues

#### Issue: Prometheus Not Scraping Metrics
**Symptoms**: Empty Grafana dashboards

**Diagnosis**:
```bash
# Check Prometheus targets
kubectl port-forward svc/prometheus 9090:9090
# Visit http://localhost:9090/targets

# Check pod annotations
kubectl get pods -o yaml | grep prometheus
```

**Solutions**:
- Verify serviceMonitor or pod annotations are correct
- Check Prometheus config includes correct scrape jobs
- Ensure Pulsar metrics endpoint is accessible:
```bash
kubectl exec -it <broker-pod> -- curl localhost:8080/metrics
```

#### Issue: Grafana Dashboards Not Loading
**Symptoms**: "Dashboard not found" or empty

**Diagnosis**:
```bash
# Check Grafana provisioning
kubectl logs <grafana-pod-name>
kubectl exec -it <grafana-pod> -- ls /etc/grafana/provisioning/dashboards/
```

**Solutions**:
- Verify dashboard JSON files are mounted correctly
- Check datasource configuration points to correct Prometheus
- Restart Grafana pod if provisioning failed:
```bash
kubectl delete pod <grafana-pod-name>
```

### Performance Issues

#### Issue: High Latency
**Symptoms**: Slow message processing, high p99 latency

**Investigation Steps**:
1. Check broker CPU/memory:
   ```bash
   kubectl top pods
   ```

2. Review JVM metrics in Grafana dashboard 06

3. Check BookKeeper write latency:
   ```bash
   kubectl exec -it <broker-pod> -- bin/pulsar-admin broker-stats destinations
   ```

4. Verify batch settings:
   ```bash
   kubectl exec -it <broker-pod> -- bin/pulsar-admin namespaces get-max-unacked-messages-per-consumer public/default
   ```

**Common Fixes**:
- Increase broker/bookie resources
- Tune batch settings
- Add more partitions for parallelism
- Review RocksDB cache sizes (see ADR-004)

#### Issue: Low Throughput
**Symptoms**: Can't achieve expected msg/sec

**Diagnosis**:
```bash
# Check producer/consumer backlog
kubectl exec -it <broker-pod> -- bin/pulsar-admin topics stats persistent://public/default/<topic>

# Monitor in Grafana
# - Dashboard 04: Topic metrics
# - Dashboard 05: Consumer lag
```

**Solutions**:
- Increase partition count
- Use batching on producer
- Scale broker replicas (in multi-broker setup)
- Check network between components

## Debug Commands Reference

### Quick Health Check
```bash
# Pod status
kubectl get pods -o wide

# Service endpoints
kubectl get svc

# Recent events
kubectl get events --sort-by='.lastTimestamp' | tail -20

# Resource usage
kubectl top pods
kubectl top nodes
```

### Detailed Investigation
```bash
# Full pod details
kubectl describe pod <pod-name>

# Container logs
kubectl logs <pod-name> -c <container-name>
kubectl logs <pod-name> --previous  # Previous instance

# Interactive shell
kubectl exec -it <pod-name> -- /bin/bash

# Port forwarding for local access
kubectl port-forward <pod-name> <local-port>:<pod-port>
```

### Pulsar Admin Commands
```bash
# From inside broker pod or via proxy
BROKER_POD=$(kubectl get pods -l component=broker -o jsonpath='{.items[0].metadata.name}')

# Cluster info
kubectl exec -it $BROKER_POD -- bin/pulsar-admin clusters list
kubectl exec -it $BROKER_POD -- bin/pulsar-admin brokers list <cluster>

# Topic management
kubectl exec -it $BROKER_POD -- bin/pulsar-admin topics list public/default
kubectl exec -it $BROKER_POD -- bin/pulsar-admin topics stats <topic-name>

# Subscription management
kubectl exec -it $BROKER_POD -- bin/pulsar-admin topics subscriptions <topic-name>
kubectl exec -it $BROKER_POD -- bin/pulsar-admin topics peek-messages <topic-name> -s <subscription> -n 10
```

## When All Else Fails

### Clean Slate
```bash
# Delete everything and start fresh
helm uninstall pulsar
kubectl delete pvc --all
minikube delete
minikube start --memory=8192 --cpus=4

# Redeploy
helm install pulsar apache/pulsar -f helm/values-minikube.yaml
```

### Get Help
1. Check Pulsar logs: `kubectl logs <pod>`
2. Review setup.md for expected configuration
3. Compare your config against helm/values-minikube.yaml
4. Check recent git commits for what changed: `git log --oneline -10`
5. Review Pulsar documentation: https://pulsar.apache.org/docs/