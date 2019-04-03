# Shell-operator

Shell-operator is a tool for running event-driven scripts in a Kubernetes cluster.

* Simple configuration
* A fixed script execution order
* Run scripts on startup, on schedule or on Kubernetes events.

## Quickstart

> You need to have a Kubernetes cluster, and the kubectl must be configured to communicate with your cluster.

To use Shell-operator you need to:
- build an image with your hooks (scripts)
- (optional) setup RBAC
- run Deployment with a built image.

### Build an image with your hooks

A hook is a script with the added events configuration code. Learn [more](HOOKS.md) about hooks.

Create project directory with the following Dockerfile, which use the [shell-operator](https://hub.docker.com/r/flant/shell-operator) image as FROM:
```
FROM flant/shell-operator:latest
ADD hooks /hooks
```

In the project directory create `hooks/002-hook-schedule-example` directory and the `hooks/002-hook-schedule-example/hook.sh` file with the following content:
```bash
#!/usr/bin/env bash

if [[ $1 == "--config" ]] ; then
  echo '{"onStartup": 10, "schedule":[{"name":"every 10sec", "crontab":"*/10 * * * * *"}]}'
  exit 0
fi

echo "002-hook-schedule-example onStartup run"
echo "Binding context:"
cat "${BINDING_CONTEXT_PATH}"
echo
echo "002-hook-schedule-example Stop"
```

Build image and push it to the Docker registry.

### Use image in your cluster

To use the built image in your Kubernetes cluster you need to create a Deployment.
Create a `shell-operator.yml` file describing deployment with the following content:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: shell-operator
spec:
  containers:
  - name: shell-operator
    image: registry.mycompany.com/shell-operator:monitor-namespaces
```
Replace `image` with your registry and add credential if necessary.

Start shell-operator by applying `shell-operator.yml`:
```
kubectl apply -f shell-operator.yml
```

Analyze Shell-operator logs with `kubectl get logs POD_NAME` (where POD_NAME is a name of a Shell-operator Pod). You should see logs like the following:
```
2019-04-03T12:27:01Z INFO     : Use working dir: /hooks
2019-04-03T12:27:01Z INFO     : HTTP SERVER Listening on :9115
2019-04-03T12:27:01Z INFO     : Use temporary dir: /tmp/shell-operator
2019-04-03T12:27:01Z INFO     : Initialize hooks manager ...
2019-04-03T12:27:01Z INFO     : Search and load hooks ...
2019-04-03T12:27:01Z INFO     : Load hook config from '/hooks/002-hook-schedule-example/hook.sh'
2019-04-03T12:27:01Z INFO     : Initializing schedule manager ...
2019-04-03T12:27:01Z INFO     : KUBE Init Kubernetes client
2019-04-03T12:27:01Z INFO     : KUBE-INIT Kubernetes client is configured successfully
2019-04-03T12:27:01Z INFO     : MAIN: run main loop
2019-04-03T12:27:01Z INFO     : MAIN: add onStartup tasks
2019-04-03T12:27:01Z INFO     : Running schedule manager ...
2019-04-03T12:27:01Z INFO     : MSTOR Create new metric shell_operator_live_ticks
2019-04-03T12:27:01Z INFO     : MSTOR Create new metric shell_operator_tasks_queue_length
2019-04-03T12:27:01Z INFO     : QUEUE add all HookRun@OnStartup
2019-04-03T12:27:04Z INFO     : TASK_RUN HookRun@ON_STARTUP 002-hook-schedule-example/hook.sh
2019-04-03T12:27:04Z INFO     : Running hook '002-hook-schedule-example/hook.sh' binding 'ON_STARTUP' ...
002-hook-schedule-example onStartup run
Binding context:
[{"binding":"onStartup"}]
002-hook-schedule-example Stop
2019-04-03T12:27:10Z INFO     : Running schedule manager entry '*/10 * * * * *' ...
2019-04-03T12:27:10Z INFO     : TASK_RUN HookRun@SCHEDULE 002-hook-schedule-example/hook.sh
2019-04-03T12:27:10Z INFO     : Running hook '002-hook-schedule-example/hook.sh' binding 'SCHEDULE' ...
002-hook-schedule-example onStartup run
Binding context:
[{"binding":"every 10sec"}]
002-hook-schedule-example Stop
2019-04-03T12:27:20Z INFO     : Running schedule manager entry '*/10 * * * * *' ...
2019-04-03T12:27:22Z INFO     : TASK_RUN HookRun@SCHEDULE 002-hook-schedule-example/hook.sh
2019-04-03T12:27:22Z INFO     : Running hook '002-hook-schedule-example/hook.sh' binding 'SCHEDULE' ...
002-hook-schedule-example onStartup run
Binding context:
[{"binding":"every 10sec"}]
002-hook-schedule-example Stop
...
```

## Examples

You can find more examples [here](examples/).

## License

Apache License 2.0, see [LICENSE](LICENSE).
