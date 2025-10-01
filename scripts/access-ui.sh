#!/bin/bash

# access-ui.sh - Helper script to access Pulsar services
# Starts port forwarding for Grafana, Pulsar Manager, and Pulsar Broker (client + admin)

set -e

NAMESPACE="${PULSAR_NAMESPACE:-pulsar}"
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
PULSAR_MANAGER_PORT="${PULSAR_MANAGER_PORT:-9527}"
BROKER_CLIENT_PORT="${BROKER_CLIENT_PORT:-6650}"
BROKER_ADMIN_PORT="${BROKER_ADMIN_PORT:-8080}"

echo "=========================================="
echo "Pulsar Local Lab - UI Access"
echo "=========================================="
echo ""

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    echo "Error: Namespace '$NAMESPACE' not found"
    echo "Please deploy Pulsar first: helm install pulsar ./helm --namespace pulsar --create-namespace"
    exit 1
fi

# Check if services exist
echo "Checking services..."
if ! kubectl get svc -n "$NAMESPACE" grafana &> /dev/null; then
    echo "Warning: Grafana service not found in namespace '$NAMESPACE'"
fi

if ! kubectl get svc -n "$NAMESPACE" pulsar-pulsar-manager &> /dev/null; then
    echo "Warning: Pulsar Manager service not found in namespace '$NAMESPACE'"
fi

if ! kubectl get svc -n "$NAMESPACE" pulsar-broker &> /dev/null; then
    echo "Warning: Pulsar Broker service not found in namespace '$NAMESPACE'"
fi

echo ""
echo "Starting port forwarding..."
echo ""

# Kill any existing port forwards on these ports
lsof -ti:$GRAFANA_PORT | xargs kill -9 2>/dev/null || true
lsof -ti:$PULSAR_MANAGER_PORT | xargs kill -9 2>/dev/null || true
lsof -ti:$BROKER_CLIENT_PORT | xargs kill -9 2>/dev/null || true
lsof -ti:$BROKER_ADMIN_PORT | xargs kill -9 2>/dev/null || true

# Start port forwarding in background
kubectl port-forward -n "$NAMESPACE" svc/grafana "$GRAFANA_PORT:3000" &
GRAFANA_PID=$!

kubectl port-forward -n "$NAMESPACE" svc/pulsar-pulsar-manager "$PULSAR_MANAGER_PORT:9527" &
MANAGER_PID=$!

kubectl port-forward -n "$NAMESPACE" svc/pulsar-broker "$BROKER_CLIENT_PORT:6650" &
BROKER_CLIENT_PID=$!

kubectl port-forward -n "$NAMESPACE" svc/pulsar-broker "$BROKER_ADMIN_PORT:8080" &
BROKER_ADMIN_PID=$!

# Wait a moment for port forwards to establish
sleep 2

# Get Pulsar Manager credentials from secret
MANAGER_USERNAME=""
MANAGER_PASSWORD=""
if kubectl get secret -n "$NAMESPACE" pulsar-pulsar-manager-secret &> /dev/null; then
    MANAGER_USERNAME=$(kubectl get secret -n "$NAMESPACE" pulsar-pulsar-manager-secret -o jsonpath='{.data.UI_USERNAME}' 2>/dev/null | base64 -d 2>/dev/null || echo "pulsar")
    MANAGER_PASSWORD=$(kubectl get secret -n "$NAMESPACE" pulsar-pulsar-manager-secret -o jsonpath='{.data.UI_PASSWORD}' 2>/dev/null | base64 -d 2>/dev/null || echo "<check secret>")
fi

echo "=========================================="
echo "Service Access URLs"
echo "=========================================="
echo ""
echo "📊 Grafana Dashboards:"
echo "   URL:         http://localhost:$GRAFANA_PORT"
echo "   Credentials: admin/admin (default)"
echo ""
echo "🎛️  Pulsar Manager (Admin Console):"
echo "   URL:         http://localhost:$PULSAR_MANAGER_PORT"
if [ -n "$MANAGER_USERNAME" ] && [ -n "$MANAGER_PASSWORD" ]; then
    echo "   Username:    $MANAGER_USERNAME"
    echo "   Password:    $MANAGER_PASSWORD"
else
    echo "   Credentials: Run './scripts/get-manager-credentials.sh'"
fi
echo ""
echo "🔌 Pulsar Broker:"
echo "   Client URL:  pulsar://localhost:$BROKER_CLIENT_PORT"
echo "   Admin API:   http://localhost:$BROKER_ADMIN_PORT"
echo ""
echo "=========================================="
echo ""
echo "Press Ctrl+C to stop port forwarding and exit"
echo ""

# Trap Ctrl+C and cleanup
cleanup() {
    echo ""
    echo "Stopping port forwarding..."
    kill $GRAFANA_PID $MANAGER_PID $BROKER_CLIENT_PID $BROKER_ADMIN_PID 2>/dev/null || true
    echo "Done."
    exit 0
}

trap cleanup INT TERM

# Wait for both processes
wait
