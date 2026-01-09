# Reloader Load Test Framework

This framework provides A/B comparison testing between two Reloader container images.

## Overview

The load test framework:
1. Creates a local kind cluster (1 control-plane + 6 worker nodes)
2. Deploys Prometheus for metrics collection
3. Loads the provided Reloader container images into the cluster
4. Runs standardized test scenarios (S1-S13)
5. Collects metrics via Prometheus scraping
6. Generates comparison reports with pass/fail criteria

## Prerequisites

- Docker or Podman
- kind (Kubernetes in Docker)
- kubectl
- Go 1.22+

## Building

```bash
cd test/loadtest
go build -o loadtest ./cmd/loadtest
```

## Quick Start

```bash
# Compare two published images (e.g., different versions)
./loadtest run \
  --old-image=stakater/reloader:v1.0.0 \
  --new-image=stakater/reloader:v1.1.0

# Run a specific scenario
./loadtest run \
  --old-image=stakater/reloader:v1.0.0 \
  --new-image=stakater/reloader:v1.1.0 \
  --scenario=S2 \
  --duration=120

# Test only a single image (no comparison)
./loadtest run --new-image=myregistry/reloader:dev

# Use local images built with docker/podman
./loadtest run \
  --old-image=localhost/reloader:baseline \
  --new-image=localhost/reloader:feature-branch

# Skip cluster creation (use existing kind cluster)
./loadtest run \
  --old-image=stakater/reloader:v1.0.0 \
  --new-image=stakater/reloader:v1.1.0 \
  --skip-cluster

# Run all scenarios in parallel on 4 clusters (faster execution)
./loadtest run \
  --new-image=localhost/reloader:dev \
  --parallelism=4

# Run all 13 scenarios in parallel (one cluster per scenario)
./loadtest run \
  --new-image=localhost/reloader:dev \
  --parallelism=13

# Generate report from existing results
./loadtest report --scenario=S2 --results-dir=./results
```

## Command Line Options

### Run Command

| Option | Description | Default |
|--------|-------------|---------|
| `--old-image=IMAGE` | Container image for "old" version | - |
| `--new-image=IMAGE` | Container image for "new" version | - |
| `--scenario=ID` | Test scenario: S1-S13 or "all" | all |
| `--duration=SECONDS` | Test duration in seconds | 60 |
| `--parallelism=N` | Run N scenarios in parallel on N kind clusters | 1 |
| `--skip-cluster` | Skip kind cluster creation (use existing, only for parallelism=1) | false |
| `--results-dir=DIR` | Directory for results | ./results |

**Note:** At least one of `--old-image` or `--new-image` is required. Provide both for A/B comparison.

### Report Command

| Option | Description | Default |
|--------|-------------|---------|
| `--scenario=ID` | Scenario to report on (required) | - |
| `--results-dir=DIR` | Directory containing results | ./results |
| `--output=FILE` | Output file (default: stdout) | - |

## Test Scenarios

| ID  | Name                  | Description                                     |
|-----|-----------------------|-------------------------------------------------|
| S1  | Burst Updates         | Many ConfigMap/Secret updates in quick succession |
| S2  | Fan-Out               | One ConfigMap used by many (50) workloads       |
| S3  | High Cardinality      | Many CMs/Secrets across many namespaces         |
| S4  | No-Op Updates         | Updates that don't change data (annotation only)|
| S5  | Workload Churn        | Deployments created/deleted rapidly             |
| S6  | Controller Restart    | Restart controller pod under load               |
| S7  | API Pressure          | Many concurrent update requests                 |
| S8  | Large Objects         | ConfigMaps > 100KB                              |
| S9  | Multi-Workload Types  | Tests all workload types (Deploy, STS, DS)      |
| S10 | Secrets + Mixed       | Secrets and mixed ConfigMap+Secret workloads    |
| S11 | Annotation Strategy   | Tests `--reload-strategy=annotations`           |
| S12 | Pause & Resume        | Tests pause-period during rapid updates         |
| S13 | Complex References    | Init containers, valueFrom, projected volumes   |

## Metrics Reference

This section explains each metric collected during load tests, what it measures, and what different values might indicate.

### Counter Metrics (Totals)

