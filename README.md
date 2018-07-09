# Reloader

This controller watches for changes to `ConfigMap` and `Secret` objects and performs rolling upgrades on their associated deployments, deamonsets and statefulsets and updating dynamically.  

This is particularly useful if the `ConfigMap` is used to define environment variables - or your app cannot easily and reliably watch the `ConfigMap` and update itself on the fly. 

## How to use Reloader

For a `Deployment` called `foo` have a `ConfigMap` called `foo`. Then add this annotation to your `Deployment`

```yaml
metadata:
  annotations:
    configmap.fabric8.io/update-on-change: "foo"
```

Then, providing `Reloader` is running, whenever you edit the `ConfigMap` called `foo` the Reloader will update the `Deployment` by adding the environment variable:

```
FABRICB_FOO_REVISION=${configMapRevision}
```

This then triggers a rolling upgrade of your deployment's pods to use the new configuration.
