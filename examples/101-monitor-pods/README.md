## monitor-pods-hook example

Example of monitoring new Pods.

### Run 

Build shell-operator image with custom script:

```
$ docker build -t "registry.mycompany.com/shell-operator:monitor-pods" .
$ docker push registry.mycompany.com/shell-operator:monitor-pods
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
$ kubectl create ns example-monitor-pods
$ kubectl -n example-monitor-pods apply -f shell-operator-rbac.yaml  
$ kubectl -n example-monitor-pods apply -f shell-operator-pod.yaml
```

Scale kubernetes-dashboard to trigger onKuberneteEvent:

```
$ kubectl -n kube-system scale --replicas=1 deploy/kubernetes-dashboard
```

See in logs that hook was run:

```
$ kubectl -n example-monitor-pods logs po/shell-operator

...
Pod 'kubernetes-dashboard-769df5545f-pzg7x' added
...
Pod 'kubernetes-dashboard-769df5545f-xnmdl' added
...
```

### cleanup

```
$ kubectl delete clusterrolebinding/monitor-pods
$ kubectl delete clusterrole/monitor-pods
$ kubectl delete ns/example-monitor-pods
$ docker rmi registry.mycompany.com/shell-operator:monitor-pods
```