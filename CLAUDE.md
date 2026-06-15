# Stakater Reloader Project Memory

## Project Purpose

Reloader is a Kubernetes operator that automatically triggers rolling restarts of workloads when the ConfigMaps or Secrets they reference are updated. Without it, Kubernetes does not restart pods when configuration changes — operators must do it manually or rely on GitOps pipelines.

**What it watches**: ConfigMaps, Secrets, Namespaces, and (optionally) `SecretProviderClassPodStatus` (CSI-mounted secrets).

**Workload types it can reload**: Deployment, StatefulSet, DaemonSet, CronJob, Job, Argo Rollout, and OpenShift DeploymentConfig.

**How restarts are triggered**: Two strategies (selected via `--reload-strategy`):
1. **env-vars** (default) — injects an environment variable (`STAKATER_{NAME}_{TYPE}`) into every container with the SHA1 hash of the resource's data. A change in data changes the env var value, causing Kubernetes to restart pods.
2. **annotations** — writes the SHA1 hash into the pod template's annotations, which also forces a rollout.

**The core problem it solves**: ConfigMaps and Secrets are decoupled from pod lifecycle in Kubernetes. Applications reading config at startup see stale data after a config update unless pods are restarted. Reloader closes that gap automatically and selectively.

**Potential improvements observed**:
- **Duplicate reload suppression**: If a workload references both a ConfigMap and a Secret that are updated in the same controller reconcile cycle, it may get reloaded twice. Could be solved with a per-workload debounce map keyed by namespace/name/resourceVersion, flushed after a short TTL.
- **CronJob/Job reload is destructive**: Jobs are deleted and recreated on change, which loses run history. Could instead only annotate the CronJob template without spawning a new Job.
- **No per-resource reload rate limiting**: A rapid-fire ConfigMap update (e.g., from a CI pipeline) can trigger many restarts. A cooldown window per resource would help.
- **CSI integration gap**: CSI volumes are watched at the `SecretProviderClassPodStatus` level, but the link back to the workload is indirect and may miss edge cases. Needs a direct map from SecretProviderClass → workloads that mount it.

---

## Repo Map

| Path | Owns | Inspect when |
|---|---|---|
| `main.go` | Entry point, delegates to `app.Run()` | Never needs changes |
| `internal/pkg/app/` | `Run()` bootstrap, Cobra command wiring | Startup sequence changes |
| `internal/pkg/cmd/` | CLI flags parsing, `startReloader()`, controller/HA wiring | Adding new flags or startup behavior |
| `internal/pkg/controller/` | Informer/queue per resource type, event handlers (Add/Update/Delete) | Watching new resource types, queue tuning |
| `internal/pkg/handler/` | Per-event handlers (create, update, delete), `doRollingUpgrade()`, pause deployment | Core reload logic changes |
| `internal/pkg/callbacks/` | Workload-specific get/list/update/patch functions, `RollingUpgradeFuncs` struct | Adding new workload types |
| `internal/pkg/options/` | All CLI flag variables, defaults, `ArgoRolloutStrategy` type | Adding or renaming flags |
| `internal/pkg/constants/` | Constants: env var postfixes, annotation prefix, strategy names, HA lock name | Renaming global identifiers |
| `internal/pkg/metrics/` | Prometheus `Collectors` struct, all metric registration and recording helpers | Adding metrics |
| `internal/pkg/alerts/` | Slack/Teams/GChat/raw webhook alerting, env var config | Alert sink changes |
| `internal/pkg/util/` | SHA generation via `crypto/sha.go`, env var name conversion, namespace/label utilities | Utility/hash changes |
| `internal/pkg/crypto/` | `GenerateSHA(data)` — SHA1 hex digest | Hash algorithm changes |
| `internal/pkg/leadership/` | Leader election via Kubernetes Lease, HA stop/start of controllers | HA behavior changes |
| `internal/pkg/testutil/` | Fake Kubernetes objects for unit tests | Writing new tests |
| `pkg/common/` | `ReloadCheckResult`, `ReloaderOptions`, `ShouldReload()` logic, `Config` struct | Reload decision logic, annotation precedence |
| `pkg/kube/` | `Clients` struct (k8s + OpenShift + Argo + CSI), `GetKubernetesClient()`, `ResourceMap` | Client initialization, new CRD clients |
| `deployments/` | Helm chart (`deployments/kubernetes/chart/reloader/`), Kustomize manifests | Helm values, RBAC, deployment config |
| `docs/` | User-facing annotation documentation, architecture notes | Writing docs or confirming annotation behavior |
| `scripts/` | Shell scripts used by CI and Makefile | Build/release pipeline |
| `test/loadtest/` | Load test CLI (`cmd/loadtest`), 13 scenarios (S1–S13), Kind cluster setup | Performance testing, regression benchmarks |
| `.github/` | CI workflows: lint, test, Kind e2e, multi-arch Docker build, release | CI changes |