#### `reconcile_total`
**What it measures:** The total number of reconciliation loops executed by the controller.

**What it indicates:**
- **Higher in new vs old:** The new controller-runtime implementation may batch events differently. This is often expected behavior, not a problem.
- **Lower in new vs old:** Better event batching/deduplication. Controller-runtime's work queue naturally deduplicates events.
- **Expected behavior:** The new implementation typically has *fewer* reconciles due to intelligent event batching.

#### `action_total`
**What it measures:** The total number of reload actions triggered (rolling restarts of Deployments/StatefulSets/DaemonSets).

**What it indicates:**
- **Should match expected value:** Both implementations should trigger the same number of reloads for the same workload.
- **Lower than expected:** Some updates were missed - potential bug or race condition.
- **Higher than expected:** Duplicate reloads triggered - inefficiency but not data loss.

#### `reload_executed_total`
**What it measures:** Successful reload operations executed, labeled by `success=true/false`.

**What it indicates:**
- **`success=true` count:** Number of workloads successfully restarted.
- **`success=false` count:** Failed restart attempts (API errors, permission issues).
- **Should match `action_total`:** If significantly lower, reloads are failing.

#### `workloads_scanned_total`
**What it measures:** Number of workloads (Deployments, etc.) scanned when checking for ConfigMap/Secret references.

**What it indicates:**
- **High count:** Controller is scanning many workloads per reconcile.
- **Expected behavior:** Should roughly match the number of workloads × number of reconciles.
- **Optimization signal:** If very high, namespace filtering or label selectors could help.

#### `workloads_matched_total`
**What it measures:** Number of workloads that matched (reference the changed ConfigMap/Secret).

**What it indicates:**
- **Should match `reload_executed_total`:** Every matched workload should be reloaded.
- **Higher than reloads:** Some matched workloads weren't reloaded (potential issue).

#### `errors_total`
**What it measures:** Total errors encountered, labeled by error type.

**What it indicates:**
- **Should be 0:** Any errors indicate problems.
- **Common causes:** API server timeouts, RBAC issues, resource conflicts.
- **Critical metric:** Non-zero errors in production should be investigated.

### API Efficiency Metrics (REST Client)

These metrics track Kubernetes API server calls made by Reloader. Lower values indicate more efficient operation with less API server load.

#### `rest_client_requests_total`
**What it measures:** Total number of HTTP requests made to the Kubernetes API server.

**What it indicates:**
- **Lower is better:** Fewer API calls means less load on the API server.
- **High count:** May indicate inefficient caching or excessive reconciles.
- **Comparison use:** Shows overall API efficiency between implementations.

#### `rest_client_requests_get`
**What it measures:** Number of GET requests (fetching individual resources or listings).

**What it indicates:**
- **Includes:** Fetching ConfigMaps, Secrets, Deployments, etc.
- **Higher count:** More frequent resource fetching, possibly due to cache misses.
- **Expected behavior:** Controller-runtime's caching should reduce GET requests compared to direct API calls.

#### `rest_client_requests_patch`
**What it measures:** Number of PATCH requests (partial updates to resources).

**What it indicates:**
- **Used for:** Rolling restart annotations on workloads.
- **Should correlate with:** `reload_executed_total` - each reload typically requires one PATCH.
- **Lower is better:** Fewer patches means more efficient batching or deduplication.

#### `rest_client_requests_put`
**What it measures:** Number of PUT requests (full resource updates).

**What it indicates:**
- **Used for:** Full object replacements (less common than PATCH).
- **Should be low:** Most updates use PATCH for efficiency.
- **High count:** May indicate suboptimal update strategy.

#### `rest_client_requests_errors`
**What it measures:** Number of failed API requests (4xx/5xx responses).

**What it indicates:**
- **Should be 0:** Errors indicate API server issues or permission problems.
- **Common causes:** Rate limiting, RBAC issues, resource conflicts, network issues.
- **Non-zero:** Investigate API server logs and Reloader permissions.

### Latency Metrics (Percentiles)

All latency metrics are reported in **seconds**. The report shows p50 (median), p95, and p99 percentiles.

#### `reconcile_duration (s)`
**What it measures:** Time spent inside each reconcile loop, from start to finish.

