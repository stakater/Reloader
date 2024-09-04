# Verify Reloader's Working

Reloader's working can be verified by three ways.

## Verify From Logs

Check the logs of Reloader and verify that you can see logs looks like below, if you are able to find these logs then it means Reloader is working.

```text
Changes Detected in test-object of type 'SECRET' in namespace: test-reloader

Updated test-resource of type Deployment in namespace: test-reloader
```

Below are the details that explain these logs:

### `test-object`

`test-object` is the name of a `secret` or a `configmap` in which change has been detected.

### `SECRET`

`SECRET` is the type of `test-object`. It can either be `SECRET` or `CONFIGMAP`

### `test-reloader`

`test-reloader` is the name of namespace in which Reloader has detected the change.

### `test-resource`

`test-resource` is the name of resource which is going to be updated

### `Deployment`

`Deployment` is the type of `test-resource`. It can either be a `Deployment`, `Daemonset` or `Statefulset`

## Verify by Checking the Age of Pod

A pod's age can tell whether Reloader is working correctly or not. If you know that a change in a `secret` or `configmap` has occurred, then check the relevant Pod's age immediately. It should be newly created few moments ago.

### Verify from Kubernetes Dashboard

`kubernetes dashboard` can be used to verify the working of Reloader. After a change in `secret` or `configmap`, check the relevant Pod's age from dashboard. It should be newly created few moments ago.

### Verify from Command Line

After a change in `secret` or `configmap`. Run the below-mentioned command and verify that the pod is newly created.

```bash
kubectl get pods <pod name> -n <namespace name>
```

## Verify From Metrics

Some metrics are exported to Prometheus endpoint `/metrics` on port `9090`.

When Reloader is unable to reload, `reloader_reload_executed_total{success="false"}` metric gets incremented and when it reloads successfully, `reloader_reload_executed_total{success="true"}` gets incremented. You will be able to see the following metrics, with some other metrics, at `/metrics` endpoint.

```text
reloader_reload_executed_total{success="false"} 15
reloader_reload_executed_total{success="true"} 12
```

### Reloads by Namespace

Reloader can also export a metric to show the number of reloads by namespace. This feature is disabled by default, as it can lead to high cardinality in clusters with many namespaces.

The metric will have both `success` and `namespace` as attributes:

```text
reloader_reload_executed_total{success="false", namespace="some-namespace"} 2
reloader_reload_executed_total{success="true", namespace="some-namespace"} 1
```

To opt in, set the environment variable `METRICS_COUNT_BY_NAMESPACE` to `enabled` or set the Helm value `reloader.enableMetricsByNamespace` to `true`.
