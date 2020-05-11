<p align="center">
<img width="485" src="docs/shell-operator-logo.png" alt="Shell-operator logo" />
</p>

<p align="center">
<a href="https://hub.docker.com/r/flant/shell-operator"><img src="https://img.shields.io/docker/pulls/flant/shell-operator.svg?logo=docker" alt="docker pull flant/shell-operator"/></a>
<a href="https://cloud-native.slack.com/messages/CJ13K3HFG"><img src="https://img.shields.io/badge/slack-EN%20chat-611f69.svg?logo=slack" alt="Slack chat EN"/></a>
<a href="https://t.me/kubeoperator"><img src="https://img.shields.io/badge/telegram-RU%20chat-179cde.svg?logo=telegram" alt="Telegram chat RU"/></a>
</p>

**Shell-operator** is a tool for running event-driven scripts in a Kubernetes cluster.

This operator is not an operator for a _particular software product_ such as `prometheus-operator` or `kafka-operator`. Shell-operator provides an integration layer between Kubernetes cluster events and shell scripts by treating scripts as hooks triggered by events. Think of it as an `operator-sdk` but for scripts.

Shell-operator is used as a base for more advanced [Addon-operator](https://github.com/flant/addon-operator) that supports Helm charts and value storages.

Shell-operator provides:

- __Ease of management of a Kubernetes cluster__: use the tools that Ops are familiar with. It can be bash, python, kubectl, etc.
- __Kubernetes object events__: hook can be triggered by `add`, `update` or `delete` events. **Learn [more](HOOKS.md) about hooks.**
- __Object selector and properties filter__: Shell-operator can monitor a particular set of objects and detect changes in their properties.
- __Simple configuration__: hook binding definition is a JSON or YAML document on script's stdout.

## Quickstart

> You need to have a Kubernetes cluster, and the `kubectl` must be configured to communicate with your cluster.

The simplest setup of Shell-operator in your cluster consists of these steps:

- build an image with your hooks (scripts)
- create necessary RBAC objects (for `kubernetes` bindings)
- run Pod or Deployment with the built image

For more configuration options see [RUNNING](RUNNING.md).

### Build an image with your hooks

A hook is a script that, when executed with `--config` option, outputs configuration to stdout in YAML or JSON format. Learn [more](HOOKS.md) about hooks.

Let's create a small operator that will watch for all Pods in all Namespaces and simply log the name of a new Pod.

`kubernetes` binding is used to tell Shell-operator about objects that we want to watch. Create the `pods-hook.sh` file with the following content:

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

```shell
chmod +x pods-hook.sh
```

You can use a prebuilt image [flant/shell-operator:latest](https://hub.docker.com/r/flant/shell-operator) with `bash`, `kubectl`, `jq` and `shell-operator` binaries to build you own image. You just need to `ADD` your hook into `/hooks` directory in the `Dockerfile`.

Create the following `Dockerfile` in the directory where you created the `pods-hook.sh` file:

```dockerfile
FROM flant/shell-operator:latest
ADD pods-hook.sh /hooks
```

Build an image (change image tag according to your Docker registry):

```shell
docker build -t "registry.mycompany.com/shell-operator:monitor-pods" .
```

Push image to the Docker registry accessible by the Kubernetes cluster:

```shell
docker push registry.mycompany.com/shell-operator:monitor-pods
```

### Create RBAC objects

We need to watch for Pods in all Namespaces. That means that we need specific RBAC definitions for shell-operator:

```shell
kubectl create namespace example-monitor-pods
kubectl create serviceaccount monitor-pods-acc --namespace example-monitor-pods
kubectl create clusterrole monitor-pods --verb=get,watch,list --resource=pods
kubectl create clusterrolebinding monitor-pods --clusterrole=monitor-pods --serviceaccount=example-monitor-pods:monitor-pods-acc
```

### Install Shell-operator in a cluster

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

Start Shell-operator by applying a `shell-operator-pod.yaml` file:

```shell
kubectl -n example-monitor-pods apply -f shell-operator-pod.yaml
```

### It all comes together

Let's deploy a [kubernetes-dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/) to trigger  `kubernetes` binding defined in our hook:

```shell
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

```shell
kubectl delete ns example-monitor-pods
kubectl delete clusterrole monitor-pods
kubectl delete clusterrolebinding monitor-pods
```

This example is also available in /examples: [monitor-pods](examples/101-monitor-pods).

## Hook binding types

Every hook should respond with JSON or YAML configuration of bindings when executed with `--config` flag.

### kubernetes

This binding defines a subset of Kubernetes objects that Shell-operator will monitor and a [jq](https://github.com/stedolan/jq/) expression to filter their properties. Read more about `onKubernetesEvent` bindings [here](HOOKS.md#kubernetes).

Example of JSON output from `hook --config`:

```json
{
  "configVersion": "v1",
  "kubernetes": [
    {
      "name":"Execute on changes of namespace labels",
      "kind": "namespace",
      "executeHookOnEvent":["Modified"],
      "jqFilter":".metadata.labels"
    }
  ]
}
```

> Note: it is possible to watch Custom Defined Resources, just use proper values for `apiVersion` and `kind` fields.

### onStartup

This binding has only one parameter: order of execution. Hooks are loaded at the start and then hooks with onStartup binding are executed in the order defined by parameter. Read more about `onStartup` bindings [here](HOOKS.md#onstartup).

Example `hook --config`:

```yaml
configVersion: v1
onStartup: 10
```

### schedule

This binding is used to execute hooks periodically. A schedule can be defined with a granularity of seconds. Read more about `schedule` bindings [here](HOOKS.md#schedule).

Example `hook --config` with 2 schedules:

```yaml
configVersion: v1
schedule:
- name: "every 10 min"
  crontab: "*/10 * * * *"
  allowFailure: true
- name: "Every Monday at 8:05"
  crontab: "5 8 * * 1"
  queue: mondays
```

## Prometheus target

Shell-operator provides a `/metrics` endpoint. More on this in [METRICS](METRICS.md) document.

## Examples

More examples can be found in [examples](examples/) directory.

## License

Apache License 2.0, see [LICENSE](LICENSE).
