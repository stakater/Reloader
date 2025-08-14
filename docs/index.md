# Introduction

Reloader can watch changes in `ConfigMap` and `Secret` and do rolling upgrades on Pods with their associated `DeploymentConfigs`, `Deployments`, `Daemonsets` `Statefulsets` and `Rollouts`.

These are the key features of Reloader:

1. Restart pod in a `deployment` on change in linked/related `ConfigMaps` or `Secrets`
1. Restart pod in a `daemonset` on change in linked/related `ConfigMaps` or `Secrets`
1. Restart pod in a `statefulset` on change in linked/related `ConfigMaps` or `Secrets`
1. Restart pod in a `rollout` on change in linked/related `ConfigMaps` or `Secrets`

This site contains more details on how Reloader works. For an overview, please see the repository's [README file](https://github.com/stakater/Reloader/blob/master/README.md).

---

<div align="center">

[![💖 Sponsor our work](https://img.shields.io/badge/Sponsor%20Our%20Work-FF8C00?style=for-the-badge&logo=github-sponsors&logoColor=white)](https://github.com/sponsors/stakater?utm_source=docs&utm_medium=footer&utm_campaign=reloader)

<p>
Your support funds maintenance, security updates, and new features for Reloader, plus continued investment in other open source tools.
</p>

</div>

---
