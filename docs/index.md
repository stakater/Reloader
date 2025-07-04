# Introduction

Reloader can watch changes in `ConfigMap` and `Secret` and do rolling upgrades on Pods with their associated `DeploymentConfigs`, `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts`.

These are the key features of Reloader:

1. Restart pod in a `deployment` on change in linked/related `ConfigMaps` or `Secrets`
1. Restart pod in a `daemonset` on change in linked/related `ConfigMaps` or `Secrets`
1. Restart pod in a `statefulset` on change in linked/related `ConfigMaps` or `Secrets`
1. Restart pod in a `rollout` on change in linked/related `ConfigMaps` or `Secrets`

This site contains more details on how Reloader works. For an overview, please see the repository's [README file](https://github.com/stakater/Reloader/blob/master/README.md).
