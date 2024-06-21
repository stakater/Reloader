# ![Reloader-logo](assets/web/reloader-round-100px.png) Reloader

[![Go Report Card](https://goreportcard.com/badge/github.com/stakater/reloader?style=flat-square)](https://goreportcard.com/report/github.com/stakater/reloader)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/stakater/reloader)
[![Release](https://img.shields.io/github/release/stakater/reloader.svg?style=flat-square)](https://github.com/stakater/reloader/releases/latest)
[![GitHub tag](https://img.shields.io/github/tag/stakater/reloader.svg?style=flat-square)](https://github.com/stakater/reloader/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/stakater/reloader.svg?style=flat-square)](https://hub.docker.com/r/stakater/reloader/)
[![Docker Stars](https://img.shields.io/docker/stars/stakater/reloader.svg?style=flat-square)](https://hub.docker.com/r/stakater/reloader/)
[![license](https://img.shields.io/github/license/stakater/reloader.svg?style=flat-square)](LICENSE)
[![Get started with Stakater](https://stakater.github.io/README/stakater-github-banner.png)](https://stakater.com/?utm_source=Reloader&utm_medium=github)

## Problem

We would like to watch if some change happens in `ConfigMap` and/or `Secret`; then perform a rolling upgrade on relevant `DeploymentConfig`, `Deployment`, `Daemonset`, `Statefulset` and `Rollout`

## Solution

Reloader can watch changes in `ConfigMap` and `Secret` and do rolling upgrades on Pods with their associated `DeploymentConfigs`, `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts`.

## Enterprise Version

Reloader is available in two different versions:

1. Open Source Version
1. Enterprise Version, which includes:
    - SLA (Service Level Agreement) for support and unique requests
    - Slack support
    - Certified images

Contact [`sales@stakater.com`](mailto:sales@stakater.com) for info about Reloader Enterprise.

## Compatibility

Reloader is compatible with Kubernetes >= 1.19

## How to use Reloader

For a `Deployment` called `foo` have a `ConfigMap` called `foo-configmap` or `Secret` called `foo-secret` or both. Then add your annotation (by default `reloader.stakater.com/auto`) to main metadata of your `Deployment`

```yaml
kind: Deployment
metadata:
  annotations:
    reloader.stakater.com/auto: "true"
spec:
  template:
    metadata:
```

This will discover deploymentconfigs/deployments/daemonsets/statefulset/rollouts automatically where `foo-configmap` or `foo-secret` is being used either via environment variable or from volume mount. And it will perform rolling upgrade on related pods when `foo-configmap` or `foo-secret`are updated.

You can filter it by the type of monitored resource and use typed versions of `auto` annotation. If you want to discover changes only in mounted `Secret`s and ignore changes in `ConfigMap`s, add `secret.reloader.stakater.com/auto` annotation instead. Analogously, you can use `configmap.reloader.stakater.com/auto` annotation to look for changes in mounted `ConfigMap`, changes in any of mounted `Secret`s will not trigger a rolling upgrade on related pods.

You can also restrict this discovery to only `ConfigMap` or `Secret` objects that
are tagged with a special annotation. To take advantage of that, annotate
your deploymentconfigs/deployments/daemonsets/statefulset/rollouts like this:

```yaml
kind: Deployment
metadata:
  annotations:
    reloader.stakater.com/search: "true"
spec:
  template:
```

and Reloader will trigger the rolling upgrade upon modification of any
`ConfigMap` or `Secret` annotated like this:

```yaml
kind: ConfigMap
metadata:
  annotations:
    reloader.stakater.com/match: "true"
data:
  key: value
```

provided the secret/configmap is being used in an environment variable, or a
volume mount.

Please note that `reloader.stakater.com/search` and
`reloader.stakater.com/auto` do not work together. If you have the
`reloader.stakater.com/auto: "true"` annotation on your deployment, then it
will always restart upon a change in configmaps or secrets it uses, regardless
of whether they have the `reloader.stakater.com/match: "true"` annotation or
not.

Similarly, `reloader.stakater.com/auto` and its typed version (`secret.reloader.stakater.com/auto` or `configmap.reloader.stakater.com/auto`) do not work together. If you have both annotations in your deployment, then only one of them needs to be true to trigger the restart. For example, having both `reloader.stakater.com/auto: "true"` and `secret.reloader.stakater.com/auto: "false"` or both `reloader.stakater.com/auto: "false"` and `secret.reloader.stakater.com/auto: "true"` will restart upon a change in a secret it uses.

We can also specify a specific configmap or secret which would trigger rolling upgrade only upon change in our specified configmap or secret, this way, it will not trigger rolling upgrade upon changes in all configmaps or secrets used in a `deploymentconfig`, `deployment`, `daemonset`, `statefulset` or `rollout`.
To do this either set the auto annotation to `"false"` (`reloader.stakater.com/auto: "false"`) or remove it altogether, and use annotations for [Configmap](.#Configmap) or [Secret](.#Secret).

It's also possible to enable auto reloading for all resources, by setting the `--auto-reload-all` flag.
In this case, all resources that do not have the auto annotation (or its typed version) set to `"false"`, will be reloaded automatically when their ConfigMaps or Secrets are updated.
Notice that setting the auto annotation to an undefined value counts as false as-well.

### Configmap

To perform rolling upgrade when change happens only on specific configmaps use below annotation.

For a `Deployment` called `foo` have a `ConfigMap` called `foo-configmap`. Then add this annotation to main metadata of your `Deployment`

```yaml
kind: Deployment
metadata:
  annotations:
    configmap.reloader.stakater.com/reload: "foo-configmap"
spec:
  template:
    metadata:
```

Use comma separated list to define multiple configmaps.

```yaml
kind: Deployment
metadata:
  annotations:
    configmap.reloader.stakater.com/reload: "foo-configmap,bar-configmap,baz-configmap"
spec:
  template: 
    metadata:
```

### Secret

To perform rolling upgrade when change happens only on specific secrets use below annotation.

For a `Deployment` called `foo` have a `Secret` called `foo-secret`. Then add this annotation to main metadata of your `Deployment`

```yaml
kind: Deployment
metadata:
  annotations:
    secret.reloader.stakater.com/reload: "foo-secret"
spec:
  template: 
    metadata:
```

Use comma separated list to define multiple secrets.

```yaml
kind: Deployment
metadata:
  annotations:
    secret.reloader.stakater.com/reload: "foo-secret,bar-secret,baz-secret"
spec:
  template: 
    metadata:
```

### NOTES

- Reloader also supports [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets). [Here](docs/Reloader-with-Sealed-Secrets.md) are the steps to use sealed-secrets with Reloader.
- For [`rollouts`](https://github.com/argoproj/argo-rollouts/) Reloader simply triggers a change is up to you how you configure the `rollout` strategy.
- `reloader.stakater.com/auto: "true"` will only reload the pod, if the configmap or secret is used (as a volume mount or as an env) in `DeploymentConfigs/Deployment/Daemonsets/Statefulsets`
- `secret.reloader.stakater.com/reload` or `configmap.reloader.stakater.com/reload` annotation will reload the pod upon changes in specified configmap or secret, irrespective of the usage of configmap or secret.
- you may override the auto annotation with the `--auto-annotation` flag
- you may override the secret typed auto annotation with the `--secret-auto-annotation` flag
- you may override the configmap typed auto annotation with the `--configmap-auto-annotation` flag
- you may override the search annotation with the `--auto-search-annotation` flag
  and the match annotation with the `--search-match-annotation` flag
- you may override the configmap annotation with the `--configmap-annotation` flag
- you may override the secret annotation with the `--secret-annotation` flag
- you may want to prevent watching certain namespaces with the `--namespaces-to-ignore` flag
- you may want to watch only a set of namespaces with certain labels by using the `--namespace-selector` flag
- you may want to watch only a set of secrets/configmaps with certain labels by using the `--resource-label-selector` flag
- you may want to prevent watching certain resources with the `--resources-to-ignore` flag
- you can configure logging in JSON format with the `--log-format=json` option
- you can configure the "reload strategy" with the `--reload-strategy=<strategy-name>` option (details below)

## Reload Strategies

Reloader supports multiple "reload" strategies for performing rolling upgrades to resources. The following list describes them:

- **env-vars**: When a tracked `configMap`/`secret` is updated, this strategy attaches a Reloader specific environment variable to any containers referencing the changed `configMap` or `secret` on the owning resource (e.g., `Deployment`, `StatefulSet`, etc.). This strategy can be specified with the `--reload-strategy=env-vars` argument. Note: This is the default reload strategy.
- **annotations**: When a tracked `configMap`/`secret` is updated, this strategy attaches a `reloader.stakater.com/last-reloaded-from` pod template annotation on the owning resource (e.g., `Deployment`, `StatefulSet`, etc.). This strategy is useful when using resource syncing tools like ArgoCD, since it will not cause these tools to detect configuration drift after a resource is reloaded. Note: Since the attached pod template annotation only tracks the last reload source, this strategy will reload any tracked resource should its `configMap` or `secret` be deleted and recreated. This strategy can be specified with the `--reload-strategy=annotations` argument.
  
## Deploying to Kubernetes

You can deploy Reloader by following methods:

### Vanilla Manifests

You can apply vanilla manifests by changing `RELEASE-NAME` placeholder provided in manifest with a proper value and apply it by running the command given below:

```bash
kubectl apply -f https://raw.githubusercontent.com/stakater/Reloader/master/deployments/kubernetes/reloader.yaml
```

By default, Reloader gets deployed in `default` namespace and watches changes `secrets` and `configmaps` in all namespaces.Additionally, in the default Reloader deployment, the following resource limits and requests are set:

```yaml
resources:
  limits:
    cpu: 150m
    memory: 512Mi
  requests:
    cpu: 10m
    memory: 128Mi
```

Reloader can be configured to ignore the resources `secrets` and `configmaps` by passing the following arguments (`spec.template.spec.containers.args`) to its container :

| Argument                         | Description          |
|----------------------------------|----------------------|
| --resources-to-ignore=configMaps | To ignore configMaps |
| --resources-to-ignore=secrets    | To ignore secrets    |

**Note:** At one time only one of these resource can be ignored, trying to do it will cause error in Reloader. Workaround for ignoring both resources is by scaling down the Reloader pods to `0`.

Reloader can be configured to only watch secrets/configmaps with one or more labels using the `--resource-label-selector` parameter. Supported operators are `!, in, notin, ==, =, !=`, if no operator is found the 'exists' operator is inferred (i.e. key only). Additional examples of these selectors can be found in the [Kubernetes Docs](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors).

**Note:** The old `:` delimited key value mappings are deprecated and if provided will be translated to `key=value`. Likewise, if a wildcard value is provided (e.g. `key:*`) it will be translated to the standalone `key` which checks for key existence.

These selectors can be combined, for example with:

```yaml
--resource-label-selector=reloader=enabled,key-exists,another-label in (value1,value2,value3)
```

Only configmaps or secrets labeled like the following will be watched:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  labels:
    reloader: enabled
    key-exists: yes
    another-label: value1
```

Reloader can be configured to only watch namespaces labeled with one or more labels using the `--namespace-selector` parameter. Supported operators are `!, in, notin, ==, =, !=`, if no operator is found the 'exists' operator is inferred (i.e. key only). Additional examples of these selectors can be found in the [Kubernetes Docs](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors).

**Note:** The old `:` delimited key value mappings are deprecated and if provided will be translated to `key=value`. Likewise, if a wildcard value is provided (e.g. `key:*`) it will be translated to the standalone `key` which checks for key existence.

These selectors can be combined, for example with:

```yaml
--namespace-selector=reloader=enabled,test=true
```

Only namespaces labeled as below would be watched and eligible for reloads:

```yaml
kind: Namespace
apiVersion: v1
metadata:
  labels:
    reloader: enabled
    test: true
```

### Vanilla Kustomize

You can also apply the vanilla manifests by running the following command

```bash
kubectl apply -k https://github.com/stakater/Reloader/deployments/kubernetes
```

Similarly to vanilla manifests get deployed in `default` namespace and watches changes `secrets` and `configmaps` in all namespaces.

### Kustomize

You can write your own `kustomization.yaml` using ours as a 'base' and write patches to tweak the configuration.

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - https://github.com/stakater/Reloader/deployments/kubernetes

namespace: reloader
```

### Helm Charts

Alternatively if you have configured helm on your cluster, you can add Reloader to helm from our public chart repository and deploy it via helm using below-mentioned commands. Follow [this](docs/Helm2-to-Helm3.md) guide, in case you have trouble migrating Reloader from Helm2 to Helm3.

#### Installation

```bash
helm repo add stakater https://stakater.github.io/stakater-charts

helm repo update

helm install stakater/reloader # For helm3 add --generate-name flag or set the release name

helm install {{RELEASE_NAME}} stakater/reloader -n {{NAMESPACE}} --set reloader.watchGlobally=false # By default, Reloader watches in all namespaces. To watch in single namespace, set watchGlobally=false

helm install stakater/reloader --set reloader.watchGlobally=false --namespace test --generate-name # Install Reloader in `test` namespace which will only watch `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts` in `test` namespace.
```

#### Uninstalling

```bash
helm uninstall {{RELEASE_NAME}} -n {{NAMESPACE}}
```

### Parameters

#### Global Parameters

| Parameter                 | Description                                                     | Type  | Default |
|---------------------------|-----------------------------------------------------------------|-------|---------|
| `global.imagePullSecrets` | Reference to one or more secrets to be used when pulling images | array | `[]`    |

#### Common Parameters

| Parameter          | Description                   | Type   | Default |
|--------------------|-------------------------------|--------|---------|
| `nameOverride`     | replace the name of the chart | string | `""`    |
| `fullnameOverride` | replace the generated name    | string | `""`    |

#### Core Reloader Parameters

| Parameter                         | Description                                                                                                                                         | Type        | Default   |
|-----------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|-------------|-----------|
| `reloader.autoReloadAll`          |                                                                                                                                                     | boolean     | `false`   |
| `reloader.isArgoRollouts`         | Enable Argo `Rollouts`. Valid value are either `true` or `false`                                                                                    | boolean     | `false`   |
| `reloader.isOpenshift`            | Enable OpenShift DeploymentConfigs. Valid value are either `true` or `false`                                                                        | boolean     | `false`   |
| `reloader.ignoreSecrets`          | To ignore secrets. Valid value are either `true` or `false`. Either `ignoreSecrets` or `ignoreConfigMaps` can be ignored, not both at the same time | boolean     | `false`   |
| `reloader.ignoreConfigMaps`       | To ignore configMaps. Valid value are either `true` or `false`                                                                                      | boolean     | `false`   |
| `reloader.reloadOnCreate`         | Enable reload on create events. Valid value are either `true` or `false`                                                                            | boolean     | `false`   |
| `reloader.syncAfterRestart`       | Enable sync after Reloader restarts for **Add** events, works only when reloadOnCreate is `true`. Valid value are either `true` or `false`          | boolean     | `false`   |
| `reloader.reloadStrategy`         | Strategy to trigger resource restart, set to either `default`, `env-vars` or `annotations`                                                          | enumeration | `default` |
| `reloader.ignoreNamespaces`       | List of comma separated namespaces to ignore, if multiple are provided, they are combined with the AND operator                                     | string      | `""`      |
| `reloader.namespaceSelector`      | List of comma separated namespaces to select, if multiple are provided, they are combined with the AND operator                                     | string      | `""`      |
| `reloader.resourceLabelSelector`  | List of comma separated label selectors, if multiple are provided they are combined with the AND operator                                           | string      | `""`      |
| `reloader.logFormat`              | Set type of log format. Value could be either `json` or `""`                                                                                        | string      | `""`      |
| `reloader.watchGlobally`          | Allow Reloader to watch in all namespaces (`true`) or just in a single namespace (`false`)                                                          | boolean     | `true`    |
| `reloader.enableHA`               | Enable leadership election allowing you to run multiple replicas                                                                                    | boolean     | `false`   |
| `reloader.readOnlyRootFileSystem` | Enforce readOnlyRootFilesystem                                                                                                                      | boolean     | `false`   |
| `reloader.legacy.rbac`            |                                                                                                                                                     | boolean     | `false`   |
| `reloader.matchLabels`            | Pod labels to match                                                                                                                                 | map         | `{}`      |

#### Deployment Reloader Parameters

| Parameter                                       | Description                                                                             | Type   | Default           |
|-------------------------------------------------|-----------------------------------------------------------------------------------------|--------|-------------------|
| `reloader.deployment.replicas`                  | Number of replicas, if you wish to run multiple replicas set `reloader.enableHA = true` | int    | 1                 |
| `reloader.deployment.revisionHistoryLimit`      | Limit the number of revisions retained in the revision history                          | int    | 2                 |
| `reloader.deployment.nodeSelector`              | Scheduling pod to a specific node based on set labels                                   | map    | `{}`              |
| `reloader.deployment.affinity`                  | Set affinity rules on pod                                                               | map    | `{}`              |
| `reloader.deployment.securityContext`           | Set pod security context                                                                | map    | `{}`              |
| `reloader.deployment.containerSecurityContext`  | Set container security context                                                          | map    | `{}`              |
| `reloader.deployment.tolerations`               | A list of `tolerations` to be applied to the deployment                                 | array  | `[]`              |
| `reloader.deployment.topologySpreadConstraints` | Topology spread constraints for pod assignment                                          | array  | `[]`              |
| `reloader.deployment.annotations`               | Set deployment annotations                                                              | map    | `{}`              |
| `reloader.deployment.labels`                    | Set deployment labels, default to stakater settings                                     | array  | `see values.yaml` |
| `reloader.deployment.image`                     | Set container image name, tag and policy                                                | array  | `see values.yaml` |
| `reloader.deployment.env`                       | Support for extra environment variables                                                 | array  | `[]`              |
| `reloader.deployment.livenessProbe`             | Set liveness probe timeout values                                                       | map    | `{}`              |
| `reloader.deployment.readinessProbe`            | Set readiness probe timeout values                                                      | map    | `{}`              |
| `reloader.deployment.resources`                 | Set container requests and limits (e.g. CPU or memory)                                  | map    | `{}`              |
| `reloader.deployment.pod.annotations`           | Set annotations for pod                                                                 | map    | `{}`              |
| `reloader.deployment.priorityClassName`         | Set priority class for pod in cluster                                                   | string | `""`              |

#### Other Reloader Parameters

| Parameter                              | Description                                                     | Type    | Default |
|----------------------------------------|-----------------------------------------------------------------|---------|---------|
| `reloader.service`                     |                                                                 | map     | `{}`    |
| `reloader.rbac.enabled`                | Specifies whether a role based access control should be created | boolean | `true`  |
| `reloader.serviceAccount.create`       | Specifies whether a ServiceAccount should be created            | boolean | `true`  |
| `reloader.custom_annotations`          | Add custom annotations                                          | map     | `{}`    |
| `reloader.serviceMonitor.enabled`      | Enable to scrape Reloader's Prometheus metrics (legacy)         | boolean | `false` |
| `reloader.podMonitor.enabled`          | Enable to scrape Reloader's Prometheus metrics                  | boolean | `false` |
| `reloader.podDisruptionBudget.enabled` | Limit the number of pods of a replicated application            | boolean | `false` |
| `reloader.netpol.enabled`              |                                                                 | boolean | `false` |
| `reloader.volumeMounts`                | Mount volume                                                    | array   | `[]`    |
| `reloader.volumes`                     | Add volume to a pod                                             | array   | `[]`    |
| `reloader.webhookUrl`                  | Add webhook to Reloader                                         | string  | `""`    |

#### Additional Remarks

- Both `namespaceSelector` & `resourceLabelSelector` can be used together. If they are then both conditions must be met for the configmap or secret to be eligible to trigger reload events. (e.g. If a configMap matches `resourceLabelSelector` but `namespaceSelector` does not match the namespace the configmap is in, it will be ignored).
- At one time only one of the resources `ignoreConfigMaps` or `ignoreSecrets` can be ignored, trying to do both will cause error in helm template compilation
- Reloading of OpenShift (DeploymentConfig) and/or Argo `Rollouts` has to be enabled explicitly because it might not be always possible to use it on a cluster with restricted permissions
- `isOpenShift` Recent versions of OpenShift (tested on 4.13.3) require the specified user to be in an `uid` range which is dynamically assigned by the namespace. The solution is to unset the runAsUser variable via ``deployment.securityContext.runAsUser=null`` and let OpenShift assign it at install
- `reloadOnCreate` controls how Reloader handles secrets being added to the cache for the first time. If `reloadOnCreate` is set to true:
  1. Configmaps/secrets being added to the cache will cause Reloader to perform a rolling update of the associated workload
  1. When applications are deployed for the first time, Reloader will perform a rolling update of the associated workload
  1. If you are running Reloader in HA mode all workloads will have a rolling update performed when a new leader is elected
- `serviceMonitor` will be removed in future releases of Reloader in favour of Pod monitor
- If `reloadOnCreate` is set to false:
  1. Updates to configmaps/secrets that occur while there is no leader will not be picked up by the new leader until a subsequent update of the configmap/secret occurs
  1. In the worst case the window in which there can be no leader is 15s as this is the LeaseDuration
- By default, `reloadOnCreate` and `syncAfterRestart` are both set to false. Both need to be enabled explicitly

## Help

### Documentation

You can find more documentation [here](docs)

### Have a question?

File a GitHub [issue](https://github.com/stakater/Reloader/issues).

### Talk to us on Slack

Join and talk to us on Slack for discussing Reloader

[![Join Slack](https://stakater.github.io/README/stakater-join-slack-btn.png)](https://stakater.slack.com/)
[![Chat](https://stakater.github.io/README/stakater-chat-btn.png)](https://stakater-community.slack.com/messages/CC5S05S12)

## Contributing

### Bug Reports & Feature Requests

Please use the [issue tracker](https://github.com/stakater/Reloader/issues) to report any bugs or file feature requests.

### Developing

1. Deploy Reloader.
1. Run `okteto up` to activate your development container.
1. `make build`
1. `./Reloader`

PRs are welcome. In general, we follow the "fork-and-pull" Git workflow.

1. **Fork** the repo on GitHub
1. **Clone** the project to your own machine
1. **Commit** changes to your own branch
1. **Push** your work back up to your fork
1. Submit a **Pull request** so that we can review your changes

**NOTE:** Be sure to merge the latest from "upstream" before making a pull request!

## Changelog

View our closed [Pull Requests](https://github.com/stakater/Reloader/pulls?q=is%3Apr+is%3Aclosed).

## License

Apache2 © [Stakater][website]

## About

`Reloader` is maintained by [Stakater][website]. Like it? Please let us know at <hello@stakater.com>

See [our other projects](https://github.com/stakater)
or contact us in case of professional services and queries on <hello@stakater.com>

[website]: https://stakater.com

## Acknowledgements

- [ConfigmapController](https://github.com/fabric8io/configmapcontroller); We documented [here](docs/Reloader-vs-ConfigmapController.md) why we re-created Reloader