---

## Core Runtime Flow

**1. Entry** — `main.go:10` calls `app.Run()`.

**2. CLI Init** — `internal/pkg/app/app.go` calls `cmd.NewReloaderCommand()` which registers all Cobra flags from `options/flags.go` and runs `startReloader()`.

**3. Client Setup** — `pkg/kube/client.go`: builds `kube.Clients` with:
- `kubernetes.Interface` — standard k8s client
- `appsclient.Interface` — OpenShift client (auto-detected by probing `deploymentconfigs`)
- `argorollout.Interface` — if `--is-Argo-Rollouts=true`
- `csiclient.Interface` — if `--enable-csi-integration`

**4. Controller Creation** — `startReloader()` iterates `kube.ResourceMap` (configmaps, secrets, namespaces, and optionally secretproviderclasspodstatuses) and calls `controller.NewController()` for each resource in each watched namespace.

**5. Informer/Queue** — `controller.NewController()`:
- Creates a `cache.NewFilteredListWatchFromClient` with label/field selectors.
- Registers `Add`, `Update`, `Delete` event handlers.
- Creates a `workqueue.TypedRateLimitingQueue` for async processing.

**6. Event Detection**:
- `Add` — enqueues only if `ReloadOnCreate` is enabled (skips during initial sync unless `SyncAfterRestart`).
- `Update` — compares SHA of old vs new object data; enqueues only on real changes.
- `Delete` — enqueues only if `ReloadOnDelete` is enabled.
- Namespace events update `selectedNamespacesCache` for namespace-selector filtering.

**7. Handler Dispatch** — The queue worker calls `handler.Handle()` on the dequeued item. Three handler types:
- `ResourceCreatedHandler` (`create.go`) — fires `doRollingUpgrade` or sends webhook.
- `ResourceUpdatedHandler` (`update.go`) — fires `doRollingUpgrade` or sends webhook.
- `ResourceDeleteHandler` (`delete.go`) — calls `invokeDeleteStrategy` (removes env vars or clears annotation).

**8. Workload Discovery** — `doRollingUpgrade()` (`upgrade.go:181`) calls `rollingUpgrade()` for each workload type. For each type, `ItemsFunc` lists all workloads in the namespace, then `pkg/common.ShouldReload()` checks annotations to decide which ones need reloading.

**9. Reload Execution** — `invokeReloadStrategy()` either:
- **env-vars**: mutates container env vars; uses JSON patch if `SupportsPatch=true`, full update otherwise.
- **annotations**: writes SHA to pod template annotations; same patch/update split.

**10. Post-reload** — optionally pauses the Deployment via `pause_deployment.go`, records Kubernetes Events via `recorder`, updates Prometheus metrics, sends alert webhooks.

**HA Mode**: if `--enable-ha`, `internal/pkg/leadership/` runs Kubernetes Lease-based leader election. Only the leader runs controllers; losing leadership stops them and marks the pod unhealthy.

