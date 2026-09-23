# Stakater Reloader Project Memory

**This file documents the `v2` branch**, a ground up controller-runtime rewrite. The `master` branch
(Reloader v1.x) has a completely different layout: `internal/pkg/handler/`, `internal/pkg/callbacks/`,
`pkg/common.ShouldReload()`, a root `main.go`. None of that exists here. Confirm the checked out branch
before trusting any architecture note.

Cross repo context (how OSS Reloader, the Enterprise Gateway and the Enterprise Console fit together)
lives in `~/Documents/work/Reloader Workspace/CLAUDE.md`.

---

## Project Purpose

Reloader is a Kubernetes operator that triggers rolling restarts of workloads when the ConfigMaps or
Secrets they reference change. Kubernetes does not restart pods on config change, so applications that
read config at startup keep serving stale data until something restarts them. Reloader closes that gap
selectively, driven by annotations.

**Watches**: ConfigMaps, Secrets, Namespaces (only when a namespace selector is set), Deployments (for
pause expiry), and optionally `SecretProviderClassPodStatus` (CSI mounted secrets).

**Reloads**: Deployment, StatefulSet, DaemonSet, CronJob, Job, Argo Rollout, OpenShift DeploymentConfig.

**Two reload strategies** (`--reload-strategy`):

1. **env-vars** (default) sets `STAKATER_{NAME}_{TYPE}` on the container with the SHA1 of the resource
   data. A data change changes the value, so the pod template changes and Kubernetes rolls the pods.
2. **annotations** writes the same SHA1 into the pod template annotations.

env-vars is the GitOps friendlier default because the mutation lands inside the pod template rather than
on workload level annotations.

---

## Repo Map

| Path | Owns | Inspect when |
|---|---|---|
| `cmd/reloader/main.go` | Entry point: cobra command, logging, capability detection, manager start | Startup sequence changes |
| `internal/pkg/config/flags/` | pflag + viper CLI layer. Deliberately internal so `pkg/config` does not drag viper into consumers | Adding or renaming flags |
| `internal/pkg/controller/` | `manager.go` plus one reconciler per watched kind, generic `ResourceReconciler[T]`, predicates wiring, retry helpers | Watching new kinds, reconcile behaviour |
| `internal/pkg/reload/` | `service`, `decision`, `strategy`, `hasher`, `change`, `predicate`, `pause` | Core reload logic |
| `internal/pkg/workload/` | Per workload type adapters, `Registry`, `Lister`, `uses.go` reference scanning | Adding a workload type |
| `internal/pkg/alerting/` | Slack, Teams, Google Chat and raw webhook sinks | Alert sink changes |
| `internal/pkg/events/` | Kubernetes Event recorder | Event reason or message changes |
| `internal/pkg/metrics/` | Prometheus collectors and recording helpers | Adding metrics |
| `internal/pkg/webhook/` | Webhook mode client, replaces reloading with an HTTP POST | Webhook payload changes |
| `internal/pkg/openshift/`, `internal/pkg/csi/` | Cluster capability detection via discovery | Capability probing |
| `internal/pkg/http/` | Shared HTTP client | Timeout or proxy handling |
| `internal/pkg/testutil/` | Fixtures for unit tests | Writing unit tests |
| `pkg/config/` | **Public.** `Config`, `AnnotationConfig`, defaults, validation | Config shape changes, breaking for the gateway |
| `pkg/matcher/` | **Public.** `Matcher.ShouldReload()`, the annotation decision tree | Reload decision changes |
| `pkg/metadata/` | **Public.** `reloader-meta-info` ConfigMap model and publisher runnable | Meta info contract changes |
| `deployments/kubernetes/chart/reloader/` | Helm chart, depends on the `reloader-enterprise` OCI subchart aliased `enterprise` | Chart values, RBAC, enterprise packaging |
| `test/e2e/` | Ginkgo suites by area: core, annotations, argo, csi, flags, advanced | Behaviour changes |
| `test/loadtest/` | Load test CLI, scenarios S1 to S13 | Performance regressions |
| `.github/workflows/` | CI: lint, test, Kind e2e, multi arch build, release, enterprise dispatch | CI changes |

---

## Public API Contract

