# Reloader Helm Chart

If you have configured helm on your cluster, you can add Reloader to helm from our public chart repository and deploy it via helm using below-mentioned commands. Follow [this](docs/Helm2-to-Helm3.md) guide, in case you have trouble migrating Reloader from Helm2 to Helm3.

## Installation

```bash
helm repo add stakater https://stakater.github.io/stakater-charts

helm repo update

helm install stakater/reloader # For helm3 add --generate-name flag or set the release name

helm install {{RELEASE_NAME}} stakater/reloader -n {{NAMESPACE}} --set reloader.watchGlobally=false # By default, Reloader watches in all namespaces. To watch in single namespace, set watchGlobally=false

helm install stakater/reloader --set reloader.watchGlobally=false --namespace test --generate-name # Install Reloader in `test` namespace which will only watch `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts` in `test` namespace.
```

## Uninstalling

```bash
helm uninstall {{RELEASE_NAME}} -n {{NAMESPACE}}
```

## Parameters

### Global Parameters

| Parameter                 | Description                                                     | Type  | Default |
| ------------------------- | --------------------------------------------------------------- | ----- | ------- |
| `global.imagePullSecrets` | Reference to one or more secrets to be used when pulling images | array | `[]`    |

### Common Parameters

| Parameter          | Description                              | Type   | Default           |
| ------------------ | ---------------------------------------- | ------ | ----------------- |
| `nameOverride`     | replace the name of the chart            | string | `""`              |
| `fullnameOverride` | replace the generated name               | string | `""`              |
| `image`            | Set container image name, tag and policy | map    | `see values.yaml` |

### Core Reloader Parameters

| Parameter                           | Description                                                                                                                                         | Type        | Default   |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- | --------- |
| `reloader.autoReloadAll`            |                                                                                                                                                     | boolean     | `false`   |
| `reloader.isArgoRollouts`           | Enable Argo `Rollouts`. Valid value are either `true` or `false`                                                                                    | boolean     | `false`   |
| `reloader.isOpenshift`              | Enable OpenShift DeploymentConfigs. Valid value are either `true` or `false`                                                                        | boolean     | `false`   |
| `reloader.ignoreSecrets`            | To ignore secrets. Valid value are either `true` or `false`. Either `ignoreSecrets` or `ignoreConfigMaps` can be ignored, not both at the same time | boolean     | `false`   |
| `reloader.ignoreConfigMaps`         | To ignore configmaps. Valid value are either `true` or `false`                                                                                      | boolean     | `false`   |
| `reloader.reloadOnCreate`           | Enable reload on create events. Valid value are either `true` or `false`                                                                            | boolean     | `false`   |
| `reloader.reloadOnDelete`           | Enable reload on delete events. Valid value are either `true` or `false`                                                                            | boolean     | `false`   |
| `reloader.syncAfterRestart`         | Enable sync after Reloader restarts for **Add** events, works only when reloadOnCreate is `true`. Valid value are either `true` or `false`          | boolean     | `false`   |
| `reloader.reloadStrategy`           | Strategy to trigger resource restart, set to either `default`, `env-vars` or `annotations`                                                          | enumeration | `default` |
| `reloader.ignoreNamespaces`         | List of comma separated namespaces to ignore, if multiple are provided, they are combined with the AND operator                                     | string      | `""`      |
| `reloader.namespaceSelector`        | List of comma separated namespaces to select, if multiple are provided, they are combined with the AND operator                                     | string      | `""`      |
| `reloader.resourceLabelSelector`    | List of comma separated label selectors, if multiple are provided they are combined with the AND operator                                           | string      | `""`      |
| `reloader.logFormat`                | Set type of log format. Value could be either `json` or `""`                                                                                        | string      | `""`      |
| `reloader.watchGlobally`            | Allow Reloader to watch in all namespaces (`true`) or just in a single namespace (`false`)                                                          | boolean     | `true`    |
| `reloader.enableHA`                 | Enable leadership election allowing you to run multiple replicas                                                                                    | boolean     | `false`   |
| `reloader.readOnlyRootFileSystem`   | Enforce readOnlyRootFilesystem                                                                                                                      | boolean     | `false`   |
| `reloader.legacy.rbac`              |                                                                                                                                                     | boolean     | `false`   |
| `reloader.matchLabels`              | Pod labels to match                                                                                                                                 | map         | `{}`      |
| `reloader.enableMetricsByNamespace` | Expose an additional Prometheus counter of reloads by namespace (this metric may have high cardinality in clusters with many namespaces)            | boolean     | `false`   |

