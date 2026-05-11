# Reloader E2E Tests

End-to-end tests verifying Reloader functionality in a real Kubernetes cluster.

## Quick Start

```bash
make e2e-setup    # Create Kind cluster, install Argo/CSI/Vault
make e2e          # Build image, run tests
make e2e-cleanup  # Teardown
```

## Prerequisites

- Go 1.25+
- Docker or Podman
- [Kind](https://kind.sigs.k8s.io/) 0.20+
- kubectl
- Helm 3.x

## Running Tests

```bash
# Run all tests
make e2e

# Run specific suite
go tool ginkgo -v ./test/e2e/core/...

# Run by pattern
go tool ginkgo -v --focus="ConfigMap" ./test/e2e/...

# Run by label
go tool ginkgo -v --label-filter="csi" ./test/e2e/...
go tool ginkgo -v --label-filter="!argo && !openshift" ./test/e2e/...

# Test a specific image
SKIP_BUILD=true RELOADER_IMAGE=ghcr.io/stakater/reloader:v1.2.0 make e2e
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RELOADER_IMAGE` | `ghcr.io/stakater/reloader:test` | Image to test |
| `SKIP_BUILD` | `false` | Skip image build |
| `KIND_CLUSTER` | `reloader-e2e` | Kind cluster name |
| `E2E_TIMEOUT` | `45m` | Test timeout |

## Test Structure

```
test/e2e/
├── core/           # Core reload functionality
├── annotations/    # Annotation behaviors (auto, exclude, search/match)
├── flags/          # CLI flag behaviors
├── advanced/       # Jobs, multi-container, regex patterns
├── csi/            # SecretProviderClass integration
├── argo/           # Argo Rollouts (requires CRDs)
└── utils/          # Shared test utilities and workload adapters
```

### Labels

| Label | Description |
|-------|-------------|
| `csi` | Requires CSI driver and Vault |
| `argo` | Requires Argo Rollouts CRDs |
| `openshift` | Requires OpenShift cluster |

## Writing Tests

Use the workload adapter pattern for cross-workload tests:

```go
DescribeTable("should reload when ConfigMap changes", func(workloadType utils.WorkloadType) {
    adapter := registry.Get(workloadType)
    if adapter == nil {
        Skip(fmt.Sprintf("%s not available", workloadType))
    }

    // Create resources
    _, err := utils.CreateConfigMap(ctx, kubeClient, ns, cmName, map[string]string{"key": "v1"}, nil)
    Expect(err).NotTo(HaveOccurred())

    err = adapter.Create(ctx, ns, name, utils.WorkloadConfig{
        ConfigMapName:       cmName,
        UseConfigMapEnvFrom: true,
        Annotations:         utils.BuildConfigMapReloadAnnotation(cmName),
    })
    Expect(err).NotTo(HaveOccurred())
    DeferCleanup(func() { _ = adapter.Delete(ctx, ns, name) })

    // Wait ready
    Expect(adapter.WaitReady(ctx, ns, name, utils.WorkloadReadyTimeout)).To(Succeed())

    // Trigger reload
    Expect(utils.UpdateConfigMap(ctx, kubeClient, ns, cmName, map[string]string{"key": "v2"})).To(Succeed())

    // Verify
    reloaded, err := adapter.WaitReloaded(ctx, ns, name, utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
    Expect(err).NotTo(HaveOccurred())
    Expect(reloaded).To(BeTrue())
},
    Entry("Deployment", utils.WorkloadDeployment),
    Entry("DaemonSet", utils.WorkloadDaemonSet),
    Entry("StatefulSet", utils.WorkloadStatefulSet),
    Entry("ArgoRollout", Label("argo"), utils.WorkloadArgoRollout),
)
```

## Debugging

```bash
# Reloader logs
kubectl logs -n <namespace> -l app.kubernetes.io/name=reloader -f

# Test resources
kubectl get deploy,ds,sts,cm,secret -n <test-namespace>

# CSI resources
kubectl get secretproviderclass,secretproviderclasspodstatus -A
```