`pkg/` is a published API, not an internal detail. `reloader-enterprise-gateway` imports
`pkg/{config,matcher,metadata}` by pseudo version so it can reuse the reload decision instead of
reimplementing annotation matching. Anything under `internal/` is explicitly not part of the contract.

Three consequences:

1. Renaming or moving anything under `pkg/` is a breaking change for a private downstream repo that CI
   here will not catch.
2. A local `pkg/` change is invisible to the gateway until a tag or pseudo version bump, or a temporary
   `replace` directive.
3. `pkg/config.Config` is serialized verbatim into the `reloader-meta-info` ConfigMap `config` key, so a
   JSON tag change is a wire format change. The gateway owns the deserialization; v2 exposes no parser.
   A missing `config` key makes the gateway fail open and report every referenced workload as reload
   eligible.

`pkg/metadata` publishes `reloader-meta-info` in Reloader's own namespace with label
`reloader.stakater.com/meta-info: reloader` and keys `buildInfo`, `config`, `deploymentInfo`. The
publisher builds its own uncached client because that namespace is outside the manager cache in scoped
mode (`cmd/reloader/main.go:158`). Failure to add it is logged and non fatal.

`Version`, `Commit`, `BuildDate` and `Edition` in `pkg/metadata` are injected with `-X` ldflags from the
Makefile. `EDITION=enterprise` is set by the enterprise image pipeline.

---

## Core Runtime Flow

**1. Command** (`cmd/reloader/main.go:44`) builds `config.NewDefault()`, binds flags via
`flags.BindFlags`, and runs `run()`.

**2. Logging first** (`main.go:60`). zerolog behind logr, so `flags.ApplyFlags` can surface warnings
through a ready logger. `--log-format=json` switches from console output to JSON.

**3. Config** `flags.ApplyFlags` then `cfg.Validate()`. HA additionally requires `POD_NAME` and
`POD_NAMESPACE`; `POD_NAME` becomes the leader identity.

**4. Capability detection** (`main.go:114`) builds a discovery client, then:

- OpenShift: `openshift.HasDeploymentConfigSupport` unless `--is-openshift` forces true or false.
- CSI: `--enable-csi-integration` **and** `csi.HasCSISupport`. The flag alone is not enough; a missing
  CRD disables the integration with a log line, because controller-runtime would otherwise crash trying
  to watch an absent CRD.

Detected capabilities feed `controller.AddOptionalSchemes`, which registers the Argo, OpenShift and CSI
schemes only when needed.

**5. Manager** (`controller.NewManager`) is a standard controller-runtime manager. Metrics on
`--metrics-addr` (default `:9090`), health on `--health-addr` (default `:8080`). `--namespaces` maps to
`cache.Options.DefaultNamespaces`, so scoped mode is enforced by the cache, not by per event filtering.
Leader election is controller-runtime's, lease name `reloader-leader-election`.

**6. Reconcilers** (`controller.SetupReconcilers`) wires one shared `reload.Service`, `events.Recorder`,
`reload.PauseHandler`, `alerting.Alerter`, optional `webhook.Client` and `workload.Registry`, then
registers:

- `NamespaceReconciler`, only when `--namespace-selector` is set, maintaining a shared `NamespaceCache`.
- `ConfigMapReconciler` unless configmaps are in `--resources-to-ignore`.
- `SecretReconciler` unless secrets are ignored.
- `SecretProviderClassReconciler` when CSI is enabled.
- `DeploymentReconciler`, always, purely to expire pauses.

ConfigMap and Secret reconcilers are both `ResourceReconciler[T]`, one generic implementation
parameterized by a `ResourceConfig[T]`.

**7. Event filtering** happens in predicates, not in the handler. `reload.ConfigMapPredicates` and
`SecretPredicates` compare the content hash of old and new so a no op update never enqueues.
`controller.BuildEventFilter` layers on namespace filtering, label selectors, the ignore annotation, and
a create predicate that uses the controller start time to tell a genuine post startup create from the
initial sync replay of pre existing objects.

**8. Reconcile** (`ResourceReconciler.Reconcile`) resolves the change, then `ReloadHandler.Process`
lists candidate workloads in the namespace through `workload.Lister`, and calls `reload.Service.Process`.