**What it indicates:**
- **p50 (median):** Typical reconcile time. Should be < 100ms for good performance.
- **p95:** 95th percentile - only 5% of reconciles take longer than this.
- **p99:** 99th percentile - indicates worst-case performance.

**Interpreting differences:**
- **New higher than old:** Controller-runtime reconciles may do more work per loop but run fewer times. Check `reconcile_total` - if it's lower, this is expected.
- **Minor differences (< 0.5s absolute):** Not significant for sub-second values.

#### `action_latency (s)`
**What it measures:** End-to-end time from ConfigMap/Secret change detection to workload restart triggered.

**What it indicates:**
- **This is the user-facing latency:** How long users wait for their config changes to take effect.
- **p50 < 1s:** Excellent - most changes apply within a second.
- **p95 < 5s:** Good - even under load, changes apply quickly.
- **p99 > 10s:** May need investigation - some changes take too long.

**What affects this:**
- API server responsiveness
- Number of workloads to scan
- Concurrent updates competing for resources

### Understanding the Report

#### Report Columns

```
Metric                           Old          New   Expected  Old✓  New✓   Status
------                           ---          ---   --------  ----  ----   ------
action_total                  100.00       100.00        100     ✓     ✓     pass
action_latency_p95 (s)          0.15         0.04          -     -     -     pass
```

- **Old/New:** Measured values from each implementation
- **Expected:** Known expected value (for throughput metrics)
- **Old✓/New✓:** Whether the value is within 15% of expected (✓ = yes, ✗ = no, - = no expected value)
- **Status:** pass/fail based on comparison thresholds

#### Pass/Fail Logic

| Metric Type | Pass Condition |
|-------------|----------------|
| Throughput (action_total, reload_executed_total) | New value within 15% of expected |
| Latency (p50, p95, p99) | New not more than threshold% worse than old, OR absolute difference < minimum threshold |
| Errors | New ≤ Old (ideally both 0) |
| API Efficiency (rest_client_requests_*) | New ≤ Old (lower is better), or New not more than 50% higher |

#### Latency Thresholds

Latency comparisons use both percentage AND absolute thresholds to avoid false failures:

| Metric | Max % Worse | Min Absolute Diff |
|--------|-------------|-------------------|
| p50 | 100% | 0.5s |
| p95 | 100% | 1.0s |
| p99 | 100% | 1.0s |

