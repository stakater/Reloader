# Reloader E2E Tests

End-to-end tests that verify Reloader works correctly in a real Kubernetes cluster. Tests create workloads, modify their referenced ConfigMaps/Secrets/SecretProviderClasses, and verify that Reloader triggers the appropriate rolling updates.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Running Tests](#running-tests)
- [Test Coverage](#test-coverage)
  - [Workload Types](#workload-types)
  - [Resource Types](#resource-types)
  - [Reload Strategies](#reload-strategies)
  - [Reference Methods](#reference-methods)
  - [Annotations](#annotations)
  - [CLI Flags](#cli-flags)
- [Test Organization](#test-organization)
- [Debugging](#debugging)
- [Writing Tests](#writing-tests)

---

## Quick Start

```bash
# One-time setup: create Kind cluster and install dependencies
make e2e-setup

# Run all e2e tests
make e2e

# Cleanup when done
make e2e-cleanup
```

---

## Prerequisites

| Requirement | Version | Purpose |
|------------|---------|---------|
| Go | 1.25+ | Test execution |
| Docker/Podman | Latest | Image building |
| [Kind](https://kind.sigs.k8s.io/) | 0.20+ | Local Kubernetes cluster |
| kubectl | Latest | Cluster interaction |
| Helm | 3.x | Reloader deployment |

### Optional Dependencies

| Component | Purpose | Auto-installed by |
|-----------|---------|-------------------|
| [Argo Rollouts](https://argoproj.github.io/rollouts/) | Argo Rollout tests | `make e2e-setup` |
| [CSI Secrets Store Driver](https://secrets-store-csi-driver.sigs.k8s.io/) | SecretProviderClass tests | `make e2e-setup` |
| [Vault](https://www.vaultproject.io/) | CSI provider backend | `make e2e-setup` |
| OpenShift | DeploymentConfig tests | Requires OpenShift cluster |

---

## Running Tests

### Make Targets

| Target | Description |
|--------|-------------|
| `make e2e-setup` | Create Kind cluster and install all dependencies (Argo, CSI, Vault) |
| `make e2e` | Build image, load to Kind, run all tests |
| `make e2e-cleanup` | Remove test resources and delete Kind cluster |
| `make e2e-ci` | Full CI pipeline: setup → test → cleanup |

### Common Workflows

```bash
# Development workflow
make e2e-setup          # Once at the start
make e2e                # Run tests (repeat as needed)
make e2e                # ...iterate...
make e2e-cleanup        # When done

# CI workflow
make e2e-ci             # Does everything

# Test specific image
SKIP_BUILD=true RELOADER_IMAGE=ghcr.io/stakater/reloader:v1.2.0 make e2e
```

### Running Specific Tests

```bash
# Run a specific test suite
go tool ginkgo -v ./test/e2e/core/...
go tool ginkgo -v ./test/e2e/annotations/...
go tool ginkgo -v ./test/e2e/csi/...

# Run tests matching a pattern
go tool ginkgo -v --focus="should reload when ConfigMap" ./test/e2e/...

# Run tests with specific labels
go tool ginkgo -v --label-filter="csi" ./test/e2e/...
go tool ginkgo -v --label-filter="!argo && !openshift" ./test/e2e/...

# Run all tests, continue on failure
go tool ginkgo --keep-going -v ./test/e2e/...
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `RELOADER_IMAGE` | Image to test | `ghcr.io/stakater/reloader:test` |
| `SKIP_BUILD` | Skip image build | `false` |
| `KIND_CLUSTER` | Kind cluster name | `reloader-e2e` |
| `KUBECONFIG` | Kubernetes config path | `~/.kube/config` |
| `E2E_TIMEOUT` | Test timeout | `45m` |

---

## Test Coverage

### Workload Types

| Workload | Annotations | EnvVars | CSI | Special Handling |
|----------|-------------|---------|-----|------------------|
| Deployment | ✅ | ✅ | ✅ | Standard rolling update |
| DaemonSet | ✅ | ✅ | ✅ | Standard rolling update |
| StatefulSet | ✅ | ✅ | ✅ | Standard rolling update |
| CronJob | ✅ | ❌ | ❌ | Updates job template |
| Job | ✅ | ❌ | ❌ | Recreates job |
| Argo Rollout | ✅ | ✅ | ❌ | Supports restart strategy |
| DeploymentConfig | ✅ | ✅ | ❌ | OpenShift only |

### Resource Types

#### ConfigMaps & Secrets

Standard Kubernetes resources that trigger reloads when their data changes.

**Tested Scenarios:**
- Data changes trigger reload
- Label-only changes do NOT trigger reload
- Annotation-only changes do NOT trigger reload
- Multiple resources in single annotation (comma-separated)
- Regex patterns for resource names

#### SecretProviderClass (CSI)

CSI Secrets Store Driver integration for external secret providers (Vault, Azure, AWS, etc.).

**Tested Scenarios:**
- SecretProviderClassPodStatus changes trigger reload
- Label-only changes on SPCPS do NOT trigger reload
- Auto-detection with `secretproviderclass.reloader.stakater.com/auto: "true"`
- Exclude specific SPCs from auto-reload
- Init containers with CSI volumes
- Multiple CSI volumes per workload

### Reload Strategies

#### Annotations Strategy (Default)

Adds/updates `reloader.stakater.com/last-reloaded-from` annotation on pod template.

```yaml
spec:
  template:
    metadata:
      annotations:
        reloader.stakater.com/last-reloaded-from: "my-configmap"
```

#### EnvVars Strategy

Adds `STAKATER_<RESOURCE>_<TYPE>` environment variable to containers.

```yaml
spec:
  template:
    spec:
      containers:
      - env:
        - name: STAKATER_MY_CONFIGMAP_CONFIGMAP
          value: "<sha256-hash>"
```

### Reference Methods

All methods are tested for Deployment, DaemonSet, and StatefulSet:

| Method | Description | ConfigMap | Secret | CSI |
|--------|-------------|-----------|--------|-----|
| `envFrom` | All keys as env vars | ✅ | ✅ | - |
| `valueFrom.configMapKeyRef` | Single key as env var | ✅ | - | - |
| `valueFrom.secretKeyRef` | Single key as env var | - | ✅ | - |
| Volume mount | Mount as files | ✅ | ✅ | ✅ |
| Projected volume | Combined sources | ✅ | ✅ | - |
| Init container (envFrom) | Init container env | ✅ | ✅ | - |
| Init container (volume) | Init container mount | ✅ | ✅ | ✅ |

### Annotations

#### Reload Triggers

| Annotation | Description |
|------------|-------------|
| `configmap.reloader.stakater.com/reload` | Reload on specific ConfigMap(s) change |
| `secret.reloader.stakater.com/reload` | Reload on specific Secret(s) change |
| `secretproviderclass.reloader.stakater.com/reload` | Reload on specific SPC(s) change |

#### Auto-Detection

| Annotation | Description |
|------------|-------------|
| `reloader.stakater.com/auto: "true"` | Auto-detect all mounted resources |
| `configmap.reloader.stakater.com/auto: "true"` | Auto-detect ConfigMaps only |
| `secret.reloader.stakater.com/auto: "true"` | Auto-detect Secrets only |
| `secretproviderclass.reloader.stakater.com/auto: "true"` | Auto-detect SPCs only |

#### Exclusions

| Annotation | Description |
|------------|-------------|
| `configmaps.exclude.reloader.stakater.com/reload` | Exclude ConfigMaps from auto |
| `secrets.exclude.reloader.stakater.com/reload` | Exclude Secrets from auto |
| `secretproviderclasses.exclude.reloader.stakater.com/reload` | Exclude SPCs from auto |
| `reloader.stakater.com/ignore: "true"` | On resource: prevents any reload |

#### Search & Match

| Annotation | Target | Description |
|------------|--------|-------------|
| `reloader.stakater.com/search: "true"` | Workload | Watch for matching resources |
| `reloader.stakater.com/match: "true"` | Resource | Trigger watchers on change |

#### Other

| Annotation | Description |
|------------|-------------|
| `reloader.stakater.com/pause-period` | Pause deployment after reload |

### CLI Flags

Tests verify these Reloader command-line flags:

| Flag | Description |
|------|-------------|
| `--namespaces-to-ignore` | Skip specified namespaces |
| `--namespace-selector` | Only watch namespaces with matching labels |
| `--watch-globally` | Watch all namespaces vs own namespace only |
| `--resource-label-selector` | Only watch resources with matching labels |
| `--ignore-secrets` | Ignore all Secret changes |
| `--ignore-configmaps` | Ignore all ConfigMap changes |
| `--ignore-cronjobs` | Skip CronJob workloads |
| `--ignore-jobs` | Skip Job workloads |
| `--reload-on-create` | Trigger reload on resource creation |
| `--reload-on-delete` | Trigger reload on resource deletion |
| `--auto-reload-all` | Auto-reload all workloads without annotations |
| `--enable-csi-integration` | Enable SecretProviderClass support |

---

## Test Organization

```
test/e2e/
├── core/                          # Core workload tests
│   ├── core_suite_test.go
│   └── workloads_test.go          # All workload types, both strategies
│
├── annotations/                   # Annotation behavior tests
│   ├── annotations_suite_test.go
│   ├── auto_reload_test.go        # Auto-detection variations
│   ├── combination_test.go        # Multiple annotations together
│   ├── exclude_test.go            # Exclude annotations
│   ├── pause_period_test.go       # Pause after reload
│   ├── resource_ignore_test.go    # Ignore annotation on resources
│   └── search_match_test.go       # Search/match pattern
│
├── flags/                         # CLI flag tests
│   ├── flags_suite_test.go
│   ├── auto_reload_all_test.go
│   ├── ignore_resources_test.go
│   ├── ignored_workloads_test.go
│   ├── namespace_ignore_test.go
│   ├── namespace_selector_test.go
│   ├── reload_on_create_test.go
│   ├── reload_on_delete_test.go
│   ├── resource_selector_test.go
│   └── watch_globally_test.go
│
├── advanced/                      # Advanced scenarios
│   ├── advanced_suite_test.go
│   ├── job_reload_test.go         # Job recreation
│   ├── multi_container_test.go    # Multiple containers
│   ├── pod_annotations_test.go    # Pod template annotations
│   └── regex_test.go              # Regex patterns
│
├── csi/                           # CSI SecretProviderClass tests
│   ├── csi_suite_test.go
│   └── csi_test.go                # SPC-specific scenarios
│
├── argo/                          # Argo Rollouts (requires installation)
│   ├── argo_suite_test.go
│   └── rollout_test.go
│
└── utils/                         # Shared test utilities
    ├── annotations.go             # Annotation builders
    ├── constants.go               # Test constants
    ├── csi.go                     # CSI client and helpers
    ├── resources.go               # Resource creation helpers
    ├── testenv.go                 # Test environment setup
    ├── wait.go                    # Wait/polling utilities
    ├── workload_adapter.go        # Workload abstraction interface
    ├── workload_deployment.go     # Deployment adapter
    ├── workload_daemonset.go      # DaemonSet adapter
    ├── workload_statefulset.go    # StatefulSet adapter
    ├── workload_cronjob.go        # CronJob adapter
    ├── workload_job.go            # Job adapter
    ├── workload_argo.go           # Argo Rollout adapter
    └── workload_openshift.go      # DeploymentConfig adapter
```

---

## Debugging

### View Test Output

```bash
# Verbose output
go tool ginkgo -v ./test/e2e/core/...

# Focus on specific test
go tool ginkgo -v --focus="should reload when ConfigMap" ./test/e2e/...

# Show all spec names
go tool ginkgo -v --dry-run ./test/e2e/...
```

### Check Reloader Logs

```bash
# Find Reloader pod
kubectl get pods -A | grep reloader

# View logs
kubectl logs -n <namespace> -l app.kubernetes.io/name=reloader --tail=100 -f

# Check events
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

### Inspect Test Resources

```bash
# List test namespaces
kubectl get ns | grep reloader

# Check workloads in test namespace
kubectl get deploy,ds,sts,cronjob,job -n <test-namespace>

# Check ConfigMaps/Secrets
kubectl get cm,secret -n <test-namespace>

# Check CSI resources
kubectl get secretproviderclass,secretproviderclasspodstatus -n <test-namespace>
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Tests timeout | Reloader not running | Check pod status and logs |
| CSI tests skipped | CSI driver not installed | Run `make e2e-setup` |
| Argo tests skipped | Argo Rollouts not installed | Run `make e2e-setup` |
| OpenShift tests skipped | Not an OpenShift cluster | Expected on Kind |
| "resource not found" | Missing CRDs | Install required components |
| Duplicate volume names | Test bug | Check CSI volume naming |

---

## Writing Tests

### Using the Workload Adapter Pattern

Test the same behavior across multiple workload types:

```go
DescribeTable("should reload when ConfigMap changes",
    func(workloadType utils.WorkloadType) {
        adapter := registry.Get(workloadType)
        if adapter == nil {
            Skip(fmt.Sprintf("%s not available", workloadType))
        }

        // Create ConfigMap
        _, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
            map[string]string{"key": "initial"}, nil)
        Expect(err).NotTo(HaveOccurred())

        // Create workload via adapter
        err = adapter.Create(ctx, testNamespace, workloadName, utils.WorkloadConfig{
            ConfigMapName:       configMapName,
            UseConfigMapEnvFrom: true,
            Annotations:         utils.BuildConfigMapReloadAnnotation(configMapName),
        })
        Expect(err).NotTo(HaveOccurred())

        // Wait for ready
        err = adapter.WaitReady(ctx, testNamespace, workloadName, utils.DeploymentReady)
        Expect(err).NotTo(HaveOccurred())

        // Update ConfigMap
        err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName,
            map[string]string{"key": "updated"})
        Expect(err).NotTo(HaveOccurred())

        // Verify reload
        reloaded, err := adapter.WaitReloaded(ctx, testNamespace, workloadName,
            utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
        Expect(err).NotTo(HaveOccurred())
        Expect(reloaded).To(BeTrue())
    },
    Entry("Deployment", utils.WorkloadDeployment),
    Entry("DaemonSet", utils.WorkloadDaemonSet),
    Entry("StatefulSet", utils.WorkloadStatefulSet),
)
```

### Direct Resource Creation

For Deployment-specific tests:

```go
It("should reload with custom setup", func() {
    _, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
        map[string]string{"key": "value"}, nil)
    Expect(err).NotTo(HaveOccurred())

    _, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
        utils.WithConfigMapEnvFrom(configMapName),
        utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
    )
    Expect(err).NotTo(HaveOccurred())

    // ... test logic ...
})
```

### CSI Tests

```go
It("should reload when SecretProviderClassPodStatus changes", func() {
    if !utils.IsCSIDriverInstalled(ctx, csiClient) {
        Skip("CSI driver not installed")
    }

    // Create SPC
    _, err := utils.CreateSecretProviderClass(ctx, csiClient, testNamespace, spcName, nil)
    Expect(err).NotTo(HaveOccurred())

    // Create SPCPS
    _, err = utils.CreateSecretProviderClassPodStatus(ctx, csiClient, testNamespace, spcpsName, spcName,
        utils.NewSPCPSObjects("secret1", "v1"))
    Expect(err).NotTo(HaveOccurred())

    // Create Deployment with CSI volume
    _, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
        utils.WithCSIVolume(spcName),
        utils.WithAnnotations(utils.BuildSecretProviderClassReloadAnnotation(spcName)),
    )
    Expect(err).NotTo(HaveOccurred())

    // Update SPCPS
    err = utils.UpdateSecretProviderClassPodStatus(ctx, csiClient, testNamespace, spcpsName,
        utils.NewSPCPSObjects("secret1", "v2"))
    Expect(err).NotTo(HaveOccurred())

    // Verify reload
    reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
        utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
    Expect(err).NotTo(HaveOccurred())
    Expect(reloaded).To(BeTrue())
})
```

### Negative Tests

Verify that something does NOT trigger a reload:

```go
It("should NOT reload when only labels change", func() {
    // Setup...

    // Make a change that shouldn't trigger reload
    err = utils.UpdateConfigMapLabels(ctx, kubeClient, testNamespace, configMapName,
        map[string]string{"new-label": "value"})
    Expect(err).NotTo(HaveOccurred())

    // Wait briefly, then verify NO reload
    time.Sleep(utils.NegativeTestWait)
    reloaded, err := utils.WaitForDeploymentReloaded(ctx, kubeClient, testNamespace, deploymentName,
        utils.AnnotationLastReloadedFrom, utils.ShortTimeout)
    Expect(err).NotTo(HaveOccurred())
    Expect(reloaded).To(BeFalse(), "Should NOT have reloaded")
})
```

### Test Labels

Use labels to categorize tests:

```go
Entry("Deployment", Label("csi"), utils.WorkloadDeployment),
Entry("with OpenShift", Label("openshift"), utils.WorkloadDeploymentConfig),
Entry("with Argo", Label("argo"), utils.WorkloadArgoRollout),
```

Run by label:
```bash
go tool ginkgo --label-filter="csi" ./test/e2e/...
go tool ginkgo --label-filter="!openshift && !argo" ./test/e2e/...
```
