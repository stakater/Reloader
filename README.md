# ![](assets/web/reloader-round-100px.png) RELOADER

[![Go Report Card](https://goreportcard.com/badge/github.com/stakater/reloader?style=flat-square)](https://goreportcard.com/report/github.com/stakater/reloader)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/stakater/reloader)
[![Release](https://img.shields.io/github/release/stakater/reloader.svg?style=flat-square)](https://github.com/stakater/reloader/releases/latest)
[![GitHub tag](https://img.shields.io/github/tag/stakater/reloader.svg?style=flat-square)](https://github.com/stakater/reloader/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/stakater/reloader.svg?style=flat-square)](https://hub.docker.com/r/stakater/reloader/)
[![Docker Stars](https://img.shields.io/docker/stars/stakater/reloader.svg?style=flat-square)](https://hub.docker.com/r/stakater/reloader/)
[![license](https://img.shields.io/github/license/stakater/reloader.svg?style=flat-square)](LICENSE)
[![Get started with Stakater](https://stakater.github.io/README/stakater-github-banner.png)](http://stakater.com/?utm_source=Reloader&utm_medium=github)

## Problem

We would like to watch if some change happens in `ConfigMap` and/or `Secret`; then perform a rolling upgrade on relevant `DeploymentConfig`, `Deployment`, `Daemonset`, `Statefulset` and `Rollout`

## Solution

Reloader can watch changes in `ConfigMap` and `Secret` and do rolling upgrades on Pods with their associated `DeploymentConfigs`, `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts`.

## Compatibility

Reloader is compatible with kubernetes >= 1.9

## How to use Reloader

For a `Deployment` called `foo` have a `ConfigMap` called `foo-configmap` or `Secret` called `foo-secret` or both. Then add your annotation (by default `reloader.stakater.com/auto`) to main metadata of your `Deployment`

```yaml
kind: Deployment
metadata:
  annotations:
    reloader.stakater.com/auto: "true"
spec:
  template: metadata:
```

This will discover deploymentconfigs/deployments/daemonsets/statefulset/rollouts automatically where `foo-configmap` or `foo-secret` is being used either via environment variable or from volume mount. And it will perform rolling upgrade on related pods when `foo-configmap` or `foo-secret`are updated.

You can restrict this discovery to only `ConfigMap` or `Secret` objects that
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

We can also specify a specific configmap or secret which would trigger rolling upgrade only upon change in our specified configmap or secret, this way, it will not trigger rolling upgrade upon changes in all configmaps or secrets used in a deploymentconfig, deployment, daemonset, statefulset or rollout.
To do this either set the auto annotation to `"false"` (`reloader.stakater.com/auto: "false"`) or remove it altogether, and use annotations mentioned [here](#Configmap) or [here](#Secret)

### Configmap

To perform rolling upgrade when change happens only on specific configmaps use below annotation.

For a `Deployment` called `foo` have a `ConfigMap` called `foo-configmap`. Then add this annotation to main metadata of your `Deployment`

```yaml
kind: Deployment
metadata:
  annotations:
    configmap.reloader.stakater.com/reload: "foo-configmap"
spec:
  template: metadata:
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

- Reloader also supports [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets). [Here](docs/Reloader-with-Sealed-Secrets.md) are the steps to use sealed-secrets with reloader.
- For [rollouts](https://github.com/argoproj/argo-rollouts/) reloader simply triggers a change is up to you how you configure the rollout strategy.
- `reloader.stakater.com/auto: "true"` will only reload the pod, if the configmap or secret is used (as a volume mount or as an env) in `DeploymentConfigs/Deployment/Daemonsets/Statefulsets`
- `secret.reloader.stakater.com/reload` or `configmap.reloader.stakater.com/reload` annotation will reload the pod upon changes in specified configmap or secret, irrespective of the usage of configmap or secret.
- you may override the auto annotation with the `--auto-annotation` flag
- you may override the search annotation with the `--auto-search-annotation` flag
  and the match annotation with the `--search-match-annotation` flag
- you may override the configmap annotation with the `--configmap-annotation` flag
- you may override the secret annotation with the `--secret-annotation` flag
- you may want to prevent watching certain namespaces with the `--namespaces-to-ignore` flag
- you may want to prevent watching certain resources with the `--resources-to-ignore` flag
- you can configure logging in JSON format with the `--log-format=json` option
- you can configure the "reload strategy" with the `--reload-strategy=<strategy-name>` option (details below)

## Reload Strategies
Reloader supports multiple "reload" strategies for performing rolling upgrades to resources. The following list describes them:
- **env-vars**: When a tracked `configMap`/`secret` is updated, this strategy attaches a Reloader specific environment variable to any containers 
  referencing the changed `configMap` or `secret` on the owning resource (e.g., `Deployment`, `StatefulSet`, etc.).
  This strategy can be specified with the `--reload-strategy=env-vars` argument. Note: This is the default reload strategy.
- **annotations**: When a tracked `configMap`/`secret` is updated, this strategy attaches a `reloader.stakater.com/last-reloaded-from` pod template annotation
  on the owning resource (e.g., `Deployment`, `StatefulSet`, etc.). This strategy is useful when using resource syncing tools like ArgoCD, since it will not cause these tools
  to detect configuration drift after a resource is reloaded. Note: Since the attached pod template annotation only tracks the last reload source, this strategy will reload any tracked resource should its 
  `configMap` or `secret` be deleted and recreated.
  This strategy can be specified with the `--reload-strategy=annotations` argument.
  

## Deploying to Kubernetes

You can deploy Reloader by following methods:

### Vanilla Manifests

You can apply vanilla manifests by changing `RELEASE-NAME` placeholder provided in manifest with a proper value and apply it by running the command given below:

```bash
kubectl apply -f https://raw.githubusercontent.com/stakater/Reloader/master/deployments/kubernetes/reloader.yaml
```

By default, Reloader gets deployed in `default` namespace and watches changes `secrets` and `configmaps` in all namespaces.

Reloader can be configured to ignore the resources `secrets` and `configmaps` by passing the following args (`spec.template.spec.containers.args`) to its container :

| Args                             | Description          |
| -------------------------------- | -------------------- |
| --resources-to-ignore=configMaps | To ignore configMaps |
| --resources-to-ignore=secrets    | To ignore secrets    |

`Note`: At one time only one of these resource can be ignored, trying to do it will cause error in Reloader. Workaround for ignoring both resources is by scaling down the reloader pods to `0`.

### Vanilla kustomize

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

bases:
  - https://github.com/stakater/Reloader/deployments/kubernetes

namespace: reloader
```

### Helm Charts

Alternatively if you have configured helm on your cluster, you can add reloader to helm from our public chart repository and deploy it via helm using below mentioned commands. Follow [this](docs/Helm2-to-Helm3.md) guide, in case you have trouble migrating reloader from Helm2 to Helm3

```bash
helm repo add stakater https://stakater.github.io/stakater-charts

helm repo update

helm install stakater/reloader # For helm3 add --generate-name flag or set the release name
```

**Note:** By default reloader watches in all namespaces. To watch in single namespace, please run following command. It will install reloader in `test` namespace which will only watch `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts` in `test` namespace.

```bash
helm install stakater/reloader --set reloader.watchGlobally=false --namespace test # For helm3 add --generate-name flag or set the release name
```

Reloader can be configured to ignore the resources `secrets` and `configmaps` by using the following parameters of `values.yaml` file:

| Parameter        | Description                                                    | Type    |
| ---------------- | -------------------------------------------------------------- | ------- |
| ignoreSecrets    | To ignore secrets. Valid value are either `true` or `false`    | boolean |
| ignoreConfigMaps | To ignore configMaps. Valid value are either `true` or `false` | boolean |

`Note`: At one time only one of these resource can be ignored, trying to do it will cause error in helm template compilation.

You can also set the log format of Reloader to json by setting `logFormat` to `json` in values.yaml and apply the chart

You can enable to scrape Reloader's Prometheus metrics by setting `serviceMonitor.enabled` or `podMonitor.enabled`  to `true` in values.yaml file. Service monitor will be removed in future releases of reloader in favour of Pod monitor.

**Note:** Reloading of OpenShift (DeploymentConfig) and/or Argo Rollouts has to be enabled explicitly because it might not be always possible to use it on a cluster with restricted permissions. This can be done by changing the following parameters:

| Parameter        | Description                                                                  | Type    |
| ---------------- |------------------------------------------------------------------------------| ------- |
| isOpenshift      | Enable OpenShift DeploymentConfigs. Valid value are either `true` or `false` | boolean |
| isArgoRollouts   | Enable Argo Rollouts. Valid value are either `true` or `false`               | boolean |
| reloadOnCreate   | Enable reload on create events. Valid value are either `true` or `false`     | boolean |

## Help

### Documentation

You can find more documentation [here](docs)

### Have a question?

File a GitHub [issue](https://github.com/stakater/Reloader/issues), or send us an [email](mailto:stakater@gmail.com).

### Talk to us on Slack

Join and talk to us on Slack for discussing Reloader

[![Join Slack](https://stakater.github.io/README/stakater-join-slack-btn.png)](https://slack.stakater.com/)
[![Chat](https://stakater.github.io/README/stakater-chat-btn.png)](https://stakater-community.slack.com/messages/CC5S05S12)

## Contributing

### Bug Reports & Feature Requests

Please use the [issue tracker](https://github.com/stakater/Reloader/issues) to report any bugs or file feature requests.

### Developing

1. Deploy Reloader.
2. Run `okteto up` to activate your development container.
3. `make build`.
4. `./Reloader`

PRs are welcome. In general, we follow the "fork-and-pull" Git workflow.

1.  **Fork** the repo on GitHub
2.  **Clone** the project to your own machine
3.  **Commit** changes to your own branch
4.  **Push** your work back up to your fork
5.  Submit a **Pull request** so that we can review your changes

NOTE: Be sure to merge the latest from "upstream" before making a pull request!

## Changelog

View our closed [Pull Requests](https://github.com/stakater/Reloader/pulls?q=is%3Apr+is%3Aclosed).

## License

Apache2 © [Stakater](http://stakater.com)

## About

`Reloader` is maintained by [Stakater][website]. Like it? Please let us know at <hello@stakater.com>

See [our other projects][community]
or contact us in case of professional services and queries on <hello@stakater.com>

[website]: http://stakater.com/
[community]: https://github.com/stakater/

## Acknowledgements

- [ConfigmapController](https://github.com/fabric8io/configmapcontroller); We documented here why we re-created [Reloader](docs/Reloader-vs-ConfigmapController.md)
