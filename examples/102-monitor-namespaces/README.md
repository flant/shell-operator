## monitor-namespaces-hook example

Example of jqFilter usage to triggered a hook on changing namespace labels.


### Run 

Build shell-operator image with custom scripts:

```
$ docker build -t "registry.mycompany.com/shell-operator:monitor-namespaces" .
$ docker push registry.mycompany.com/shell-operator:monitor-namespaces
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
$ kubectl create ns example-monitor-namespaces
$ kubectl -n example-monitor-namespaces apply -f shell-operator-rbac.yaml  
$ kubectl -n example-monitor-namespaces apply -f shell-operator-pod.yaml
```

Create ns to trigget onKuberneteEvent:

```
$ kubectl create ns foobar
```

See in logs that hook was run:

```
$ kubectl -n example-monitor-namespaces logs po/shell-operator

...
Namespace foobar was created
...
```

### cleanup

```
$ kubectl delete ns/example-monitor-namespaces
$ docker rmi registry.mycompany.com/shell-operator:monitor-namespaces

```