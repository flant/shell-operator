# Shell-operator

Shell-operator is a tool for running event-driven scripts in a Kubernetes cluster.

This operator is not an operator for a _particular software product_ as prometheus-operator or kafka-operator. Shell-operator provides an integration layer between Kubernetes cluster events and shell scripts by treating scripts as hooks triggered by events. Think of it as an operator-sdk but for scripts.

Shell-operator provides:
- __Ease the management of a Kubernetes cluster__: use the tools that Ops are familiar with. It can be bash, python, ruby, kubectl, etc.
- __Kubernetes object events__: hook can be triggered by add, update or delete events.
- __Object selector and properties filter__: Shell-operator can monitor only particular objects and detect changes in their properties.
- __Simple configuration__: hook binding definition is a JSON structure on stdout.

## Quickstart

> You need to have a Kubernetes cluster, and the kubectl must be configured to communicate with your cluster.

Steps to setup Shell-operator in your cluster are:
- build an image with your hooks (scripts)
- create necessary RBAC objects (for onKubernetesEvent binding)
- run Pod with a built image

### Build an image with your hooks

A hook is a script with the added events configuration code. Learn [more](HOOKS.md) about hooks. Here is a simple hook with `onStartup` binding:

```bash
#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{"onStartup": 10}'
else
  echo "SIMPLE: Run simple onStartup hook"
fi
```


You can use a prebuilt image [flant/shell-operator:latest](https://hub.docker.com/r/flant/shell-operator) with bash, kubectl and shell-operator binaries to build you own image. You just need to ADD your hook into /hooks:
```dockerfile
# Dockerfile
FROM flant/shell-operator:latest
ADD hook.sh /hooks
```

Build image and push it to the Docker registry accessible by Kubernetes cluster.
```
docker build -t "my.registry.url/shell-operator:simple-hook" .
docker push my.registry.url/shell-operator:simple-hook
```

### Install shell-operator in a cluster

This simple image can be deployed as a Pod. Put this manifest into shell-operator-pod.yaml:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: shell-operator
spec:
  containers:
  - name: shell-operator
    image: my.registry.url/shell-operator:simple-hook
```

Start shell-operator by applying `shell-operator.yml`:
```
kubectl apply -f shell-operator.yml
```

Run `kubectl logs po/shell-operator` and see that hook.sh was ran at startup:
```
INFO     : QUEUE add all HookRun@OnStartup
INFO     : TASK_RUN HookRun@ON_STARTUP hook.sh
INFO     : Running hook 'hook.sh' binding 'ON_STARTUP' ...
SIMPLE: Run simple onStartup hook
...
```

## Hook binding types

[__onStartup__](HOOKS.md)

This binding has only one parameter: order of execution. Hooks are loaded at start and then hooks with onStartup binding are executed in order defined by parameter.

Example `hook --config`:

```
{"onStartup":10}
```

[__schedule__](HOOKS.md#schedule)

This binding is for periodical running of hooks. Schedule can be defined with granularity of seconds.

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

[__onKubernetesEvent__](HOOKS.md#onKubernetesEvent)

This binding defines a subset of Kubernetes objects that Shell-operator will monitor and a jq filter for their properties.

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

Shell-operator provides /metrics endpoint. More on this in [METRICS](METRICS.md) document.

## Examples

More examples can be found in [examples](examples/) directory.

## License

Apache License 2.0, see [LICENSE](LICENSE).
