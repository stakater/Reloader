# ![](assets/web/reloader-round-100px.png) RELOADER

[![Go Report Card](https://goreportcard.com/badge/github.com/stakater/reloader?style=flat-square)](https://goreportcard.com/report/github.com/stakater/reloader)
[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/stakater/reloader)
[![Release](https://img.shields.io/github/release/stakater/reloader.svg?style=flat-square)](https://github.com/stakater/reloader/releases/latest)
[![GitHub tag](https://img.shields.io/github/tag/stakater/reloader.svg?style=flat-square)](https://github.com/stakater/reloader/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/stakater/reloader.svg?style=flat-square)](https://hub.docker.com/r/stakater/reloader/)
[![Docker Stars](https://img.shields.io/docker/stars/stakater/reloader.svg?style=flat-square)](https://hub.docker.com/r/stakater/reloader/)
[![MicroBadger Size](https://img.shields.io/microbadger/image-size/stakater/reloader.svg?style=flat-square)](https://microbadger.com/images/stakater/reloader)
[![MicroBadger Layers](https://img.shields.io/microbadger/layers/stakater/reloader.svg?style=flat-square)](https://microbadger.com/images/stakater/reloader)
[![license](https://img.shields.io/github/license/stakater/reloader.svg?style=flat-square)](LICENSE)
[![Get started with Stakater](https://stakater.github.io/README/stakater-github-banner.png)](http://stakater.com/?utm_source=Reloader&utm_medium=github)

## Problem

We would like to watch if some change happens in `ConfigMap` and/or `Secret`; then perform a rolling upgrade on relevant `DeploymentConfig`, `Deployment`, `Daemonset` and `Statefulset`

## Solution

Reloader can watch changes in `ConfigMap` and `Secret` and do rolling upgrades on Pods with their associated `DeploymentConfigs`, `Deployments`, `Daemonsets` and `Statefulsets`.

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

This will discover deployments/daemonsets/statefulset automatically where `foo-configmap` or `foo-secret` is being used either via environment variable or from volume mount. And it will perform rolling upgrade on related pods when `foo-configmap` or `foo-secret`are updated.

We can also specify a specific configmap or secret which would trigger rolling upgrade only upon change in our specified configmap or secret, this way, it will not trigger rolling upgrade upon changes in all configmaps or secrets used in a deployment, daemonset or statefulset.
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

For a `Deployment` called `foo` have a `Secret` called `foo-secret`. Then add this annotation to main metadata of  your `Deployment`

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
- `reloader.stakater.com/auto: "true"` will only reload the pod, if the configmap or secret is used (as a volume mount or as an env) in `DeploymentConfigs/Deployment/Daemonsets/Statefulsets`
- `secret.reloader.stakater.com/reload` or `configmap.reloader.stakater.com/reload` annotation will reload the pod upon changes in specified configmap or secret, irrespective of the usage of configmap or secret.
- you may override the auto annotation with the `--auto-annotation` flag
- you may override the configmap annotation with the `--configmap-annotation` flag
- you may override the secret annotation with the `--secret-annotation` flag

## Deploying to Kubernetes

You can deploy Reloader by following methods:

### Vanilla Manifests

You can apply vanilla manifests by running the following command

```bash
kubectl apply -f https://raw.githubusercontent.com/stakater/Reloader/master/deployments/kubernetes/reloader.yaml
```

By default Reloader gets deployed in `default` namespace and watches changes `secrets` and `configmaps` in all namespaces. 

Reloader can be configured to ignore the resources `secrets` and `configmaps` by passing the following args (`spec.template.spec.containers.args`) to its container :

| Args | Description |
|---|---|
| --resources-to-ignore=configMaps | To ignore configMaps |
| --resources-to-ignore=secrets | To ignore secrets |



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

namePrefix: reloader-

bases:
  - https://github.com/stakater/Reloader/deployments/kubernetes

namespace: reloader
```

### Helm Charts

Alternatively if you have configured helm on your cluster, you can add reloader to helm from our public chart repository and deploy it via helm using below mentioned commands

 ```bash
helm repo add stakater https://stakater.github.io/stakater-charts

helm repo update

helm install stakater/reloader
```

**Note:**  By default reloader watches in all namespaces. To watch in single namespace, please run following command. It will install reloader in `test` namespace which will only watch `Deployments`, `Daemonsets` and `Statefulsets` in `test` namespace.

```bash
helm install stakater/reloader --set reloader.watchGlobally=false --namespace test
```

Reloader can be configured to ignore the resources `secrets` and `configmaps` by using the following parameters of `values.yaml` file:

| Parameter | Description |
|---|---|
| ignoreSecrets | To ignore secrets |
| ignoreConfigMaps | To ignore configMaps |


## Help

### Documentation
You can find more documentation [here](docs/)

### Have a question?
File a GitHub [issue](https://github.com/stakater/Reloader/issues), or send us an [email](mailto:stakater@gmail.com).

### Talk to us on Slack

Join and talk to us on Slack for discussing Reloader

[![Join Slack](https://stakater.github.io/README/stakater-join-slack-btn.png)](https://stakater-slack.herokuapp.com/)
[![Chat](https://stakater.github.io/README/stakater-chat-btn.png)](https://stakater.slack.com/messages/CC5S05S12)

## Contributing

### Bug Reports & Feature Requests

Please use the [issue tracker](https://github.com/stakater/Reloader/issues) to report any bugs or file feature requests.

### Developing

PRs are welcome. In general, we follow the "fork-and-pull" Git workflow.

 1. **Fork** the repo on GitHub
 2. **Clone** the project to your own machine
 3. **Commit** changes to your own branch
 4. **Push** your work back up to your fork
 5. Submit a **Pull request** so that we can review your changes

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
