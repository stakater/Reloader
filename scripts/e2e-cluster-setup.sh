#!/bin/bash
# Setup script for e2e test cluster
# Run this before running e2e tests: ./scripts/e2e-cluster-setup.sh
# This installs Argo Rollouts and any other prerequisites needed for e2e tests.

set -e

ARGO_ROLLOUTS_VERSION="${ARGO_ROLLOUTS_VERSION:-v1.7.2}"
ARGO_ROLLOUTS_NAMESPACE="argo-rollouts"

echo "=== E2E Cluster Setup ==="

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check cluster connectivity
echo "Checking cluster connectivity..."
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster"
    exit 1
fi
echo "Cluster connectivity verified"

# Install Argo Rollouts
echo ""
echo "=== Installing Argo Rollouts ${ARGO_ROLLOUTS_VERSION} ==="

# Check if Argo Rollouts is already installed
if kubectl get crd rollouts.argoproj.io &> /dev/null; then
    echo "Argo Rollouts CRD already exists, checking if controller is running..."
    if kubectl get deployment argo-rollouts -n ${ARGO_ROLLOUTS_NAMESPACE} &> /dev/null; then
        echo "Argo Rollouts is already installed and running"
    else
        echo "Argo Rollouts CRD exists but controller not running, reinstalling..."
    fi
else
    echo "Installing Argo Rollouts..."
fi

# Create namespace (ignore if exists)
kubectl create namespace ${ARGO_ROLLOUTS_NAMESPACE} 2>/dev/null || true

# Install Argo Rollouts
ARGO_URL="https://github.com/argoproj/argo-rollouts/releases/download/${ARGO_ROLLOUTS_VERSION}/install.yaml"
echo "Applying manifest from: ${ARGO_URL}"
kubectl apply -n ${ARGO_ROLLOUTS_NAMESPACE} -f ${ARGO_URL}

# Wait for deployment to exist
echo "Waiting for deployment to be created..."
sleep 2

# Patch deployment to remove resource requirements (for Kind cluster compatibility)
# This avoids "Insufficient ephemeral-storage" errors in resource-constrained environments
echo "Patching deployment for Kind compatibility..."
PATCH_JSON='[{"op": "remove", "path": "/spec/template/spec/containers/0/resources"}]'
if ! kubectl patch deployment argo-rollouts -n ${ARGO_ROLLOUTS_NAMESPACE} --type=json -p "${PATCH_JSON}" 2>/dev/null; then
    echo "JSON patch failed, trying strategic merge..."
    PATCH_JSON='{"spec":{"template":{"spec":{"containers":[{"name":"argo-rollouts","resources":{"limits":null,"requests":null}}]}}}}'
    kubectl patch deployment argo-rollouts -n ${ARGO_ROLLOUTS_NAMESPACE} --type=strategic -p "${PATCH_JSON}" || echo "Warning: Failed to patch resources"
fi

# Wait for controller to be ready
echo "Waiting for Argo Rollouts controller to be ready..."
kubectl wait --for=condition=available deployment/argo-rollouts -n ${ARGO_ROLLOUTS_NAMESPACE} --timeout=180s

# Wait for CRD to be established
echo "Waiting for Argo Rollouts CRD to be established..."
kubectl wait --for=condition=established crd/rollouts.argoproj.io --timeout=60s

echo ""
echo "=== E2E Cluster Setup Complete ==="
echo "Argo Rollouts ${ARGO_ROLLOUTS_VERSION} is installed and ready"
echo ""
echo "You can now run e2e tests:"
echo "  make e2e-test"
echo "  # or"
echo "  SKIP_BUILD=true RELOADER_IMAGE=ghcr.io/stakater/reloader:test go test -v ./test/e2e/..."
