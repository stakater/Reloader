# Reloader vs k8s-trigger-controller

Reloader and k8s-trigger-controller are both built for same purpose. So there are quite a few similarities and differences between these.

## Similarities

- Both controllers support change detection in configmap and secrets
- Both controllers support deployment `rollout`
- Both controllers use SHA1 for hashing
- Both controllers have end to end as well as unit test cases.

## Differences

### Support for `Daemonsets` and `Statefulsets`

#### `k8s-trigger-controller`:

`k8s-trigger-controller` only support for deployment `rollout`. It does not support `daemonsets` and `statefulsets` `rollout`.

#### Reloader:

Reloader supports deployment `rollout` as well as `daemonsets` and `statefulsets` `rollout`.

### Hashing usage

#### `k8s-trigger-controller`:

`k8s-trigger-controller` stores the hash value in an annotation `trigger.k8s.io/[secret|configMap]-NAME-last-hash`

#### Reloader:

Reloader stores the hash value in an environment variable `STAKATER_NAME_[SECRET|CONFIGMAP]`

### Customization

#### `k8s-trigger-controller`:

`k8s-trigger-controller` restricts you to using the `trigger.k8s.io/[secret-configMap]-NAME-last-hash` annotation

#### Reloader:

Reloader allows you to customize the annotation to fit your needs with command line flags:

- `--auto-annotation <annotation>`
- `--configmap-annotation <annotation>`
- `--secret-annotation <annotation>`