**Example:** If old p50 = 0.01s and new p50 = 0.08s:
- Percentage difference: +700% (would fail % check)
- Absolute difference: 0.07s (< 0.5s threshold)
- **Result: PASS** (both values are fast enough that the difference doesn't matter)

### Resource Consumption Metrics

These metrics track CPU, memory, and Go runtime resource usage. Lower values generally indicate more efficient operation.

#### Memory Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `memory_rss_mb_avg` | Average RSS (resident set size) memory | MB |
| `memory_rss_mb_max` | Peak RSS memory during test | MB |
| `memory_heap_mb_avg` | Average Go heap allocation | MB |
| `memory_heap_mb_max` | Peak Go heap allocation | MB |

**What to watch for:**
- **High RSS:** May indicate memory leaks or inefficient caching
- **High heap:** Many objects being created (check GC metrics)
- **Growing over time:** Potential memory leak

#### CPU Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `cpu_cores_avg` | Average CPU usage rate | cores |
| `cpu_cores_max` | Peak CPU usage rate | cores |

**What to watch for:**
- **High CPU:** Inefficient algorithms or excessive reconciles
- **Spiky max:** May indicate burst handling issues

#### Go Runtime Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `goroutines_avg` | Average goroutine count | count |
| `goroutines_max` | Peak goroutine count | count |
| `gc_pause_p99_ms` | 99th percentile GC pause time | ms |

**What to watch for:**
- **High goroutines:** Potential goroutine leak or unbounded concurrency
- **High GC pause:** Large heap or allocation pressure

### Scenario-Specific Expectations

| Scenario | Key Metrics to Watch | Expected Behavior |
|----------|---------------------|-------------------|
| S1 (Burst) | action_latency_p99, cpu_cores_max, goroutines_max | Should handle bursts without queue backup |
| S2 (Fan-Out) | reconcile_total, workloads_matched, memory_rss_mb_max | One CM change → 50 workload reloads |
| S3 (High Cardinality) | reconcile_duration, memory_heap_mb_avg | Many namespaces shouldn't increase memory |
| S4 (No-Op) | action_total = 0, cpu_cores_avg should be low | Minimal resource usage for no-op |
| S5 (Churn) | errors_total, goroutines_avg | Graceful handling, no goroutine leak |
| S6 (Restart) | All metrics captured | Metrics survive controller restart |
| S7 (API Pressure) | errors_total, cpu_cores_max, goroutines_max | No errors under concurrent load |
| S8 (Large Objects) | memory_rss_mb_max, gc_pause_p99_ms | Large ConfigMaps don't cause OOM or GC issues |
| S9 (Multi-Workload) | reload_executed_total per type | All workload types (Deploy, STS, DS) reload |
| S10 (Secrets) | reload_executed_total, workloads_matched | Both Secrets and ConfigMaps trigger reloads |
| S11 (Annotation) | workload annotations present | Deployments get `last-reloaded-from` annotation |
| S12 (Pause) | reload_executed_total << updates | Pause-period reduces reload frequency |
| S13 (Complex) | reload_executed_total | All reference types trigger reloads |

### Troubleshooting

#### New implementation shows 0 for all metrics
- Check if Prometheus is scraping the new Reloader pod
- Verify pod annotations: `prometheus.io/scrape: "true"`
- Check Prometheus targets: `http://localhost:9091/targets`

#### Metrics don't match expected values
- Verify test ran to completion (check logs)
- Ensure Prometheus scraped final metrics (18s wait after test)
- Check for pod restarts during test (metrics reset on restart - handled by `increase()`)

#### High latency in new implementation
- Check Reloader pod resource limits
- Look for API server throttling in logs
- Compare `reconcile_total` - fewer reconciles with higher duration may be normal

#### REST client errors are non-zero
- **Common causes:**
  - Optional CRD schemes registered but CRDs not installed (e.g., Argo Rollouts, OpenShift DeploymentConfig)
  - API server rate limiting under high load
  - RBAC permissions missing for certain resource types
- **Argo Rollouts errors:** If you see ~4 errors per test, ensure `--enable-argo-rollouts=false` if not using Argo Rollouts
- **OpenShift errors:** Similarly, ensure DeploymentConfig support is disabled on non-OpenShift clusters

#### REST client requests much higher in new implementation
- Check if caching is working correctly
- Look for excessive re-queuing in controller logs
- Compare `reconcile_total` - more reconciles naturally means more API calls

## Report Format

The report generator produces a comparison table with units and expected value indicators:

```
================================================================================
                     RELOADER A/B COMPARISON REPORT
================================================================================

Scenario:     S2
Generated:    2026-01-03 14:30:00
Status:       PASS
Summary:      All metrics within acceptable thresholds

Test:         S2: Fan-out test - 1 CM update triggers 50 deployment reloads

--------------------------------------------------------------------------------
                           METRIC COMPARISONS
--------------------------------------------------------------------------------
(Old✓/New✓ = meets expected value within 15%)

Metric                                   Old          New   Expected  Old✓  New✓   Status
------                                   ---          ---   --------  ----  ----   ------
reconcile_total                        50.00        25.00          -     -     -     pass
reconcile_duration_p50 (s)              0.01         0.05          -     -     -     pass
reconcile_duration_p95 (s)              0.02         0.15          -     -     -     pass
action_total                           50.00        50.00         50     ✓     ✓     pass
action_latency_p50 (s)                  0.05         0.03          -     -     -     pass
action_latency_p95 (s)                  0.12         0.08          -     -     -     pass
errors_total                            0.00         0.00          -     -     -     pass
reload_executed_total                  50.00        50.00         50     ✓     ✓     pass
workloads_scanned_total                50.00        50.00         50     ✓     ✓     pass
workloads_matched_total                50.00        50.00         50     ✓     ✓     pass
rest_client_requests_total              850         720            -     -     -     pass
rest_client_requests_get                500         420            -     -     -     pass
rest_client_requests_patch              300         250            -     -     -     pass
rest_client_requests_errors               0           0            -     -     -     pass
```

Reports are saved to `results/<scenario>/report.txt` after each test.

## Directory Structure

```
test/loadtest/
├── cmd/
│   └── loadtest/              # Unified CLI (run + report)
│       └── main.go
├── internal/
│   ├── cluster/               # Kind cluster management
│   │   └── kind.go
│   ├── prometheus/            # Prometheus deployment & querying
│   │   └── prometheus.go
│   ├── reloader/              # Reloader deployment
│   │   └── deploy.go
│   └── scenarios/             # Test scenario implementations
│       └── scenarios.go
├── manifests/
│   └── prometheus.yaml        # Prometheus deployment manifest
├── results/                   # Generated after tests
│   └── <scenario>/
│       ├── old/               # Old version data
│       │   ├── *.json         # Prometheus metric snapshots
│       │   └── reloader.log   # Reloader pod logs
│       ├── new/               # New version data
│       │   ├── *.json         # Prometheus metric snapshots
│       │   └── reloader.log   # Reloader pod logs
│       ├── expected.json      # Expected values from test
│       └── report.txt         # Comparison report
├── go.mod
├── go.sum
└── README.md
```

## Building Local Images for Testing

If you want to test local code changes:

```bash
# Build the new Reloader image from current source
docker build -t localhost/reloader:dev -f Dockerfile .

# Build from a different branch/commit
git checkout feature-branch
docker build -t localhost/reloader:feature -f Dockerfile .

# Then run comparison
./loadtest run \
  --old-image=stakater/reloader:v1.0.0 \
  --new-image=localhost/reloader:feature
```

## Interpreting Results

### PASS
All metrics are within acceptable thresholds. The new implementation is comparable or better than the old one.

### FAIL
One or more metrics exceeded thresholds. Review the specific metrics:
- **Latency degradation**: p95/p99 latencies are significantly higher
- **Missed reloads**: `reload_executed_total` differs significantly
- **Errors increased**: `errors_total` is higher in new version

### Investigation

If tests fail, check:
1. Pod logs: `kubectl logs -n reloader-new deployment/reloader` (or check `results/<scenario>/new/reloader.log`)
2. Resource usage: `kubectl top pods -n reloader-new`
3. Events: `kubectl get events -n reloader-test`

## Parallel Execution

The `--parallelism` option enables running scenarios on multiple kind clusters simultaneously, significantly reducing total test time.

### How It Works

1. **Multiple Clusters**: Creates N kind clusters named `reloader-loadtest-0`, `reloader-loadtest-1`, etc.
2. **Separate Prometheus**: Each cluster gets its own Prometheus instance with a unique port (9091, 9092, etc.)
3. **Worker Pool**: Scenarios are distributed to workers via a channel, with each worker running on its own cluster
4. **Independent Execution**: Each scenario runs in complete isolation with no resource contention

### Usage

```bash
# Run 4 scenarios at a time (creates 4 clusters)
./loadtest run --new-image=my-image:tag --parallelism=4

# Run all 13 scenarios in parallel (creates 13 clusters)
./loadtest run --new-image=my-image:tag --parallelism=13 --scenario=all
```

### Resource Requirements

Parallel execution requires significant system resources:

| Parallelism | Clusters | Est. Memory | Est. CPU |
|-------------|----------|-------------|----------|
| 1 (default) | 1 | ~4GB | 2-4 cores |
| 4 | 4 | ~16GB | 8-16 cores |
| 13 | 13 | ~52GB | 26-52 cores |

### Notes

- The `--skip-cluster` option is not supported with parallelism > 1
- Each worker loads images independently, so initial setup takes longer
- All results are written to the same `--results-dir` with per-scenario subdirectories
- If a cluster setup fails, remaining workers continue with available clusters
- Parallelism automatically reduces to match scenario count if set higher

## CI Integration

### GitHub Actions

Load tests can be triggered on pull requests by commenting `/loadtest`:

```
/loadtest
```

This will:
1. Build a container image from the PR branch
2. Run all load test scenarios against it
3. Post results as a PR comment
4. Upload detailed results as artifacts

### Make Target

Run load tests locally or in CI:

```bash
# From repository root
make loadtest
```

This builds the container image and runs all scenarios with a 60-second duration.
