## common library example

Example with common libraries.

### run

Build shell-operator image with custom scripts:

```shell
docker build -t "registry.mycompany.com/shell-operator:common-library" .
docker push registry.mycompany.com/shell-operator:common-library
```

Edit image in shell-operator-pod.yaml and apply manifests:

```shell
kubectl create ns example-common-library
kubectl -n example-common-library apply -f shell-operator-pod.yaml
```

Verify that hook.sh was run:

```shell
kubectl -n example-common-library logs -f po/shell-operator
...
INFO     : Running hook 'hook.sh' binding 'ON_STARTUP' ...
hook::trigger function is called!
...
```

### cleanup

```shell
kubectl delete ns/example-common-library
docker rmi registry.mycompany.com/shell-operator:common-library
```
