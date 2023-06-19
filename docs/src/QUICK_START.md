# Quickstart

> You need to have a Kubernetes cluster, and the `kubectl` must be configured to communicate with your cluster.

The simplest setup of shell-operator in your cluster consists of these steps:

- build an image with your hooks (scripts)
- create necessary RBAC objects (for `kubernetes` bindings)
- run Pod or Deployment with the built image

For more configuration options see [RUNNING](RUNNING.md).

## Build an image with your hooks

A hook is a script that, when executed with `--config` option, outputs configuration to stdout in YAML or JSON format. [Learn more](HOOKS.md) about hooks.

Let's create a small operator that will watch for all Pods in all Namespaces and simply log the name of a new Pod.

`kubernetes` binding is used to tell shell-operator about objects that we want to watch. Create the `pods-hook.sh` file with the following content:

```bash
#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
configVersion: v1
kubernetes:
- apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
EOF
else
  podName=$(jq -r .[0].object.metadata.name $BINDING_CONTEXT_PATH)
  echo "Pod '${podName}' added"
fi
```

Make the `pods-hook.sh` executable:

```sh
chmod +x pods-hook.sh
```

You can use a prebuilt image [ghcr.io/flant/shell-operator:latest][shell-operator-container-image] with `bash`, `kubectl`, `jq` and `shell-operator` binaries to build you own image. You just need to `ADD` your hook into `/hooks` directory in the `Dockerfile`.

Create the following `Dockerfile` in the directory where you created the `pods-hook.sh` file:

```dockerfile
FROM ghcr.io/flant/shell-operator:latest
ADD pods-hook.sh /hooks
```

Build an image (change image tag according to your Docker registry):

```sh
docker build -t "registry.mycompany.com/shell-operator:monitor-pods" .
```

Push image to the Docker registry accessible by the Kubernetes cluster:

```sh
docker push registry.mycompany.com/shell-operator:monitor-pods
```

## Create RBAC objects

We need to watch for Pods in all Namespaces. That means that we need specific RBAC definitions for shell-operator:

```sh
kubectl create namespace example-monitor-pods
kubectl create serviceaccount monitor-pods-acc --namespace example-monitor-pods
kubectl create clusterrole monitor-pods --verb=get,watch,list --resource=pods
kubectl create clusterrolebinding monitor-pods --clusterrole=monitor-pods --serviceaccount=example-monitor-pods:monitor-pods-acc
```

## Install shell-operator in a cluster

Shell-operator can be deployed as a Pod. Put this manifest into the `shell-operator-pod.yaml` file:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: shell-operator
spec:
  containers:
  - name: shell-operator
    image: registry.mycompany.com/shell-operator:monitor-pods
    imagePullPolicy: Always
  serviceAccountName: monitor-pods-acc
```

Start shell-operator by applying a `shell-operator-pod.yaml` file:

```sh
kubectl -n example-monitor-pods apply -f shell-operator-pod.yaml
```

## It all comes together

Let's deploy a [kubernetes-dashboard][kubernetes-dashboard] to trigger `kubernetes` binding defined in our hook:

```sh
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/master/aio/deploy/recommended.yaml
```

Now run `kubectl -n example-monitor-pods logs po/shell-operator` and observe that the hook will print dashboard pod name:

```plain
...
INFO[0027] queue task HookRun:main                       operator.component=handleEvents queue=main
INFO[0030] Execute hook                                  binding=kubernetes hook=pods-hook.sh operator.component=taskRunner queue=main task=HookRun
INFO[0030] Pod 'kubernetes-dashboard-775dd7f59c-hr7kj' added  binding=kubernetes hook=pods-hook.sh output=stdout queue=main task=HookRun
INFO[0030] Hook executed successfully                    binding=kubernetes hook=pods-hook.sh operator.component=taskRunner queue=main task=HookRun
...
```

> *Note:* hook output is logged with output=stdout label.

To clean up a cluster, delete namespace and RBAC objects:

```sh
kubectl delete ns example-monitor-pods
kubectl delete clusterrole monitor-pods
kubectl delete clusterrolebinding monitor-pods
```

This example is also available in /examples: [monitor-pods][pods-example].

[kubernetes-dashboard]: https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/
[pods-example]: https://github.com/flant/shell-operator/tree/main/examples/101-monitor-pods
[shell-operator-container-image]: https://github.com/flant/shell-operator/pkgs/container/shell-operator
