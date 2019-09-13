## monitor-events example

PoC example of monitoring Event objects.

### Run

Build shell-operator image with custom script:

```
docker build -t "registry.mycompany.com/shell-operator:monitor-events" .
docker push registry.mycompany.com/shell-operator:monitor-events
```

Edit image in shell-operator-pod.yaml and apply manifests:

```
kubectl create ns example-monitor-events
kubectl -n example-monitor-events apply -f shell-operator-rbac.yaml  
kubectl -n example-monitor-events apply -f shell-operator-pod.yaml
```

Create faulty pod to get scary Event:

```
$ kubectl -n example-monitor-events apply -f failed-pod.yaml
```

Events are appeared in logs (the output is greatly reduced):

```
$ kubectl -n example-monitor-events logs po/shell-operator

...
Got Event: 
{
  ...
  "note": "Successfully assigned example-monitor-events/failed-pod to kube-13-worker",
  "reason": "Scheduled",
  "regarding": {
    "apiVersion": "v1",
    "kind": "Pod",
    "name": "failed-pod",
    "namespace": "example-monitor-events",
    "resourceVersion": "133188",
    "uid": "1e557d91-d5f0-11e9-8ed3-0242ac110004"
  },
  ...
}
... 
Got Event: 
{ ...
  "note": "Container image \"alpine:3.9\" already present on machine",
  "reason": "Pulled",
  ...
}
...
Got Event: 
{
  "note": "Created container",
  "reason": "Created",
}
...
Got Event: 
{
  "note": "Started container",
  "reason": "Started",
}
...
Got Event: 
{
  "note": "Container image \"alpine:3.9\" already present on machine",
  "reason": "Pulled",
}
...
Got Event: 
{
  "note": "Created container",
  "reason": "Created",
}
...
Got Event: 
{
  "note": "Started container",
  "reason": "Started",
}
...
Got Event: 
{
  "note": "Back-off restarting failed container",
  "reason": "BackOff",
}
...
Got Event: 
{
  "note": "Back-off restarting failed container",
  "reason": "BackOff",
}
...
```

> Note: this example is only a PoC based on comments in [issue #22](https://github.com/flant/shell-operator/issues/22). For now you can't handle only BackOff events from Pods of particular Deployment, hook is run for every Event from every Pod in Namespace. Binding to Events should be more configurable, this issue will be addressed in future.

### cleanup

```
$ kubectl delete clusterrolebinding/monitor-events
$ kubectl delete clusterrole/monitor-events
$ kubectl delete ns/example-monitor-events
$ docker rmi registry.mycompany.com/shell-operator:monitor-events
```
