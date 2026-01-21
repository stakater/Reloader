#!/bin/bash
# Setup script for e2e test cluster
# Run this before running e2e tests: ./scripts/e2e-cluster-setup.sh
#
# This installs:
#   - Argo Rollouts (for Rollout workload testing)
#   - CSI Secrets Store Driver (for SecretProviderClass testing)
#   - Vault with CSI Provider (as the secrets backend for CSI)
#
# All versions are pinned for reproducibility and can be overridden via environment variables.

set -euo pipefail

# =============================================================================
# Configuration (all versions pinned for reproducibility)
# =============================================================================

# Argo Rollouts
ARGO_ROLLOUTS_VERSION="${ARGO_ROLLOUTS_VERSION:-v1.7.2}"
ARGO_ROLLOUTS_NAMESPACE="argo-rollouts"

# CSI Secrets Store Driver
CSI_DRIVER_VERSION="${CSI_DRIVER_VERSION:-1.5.5}"
CSI_NAMESPACE="kube-system"

# Vault (HashiCorp)
VAULT_CHART_VERSION="${VAULT_CHART_VERSION:-0.31.0}"
VAULT_VERSION="${VAULT_VERSION:-1.20.4}"
VAULT_CSI_PROVIDER_VERSION="${VAULT_CSI_PROVIDER_VERSION:-1.7.0}"
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

wait_for_rollout() {
    local resource_type="$1"
    local resource_name="$2"
    local namespace="$3"
    local timeout="${4:-180s}"

    kubectl rollout status "$resource_type/$resource_name" -n "$namespace" --timeout="$timeout"
}

wait_for_condition() {
    local condition="$1"
    local resource="$2"
    local namespace="${3:-}"
    local timeout="${4:-60s}"

    if [[ -n "$namespace" ]]; then
        kubectl wait --for="condition=$condition" "$resource" -n "$namespace" --timeout="$timeout"
    else
        kubectl wait --for="condition=$condition" "$resource" --timeout="$timeout"
    fi
}

# =============================================================================
# Dependency Checks
# =============================================================================

check_dependencies() {
    log_header "Checking Dependencies"

    local missing_deps=()

    # Required: kubectl
    if ! check_command kubectl; then
        missing_deps+=("kubectl")
    fi

    # Required: helm (for CSI driver and Vault installation)
    if ! check_command helm; then
        missing_deps+=("helm")
    fi

    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_error "Please install the missing tools and try again."
        exit 1
    fi

    log_success "All required dependencies are available"
}

check_cluster_connectivity() {
    log_header "Checking Cluster Connectivity"

    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        log_error "Please ensure your kubeconfig is correctly configured"
        exit 1
    fi

    local context
    context=$(kubectl config current-context)
    log_success "Connected to cluster (context: $context)"
}

# =============================================================================
# Argo Rollouts Installation
# =============================================================================

install_argo_rollouts() {
    log_header "Installing Argo Rollouts ${ARGO_ROLLOUTS_VERSION}"

    # Check if already installed
    if kubectl get crd rollouts.argoproj.io &> /dev/null; then
        if kubectl get deployment argo-rollouts -n "$ARGO_ROLLOUTS_NAMESPACE" &> /dev/null; then
            log_success "Argo Rollouts is already installed"
            return 0
        fi
        log_info "Argo Rollouts CRD exists but controller not running, reinstalling..."
    fi

    # Create namespace
    kubectl create namespace "$ARGO_ROLLOUTS_NAMESPACE" 2>/dev/null || true

    # Install from official manifest
    local argo_url="https://github.com/argoproj/argo-rollouts/releases/download/${ARGO_ROLLOUTS_VERSION}/install.yaml"
    log_info "Applying manifest from: $argo_url"
    kubectl apply -n "$ARGO_ROLLOUTS_NAMESPACE" -f "$argo_url"

    # Wait for deployment to be created
    sleep 2

    # Patch deployment to remove resource requirements (for Kind cluster compatibility)
    log_info "Patching deployment for Kind compatibility..."
    local patch_json='[{"op": "remove", "path": "/spec/template/spec/containers/0/resources"}]'
    if ! kubectl patch deployment argo-rollouts -n "$ARGO_ROLLOUTS_NAMESPACE" --type=json -p "$patch_json" 2>/dev/null; then
        patch_json='{"spec":{"template":{"spec":{"containers":[{"name":"argo-rollouts","resources":{"limits":null,"requests":null}}]}}}}'
        kubectl patch deployment argo-rollouts -n "$ARGO_ROLLOUTS_NAMESPACE" --type=strategic -p "$patch_json" 2>/dev/null || true
    fi

    # Wait for controller to be ready
    log_info "Waiting for Argo Rollouts controller..."
    wait_for_condition "available" "deployment/argo-rollouts" "$ARGO_ROLLOUTS_NAMESPACE" "180s"
    wait_for_condition "established" "crd/rollouts.argoproj.io" "" "60s"

    log_success "Argo Rollouts ${ARGO_ROLLOUTS_VERSION} installed"
}

