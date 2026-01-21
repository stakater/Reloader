#!/bin/bash
# Cleanup script for e2e test cluster
# Run this after e2e tests complete: ./scripts/e2e-cluster-cleanup.sh
#
# This removes:
#   - Reloader test resources (namespaces, cluster roles, etc.)
#   - Vault and its namespace
#   - CSI Secrets Store Driver
#   - Argo Rollouts
#
# Resources are removed in reverse dependency order.

set -euo pipefail

# =============================================================================
# Configuration
# =============================================================================

ARGO_ROLLOUTS_VERSION="${ARGO_ROLLOUTS_VERSION:-v1.7.2}"
ARGO_ROLLOUTS_NAMESPACE="argo-rollouts"
CSI_DRIVER_VERSION="${CSI_DRIVER_VERSION:-1.5.5}"
CSI_NAMESPACE="kube-system"
VAULT_NAMESPACE="vault"

# =============================================================================
# Helper Functions
# =============================================================================

log_header() {
    echo ""
    echo "=== $1 ==="
}

log_info() {
    echo "$1"
}

log_success() {
    echo "✓ $1"
}

log_warning() {
    echo "⚠ $1"
}

log_error() {
    echo "✗ $1" >&2
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "$1 is not installed or not in PATH"
        return 1
    fi
    return 0
}

# Safe delete that ignores "not found" errors
safe_delete() {
    kubectl delete "$@" --ignore-not-found 2>/dev/null || true
}

# =============================================================================
# Dependency Checks
# =============================================================================

check_dependencies() {
    log_header "Checking Dependencies"

    if ! check_command kubectl; then
        log_error "kubectl is required for cleanup"
        exit 1
    fi

    log_success "Dependencies available"
}

check_cluster_connectivity() {
    log_header "Checking Cluster Connectivity"

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    local context
    context=$(kubectl config current-context)
    log_success "Connected to cluster (context: $context)"
}

# =============================================================================
# Reloader Test Resources Cleanup
# =============================================================================

cleanup_reloader_resources() {
    log_header "Cleaning Up Reloader Test Resources"

    # Delete test namespaces (created by test suites)
    log_info "Deleting test namespaces..."
    local namespaces
    namespaces=$(kubectl get namespaces -o name 2>/dev/null | grep "reloader-" | cut -d/ -f2 || true)
    if [[ -n "$namespaces" ]]; then
        for ns in $namespaces; do
            log_info "  Deleting namespace: $ns"
            kubectl delete namespace "$ns" --ignore-not-found --wait=false 2>/dev/null || true
        done
    else
        log_info "  No test namespaces found"
    fi

    # Delete Reloader cluster-scoped resources
    log_info "Deleting cluster roles..."
    local clusterroles
    clusterroles=$(kubectl get clusterrole -o name 2>/dev/null | grep "reloader-" | cut -d/ -f2 || true)
    for cr in $clusterroles; do
        log_info "  Deleting ClusterRole: $cr"
        safe_delete clusterrole "$cr"
    done

    log_info "Deleting cluster role bindings..."
    local clusterrolebindings
    clusterrolebindings=$(kubectl get clusterrolebinding -o name 2>/dev/null | grep "reloader-" | cut -d/ -f2 || true)
    for crb in $clusterrolebindings; do
        log_info "  Deleting ClusterRoleBinding: $crb"
        safe_delete clusterrolebinding "$crb"
    done

    log_success "Reloader test resources cleaned up"
}

# =============================================================================
# Vault Cleanup
# =============================================================================

cleanup_vault() {
    log_header "Uninstalling Vault"

    # Check if Vault is installed
    if ! kubectl get namespace "$VAULT_NAMESPACE" &> /dev/null; then
        log_info "Vault namespace not found, skipping"
        return 0
    fi

    # Uninstall via Helm if available
    if command -v helm &> /dev/null; then
        if helm list -n "$VAULT_NAMESPACE" 2>/dev/null | grep -q vault; then
            log_info "Uninstalling Vault via Helm..."
            helm uninstall vault -n "$VAULT_NAMESPACE" --wait --timeout 60s 2>/dev/null || true
        fi
    fi

    # Delete namespace
    log_info "Deleting Vault namespace..."
    safe_delete namespace "$VAULT_NAMESPACE" --timeout=60s

    log_success "Vault cleaned up"
}

# =============================================================================
# CSI Secrets Store Driver Cleanup
# =============================================================================

