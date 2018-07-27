# How it works

Reloader watches for `ConfigMap` and `Secret` objects and detects if there are any changes in data of these objects. After change detection perform rolling upgrade on relevant `Deployment`, `Deamonset` and `Statefulset` and recreates associated pods.

## How change detection works

Reloader watches any changes in `configmaps` and `secrets`. As soon as it detects any change in these objects. It forwards these objects to an update handler which decides if and how to perform the rolling upgrade.

## Requirements for rolling upgrade

To perform rolling upgrade a `deployment`, `daemonset` or `statefulset` must have 

- support for rolling upgrade strategy
- specific annotation for `configmaps` or `secrets`

The annotation value is comma separated list of `configmaps` or `secrets`. If any change is detected in these `configmaps` or `secrets`, reloader will perform rolling upgrades on these associated `deployments`, `daemonsets` or `statefulsets`.

### Annotation fof Configmap

For a `Deployment` called `foo` have a `ConfigMap` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    configmap.reloader.stakater.com/reload: "foo"
```

### Annotation fof Secret

For a `Deployment` called `foo` have a `Secret` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    secret.reloader.stakater.com/reload: "foo"
```

Above mentioned annotation are also work for `Daemonsets` and `Statefulsets`

## How Rolling upgrade works

After change detection in configmap or secret, reloader first compare the old configmap data hash with new config data hash and if finds any difference only then move forward with rolling upgrade.

After that, reloader gets the list of all deployments, daemonsets and statefulset and looks for relevant annotation. If the annotation is found, it then looks for an environment variable which can contain the configmap or secret data change hash.

### Environment variable for Configmap

If configmap name is foo then

```yaml
STAKATER_FOO_CONFIGMAP
```

### Environment variable for Secret

If Secret name is foo then

```yaml
STAKATER_FOO_SECRET
```

If the environment variable is found then it checks the hash value and only if it differs from new hash value then reloader updates it. If the environment variable does not exist then it creates a new environment variable with latest hash value and updates the relevant `deployment`, `daemonset` or `statefulset`

### Hash value Computation

Reloader uses SHA1 to compute hash value. SHA1 is used because it is efficient and less prone to collision.

## Monitor All namespaces

By default reloader deploys in default namespace but monitors changes in all namespaces. To monitor changes in a specific namespace deploy the reloader in that namespace and set the `watchGlobally` flag to `false` in values file located under `deployments/kubernetes/chart/reloader`