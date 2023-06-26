# Helm2 to Helm3 Migration

Follow below-mentioned instructions to migrate Reloader from Helm2 to Helm3

## Instructions

There are 3 steps involved in migrating the Reloader from Helm2 to Helm3.

### Step 1

Install the helm-2to3 plugin

```bash
helm3 plugin install https://github.com/helm/helm-2to3

helm3 2to3 convert <release-name>

helm3 2to3 cleanup --release-cleanup --skip-confirmation
```

### Step 2

Add the following Helm3 labels and annotations on Reloader resources.

Label:

```yaml
app.kubernetes.io/managed-by=Helm
```

Annotations:

```yaml
meta.helm.sh/release-name=<release-name>
meta.helm.sh/release-namespace=<namespace>
```

For example, to label and annotate the ClusterRoleBinding and ClusterRole:

```bash
KIND=ClusterRoleBinding
NAME=reloader-reloader-role-binding
RELEASE=reloader
NAMESPACE=kube-system
kubectl annotate $KIND $NAME meta.helm.sh/release-name=$RELEASE
kubectl annotate $KIND $NAME meta.helm.sh/release-namespace=$NAMESPACE
kubectl label $KIND $NAME app.kubernetes.io/managed-by=Helm

KIND=ClusterRole
NAME=reloader-reloader-role
RELEASE=reloader
NAMESPACE=kube-system
kubectl annotate $KIND $NAME meta.helm.sh/release-name=$RELEASE
kubectl annotate $KIND $NAME meta.helm.sh/release-namespace=$NAMESPACE
kubectl label $KIND $NAME app.kubernetes.io/managed-by=Helm
```

### Step 3

Upgrade to desired version

```bash
helm3 repo add stakater https://stakater.github.io/stakater-charts

helm3 repo update

helm3 upgrade <release-name> stakater/reloader --version=v0.0.72
```