**9. Decision** `reload.Service` delegates to `matcher.Matcher.ShouldReload` per workload and returns
`[]ReloadDecision`. In webhook mode (`--webhook-url`) the handler POSTs and stops here, nothing is
reloaded.

**10. Apply** `Service.ApplyReload` finds the target container (by volume mount, then by env reference),
applies the strategy, and sets the `reloader.stakater.com/last-reloaded-from` attribution annotation.
`controller.UpdateWorkloadWithRetry` then persists it, re fetching and re applying on conflict.

**11. Update mechanism** depends on `Workload.UpdateStrategy()`:

- `UpdateStrategyPatch` for Deployment, StatefulSet, DaemonSet, Rollout, DeploymentConfig.
- `UpdateStrategyRecreate` for Job: delete then recreate.
- `UpdateStrategyCreateNew` for CronJob: create a fresh Job from the CronJob template.

**12. Post reload** optional Deployment pause, Kubernetes Event, Prometheus counters, alert webhook.

---

## Reload Behavior And Annotations

Annotation keys are configurable via flags; the values below are the defaults from
`config.DefaultAnnotations()` (`pkg/config/config.go:146`).

### Trigger Annotations (on workloads)

| Annotation | Value | Behavior |
|---|---|---|
| `reloader.stakater.com/auto` | `"true"` | Reload on change to any referenced ConfigMap or Secret |
| `configmap.reloader.stakater.com/auto` | `"true"` | Referenced ConfigMaps only |
| `secret.reloader.stakater.com/auto` | `"true"` | Referenced Secrets only |
| `secretproviderclass.reloader.stakater.com/auto` | `"true"` | Referenced SecretProviderClasses only |
| `configmap.reloader.stakater.com/reload` | `"cm1,cm2"` | Only the named ConfigMaps, each entry treated as a regex anchored with `^...$` |
| `secret.reloader.stakater.com/reload` | `"sec1,sec2"` | Only the named Secrets, same regex handling |
| `secretproviderclass.reloader.stakater.com/reload` | `"spc1"` | Only the named SecretProviderClasses |
| `reloader.stakater.com/search` | `"true"` | Reload when any resource carrying `reloader.stakater.com/match: "true"` changes |

An entry in a `.../reload` list that fails to compile as a regex falls back to an exact name comparison
and is reported through `MatchResult.Errors`. Every entry is evaluated even after one matches, so a typo
is reported wherever it sits in the list. `Service.reportAnnotationError` deduplicates by error text so
one bad annotation templated across many workloads logs once.

### Exclude And Ignore

| Annotation | Value | Behavior |
|---|---|---|
| `reloader.stakater.com/ignore` | `"true"` | On the **resource**, skip it entirely. Checked first, before annotation selection |
| `configmaps.exclude.reloader.stakater.com/reload` | `"cm1,cm2"` | Exact names only, no regex |
| `secrets.exclude.reloader.stakater.com/reload` | `"sec1,sec2"` | Exact names only |
| `secretproviderclasses.exclude.reloader.stakater.com/reload` | `"spc1"` | Exact names only |

### Behavior Annotations

| Annotation | Value | Behavior |
|---|---|---|
| `reloader.stakater.com/rollout-strategy` | `"restart"` or `"rollout"` | Argo Rollouts only. `restart` uses `restartAt`, `rollout` (default) does a full update |
| `deployment.reloader.stakater.com/pause-period` | Go duration, e.g. `"30s"` | Pause the Deployment for this long after reload. Only positive durations are accepted |
| `deployment.reloader.stakater.com/paused-at` | RFC3339 | Written by Reloader, do not set by hand |
| `reloader.stakater.com/last-reloaded-from` | JSON | Written by Reloader: kind, name, namespace, hash, containers, reloadedAt |

### Decision Order

`matcher.Matcher.ShouldReload` (`pkg/matcher/matcher.go:47`) evaluates in this order:

1. Resource carries the ignore annotation, stop, no reload.
2. Pick the annotation source: workload annotations if they carry any relevant key for this resource
   type, else pod template annotations, else workload annotations. **Whole maps are selected, never
   merged**, so a workload with one relevant annotation shadows the pod template entirely.
