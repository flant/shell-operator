## common library example

Example with common libraries.

### run

Build shell-operator image with custom scripts:

```
$ docker build -t "registry.mycompany.com/shell-operator:common-library" .
$ docker push registry.mycompany.com/shell-operator:common-library
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
$ kubectl create ns example-common-library
$ kubectl -n example-common-library apply -f shell-operator-pod.yaml
```

See in logs that hook.sh was run:

```
$ kubectl -n example-common-library logs -f po/shell-operator
...
INFO     : Running hook 'hook.sh' binding 'ON_STARTUP' ...
hook::trigger function is called!
...
```

### cleanup

```
$ kubectl delete ns/example-common-library
$ docker rmi registry.mycompany.com/shell-operator:common-library
```
