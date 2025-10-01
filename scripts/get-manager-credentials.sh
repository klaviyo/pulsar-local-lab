#!/bin/bash

# get-manager-credentials.sh - Retrieve Pulsar Manager credentials from Kubernetes secret

set -e

NAMESPACE="${PULSAR_NAMESPACE:-pulsar}"
SECRET_NAME="pulsar-pulsar-manager-secret"

echo "=========================================="
echo "Pulsar Manager Credentials"
echo "=========================================="
echo ""

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    echo "Error: Namespace '$NAMESPACE' not found"
    exit 1
fi

# Check if secret exists
if ! kubectl get secret -n "$NAMESPACE" "$SECRET_NAME" &> /dev/null; then
    echo "Error: Secret '$SECRET_NAME' not found in namespace '$NAMESPACE'"
    echo ""
    echo "The credentials may not have been created yet."
    echo "You may need to manually create a superuser account."
    exit 1
fi

# Decode credentials
USERNAME=$(kubectl get secret -n "$NAMESPACE" "$SECRET_NAME" -o jsonpath='{.data.UI_USERNAME}' | base64 -d)
PASSWORD=$(kubectl get secret -n "$NAMESPACE" "$SECRET_NAME" -o jsonpath='{.data.UI_PASSWORD}' | base64 -d)

echo "🔑 Login Credentials:"
echo "   URL:      http://localhost:9527"
echo "   Username: $USERNAME"
echo "   Password: $PASSWORD"
echo ""
echo "=========================================="
echo ""
echo "💡 Tip: Use './scripts/access-ui.sh' to start port forwarding"
echo ""