3. Resource is in the exclude list, stop.
4. Explicit `.../reload` regex match.
5. `search` on the workload paired with `match: "true"` on the resource.
6. `auto` or the type specific auto annotation set to `"true"`.
7. `--auto-reload-all`, which an explicit `auto: "false"` on the workload still vetoes.

### Global Flag Overrides

`--auto-reload-all`, `--resources-to-ignore`, `--ignored-workload-types`, `--namespaces`,
`--namespaces-to-ignore`, `--namespace-selector`, `--resource-label-selector`.

---

## Workload Support

| Workload | Update Strategy | Notes |
|---|---|---|
| Deployment | Patch | Extra pause handling in `updateDeploymentWithPause` |
| StatefulSet | Patch | |
| DaemonSet | Patch | |
| DeploymentConfig | Patch | OpenShift, auto detected |
| Argo Rollout | Patch | Needs `--is-Argo-Rollouts=true`, honours the rollout-strategy annotation |
| Job | Recreate | Deletes the old Job first, so any in flight pod is terminated |
| CronJob | CreateNew | Creates a Job from the CronJob template rather than touching the CronJob |

Adding a workload type means a new file in `internal/pkg/workload/` embedding `BaseWorkload[T]`, a lister
in `lister.go`, and registration in `registry.go`. `BaseWorkload` supplies the default patch strategy;
override `UpdateStrategy()` and `PerformSpecialUpdate()` only for the Job and CronJob shaped cases.

---

## CSI Support

Enabled by `--enable-csi-integration` **and** the presence of the secrets-store CSI driver CRDs.

`SecretProviderClassPodStatusPredicates` ignores create and delete and passes an update only when the
hashed status changes. The hash is the sorted set of object `ID=Version` entries plus the
SecretProviderClass name (`reload.Hasher.HashSecretProviderClass`). The reconciler resolves the
`SecretProviderClass` behind the status and builds a `SecretProviderClassChange`, which then flows
through the same decision and strategy path as a Secret. Env var postfix is
`STAKATER_{NAME}_SECRETPROVIDERCLASS`.

Limits: volume mounted CSI secrets only, not env var injection. If the CSI driver updates the status
without changing the tracked IDs or versions, the reload is missed.

---

## Helm Chart

`deployments/kubernetes/chart/reloader/`, chart name `reloader`.

- `image.tag` defaults to `Chart.yaml` `appVersion`, so a release only bumps `appVersion`.
- `enterprise.enabled` (default `false`) pulls in the `reloader-enterprise` chart from
  `oci://ghcr.io/stakater/public/charts` under alias `enterprise`, which brings the console, the gateway
  and Dragonfly. The gateway defaults to `tier: free`.
- Gateway licensing values sit under `gateway:` inside the enterprise chart, so from here the keys are
  `enterprise.gateway.tier` and `enterprise.gateway.namespaces`. `enterprise.tier` looks right and is
  silently ignored.
- Enterprise mode also expects the operator image swapped to the enterprise image plus
  `global.imagePullSecrets`.
- `global.host` plus `global.gatewayBasePath` (default `/gateway`) are the single hostname for both
  enterprise components; console at `/`, gateway at the base path.
- If `global.imageRegistry` is set, the operator image resolves from `global.imageRegistry` plus
  `image.name` and `image.repository` is ignored. Override `image.name` in that mode.
- Chart unit tests live in `chart/reloader/tests/` and run with helm unittest.

---

## Build, Test, And Run

Go version `1.26.8` (`go.mod`). Tooling is pinned as Go tool dependencies, hence `go tool <name>`.

| Purpose | Command |
|---|---|
| Run locally | `make run` |
| Build | `make build` (injects version ldflags) |
| Unit tests | `make test` (`./internal/...` and `./test/e2e/utils/...`) |
| Single unit test | `go test ./internal/pkg/reload/ -run TestShouldReload -v` |
| Lint | `make lint` (`go tool golangci-lint`) |
| Format | `make fmt` (`goimports -local github.com/stakater/Reloader` then gofmt) |
| e2e cluster setup | `make e2e-setup` (Kind plus Argo, CSI, Vault) |
| e2e | `make e2e`, or `SKIP_BUILD=true make e2e` to reuse an image |
| Single e2e suite | `go tool ginkgo -v ./test/e2e/annotations/` |
| e2e teardown | `make e2e-cleanup` |
| Manifests | `make k8s-manifests` (kustomize) |
| Load test | `make loadtest-quick LOADTEST_OLD_IMAGE=... LOADTEST_NEW_IMAGE=...` |

