# Shell-operator [![docker pull flant/shell-operator](https://img.shields.io/docker/pulls/flant/shell-operator.svg?logo=docker)](https://hub.docker.com/r/flant/shell-operator) [![Slack chat EN](https://img.shields.io/badge/slack-EN%20chat-611f69.svg?logo=slack)](https://cloud-native.slack.com/messages/CJ13K3HFG) [![Telegram chat RU](https://img.shields.io/badge/telegram-RU%20chat-179cde.svg?logo=telegram)](https://t.me/shelloperator)

<img width="70" height="70" src="logo-shell.png" alt="Shell-operator logo" />

Shell-operator is a tool for running event-driven scripts in a Kubernetes cluster.

This operator is not an operator for a _particular software product_ as prometheus-operator or kafka-operator. Shell-operator provides an integration layer between Kubernetes cluster events and shell scripts by treating scripts as hooks triggered by events. Think of it as an operator-sdk but for scripts.

Shell-operator provides:
- __Ease the management of a Kubernetes cluster__: use the tools that Ops are familiar with. It can be bash, python, kubectl, etc.
- __Kubernetes object events__: hook can be triggered by add, update or delete events. **Learn [more](HOOKS.md) about hooks.**
- __Object selector and properties filter__: Shell-operator can monitor only particular objects and detect changes in their properties.
- __Simple configuration__: hook binding definition is a JSON structure on stdout.

## Quickstart

> You need to have a Kubernetes cluster, and the kubectl must be configured to communicate with your cluster.

Steps to setup Shell-operator in your cluster are:
- build an image with your hooks (scripts)
- create necessary RBAC objects (for onKubernetesEvent binding)
- run Pod with a built image

### Build an image with your hooks

A hook is a script that, when executed with `--config` option, returns configuration in JSON. Learn [more](HOOKS.md) about hooks.

Let's create a small operator that will watch for all Pods in all Namespaces and simply log a name of a new Pod.

"onKubernetesEvent" binding is used to tell shell-operator about what objects we want to watch. Create the `pods-hook.sh` file with the following content:
```bash
#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  cat <<EOF
  {"onKubernetesEvent": [
    {"kind":"Pod",
     "event":["add"]
  }
  ]}
EOF
else
  podName=$(jq -r .[0].resourceName $BINDING_CONTEXT_PATH)
  echo "Pod '${podName}' added"
fi
```

Make the `pods-hook.sh` executable:
```
chmod +x pods-hook.sh
```

You can use a prebuilt image [flant/shell-operator:latest](https://hub.docker.com/r/flant/shell-operator) with bash, kubectl, jq and shell-operator binaries to build you own image. You just need to ADD your hook into `/hooks` directory in the Dockerfile.

Create the following `Dockerfile` in the directory where you created the `pods-hook.sh` file:
```dockerfile
FROM flant/shell-operator:latest
ADD pods-hook.sh /hooks
```

Build image:
```shell
docker build -t "registry.mycompany.com/shell-operator:monitor-pods" .
```

Push image to the Docker registry accessible by Kubernetes cluster:
```shell
docker push registry.mycompany.com/shell-operator:monitor-pods
```

### Install shell-operator in a cluster

We need to watch for Pods in all Namespaces. That means that we need specific RBAC definitions for shell-operator:

```shell
kubectl create namespace example-monitor-pods &&
kubectl create serviceaccount monitor-pods-acc \
  --namespace monitor-pods &&
kubectl create clusterrole monitor-pods --verb=get,watch,list --resource=pods &&
kubectl create clusterrolebinding monitor-pods \
  --clusterrole=monitor-pods \
  --serviceaccount=example-monitor-pods:monitor-pods-acc
```

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
```shell
kubectl -n example-monitor-pods apply -f shell-operator-pod.yaml
```

For instance, deploy [kubernetes-dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/) to trigger `onKuberneteEvent`:

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/master/aio/deploy/recommended/kubernetes-dashboard.yaml
```

Run `kubectl logs -f po/shell-operator` and see that the hook will print new pod names:
```
... deploy/kubernetes-dashboard replicas increased by 2 ...
...
INFO     : QUEUE add TASK_HOOK_RUN@KUBE_EVENTS pods-hook.sh
INFO     : QUEUE add TASK_HOOK_RUN@KUBE_EVENTS pods-hook.sh
INFO     : TASK_RUN HookRun@KUBE_EVENTS pods-hook.sh
INFO     : Running hook 'pods-hook.sh' binding 'KUBE_EVENTS' ...
Pod 'kubernetes-dashboard-769df5545f-4wnb4' added
INFO     : TASK_RUN HookRun@KUBE_EVENTS pods-hook.sh
INFO     : Running hook 'pods-hook.sh' binding 'KUBE_EVENTS' ...
Pod 'kubernetes-dashboard-769df5545f-99xsb' added
...
```

This example is also available in /examples: [monitor-pods](examples/101-monitor-pods).

## Hook binding types

__onStartup__

This binding has only one parameter: order of execution. Hooks are loaded at start and then hooks with onStartup binding are executed in order defined by parameter. Read more about `onStartup` bindings [here](HOOKS.md#onstartup).

Example `hook --config`:

```
{"onStartup":10}
```

__schedule__

This binding is for periodical running of hooks. Schedule can be defined with granularity of seconds. Read more about `schedule` bindings [here](HOOKS.md#schedule).

Example `hook --config` with 2 schedules:

```
{
  "schedule": [
   {"name":"every 10 min",
    "schedule":"*/10 * * * * *",
    "allowFailure":true
   },
   {"name":"Every Monday at 8:05",
    "schedule":"* 5 8 * * 1"
    }
  ]
}
```

__onKubernetesEvent__

This binding defines a subset of Kubernetes objects that Shell-operator will monitor and a [jq](https://github.com/stedolan/jq/) filter for their properties. Read more about `onKubernetesEvent` bindings [here](HOOKS.md#onKubernetesEvent).

Example of `hook --config`:

```
{
  "onKubernetesEvent": [
  {"name":"Execute on changes of namespace labels",
   "kind": "namespace",
   "event":["update"],
   "jqFilter":".metadata.labels"
  }]
}
```

## Prometheus target

Shell-operator provides `/metrics` endpoint. More on this in [METRICS](METRICS.md) document.

## Examples

More examples can be found in [examples](examples/) directory.

## License

Apache License 2.0, see [LICENSE](LICENSE).
