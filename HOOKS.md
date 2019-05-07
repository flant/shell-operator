# Hooks

A hook is an executable file that Shell-operator runs when some event occurs. It can be a script or a compiled program written in any programming language. For illustrative purposes, we will use bash scripts. An example with a hook in the form of a Python script is available here: [002-startup-python](examples/002-startup-python).

The hook receives the data and returns the result via files. Paths to files are passed to the hook via environment variables.

## Initialization: search for and configuration of hooks

At startup Shell-operator initializes the hooks:

- The recursive search for hook files is performed in the working directory. You can specify it with `--working-dir` command-line argument or with the `SHELL_OPERATOR_WORKING_DIR` environment variable (the default path is `/hooks`).
  - The executable files found in the path are considered hooks.

- The found hooks are sorted alphabetically according to the directories’ and hooks’ names. Then they are executed with the `--config` flag to get bindings to events in JSON format.

- Shell-operator subscribes to events using received bindings.

## Hook configuration

Shell-operator runs the hook with the `--config` flag. In response the hook should print out its event binding configuration to stdout:

```
{
  "onStartup": ORDER,
  "schedule": [
    {SCHEDULE_PARAMETERS},
    {SCHEDULE_PARAMETERS}
  ],
  "onKubernetesEvent": [
       {ON_KUBERNETES_EVENT_PARAMETERS},
       {ON_KUBERNETES_EVENT_PARAMETERS}
  ]
}
```

Event binding is an event type (onStartup, schedule, and onKubernetesEvent) plus parameters required for a subscription.

#### onStartup

The execution at the Shell-operator’ startup.

Syntax:
```
{
  "onStartup": ORDER
}
```

Parameters:

ORDER — the execution order (when added to the queue, the hooks will be sorted in the specified order, and then alphabetically). The value should be an integer.

#### schedule

Scheduled execution. You can bind a hook to any number of schedules.

Syntax:
```
{
  "schedule": [
     {
      "name": "Every 20 minutes",
      "crontab": "0 */20 * * * *",
      "allowFailure": true|false,
    },
    {
      "crontab": "* * * * * *,
      "allowFailure": true|false,
    },
    ...
  ]
}
```

Parameters:

