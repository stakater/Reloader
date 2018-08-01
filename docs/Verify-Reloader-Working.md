# Verify Reloader's Working

Reloader's working can be verified by two ways.

## Verify from logs

Check the logs of reloader and verify that you can see logs looks like below, if you are able to find these logs then it means reloader is working.

```text
Changes Detected in test-object of type 'SECRET' in namespace: test-reloader

Updated test-resource of type Deployment in namespace: test-reloader
```

Below are the details that explain these logs:

### test-object

`test-object` is the name of a `secret` or a `deployment` in which change has been detected.

### SECRET

`SECRET` is the type of `test-object`. It can either be `SECRET` or `CONFIGMAP`

### test-reloader

`test-reloader` is the name of namespace in which reloader has detected the change.

### test-resource

`test-resource` is the name of resource which is going to be updated

### Deployment

`Deployment` is the type of `test-resource`. It can either be a `Deployment`, `Daemonset` or `Statefulset`

## Verify by checking the age of Pod

A pod's age can tell whether reloader is working correctly or not. If you know that a change in a `secret` or `configmap` has occurred, then check the relevant Pod's age immediately. It should be newly created few moments ago.

### Verify from kubernetes Dashboard

`kubernetes dashboard` can be used to verify the working of Reloader. After a change in `secret` or `configmap`, check the relevant Pod's age from dashboard. It should be newly created few moments ago.

### Verify from command line

After a change in `secret` or `configmap`. Run the below mentioned command and verify that the pod is newly created.

```bash
kubectl get pods <pod name> -n <namespace name>
```
