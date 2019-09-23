## crd-simple example

Example of monitoring custom resources.

### Run

Build shell-operator image with custom script:

```
docker build -t "registry.mycompany.com/shell-operator:crd-simple" .
docker push registry.mycompany.com/shell-operator:crd-simple
```

Edit image in shell-operator-pod.yaml and apply manifests, also create CRD before starting Shell-operator:

```
kubectl create ns example-crd-simple
kubectl -n example-crd-simple apply -f shell-operator-rbac.yaml
kubectl -n example-crd-simple apply -f crd-simple.yaml  
kubectl -n example-crd-simple apply -f shell-operator-pod.yaml
```

crd-simple.yaml defines a Crontab kind. Create new Crontab object:

```
kubectl -n example-crd-simple apply -f cr-crontab.yaml
```

See in logs that hook was run:

```
kubectl -n example-crd-simple logs po/shell-operator


... 
TASK_RUN HookRun@KUBERNETES crd-hook.sh
...
CronTab/crontab object is added
...
```

### cleanup

```
kubectl delete clusterrolebinding/crd-simple
kubectl delete clusterrole/crd-simple
kubectl delete ns/example-crd-simple
kubectl delete crd/crontabs.stable.example.com
docker rmi registry.mycompany.com/shell-operator:crd-simple
```
