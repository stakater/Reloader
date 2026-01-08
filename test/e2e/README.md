# Reloader E2E Tests

These tests verify that Reloader actually works in a real Kubernetes cluster. They spin up a Kind cluster, build and deploy Reloader, then create workloads and change their ConfigMaps/Secrets to make sure everything reloads correctly.

## Running the Tests

```bash
# Run everything (creates Kind cluster, builds image, runs tests)
make e2e

# Test a specific image without building
SKIP_BUILD=true RELOADER_IMAGE=stakater/reloader:v1.0.0 make e2e

# Run just one test suite
go test -v -timeout 30m ./test/e2e/core/...
go test -v -timeout 30m ./test/e2e/annotations/...
go test -v -timeout 30m ./test/e2e/flags/...

# Skip Argo/OpenShift tests (if you don't have them installed)
go test -v ./test/e2e/core/... --ginkgo.label-filter="!argo && !openshift"
```

## What You Need

- Go 1.21+
- Docker
- [Kind](https://kind.sigs.k8s.io/)
- kubectl
- Helm 3
- Argo Rollouts (optional, for Argo tests)
- OpenShift (optional, for DeploymentConfig tests)

---

## What Gets Tested

### Deployments

Deployments are the most thoroughly tested workload. Here's everything we verify:

**Basic Reload Behavior**
- Reloads when a referenced ConfigMap's data changes
- Reloads when a referenced Secret's data changes
- Reloads when using `auto=true` annotation (auto-detects all mounted ConfigMaps/Secrets)
- Does NOT reload when only ConfigMap/Secret labels change (data must change)
- Does NOT reload when `auto=false` is set

**Different Ways to Reference ConfigMaps/Secrets**
- `envFrom` - inject all keys as environment variables
- `valueFrom.configMapKeyRef` - single key as env var
- `valueFrom.secretKeyRef` - single key as env var
- Volume mounts - mount ConfigMap/Secret as files
- Projected volumes - multiple sources combined into one mount
- Init containers with envFrom
- Init containers with volume mounts

**Annotation Variations**
- `configmap.reloader.stakater.com/reload: my-config` - explicit ConfigMap
- `secret.reloader.stakater.com/reload: my-secret` - explicit Secret
- `reloader.stakater.com/auto: "true"` - auto-detect everything
- `configmap.reloader.stakater.com/auto: "true"` - auto-detect only ConfigMaps
- `secret.reloader.stakater.com/auto: "true"` - auto-detect only Secrets
- Multiple ConfigMaps/Secrets in one annotation (comma-separated)
- Annotations on pod template vs deployment metadata (both work)

**Search & Match**
- Deployments with `search` annotation find ConfigMaps with `match` annotation
- Only reloads if both sides have the right annotations

**Exclude & Ignore**
- Exclude specific ConfigMaps/Secrets from auto-reload
- Ignore annotation on ConfigMap/Secret prevents any reload

**Pause Period**
- Deployment gets paused after reload when pause-period annotation is set

**Regex Patterns**
- Pattern matching for ConfigMap/Secret names (e.g., `app-config-.*`)

**Multi-Container**
- Works when multiple containers share the same ConfigMap
- Works when different containers use different ConfigMaps

**EnvVars Strategy**
- Adds `STAKATER_` environment variables instead of pod annotations
- Verifies the env var appears after ConfigMap/Secret change

### DaemonSets

DaemonSets get the same treatment as Deployments:

- Reloads when ConfigMap data changes
- Reloads when Secret data changes
- Works with `auto=true` annotation
- Does NOT reload on label-only changes
- Supports all reference methods (envFrom, valueFrom, volumes, projected, init containers)
- EnvVars strategy works

### StatefulSets

StatefulSets are tested identically to Deployments and DaemonSets:

- Reloads when ConfigMap data changes
- Reloads when Secret data changes
- Works with `auto=true` annotation
- Does NOT reload on label-only changes
- Supports all reference methods
- EnvVars strategy works

### CronJobs

CronJobs are a bit special - when a CronJob's ConfigMap changes, Reloader updates the CronJob spec so the *next* Job it creates will have the new config.

**What's Tested**
- CronJob spec updates when referenced ConfigMap changes
- CronJob spec updates when referenced Secret changes
- Works with `auto=true` annotation
- Works with explicit reload annotations
- Does NOT update on label-only changes

**Note:** CronJobs don't support the EnvVars strategy since they don't have running pods to inject env vars into.

### Jobs

Jobs require special handling - since you can't modify a running Job, Reloader deletes and recreates it with the new config.

**What's Tested**
- Job gets recreated (new UID) when ConfigMap changes
- Job gets recreated when Secret changes
- Works with `auto=true` annotation
- Works with explicit reload annotations
- Works with `valueFrom.configMapKeyRef` references
- Works with `valueFrom.secretKeyRef` references

**Note:** Jobs don't support the EnvVars strategy.

### Argo Rollouts

Argo Rollouts are Kubernetes Deployments on steroids with advanced deployment strategies. Tests require Argo Rollouts to be installed.

**What's Tested**
- Reloads when ConfigMap data changes
- Reloads when Secret data changes
- Works with `auto=true` annotation
- Does NOT reload on label-only changes
- Default strategy (annotation-based, like Deployments)
- Restart strategy (sets `spec.restartAt` field instead of annotations)
- Supports all reference methods
- EnvVars strategy works

### DeploymentConfigs (OpenShift)

OpenShift's legacy workload type. Tests only run on OpenShift clusters.

**What's Tested**
- Reloads when ConfigMap data changes
- Reloads when Secret data changes
- Works with `auto=true` annotation
- Does NOT reload on label-only changes
- Supports all reference methods
- EnvVars strategy works

---

## CLI Flag Tests

These tests verify Reloader's command-line options work correctly. Each test deploys Reloader with different flags.

### Namespace Filtering

**`namespaceSelector`**
- Only watches namespaces with matching labels
- Ignores ConfigMap changes in non-matching namespaces

**`ignoreNamespaces`**
- Skips specified namespaces entirely
- Still watches all other namespaces

**`watchGlobally`**
- `true` (default): watches all namespaces
- `false`: only watches Reloader's own namespace

### Resource Filtering

**`resourceLabelSelector`**
- Only watches ConfigMaps/Secrets with matching labels
- Ignores changes to resources without the label

**`ignoreSecrets`**
- Completely ignores all Secret changes
- Still watches ConfigMaps

**`ignoreConfigMaps`**
- Completely ignores all ConfigMap changes
- Still watches Secrets

### Workload Filtering

**`ignoreCronJobs`**
- Skips CronJobs, still handles Deployments/etc

**`ignoreJobs`**
- Skips Jobs, still handles other workloads

### Reload Triggers

**`reloadOnCreate`**
- `true`: triggers reload when a new ConfigMap/Secret is created
- `false` (default): only triggers on updates

**`reloadOnDelete`**
- `true`: triggers reload when a ConfigMap/Secret is deleted
- `false` (default): only triggers on updates

### Global Auto-Reload

**`autoReloadAll`**
- `true`: all workloads auto-reload without needing annotations
- `auto=false` on a workload still opts it out

---

## Annotation-Specific Tests

### Auto Reload Variations

- `reloader.stakater.com/auto: "true"` - watches both ConfigMaps and Secrets
- `reloader.stakater.com/auto: "false"` - completely disables reload
- `configmap.reloader.stakater.com/auto: "true"` - only watches ConfigMaps
- `secret.reloader.stakater.com/auto: "true"` - only watches Secrets

### Combining Annotations

- `auto=true` + explicit reload annotation work together
- Auto-detected resources + explicitly listed resources both trigger reload
- Exclude annotations override auto-detection

### Search & Match

The search/match system lets you decouple workloads from specific resource names:

1. Workload has `reloader.stakater.com/search: "true"`
2. ConfigMap has `reloader.stakater.com/match: "true"`
3. When ConfigMap changes, workload reloads

**Tests verify:**
- Reload happens when both annotations present
- No reload when workload has search but ConfigMap lacks match
- No reload when ConfigMap has match but no workload has search
- Multiple workloads can have search, only ones with search reload

### Exclude Annotations

Exclude specific resources from auto-reload:

- `configmap.reloader.stakater.com/exclude: "config-to-skip"`
- `secret.reloader.stakater.com/exclude: "secret-to-skip"`

**Tests verify:**
- Excluded ConfigMap changes don't trigger reload
- Non-excluded ConfigMap changes still trigger reload
- Same behavior for Secrets

### Resource Ignore

Put this on the ConfigMap/Secret itself to prevent any reload:

- `reloader.stakater.com/ignore: "true"`

**Tests verify:**
- ConfigMap with ignore annotation never triggers reload
- Secret with ignore annotation never triggers reload
- Even with explicit reload annotation on workload

### Pause Period

Delay between detecting change and triggering reload:

- `reloader.stakater.com/pause-period: "10s"`

**Tests verify:**
- Deployment gets paused-at annotation after reload
- Without pause-period, no paused-at annotation

---

## Advanced Scenarios

### Pod Template Annotations

Reloader reads annotations from both places:

1. Deployment/DaemonSet/etc metadata
2. Pod template metadata (inside spec.template.metadata)

**Tests verify:**
- Annotation only on pod template still works
- Annotation on both locations works
- Mismatched annotations (ConfigMap annotation but updating Secret) correctly doesn't reload

### Regex Patterns

Use regex in the reload annotation:

- `configmap.reloader.stakater.com/reload: "app-config-.*"`
- `secret.reloader.stakater.com/reload: "db-creds-.*"`

**Tests verify:**
- Matching ConfigMap/Secret triggers reload
- Non-matching ConfigMap/Secret doesn't trigger reload

### Multiple Containers

**Tests verify:**
- Multiple containers sharing one ConfigMap - changes trigger reload
- Multiple containers with different ConfigMaps - change to either triggers reload

---

## Test Organization

```
test/e2e/
├── core/                    # Main tests (all workload types)
│   ├── workloads_test.go    # Basic reload behavior
│   └── reference_methods_test.go  # envFrom, volumes, etc.
├── annotations/             # Annotation-specific behavior
│   ├── auto_reload_test.go
│   ├── combination_test.go
│   ├── exclude_test.go
│   ├── search_match_test.go
│   ├── pause_period_test.go
│   └── resource_ignore_test.go
├── flags/                   # CLI flag behavior
│   ├── namespace_selector_test.go
│   ├── namespace_ignore_test.go
│   ├── resource_selector_test.go
│   ├── ignore_resources_test.go
│   ├── ignored_workloads_test.go
│   ├── auto_reload_all_test.go
│   ├── reload_on_create_test.go
│   ├── reload_on_delete_test.go
│   └── watch_globally_test.go
├── advanced/                # Edge cases
│   ├── job_reload_test.go
│   ├── multi_container_test.go
│   ├── pod_annotations_test.go
│   └── regex_test.go
├── argo/                    # Argo Rollouts (requires installation)
│   └── rollout_test.go
├── openshift/               # OpenShift (requires cluster)
│   └── deploymentconfig_test.go
└── utils/                   # Shared test helpers
```

---

## Debugging Failed Tests

### See What's Happening

```bash
# Verbose output
go test -v ./test/e2e/core/...

# Run one specific test
go test -v ./test/e2e/core/... --ginkgo.focus="should reload when ConfigMap"

# Keep the cluster around after tests
SKIP_CLEANUP=true make e2e
```

### Check Reloader Logs

```bash
# Find the Reloader pod
kubectl get pods -A | grep reloader

# Check its logs
kubectl logs -n <namespace> -l app=reloader-reloader --tail=100
```

### Common Problems

| Problem | Solution |
|---------|----------|
| Test timeout | Reloader might not be running - check pod status |
| Argo tests skipped | Install Argo Rollouts first |
| OpenShift tests skipped | Only work on OpenShift clusters |
| "resource not found" | Missing CRDs (Argo, OpenShift) |

---

## Environment Variables

| Variable | What it does | Default |
|----------|--------------|---------|
| `RELOADER_IMAGE` | Image to test | `ghcr.io/stakater/reloader:test` |
| `SKIP_BUILD` | Don't build the image | `false` |
| `SKIP_CLEANUP` | Keep cluster after tests | `false` |
| `KIND_CLUSTER` | Kind cluster name | `kind` |
| `KUBECONFIG` | Kubernetes config path | `~/.kube/config` |

---

## Writing New Tests

### For Multiple Workload Types

Use the adapter pattern to test the same behavior across Deployments, DaemonSets, etc:

```go
DescribeTable("should reload when ConfigMap changes",
    func(workloadType utils.WorkloadType) {
        adapter := registry.Get(workloadType)
        // ... create ConfigMap, workload, update ConfigMap, verify reload
    },
    Entry("Deployment", utils.WorkloadDeployment),
    Entry("DaemonSet", utils.WorkloadDaemonSet),
    Entry("StatefulSet", utils.WorkloadStatefulSet),
)
```

### For Deployment-Only Tests

Use the direct creation helpers:

```go
It("should reload with my specific setup", func() {
    _, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
        map[string]string{"key": "value"}, nil)

    _, err = utils.CreateDeployment(ctx, kubeClient, testNamespace, deploymentName,
        utils.WithConfigMapEnvFrom(configMapName),
        utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
    )

    // Update and verify...
})
```

### Negative Tests (Verifying Nothing Happens)

```go
It("should NOT reload when only labels change", func() {
    // Setup...

    // Make a change that shouldn't trigger reload
    err = utils.UpdateConfigMapLabels(ctx, kubeClient, testNamespace, configMapName,
        map[string]string{"new-label": "value"})

    // Wait a bit, then verify NO reload happened
    time.Sleep(utils.NegativeTestWait)
    reloaded, _ := utils.WaitForDeploymentReloaded(...)
    Expect(reloaded).To(BeFalse())
})
```
