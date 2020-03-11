## onStartup shell example

Example of a hook written as bash script.

### run

Build Shell-operator image with custom scripts:

```
docker build -t "registry.mycompany.com/shell-operator:startup-shell" .
docker push registry.mycompany.com/shell-operator:startup-shell
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
kubectl create ns example-startup-shell
kubectl -n example-startup-shell apply -f shell-operator-pod.yaml
```

See in logs that shell-hook.sh was run:

```
kubectl -n example-startup-shell logs -f po/shell-operator
...
OnStartup shell hook
...
```

### cleanup

```
kubectl delete ns/example-startup-shell
docker rmi registry.mycompany.com/shell-operator:startup-shell
```
