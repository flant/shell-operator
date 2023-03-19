## example with helm chart

A helm version of [102-monitor-namespaces](https://github.com/flant/shell-operator/tree/master/examples/102-monitor-namespaces) example. It uses `ghcr.io/flant/shell-operator:latest` image in chart template to run shell-operator and a ConfigMap as a storage for hooks.


### Run

Tiller should be configured with ServiceAccount to be able to install releases in different namespaces:

```
kubectl create serviceaccount tiller --namespace kube-system 

kubectl create clusterrolebinding tiller --clusterrole=cluster-admin --serviceaccount=kube-system:tiller

helm init --service-account tiller
```

Install example to ns/example-helm:

```
helm install . --namespace example-helm --name example-helm
```

### See hook in action

1. Create ns/foobar to trigger a hook:

```
kubectl create ns foobar
```

See in logs that hook was run:

```
kubectl -n example-helm logs deploy/shell-operator

...
Namespace foobar was created
...
```

2. Delete ns/foobar to trigger a hook:

```
kubectl create ns foobar
```

See in logs that hook was run:

```
kubectl -n example-helm logs deploy/shell-operator

...
Namespace foobar was deleted
...
```

### Make changes

Deployment/shell-operator has annotation with checksum of hook file, so after editing namespace-hook.sh release can be upgraded without additional kubectl commands:

```
helm upgrade example-helm . --namespace example-helm
```

### Cleanup

```
helm delete --purge example-helm
kubectl delete ns/example-helm
```