**HTTP Server**: port `:9090` serves `/metrics` (Prometheus) and liveness/readiness probes.

---

## Reload Behavior And Annotations

All annotation names are configurable via CLI flags; the values below are defaults.

### Trigger Annotations (on workloads)

| Annotation | Value | Behavior |
|---|---|---|
| `reloader.stakater.com/auto` | `"true"` | Reload on change to **any** ConfigMap or Secret referenced by the workload (via envFrom, env valueFrom, or volumes) |
| `configmap.reloader.stakater.com/auto` | `"true"` | Reload on change to **any referenced ConfigMap** only |
| `secret.reloader.stakater.com/auto` | `"true"` | Reload on change to **any referenced Secret** only |
| `secretproviderclass.reloader.stakater.com/auto` | `"true"` | Reload on change to **any referenced SecretProviderClass** only |
| `configmap.reloader.stakater.com/reload` | `"cm1,cm2"` | Reload only when the **named ConfigMaps** change (regex supported) |
| `secret.reloader.stakater.com/reload` | `"sec1,sec2"` | Reload only when the **named Secrets** change (regex supported) |
| `secretproviderclass.reloader.stakater.com/reload` | `"spc1"` | Reload only when the **named SecretProviderClass** changes |
| `reloader.stakater.com/search` | `"true"` | Reload when any ConfigMap/Secret tagged with `reloader.stakater.com/match: "true"` changes |

### Exclude Annotations (on workloads)

| Annotation | Value | Behavior |
|---|---|---|
| `reloader.stakater.com/ignore` | `"true"` | Skip this workload entirely |
| `configmaps.exclude.reloader.stakater.com/reload` | `"cm1,cm2"` | Exclude these named ConfigMaps from triggering reload |
| `secrets.exclude.reloader.stakater.com/reload` | `"sec1,sec2"` | Exclude these named Secrets |
| `secretproviderclasses.exclude.reloader.stakater.com/reload` | `"spc1"` | Exclude these named SecretProviderClasses |

### Behavior Annotations (on workloads)

| Annotation | Value | Behavior |
|---|---|---|
| `reloader.stakater.com/rollout-strategy` | `"restart"` or `"rollout"` | For Argo Rollouts: `"restart"` uses restartAt, `"rollout"` (default) uses full rollout update |
| `deployment.reloader.stakater.com/pause-period` | Go duration e.g. `"30s"` | Pause Deployment for this duration after reload |
| `deployment.reloader.stakater.com/paused-at` | RFC3339 timestamp | Set by Reloader to track pause start time; do not set manually |

### Search/Match Pattern

The `reloader.stakater.com/search` annotation on a workload pairs with `reloader.stakater.com/match: "true"` on a ConfigMap or Secret. Any workload with `search: true` will reload when any `match: true` resource changes.

### Global Flag Overrides

- `--auto-reload-all` — reload all workloads on any ConfigMap/Secret change; annotation not required.
- `--resources-to-ignore=configMaps` or `=secrets` — skip one type entirely.
- `--ignored-workload-types=jobs,cronjobs` — skip Job and CronJob reload.
- `--namespaces-to-ignore` — comma-separated namespace names to skip.
- `--namespace-selector` — only watch namespaces with matching labels.
- `--resource-label-selector` — only watch ConfigMaps/Secrets with matching labels.

### Precedence Rules

1. `reloader.stakater.com/ignore: "true"` wins everything — workload is skipped.
2. Exclude annotations override include annotations for specific named resources.
3. Named annotations (`.../reload`) are checked before auto annotations.
4. `--auto-reload-all` is the lowest-priority fallback (only applies if no annotation matches).
5. Annotations are checked on both the workload and its pod template (pod template takes precedence in some paths — verify in `pkg/common/common.go:ShouldReload()`).

---

## Workload Support