cleanup_csi_driver() {
    log_header "Uninstalling CSI Secrets Store Driver"

    # Delete all SecretProviderClass resources first
    log_info "Deleting SecretProviderClass resources..."
    kubectl delete secretproviderclasses.secrets-store.csi.x-k8s.io \
        --all --all-namespaces --ignore-not-found --timeout=30s 2>/dev/null || true

    log_info "Deleting SecretProviderClassPodStatus resources..."
    kubectl delete secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io \
        --all --all-namespaces --ignore-not-found --timeout=30s 2>/dev/null || true

    # Uninstall via Helm if available
    if command -v helm &> /dev/null; then
        if helm list -n "$CSI_NAMESPACE" 2>/dev/null | grep -q csi-secrets-store; then
            log_info "Uninstalling CSI Secrets Store Driver via Helm..."
            helm uninstall csi-secrets-store -n "$CSI_NAMESPACE" --wait --timeout 60s 2>/dev/null || true
        fi
    else
        # Fallback to kubectl delete
        log_info "Deleting CSI Secrets Store Driver resources via kubectl..."
        local csi_url="https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/v${CSI_DRIVER_VERSION}/deploy/secrets-store-csi-driver.yaml"
        kubectl delete -f "$csi_url" --ignore-not-found --timeout=60s 2>/dev/null || true
    fi

    # Delete CRDs
    log_info "Deleting CSI Secrets Store CRDs..."
    local csi_crds="secretproviderclasses.secrets-store.csi.x-k8s.io secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io"
    for crd in $csi_crds; do
        safe_delete crd "$crd" --timeout=30s
    done

    log_success "CSI Secrets Store Driver cleaned up"
}

# =============================================================================
# Argo Rollouts Cleanup
# =============================================================================

cleanup_argo_rollouts() {
    log_header "Uninstalling Argo Rollouts"

    # Check if Argo Rollouts is installed
    if ! kubectl get namespace "$ARGO_ROLLOUTS_NAMESPACE" &> /dev/null; then
        log_info "Argo Rollouts namespace not found, skipping"
        return 0
    fi

    # Stop the controller first
    log_info "Stopping Argo Rollouts controller..."
    safe_delete deployment argo-rollouts -n "$ARGO_ROLLOUTS_NAMESPACE" --timeout=30s

    # Delete all Argo Rollouts custom resources to avoid finalizer issues
    log_info "Deleting Argo Rollouts custom resources..."
    local argo_resources="rollouts analysisruns analysistemplates experiments"
    for res in $argo_resources; do
        kubectl delete "${res}.argoproj.io" --all --all-namespaces --ignore-not-found --timeout=30s 2>/dev/null || true
    done

    # Delete using the install manifest
    log_info "Deleting Argo Rollouts installation..."
    local argo_url="https://github.com/argoproj/argo-rollouts/releases/download/${ARGO_ROLLOUTS_VERSION}/install.yaml"
    kubectl delete -f "$argo_url" --ignore-not-found --timeout=60s 2>/dev/null || true

    # Give resources time to be cleaned up
    sleep 2

    # Delete CRDs
    log_info "Deleting Argo Rollouts CRDs..."
    local argo_crds="rollouts.argoproj.io analysisruns.argoproj.io analysistemplates.argoproj.io clusteranalysistemplates.argoproj.io experiments.argoproj.io"
    for crd in $argo_crds; do
        safe_delete crd "$crd" --timeout=30s
    done

    # Delete namespace
    log_info "Deleting Argo Rollouts namespace..."
    safe_delete namespace "$ARGO_ROLLOUTS_NAMESPACE" --timeout=30s

    # Delete cluster-scoped RBAC
    log_info "Deleting Argo Rollouts cluster RBAC..."
    safe_delete clusterrole argo-rollouts argo-rollouts-aggregate-to-admin argo-rollouts-aggregate-to-edit argo-rollouts-aggregate-to-view
    safe_delete clusterrolebinding argo-rollouts

    log_success "Argo Rollouts cleaned up"
}

# =============================================================================
# Main
# =============================================================================

main() {
    echo "=== E2E Cluster Cleanup ==="

    # Pre-flight checks
    check_dependencies
    check_cluster_connectivity

    # Cleanup in reverse dependency order
    # 1. First cleanup test resources (they depend on everything else)
    cleanup_reloader_resources

    # 2. Then Vault (depends on CSI driver)
    cleanup_vault

    # 3. Then CSI driver
    cleanup_csi_driver

    # 4. Finally Argo Rollouts (independent)
    cleanup_argo_rollouts

    # Summary
    log_header "E2E Cluster Cleanup Complete"
    echo ""
    echo "Removed components:"
    echo "  ✓ Reloader test namespaces and cluster resources"
    echo "  ✓ Vault"
    echo "  ✓ CSI Secrets Store Driver"
    echo "  ✓ Argo Rollouts"
}

main "$@"
