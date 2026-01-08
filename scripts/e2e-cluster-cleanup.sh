#!/bin/bash
# Cleanup script for e2e test cluster
# Run this after e2e tests complete: ./scripts/e2e-cluster-cleanup.sh
# This removes Argo Rollouts, test namespaces, and cluster-scoped resources.

set -e

ARGO_ROLLOUTS_VERSION="${ARGO_ROLLOUTS_VERSION:-v1.7.2}"
ARGO_ROLLOUTS_NAMESPACE="argo-rollouts"

echo "=== E2E Cluster Cleanup ==="

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

# ============================================================
# Cleanup Reloader Test Resources
# ============================================================
echo ""
echo "=== Cleaning up Reloader test resources ==="

# Delete test namespaces (created by test suites)
echo "Deleting test namespaces..."
for ns in $(kubectl get namespaces -o name | grep -E "reloader-" | cut -d/ -f2); do
    echo "  Deleting namespace: ${ns}"
    kubectl delete namespace "${ns}" --ignore-not-found --wait=false
done

# Delete Reloader cluster-scoped resources
echo "Deleting Reloader cluster-scoped resources..."
for cr in $(kubectl get clusterrole -o name 2>/dev/null | grep -E "reloader-" | cut -d/ -f2); do
    echo "  Deleting ClusterRole: ${cr}"
    kubectl delete clusterrole "${cr}" --ignore-not-found
done

for crb in $(kubectl get clusterrolebinding -o name 2>/dev/null | grep -E "reloader-" | cut -d/ -f2); do
    echo "  Deleting ClusterRoleBinding: ${crb}"
    kubectl delete clusterrolebinding "${crb}" --ignore-not-found
done

# ============================================================
# Cleanup Argo Rollouts
# ============================================================
echo ""
echo "=== Uninstalling Argo Rollouts ==="

# First, delete the deployment to stop the controller
echo "Stopping Argo Rollouts controller..."
kubectl delete deployment argo-rollouts -n ${ARGO_ROLLOUTS_NAMESPACE} --ignore-not-found --timeout=30s 2>/dev/null || true

# Delete all Rollouts and other CRs in all namespaces to avoid finalizer issues
echo "Deleting Argo Rollouts custom resources..."
ARGO_RESOURCES="rollouts analysisruns analysistemplates experiments"
for res in ${ARGO_RESOURCES}; do
    kubectl delete "${res}.argoproj.io" --all --all-namespaces --ignore-not-found --timeout=30s 2>/dev/null || true
done

# Delete using the install manifest
echo "Deleting Argo Rollouts installation..."
ARGO_URL="https://github.com/argoproj/argo-rollouts/releases/download/${ARGO_ROLLOUTS_VERSION}/install.yaml"
kubectl delete -f ${ARGO_URL} --ignore-not-found --timeout=60s 2>/dev/null || true

# Give resources time to be cleaned up before deleting CRDs
sleep 2

# Explicitly delete CRDs (cluster-scoped)
echo "Deleting Argo Rollouts CRDs..."
ARGO_CRDS="rollouts.argoproj.io analysisruns.argoproj.io analysistemplates.argoproj.io clusteranalysistemplates.argoproj.io experiments.argoproj.io"
for crd in ${ARGO_CRDS}; do
    kubectl delete crd "${crd}" --ignore-not-found --timeout=30s 2>/dev/null || true
done

# Delete namespace
echo "Deleting Argo Rollouts namespace..."
kubectl delete namespace ${ARGO_ROLLOUTS_NAMESPACE} --ignore-not-found --timeout=30s 2>/dev/null || true

# Delete cluster-scoped RBAC
echo "Deleting Argo Rollouts cluster RBAC..."
kubectl delete clusterrole argo-rollouts argo-rollouts-aggregate-to-admin argo-rollouts-aggregate-to-edit argo-rollouts-aggregate-to-view --ignore-not-found 2>/dev/null || true
kubectl delete clusterrolebinding argo-rollouts --ignore-not-found 2>/dev/null || true

echo ""
echo "=== E2E Cluster Cleanup Complete ==="