| Workload | SupportsPatch | Update Mechanism | Key files |
|---|---|---|---|
| **Deployment** | Yes | JSON patch or full update | `callbacks/rolling_upgrade.go`, `handler/upgrade.go:38` |
| **StatefulSet** | Yes | JSON patch or full update | `callbacks/rolling_upgrade.go`, `handler/upgrade.go:109` |
| **DaemonSet** | Yes | JSON patch or full update | `callbacks/rolling_upgrade.go`, `handler/upgrade.go:91` |
| **CronJob** | No | Creates a new Job from CronJob spec (adds `cronjob.kubernetes.io/instantiate: manual`) | `callbacks.CreateJobFromCronjob`, `handler/upgrade.go:55` |
| **Job** | No | Deletes old Job, creates new one (strips ResourceVersion, UID, Status, controller labels) | `callbacks.ReCreateJobFromjob`, `handler/upgrade.go:73` |
| **Argo Rollout** | No | Full update via Argo Rollouts client | `callbacks.UpdateRollout`, `handler/upgrade.go:127`; requires `--is-Argo-Rollouts=true` |
| **DeploymentConfig** | Yes | OpenShift DeploymentConfigs API | `callbacks/rolling_upgrade.go`; auto-detected by probing `deploymentconfigs` |

**Reload flow per workload**: `doRollingUpgrade()` → `rollingUpgrade()` per type → `ItemsFunc` lists workloads → `ShouldReload()` filters → `invokeReloadStrategy()` patches or updates → optional pause + metrics + alert.

---

## CSI Support

**Enabled by**: `--enable-csi-integration`

**What is watched**: `SecretProviderClassPodStatus` resources (from `sigs.k8s.io/secrets-store-csi-driver`). Resource name constant: `constants.SecretProviderClassController = "secretproviderclasspodstatuses"`.

**How it works**:
1. The CSI driver injects secrets into pods as volume mounts and tracks injection state via `SecretProviderClassPodStatus` objects.
2. Reloader watches these objects for version changes.
3. When a version change is detected, it computes a SHA of the object's IDs and versions.
4. It then looks up the referenced `SecretProviderClass` and treats the event like a Secret update, triggering workload reloads.

**Workload annotation**: `secretproviderclass.reloader.stakater.com/reload: "my-spc"` or `secretproviderclass.reloader.stakater.com/auto: "true"`.

**Required**: CSI CRDs must be installed in the cluster. Reloader auto-detects their presence at startup.

**Env var postfix**: `STAKATER_{NAME}_SECRETPROVIDERCLASS`.

**Known limitations**:
- Only works for secrets mounted as volumes via CSI, not env-var-based CSI injection.
- The link from `SecretProviderClassPodStatus` → workload is indirect; edge cases may be missed.
- Requires the CSI driver CRDs to be pre-installed; Reloader won't start CSI controller if CRDs are absent.

---

## Build, Test, And Run Commands

**Go version**: `go 1.26.2` (from `go.mod`)

| Purpose | Command |
|---|---|
| Run locally | `go run ./main.go` |
| Build binary | `make build` → `go build -o Reloader` |
| Unit tests | `make test` → `go test -timeout 1800s -v ./...` |
| Lint | `make lint` → `golangci-lint run ./...` (v2.6.1) |
| Docker build (single arch) | `make build-image ARCH=amd64` |
| Docker push | `make push` |
| Full release (build+push+manifest) | `make release ARCH=amd64` |
| Multi-arch release | `make release-all` |
| Generate k8s manifests | `make k8s-manifests` (Kustomize v5.3.0) |
| Load test (quick) | `make loadtest-quick LOADTEST_OLD_IMAGE=... LOADTEST_NEW_IMAGE=...` (runs S1, S4, S6) |
| Load test (full) | `make loadtest-full LOADTEST_OLD_IMAGE=... LOADTEST_NEW_IMAGE=...` |
| Load test (custom) | `make loadtest LOADTEST_SCENARIOS=S1,S3 LOADTEST_DURATION=120` |