`name` — is an optional identifier. It is used to distinguish between multiple schedules during runtime. For more information see [binding context](#binding-context).

`crontab` – is a mandatory schedule with a regular crontab syntax (with [additional](https://godoc.org/github.com/robfig/cron) features).

`allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

#### onKubernetesEvent

Run a hook on a Kubernetes object changes.

Syntax:
```
{
  "onKubernetesEvent": [
    {
      "name": "Monitor labeled pods in cache tier",
      "kind": "Pod",
      "event": [ "add", "update", "delete" ],
      "selector": {
        "matchLabels": {
          "myLabel": "myLabelValue",
          "someKey": "someValue",
          ...
        },
        "matchExpressions": [
          {
            "key": "tier",
            "operation": "In",
            "values": ["cache"],
          },
          ...
        ],
      },
      "namespaceSelector": {
        "matchNames": ["somenamespace", "proj-production", "proj-stage"],
        "any": true|false,
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true|false,
    },
    {"name":"monitor Pods", "kind": "pod", ...},
    ...
    {...}
  ]
}
```

Parameters:

- `name` is an optional identifier. It is used to distinguish different bindings during runtime. For more info see [binding context](#binding-context).

- `kind` is the case-insensitive type of a monitored Kubernetes object from the list:
    - namespace
    - cronjob
    - daemonset
    - deployment
    - job
    - pod
    - replicaset
    - replicationcontroller
    - statefulset
    - endpoints
    - ingress
    - service
    - configmap
    - secret
    - persistentvolumeclaim
    - storageclass
    - node
    - serviceaccount
- `event` — the list of monitored events (add, update, delete). By default all events will be monitored.

- `selector` — [standard](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#labelselector-v1-meta) selector of objects (examples [of use](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels)).
  If the selector is not set, then all objects are selected.

- `namespaceSelector` — specifies the filter for selecting namespaces. If you omit it, the events from all namespaces will be monitored.

- `jqFilter` —  an optional parameter that specifies additional event filtering with [jq syntax](https://stedolan.github.io/jq/manual/). The hook will be triggered only when the properties of an object are changed after the filter is applied. See example [102-monitor-namespaces](examples/102-monitor-namespaces).

- `allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

Example:
```
{
  "onKubernetesEvent": [
    {
      "name": "Trigger on labels changes of Pods with myLabel:myLabelValue in any namespace",
      "kind": "pod",
      "event": ["update"],
      "selector": {
        "matchLabels": {
          "myLabel": "myLabelValue"
        }
      },
      "namespaceSelector": {
        "any": true
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true
    }
  ]
}
```

> Note: Shell-operator requires a ServiceAccount with the appropriate [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) permissions. See examples with RBAC: [monitor-pods](examples/101-monitor-pods) and [monitor-namespaces](examples/102-monitor-namespaces).

## Main cycle: events tracking and hooks execution

Events may happen at any time, so the working queue for hooks is provided in the Shell-operator:

- Adding events to the queue:
  - `onStartup` hooks are added to the queue during initialization,
  - `onKubernetesEvent` hooks are added to the queue when events occur in Kubernetes,
  - `schedule` hooks are added according to the schedule.

- Queue handler executes hooks strictly sequentially. If hook fails with an error (non-zero exit code), Shell-operator restarts it (every 5 seconds) until success. In case of an erroneous execution of some hook, when other events occur, the queue will be filled with new tasks, but their execution will be blocked.
  - You can change this behavior for a specific hook by adding `allowFailure: true` to the binding configuration (not available for `onStartup` hooks).

- Several metrics are available for monitoring the activity of a queue: queue size, number of execution errors for specific hooks, etc. See [METRICS](METRICS.md) for more details.

## Event triggered execution

When an event associated with a hook is triggered, Shell-operator executes the hook without arguments and sets the following environment variables:

- `BINDING_CONTEXT_PATH` — a path to a file containing data about an event that was triggered (binding context);

- `WORKING_DIR` — a path to a hooks directory (see [an example](examples/003-common-library) — using libraries via `WORKING_DIR`).

### Binding context

Binding context is the information about the event that led to the hook execution.

The `BINDING_CONTEXT_PATH` environment variable contains the path to a file with JSON-array of structures with the following fields:

- `binding` is a string from the `name` parameter. If the parameter has not been set in the binding configuration, then `schedule` or `onKubernetesEvent` binding types are used. For a hook executed at startup, the binding type is always `onStartup`.

There are some extra fields for `onKubernetesEvent`-type events:

- `resourceEvent` — the event type is identical to the values in the `event` parameter: “add”, “update” or “delete”.
- `resourceNamespace`, `resourceKind`, `resourceName` — the information about the Kubernetes object associated with an event.

For example, if you have the following binding configuration of a hook:

```json
{
  "schedule": [
    {
      "name": "incremental",
      "crontab": "0 2 */3 * * *",
      "allowFailure": true
    }  ]
}
```

... then at 12:02 it will be executed with the following binding context:

```json
[{ "binding": "incremental"}]
```

## Debugging

The following tools for debugging and fine-tuning of Shell-operator and hooks are available:

- Analysis of logs of a Shell-operator’s pod (enter `kubectl logs -f po/POD_NAME` in terminal),
- The environment variable can be set to `RLOG_LOG_LEVEL=DEBUG` to include the detailed debugging information into logs,
- You can run the Shell-operator with the `--debug=yes` flag to obtain even more detailed debugging information and save it to logs,
- You can view the contents of the working queue via the HTTP request `/queue` (`queue` endpoint):
   ```
   kubectl port-forward po/shell-operator 9115:9115
   curl localhost:9115/queue
   ```
