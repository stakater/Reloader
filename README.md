# RELOADER

## WHY NAME RELOADER

In english language, Reloader is a thing/tool that can reload certain stuff. So referring to that meaning reloader can reload

## Problem

We would like to watch if some change happens in `ConfigMap` and `Secret` objects and then perform certain upgrade on relevant `Deployment`, `Deamonset` and `Statefulset`

## Solution

Reloader can watch any changes in `ConfigMap` and `Secret` objects and update or recreate Pods for their associated `Deployments`, `Deamonsets` and `Statefulsets`. In this way Pods can get the latest changes in `ConfigMap` or `Secret` objects.

**NOTE:** This controller has been inspired from [configmapController](https://github.com/fabric8io/configmapcontroller)

## How to use Reloader

For a `Deployment` called `foo` have a `ConfigMap` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    reloader.stakater.com/configmap.update-on-change: "foo"
```

OR

For a `Deployment` called `foo` have a `Secret` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    reloader.stakater.com/secret.update-on-change: "foo"
```

Then, providing `Reloader` is running, whenever you edit the `ConfigMap` or `Secret` called `foo` the Reloader will update the `Deployment` by adding the environment variable:

```
STAKATER_FOO_REVISION=${reloaderRevision}
```

This then triggers a rolling upgrade of your deployment's pods to use the new configuration.

Same procedure can be followed to perform rolling upgrade on `Deamonsets` and `Statefulsets` as well.

## Deploying to Kubernetes

You can deploy Reloader by running the following kubectl commands:

```bash
kubectl apply -f rbac.yaml -n <namespace>
kubectl apply -f deployment.yaml -n <namespace>
```

### Helm Charts

Or alternatively if you configured `helm` on your cluster, you can deploy Reloader via helm chart located under `deployments/kubernetes/chart/reloader` folder.

## Help

**Got a question?**
File a GitHub [issue](https://github.com/stakater/Reloader/issues), or send us an [email](mailto:stakater@gmail.com).

### Talk to us on Slack

Join and talk to us on the #tools-imc channel for discussing Reloader

[![Join Slack](https://stakater.github.io/README/stakater-join-slack-btn.png)](https://stakater-slack.herokuapp.com/)
[![Chat](https://stakater.github.io/README/stakater-chat-btn.png)](https://stakater.slack.com/messages/CAN960CTG/)

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
