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
  "configVersion": "v1",
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

`configVersion` field specifies a version of configuration schema. The latest schema version is "v1" and it is described below.

Event binding is an event type (onStartup, schedule, and onKubernetesEvent) plus parameters required for a subscription.

#### onStartup

The execution at the Shell-operator’ startup.

Syntax:
```
{
  "configVersion": "v1",
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
  "configVersion": "v1",
  "schedule": [
     {
      "name": "Every 20 minutes",
      "crontab": "0 */20 * * * *",
      "allowFailure": true|false,
    },
    {
      "crontab": "* * * * * *",
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
  "configVersion": "v1",
  "onKubernetesEvent": [
    {
      "name": "Monitor labeled pods in cache tier",
      "apiVersion": "v1",
      "kind": "Pod",
      "watchEvent": [ "Added", "Modified", "Deleted" ],
      "nameSelector": {
        "matchNames": ["pod-0", "pod-1"],
      },
      "labelSelector": {
        "matchLabels": {
          "myLabel": "myLabelValue",
          "someKey": "someValue",
          ...
        },
        "matchExpressions": [
          {
            "key": "tier",
            "operator": "In",
            "values": ["cache"],
          },
          ...
        ],
      },
      "fieldSelector": { 
        "matchExpressions": [
          {
            "field": "status.phase",
            "operator": "Equals",
            "value": "Pending",
          },
          ...
        ],
      },
      "namespace": {
        "nameSelector": {
          "matchNames": ["somenamespace", "proj-production", proj-stage"],
        }
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

- `apiVersion` is an optional group and version of object API.

- `kind` is the type of a monitored Kubernetes resource. This field is required. CRDs are supported, but resource should be registered in cluster before shell-operator starts. This can be checked with `kubectl api-resources` command. You can specify case-insensitive name, kind or short name in this field. For example, to monitor a DaemonSet these forms are valid:

```
"kind": "DaemonSet"
"kind": "Daemonset"
"kind": "daemonsets"
"kind": "DaemonSets"
"kind": "ds"
```

- `watchEvent` — the list of monitored events (Added, Modified, Deleted). By default all events will be monitored. Docs: [Using API](https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes) [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#watchevent-v1-meta).

- `nameSelector` — selector of objects by their name. If this selector is not set, then all objects of specified kind are selected.

- `labelSelector` — [standard](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#labelselector-v1-meta) selector of objects by labels (examples [of use](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels)).
  If the selector is not set, then all objects are selected.

- `fieldSelector` — selector of objects by their fields, works like `--field-selector=''` flag of `kubectl`. Due to limits of API, supported operators are Equals (or `=`, `==`) and NotEquals (or `!=`) and all expressions are combined with AND. Note that fieldSelector with 'metadata.name' field is mutually exclusive with nameSelector. 

- `namespace` — a filter to choose namespaces. If omitted, then the events from all namespaces will be monitored. Currently supported only `nameSelector` filter which can monitor event from particular list of namespaces. 

- `jqFilter` —  an optional parameter that specifies additional event filtering with [jq syntax](https://stedolan.github.io/jq/manual/). The hook will be triggered only when the properties of an object are changed after the filter is applied. See example [102-monitor-namespaces](examples/102-monitor-namespaces).

- `allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

Example:
```json
{
  "configVersion": "v1",
  "onKubernetesEvent": [
    {
      "name": "Trigger on labels changes of Pods with myLabel:myLabelValue in any namespace",
      "kind": "pod",
      "watchEvent": ["Modified"],
      "labelSelector": {
        "matchLabels": {
          "myLabel": "myLabelValue"
        }
      },
      "namespace": {
        "nameSelector": ["default"]
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true
    }
  ]
}
```

This hook configuration will execute hook on each change in labels of pods labeled with myLabel=myLabelValue in default namespace.  

> Note: unlike `kubectl` you should explicitly define namespace.nameSelector to monitor events from `default` namespace.

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

- `BINDING_CONTEXT_PATH` — a path to a temporary file containing data about an event that was triggered (binding context);

- `WORKING_DIR` — a path to a hooks directory (see [an example](examples/003-common-library) — using libraries via `WORKING_DIR`).

### Binding context

Binding context is the information about the event that led to the hook execution.

The `BINDING_CONTEXT_PATH` environment variable contains the path to a file with JSON-array of structures with the following fields:

- `binding` is a string from the `name` parameter. If the parameter has not been set in the binding configuration, then `schedule` or `onKubernetesEvent` binding types are used. For a hook executed at startup, the binding type is always `onStartup`.

There are some extra fields for `onKubernetesEvent`-type events:

- `watchEvent` — the event type is identical to the values in the `event` parameter: “Added”, “Modified” or “Deleted”.
- `object` — an object related to event.
- `filterResult` — result of jq execution with specified `jqFilter` expression. If `jqFilter` is not specified, then filterResult is omitted.

#### schedule binding context example

Let's hook have this schedule configuration:

```json
{
  "configVersion": "v1",
  "schedule": [
    {
      "name": "incremental",
      "crontab": "0 2 */3 * * *",
      "allowFailure": true
    }
  ]
}
```

... then at 12:02 it will be executed with the following binding context:

```json
[{ "binding": "incremental"}]
```

#### onKubernetesEvent binding context example

A hook can monitor Pods in all namespaces with this simple configuration:

```json
{
  "configVersion": "v1",
  "onKubernetesEvent": [
  {"kind": "Pod"}
  ]
}
```

If pod `pod-321d12` will be added into namespace 'default', then hook will be executed with following binding context file:

```
[
  {
    "binding": "onKubernetesEvent",
    "watchEvent": "Added",
    "object": {
      "kind": "Pod",
      "name": "pod-321d12",
      "metadata": {
      ...
      }, ...
    }
  }
]
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