### Deployment Reloader Parameters

| Parameter                                       | Description                                                                                                                                                 | Type   | Default           |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ----------------- |
| `reloader.deployment.replicas`                  | Number of replicas, if you wish to run multiple replicas set `reloader.enableHA = true`. The replicas will be limited to 1 when `reloader.enableHA = false` | int    | 1                 |
| `reloader.deployment.revisionHistoryLimit`      | Limit the number of revisions retained in the revision history                                                                                              | int    | 2                 |
| `reloader.deployment.nodeSelector`              | Scheduling pod to a specific node based on set labels                                                                                                       | map    | `{}`              |
| `reloader.deployment.affinity`                  | Set affinity rules on pod                                                                                                                                   | map    | `{}`              |
| `reloader.deployment.securityContext`           | Set pod security context                                                                                                                                    | map    | `{}`              |
| `reloader.deployment.containerSecurityContext`  | Set container security context                                                                                                                              | map    | `{}`              |
| `reloader.deployment.tolerations`               | A list of `tolerations` to be applied to the deployment                                                                                                     | array  | `[]`              |
| `reloader.deployment.topologySpreadConstraints` | Topology spread constraints for pod assignment                                                                                                              | array  | `[]`              |
| `reloader.deployment.annotations`               | Set deployment annotations                                                                                                                                  | map    | `{}`              |
| `reloader.deployment.labels`                    | Set deployment labels, default to Stakater settings                                                                                                         | array  | `see values.yaml` |
| `reloader.deployment.env`                       | Support for extra environment variables                                                                                                                     | array  | `[]`              |
| `reloader.deployment.livenessProbe`             | Set liveness probe timeout values                                                                                                                           | map    | `{}`              |
| `reloader.deployment.readinessProbe`            | Set readiness probe timeout values                                                                                                                          | map    | `{}`              |
| `reloader.deployment.resources`                 | Set container requests and limits (e.g. CPU or memory)                                                                                                      | map    | `{}`              |
| `reloader.deployment.pod.annotations`           | Set annotations for pod                                                                                                                                     | map    | `{}`              |
| `reloader.deployment.priorityClassName`         | Set priority class for pod in cluster                                                                                                                       | string | `""`              |

### Other Reloader Parameters

| Parameter                              | Description                                                     | Type    | Default |
| -------------------------------------- | --------------------------------------------------------------- | ------- | ------- |
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

## Release Process

_Helm chart versioning_: The Reloader Helm chart is maintained in [this repository](./deployments/kubernetes/chart/reloader). The Helm chart has its own semantic versioning. Helm charts and code releases are separate artifacts and separately versioned. Manifest making strategy relies on Kustomize. The Reloader Helm chart manages the two artifacts with these two fields:

- [`appVersion`](./deployments/kubernetes/chart/reloader/Chart.yaml) points to released Reloader application image version listed on the [releases page](https://github.com/stakater/Reloader/releases)
- [`version`](./deployments/kubernetes/chart/reloader/Chart.yaml) sets the Reloader Helm chart version

Helm chart will be released to the chart registry whenever files in `deployments/kubernetes/chart/reloader/**` change on the main branch.

Helm Chart will be released by the maintainers, on labelling a PR with `release/helm-chart` and pre-maturely updating the `version` field in `Chart.yaml` file.