**Docker image**: `ghcr.io/stakater/reloader` — multi-arch (amd64, arm64, arm), distroless nonroot base.

**Helm chart**: `deployments/kubernetes/chart/reloader/` — install via Helm or `kubectl apply -f deployments/kubernetes/reloader.yaml`.

---

## Coding Conventions

**Package boundaries**: Each `internal/pkg/<name>` package has a single clear responsibility. Cross-package access goes through exported types/functions only.

**Error handling**: `logrus.Errorf(...)` for non-fatal, `logrus.Fatalf(...)` for startup failures. Errors are returned up the call stack and logged at the point of action, not at every layer. Retry uses `k8s.io/client-go/util/retry.RetryOnConflict`.

**Logging**: `logrus` with structured fields. Format controlled by `--log-format=json` flag. Log level controlled by `--log-level`. Messages follow the pattern: `"Changes detected in '%s' of type '%s' in namespace '%s'"`.

**Kubernetes client patterns**: All k8s operations go through the `kube.Clients` struct. Use `context.TODO()` for context (no request-scoped contexts). List/watch via informers, not polling.

**Callback pattern**: Workload-specific logic is encapsulated in `callbacks.RollingUpgradeFuncs` structs returned by `handler.Get*RollingUpgradeFuncs()`. Adding a new workload type = add a new `RollingUpgradeFuncs` factory function and call it in `doRollingUpgrade()`.

**Test style**: Standard `testing.T`, `testify/assert`. Fake k8s objects via `testutil/kube.go`. Tests live alongside source in the same package. Large integration-style tests in `handler/upgrade_test.go`.

**Naming patterns**:
- Annotation variables: `XxxUpdateOnChangeAnnotation`, `XxxReloaderAutoAnnotation`
- Callback funcs: `GetXxxItem`, `GetXxxItems`, `UpdateXxx`, `PatchXxx`
- Handler factories: `GetXxxRollingUpgradeFuncs()`

**Adding new behavior**: Add flag to `options/flags.go` + `common.ReloaderOptions` struct → wire in `cmd/reloader.go` → implement logic in `handler/` or `callbacks/` → add metrics recording → write tests in `*_test.go`.

---

## Gotchas And Risks

**Duplicate reloads**: If a workload references multiple ConfigMaps/Secrets and all change simultaneously, each change event fires a separate reload. No deduplication exists within a reconcile window. This can cause unnecessary rolling restarts.

**Controller init guard**: `secretControllerInitialized` and `configmapControllerInitialized` booleans in `controller/controller.go` prevent processing Add events during the initial list/sync (to avoid reloading everything on startup). If `--sync-after-restart` is set, both are pre-set to `true`, bypassing the guard. Be careful when this interacts with `--reload-on-create`.

**Namespace filtering**: `--namespaces-to-ignore` does a name match; `--namespace-selector` watches namespaces by label and caches them in `selectedNamespacesCache`. The cache is updated on Namespace Add/Update/Delete events. A race between cache population and first ConfigMap event could cause missed reloads on startup in label-selected deployments.

**RBAC**: Reloader requires get/list/watch on secrets and configmaps, and get/list/watch/update/patch on all workload types it manages. Missing RBAC silently causes no reloads (not an error — just empty lists). Check ClusterRole in `deployments/kubernetes/chart/reloader/templates/`.

**GitOps drift**: If a GitOps tool (Flux, ArgoCD) manages the same Deployments, annotation or env var changes made by Reloader will be detected as drift and reverted. Use `--reload-strategy=annotations` with care in GitOps setups; `env-vars` strategy is generally safer since it modifies the pod template rather than workload-level annotations.

**Annotation precedence edge case**: Annotations are checked first on the workload object, then on the pod template. If both are set to conflicting values, the behavior depends on which path `ShouldReload()` hits first. Verify in `pkg/common/common.go`.