# =============================================================================
# CSI Secrets Store Driver Installation
# =============================================================================

install_csi_driver() {
    log_header "Installing CSI Secrets Store Driver ${CSI_DRIVER_VERSION}"

    # Check if already installed
    if kubectl get crd secretproviderclasses.secrets-store.csi.x-k8s.io &> /dev/null; then
        if kubectl get daemonset -n "$CSI_NAMESPACE" -l app=secrets-store-csi-driver &> /dev/null 2>&1; then
            log_success "CSI Secrets Store Driver is already installed"
            return 0
        fi
        log_info "CSI Driver CRD exists but DaemonSet not found, installing..."
    fi

    # Add Helm repo
    helm repo add secrets-store-csi-driver https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts 2>/dev/null || true
    helm repo update secrets-store-csi-driver

    # Install via Helm with pinned version
    log_info "Installing via Helm (version ${CSI_DRIVER_VERSION})..."
    helm upgrade --install csi-secrets-store secrets-store-csi-driver/secrets-store-csi-driver \
        --namespace "$CSI_NAMESPACE" \
        --version "$CSI_DRIVER_VERSION" \
        --set syncSecret.enabled=true \
        --set enableSecretRotation=true \
        --set rotationPollInterval=2s \
        --wait \
        --timeout 180s

    # Wait for CRDs to be established
    log_info "Waiting for CRDs to be established..."
    wait_for_condition "established" "crd/secretproviderclasses.secrets-store.csi.x-k8s.io" "" "60s"
    wait_for_condition "established" "crd/secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io" "" "60s"

    # Wait for DaemonSet to be ready (try different names as they vary by installation method)
    log_info "Waiting for CSI driver pods..."
    kubectl rollout status daemonset/csi-secrets-store-secrets-store-csi-driver -n "$CSI_NAMESPACE" --timeout=180s 2>/dev/null || \
        kubectl rollout status daemonset/secrets-store-csi-driver -n "$CSI_NAMESPACE" --timeout=180s 2>/dev/null || \
        log_warning "Could not verify DaemonSet status (name may vary)"

    log_success "CSI Secrets Store Driver ${CSI_DRIVER_VERSION} installed"
}

# =============================================================================
# Vault Installation
# =============================================================================

install_vault() {
    log_header "Installing Vault ${VAULT_VERSION} (Chart ${VAULT_CHART_VERSION})"

    # Check if already installed
    if kubectl get pods -n "$VAULT_NAMESPACE" -l app.kubernetes.io/name=vault 2>/dev/null | grep -q Running; then
        log_success "Vault is already installed and running"
        return 0
    fi

    # Add Helm repo
    helm repo add hashicorp https://helm.releases.hashicorp.com 2>/dev/null || true
    helm repo update hashicorp

    # Install Vault in dev mode with CSI provider
    # Dev mode: single server, in-memory storage, pre-unsealed, root token = "root"
    log_info "Installing Vault via Helm..."
    helm upgrade --install vault hashicorp/vault \
        --namespace "$VAULT_NAMESPACE" \
        --create-namespace \
        --version "$VAULT_CHART_VERSION" \
        --set "server.image.tag=${VAULT_VERSION}" \
        --set "server.dev.enabled=true" \
        --set "server.dev.devRootToken=root" \
        --set "server.resources.requests.memory=64Mi" \
        --set "server.resources.requests.cpu=50m" \
        --set "server.resources.limits.memory=128Mi" \
        --set "server.resources.limits.cpu=100m" \
        --set "injector.enabled=false" \
        --set "csi.enabled=true" \
        --set "csi.image.tag=${VAULT_CSI_PROVIDER_VERSION}" \
        --set "csi.resources.requests.memory=64Mi" \
        --set "csi.resources.requests.cpu=50m" \
        --set "csi.resources.limits.memory=128Mi" \
        --set "csi.resources.limits.cpu=100m" \
        --wait \
        --timeout 180s

    # Wait for pods to be ready
    log_info "Waiting for Vault pod..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=vault -n "$VAULT_NAMESPACE" --timeout=120s

    log_info "Waiting for Vault CSI provider..."
    wait_for_rollout "daemonset" "vault-csi-provider" "$VAULT_NAMESPACE" "120s"

    log_success "Vault ${VAULT_VERSION} installed"
}

