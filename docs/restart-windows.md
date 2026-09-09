# Optional restart windows

When a Secret or ConfigMap changes outside a maintenance window, defer its
workload rollout until the next allowed interval. Pending changes survive
controller restarts and are coalesced using current source data.

Enable `--enable-restart-windows` or Helm value
`reloader.enableRestartWindows: true` (default: false), then add a policy to
workload metadata alongside its normal Reloader opt-in:

```yaml
metadata:
  annotations:
    secret.reloader.stakater.com/reload: "application-tls"
    reloader.stakater.com/restart-window: >-
      {"timezone":"UTC","windows":[{"cron":"0 0 * * *","duration":"1h"}]}
```

This allows rollout starts daily from 00:00 inclusive to 01:00 exclusive UTC.
Place the policy on workload metadata, not the pod template. Policies require
an explicit IANA timezone and 1–16 five-field cron windows, with durations from
1m to 24h. Day-of-month/day-of-week matching follows cron OR semantics; elapsed
duration is preserved across DST changes. Timezone data is embedded in the binary.

First-time enrollment with `--reload-on-create` and `--sync-after-restart`
queues existing sources even if the workload was manually restarted earlier.
Applied hashes track only this scheduler's successful patches. Recovery of
already-persisted pending work does not require either startup flag.

## Behavior and failure handling

- Without a window annotation, upstream restart behavior is unchanged.
- The annotation without the enabling flag fails closed; it does not silently
  restart immediately. Unsupported workload/source kinds also fail closed.
- Secret and ConfigMap events persist source names and observed hashes on the
  workload. Secret contents, private keys and passwords are never persisted in
  annotations or logs by this extension.
- Recovery covers requests already persisted before an outage. Secret events
  lost before persistence retain upstream Reloader startup-sync semantics.
- A workload informer recovers pending work on startup/leader election. A delayed
  workqueue wakes when the window opens and revisits pending work at least once
  a minute. No new Secret event is needed. No CronJob or new CRD is required.
- At execution, source content and opt-in/ignore/filter settings are read again.
  Multiple pending sources produce one pod-template annotation change, using
  their current hashes. Replayed events do not restart an already applied hash.
- Applied hashes and pending removal are committed atomically with the template
  patch. resourceVersion and UID protect concurrent updates and recreated targets.
  API errors retain the pending request and are retried.
- Invalid policies, missing sources, and paused Deployments hold pending work.
  Missing Secrets are never a reason to restart. No automatic expiry override
  exists: operational alerting must cover certificates and blocked rotations.
- `WindowReloaded` and `RestartWindowBlocked` Kubernetes Events expose outcomes;
  errors are logged. `reloader_skipped_total{reason="restart_window_queued"}`
  records deferred event handling through upstream metrics.
- Removing the window annotation releases pending work immediately while the
  scheduler is enabled. To cancel a request, remove its Reloader trigger or
  explicitly remove its pending annotation. Disabling the scheduler retains
  pending state for the next time it is enabled.
- The window governs **starting** a rollout, not finishing it. Kubernetes may
  complete replacement after the window closes. This extension does not verify
  application health, TLS expiry, served certificates, or query completion.
- Windowed workloads use a deterministic pod-template hash annotation regardless
  of upstream `--reload-strategy`; they do not use the upstream pause-period
  cooldown. Manually paused Deployments remain paused and are not modified.
- Supported: apps/v1 Deployments, unpartitioned RollingUpdate StatefulSets, and
  RollingUpdate DaemonSets. Jobs, CronJobs, Argo Rollouts, DeploymentConfigs,
  CSI sources, and directly operator-managed pods are not supported.
- At most 128 pending/applied source references per workload. Prune obsolete
  applied entries when permanently removing sources. Each new Secret content
  update occurring after a rollout may produce another rollout in the same
  open window; this is coalescing of queued work, not a global debounce policy.

For strict windows, run Reloader as the sole Reloader for these targets. Other
controllers, GitOps changes, manual restarts, and native application certificate
reloads are outside its scheduling scope. Webhook-only mode does not use the
workload restart scheduler.

## GitOps ownership

Keep policy annotations in Git. The following paths are controller-owned and
must not be reverted by Argo CD self-heal:

```yaml
ignoreDifferences:
  - group: apps
    kind: Deployment
    jsonPointers:
      - /metadata/annotations/reloader.stakater.com~1pending
      - /metadata/annotations/reloader.stakater.com~1applied
      - /spec/template/metadata/annotations/reloader.stakater.com~1restart-hash
syncPolicy:
  syncOptions:
    - RespectIgnoreDifferences=true
```

Scope the rule to opted-in resource names/namespaces, and add analogous rules
for StatefulSet/DaemonSet only if used. Do not ignore entire annotation maps.
The bundled chart conditionally adds workload `watch` permission when enabling
windows, in both cluster-wide and namespace-scoped RBAC modes.

## Build and validation

```sh
CGO_ENABLED=0 go build -o /tmp/reloader ./
go test -race ./internal/pkg/restartwindow
helm lint deployments/kubernetes/chart/reloader --set reloader.enableRestartWindows=true
```

For upstream handler/controller regression tests, use a temporary **loopback-only**
kubeconfig (`server: http://127.0.0.1:9`) because upstream package initialization
probes for OpenShift/CSI APIs. Never point test runs at a production kubeconfig.
The regression suites use fake Kubernetes clients.

```sh
KUBECONFIG=/path/to/loopback-test-kubeconfig.yaml go test -race \
  ./internal/pkg/restartwindow ./internal/pkg/controller ./internal/pkg/handler ./pkg/common
```

Before production release, exercise a short real window in an isolated cluster:
update two Secrets outside the window, restart the controller, verify no rollout
until opening and exactly one combined template change, then check policy edits,
missing Secrets, leader failover and GitOps reconciliation. Live cluster and
container-image deployment validation are not implied by local tests.