**CronJob/Job destructive reload**: Job recreation deletes the old Job. Any in-flight pod from that Job will be terminated. This is intentional but surprising. There is no protection for long-running jobs.

**OpenShift DeploymentConfig**: Auto-detected by probing for the `deploymentconfigs` resource. If the probe fails at startup, OpenShift support is silently disabled. Check `pkg/kube/client.go`.

**Argo Rollouts**: Must be explicitly enabled via `--is-Argo-Rollouts=true`. Without it, Rollout objects are never listed. The `SupportsPatch=false` means full object updates are used — be aware of potential conflicts with Argo's own controller.

**CSI rotation behavior**: `SecretProviderClassPodStatus` is updated by the CSI driver when secrets rotate. Reloader reacts to those updates. However, if the CSI driver updates the status in a way that doesn't change the versions Reloader tracks, the reload will be missed.

**Backward compatibility**: Annotation names are configurable, so changing defaults would break existing clusters. Never change default annotation values without a migration path.

**Tests to update for risky changes**: `handler/upgrade_test.go` (large suite covering all workload types), `controller/controller_test.go` (event handling), `pkg/common/common_test.go` (reload decision logic).

---

## Open Questions

- **Exact `ShouldReload()` precedence**: The code in `pkg/common/common.go` checks annotations in a specific order. The exact tie-breaking when both workload-level and pod-template-level annotations are set should be verified by reading that function fully before making annotation behavior changes.
- **CSI → workload mapping**: How exactly does Reloader map a `SecretProviderClassPodStatus` change back to workloads? Is it via the SecretProviderClass name matching an annotation on the workload, or via volume reference scanning? Needs confirmation before adding CSI-related features.
- **`ContainerPatchPathFunc` field**: `RollingUpgradeFuncs` has a `ContainerPatchPathFunc` field, but it is not documented — unclear if/how it differs from `ContainersFunc` in patch scenarios.
- **Webhook vs alert**: `--webhook-url` replaces reloading with a POST request. `ALERT_WEBHOOK_URL` env var sends an alert *after* reloading. These are two different mechanisms; the naming is confusing and easy to conflate.
- **Load test scenarios S7–S13**: Only S1, S4, and S6 are confirmed from CI. The behavior and coverage of the remaining scenarios is unknown without reading `test/loadtest/` in full.
- **`SyncAfterRestart` semantics**: Flag docs say it "syncs add events after restart" but only if `ReloadOnCreate` is also true. The interaction between these two flags in HA mode (where controllers restart on leader change) needs verification.

---

## Important Files

| File | Description |
|---|---|
| `internal/pkg/cmd/reloader.go` | `startReloader()` — main wiring of clients, controllers, HA, and HTTP server |
| `internal/pkg/handler/upgrade.go` | `doRollingUpgrade()` + all `Get*RollingUpgradeFuncs()` factories |
| `internal/pkg/callbacks/rolling_upgrade.go` | All workload-specific get/update/patch implementations |
| `pkg/common/common.go` | `ShouldReload()` — the annotation decision tree |
| `internal/pkg/options/flags.go` | Every configurable option with defaults |
| `internal/pkg/controller/controller.go` | Informer setup, queue, event handlers |
| `pkg/kube/client.go` | Multi-client initialization and OpenShift/CSI detection |
| `internal/pkg/handler/pause_deployment.go` | Pause/resume deployment logic with timers |
| `internal/pkg/leadership/leadership.go` | HA leader election |
| `internal/pkg/metrics/prometheus.go` | All Prometheus collector definitions |
| `internal/pkg/alerts/alert.go` | Slack/Teams/GChat alerting |
| `internal/pkg/constants/constants.go` | Global constants (env var prefixes, annotation prefix, strategy names) |
| `deployments/kubernetes/chart/reloader/values.yaml` | Helm chart defaults — source of truth for production config |
| `handler/upgrade_test.go` | Largest test suite; must be updated for any reload logic change |
| `Makefile` | All build/test/release/loadtest commands |
