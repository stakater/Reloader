# RELOADER

## Problem

We would like to watch if some change happens in `ConfigMap` and `Secret` objects and then perform rolling upgrade on relevant `Deployment`, `Deamonset` and `Statefulset`

## Solution

Reloader can watch changes in `ConfigMap` and `Secret` and do rolling upgrades on Pods with their associated `Deployments`, `Deamonsets` and `Statefulsets`.

**NOTE:** This controller has been inspired from [configmapController](https://github.com/fabric8io/configmapcontroller)

## How to use Reloader

For a `Deployment` called `foo` have a `ConfigMap` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    configmap.reloader.stakater.com/reload: "foo"
```

OR

For a `Deployment` called `foo` have a `Secret` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    secret.reloader.stakater.com/reload: "foo"
```

## Deploying to Kubernetes

You can deploy Reloader by following methods:

### Vanilla Manifests

You can apply vanilla manifests by running the following command

```bash
kubectl apply -f https://raw.githubusercontent.com/stakater/Reloader/master/deployments/kubernetes/reloader.yaml
```

By default Reloader gets deployed in `default` namespace and watches changes `secrets` and `configmaps` in all namespaces.

### Helm Charts

Alternatively if you have configured helm on your cluster, you can add reloader to helm from our public chart repository and deploy it via helm using below mentioned commands

 ```bash
helm repo add stakater https://stakater.github.io/stakater-charts

helm repo update

helm install stakater/reloader
```

## Help

**Got a question?**
File a GitHub [issue](https://github.com/stakater/Reloader/issues), or send us an [email](mailto:stakater@gmail.com).

### Talk to us on Slack

Join and talk to us on Slack for discussing Reloader

[![Join Slack](https://stakater.github.io/README/stakater-join-slack-btn.png)](https://stakater-slack.herokuapp.com/)
[![Chat](https://stakater.github.io/README/stakater-chat-btn.png)](https://stakater.slack.com/)

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