configure_vault() {
    log_header "Configuring Vault for Kubernetes Authentication"

    # Enable KV secrets engine (ignore error if already enabled - dev mode has it by default)
    log_info "Enabling KV secrets engine..."
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault secrets enable -path=secret kv-v2 2>/dev/null || true

    # Create test secrets for e2e tests
    log_info "Creating test secrets..."
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault kv put secret/test username="test-user" password="test-password"
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault kv put secret/app1 api_key="app1-api-key-v1" db_password="app1-db-pass-v1"
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault kv put secret/app2 api_key="app2-api-key-v1" db_password="app2-db-pass-v1"
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault kv put secret/rotation-test value="initial-value-v1"

    # Enable Kubernetes auth method
    log_info "Enabling Kubernetes auth..."
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault auth enable kubernetes 2>/dev/null || true

    # Configure Kubernetes auth to use in-cluster config
    log_info "Configuring Kubernetes auth..."
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- sh -c \
        'vault write auth/kubernetes/config kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443"'

    # Create policy for reading test secrets
    log_info "Creating Vault policy..."
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- sh -c 'vault policy write test-policy - <<EOF
path "secret/data/*" {
  capabilities = ["read"]
}
EOF'

    # Create role that binds to any service account (for e2e tests)
    log_info "Creating Vault role..."
    kubectl exec -n "$VAULT_NAMESPACE" vault-0 -- vault write auth/kubernetes/role/test-role \
        bound_service_account_names="*" \
        bound_service_account_namespaces="*" \
        policies=test-policy \
        ttl=1h

    log_success "Vault configured for CSI testing"
}

# =============================================================================
# Main
# =============================================================================

main() {
    echo "=== E2E Cluster Setup ==="
    echo ""
    echo "Versions:"
    echo "  Argo Rollouts:      ${ARGO_ROLLOUTS_VERSION}"
    echo "  CSI Driver:         ${CSI_DRIVER_VERSION}"
    echo "  Vault Chart:        ${VAULT_CHART_VERSION}"
    echo "  Vault Server:       ${VAULT_VERSION}"
    echo "  Vault CSI Provider: ${VAULT_CSI_PROVIDER_VERSION}"

    # Pre-flight checks
    check_dependencies
    check_cluster_connectivity

    # Install components in dependency order
    install_argo_rollouts
    install_csi_driver
    install_vault
    configure_vault

    # Summary
    log_header "E2E Cluster Setup Complete"
    echo ""
    echo "Installed components:"
    echo "  ✓ Argo Rollouts ${ARGO_ROLLOUTS_VERSION}"
    echo "  ✓ CSI Secrets Store Driver ${CSI_DRIVER_VERSION}"
    echo "  ✓ Vault ${VAULT_VERSION} (CSI Provider ${VAULT_CSI_PROVIDER_VERSION})"
    echo ""
    echo "Vault is running in dev mode with root token: root"
    echo "Test secrets created at: secret/test, secret/app1, secret/app2, secret/rotation-test"
    echo ""
    echo "You can now run e2e tests:"
    echo "  make e2e"
    echo "  # or"
    echo "  SKIP_BUILD=true RELOADER_IMAGE=ghcr.io/stakater/reloader:test go test -v ./test/e2e/..."
}

main "$@"