`make test` does **not** cover `pkg/`, and CI calls `make test`, so the public API tests never run
automatically. Run `go test ./pkg/...` by hand when touching `pkg/config`, `pkg/matcher` or
`pkg/metadata`.

---

## Coding Conventions

**Logging** is `logr` backed by zerolog. Never `logrus`, that is the master branch. Structured key value
pairs, not formatted strings.

**Errors** are wrapped with `fmt.Errorf("doing thing: %w", err)` and returned up. Startup failures come
back from `run()` as errors, they do not call `os.Exit` mid flow.

**Kubernetes access** goes through the manager's cached `client.Client`. Use `mgr.GetAPIReader()` only
when the cache genuinely cannot serve the read, as the SecretProviderClass reconciler does. Conflicts
are handled by `controller.UpdateWorkloadWithRetry`, which re fetches and re applies rather than
retrying a stale object.

**Filtering belongs in predicates**, not in `Reconcile`. If a change should never wake the reconciler,
add a predicate in `internal/pkg/reload/predicate.go` and wire it through
`controller.BuildEventFilter`.

**Public versus internal.** New behaviour is internal by default. Promote to `pkg/` only when a
downstream consumer genuinely needs it, and treat the promotion as an API commitment.

**Adding a flag**: `internal/pkg/config/flags/flags.go` to bind and apply, field on `config.Config`,
default in `config.NewDefault()`, validation in `pkg/config/validation.go`, then the chart values and
deployment template. Flags are viper backed with `-` to `_` env var mapping, so `--alert-webhook-url`
also reads `ALERT_WEBHOOK_URL`.

**Tests** use standard `testing` plus testify, fixtures from `internal/pkg/testutil/`. e2e uses Ginkgo
and Gomega with helpers in `test/e2e/utils/`.

---

## Gotchas And Risks

**The chart values template is dead.** `deployments/kubernetes/chart/reloader/values.yaml` opens with
"Generated from deployments/kubernetes/templates/chart/values.yaml.tmpl", but nothing in the Makefile,
scripts or workflows runs that generation, and the template is 142 lines against the chart's 446 with no
enterprise section. Edit the chart values directly; the header comment is stale.

**`pkg/` unit tests are not in CI.** `make test` targets `./internal/...` and `./test/e2e/utils/...`, and
`pull_request.yaml` runs `make test`, so the five test files under `pkg/` are never executed by a PR
check even though `pkg/` is the contract the enterprise gateway builds against.

**`VERSION` at the repo root says `1.4.14`** and is not read by any workflow or script. Do not treat it
as the release version; `Chart.yaml` and the git tag are the real sources.

**Annotation source selection does not merge.** See Decision Order step 2. A workload level annotation
suppresses every pod template annotation for that resource type, rather than the two combining.

**Scoped mode hides more than it filters.** `--namespaces` scopes the controller-runtime cache, so
resources outside the list are not merely skipped, they are never listed. Meta info publishing works
around this with an uncached client.

**`--enable-csi-integration` can silently no op.** Missing CRDs disable it with an info log, not an
error.

**Job reload is destructive.** `UpdateStrategyRecreate` deletes the Job first, terminating any running
pod. Intentional, and there is no long running job protection.

**Argo Rollouts must be explicitly enabled.** Without `--is-Argo-Rollouts=true` the scheme is not even
registered, so Rollouts are invisible rather than skipped.

**RBAC failures are silent.** Missing get/list/watch produces empty lists, not errors, so reloads simply
never happen. Check the ClusterRole in the chart templates.

**GitOps drift.** Reloader mutates workloads, so Flux or ArgoCD may revert it. `env-vars` is safer than
`annotations` because the change lands inside the pod template.

**Annotation defaults are a compatibility surface.** They are configurable, so changing a default breaks
existing clusters. Never change one without a migration path.

**Tests to update for risky changes**: `internal/pkg/reload/*_test.go`,
`internal/pkg/workload/workload_test.go`, `internal/pkg/controller/*_test.go`, `pkg/matcher/`, and the
matching `test/e2e/` suite.
