# Hooks

A hook is an executable file that Shell-operator runs when some event occurs. It can be a script or a compiled program written in any programming language. For illustrative purposes, we will use bash scripts. An example with a hook in the form of a Python script is available here: [002-startup-python](examples/002-startup-python).

The hook receives the data and returns the result via files. Paths to files are passed to the hook via environment variables.

## Shell-operator lifecycle

At startup Shell-operator initializes the hooks:

- The recursive search for hook files is performed in the working directory. You can specify it with `--working-dir` command-line argument or with the `SHELL_OPERATOR_WORKING_DIR` environment variable (the default path is `/hooks`).
  - Every executable file found in the path is considered a hook.

- The found hooks are sorted alphabetically according to the directories’ and hooks’ names. Then they are executed with the `--config` flag to get bindings to events in JSON format.

- If hook's configuration is successful, the workqueue is filled with `onStartup` hooks.

- After executing `onStartup` hooks, Shell-operator subscribes to Kubernetes events according to configured `kubernetes` bindings.

- Then, the work queue is filled with `kubernetes` hooks with `Synchronization` [binding context](#binding-context) type, so that each hook receives all existing objects described in hook's configuration.

Next the main cycle is started:

- Hooks are adding to the queue on events:
  - `kubernetes` hooks are added to the queue from events that occur in Kubernetes,
  - `schedule` hooks are added according to the schedule.

- Queue handler executes hooks strictly sequentially. If hook fails with an error (non-zero exit code), Shell-operator restarts it (every 5 seconds) until success. In case of an erroneous execution of some hook, when other events occur, the queue will be filled with new tasks, but their execution will be blocked until the failing hook succeeds.
  - You can change this behavior for a specific hook by adding `allowFailure: true` to the binding configuration (not available for `onStartup` hooks).

- Several metrics are available for monitoring the activity of a queue: queue size, number of execution errors for specific hooks, etc. See [METRICS](METRICS.md) for more details.

## Hook configuration

Shell-operator runs the hook with the `--config` flag. In response the hook should print its event binding configuration to stdout. For example:

```
{
  "configVersion": "v1",
  "onStartup": ORDER,
  "schedule": [
    {SCHEDULE_PARAMETERS},
    {SCHEDULE_PARAMETERS}
  ],
  "kubernetes": [
       {ON_KUBERNETES_EVENT_PARAMETERS},
       {ON_KUBERNETES_EVENT_PARAMETERS}
  ]
}
```

`configVersion` field specifies a version of configuration schema. The latest schema version is "v1" and it is described below.

Event binding is an event type ("onStartup", "schedule" or "kubernetes") plus parameters required for a subscription.

### onStartup

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

### schedule

Scheduled execution. You can bind a hook to any number of schedules.

Syntax:
```
{
  "configVersion": "v1",
  "schedule": [
    {
      "crontab": "*/5 * * * *",
      "allowFailure": true|false,
    },
    {
      "name": "Every 20 minutes",
      "crontab": "*/20 * * * *",
      "allowFailure": true|false,
    },
    {
      "name": "every 10 seconds",
      "crontab": "*/10 * * * * *",
      "allowFailure": true|false,
    },
    ...
  ]
}
```

Parameters:

`name` — is an optional identifier. It is used to distinguish between multiple schedules during runtime. For more information see [binding context](#binding-context).

`crontab` – is a mandatory schedule with a regular crontab syntax with 5 fields. 6 fields style crontab also supported, for more information see [documentation on robfig/cron.v2 library](https://godoc.org/gopkg.in/robfig/cron.v2).

`allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

### kubernetes

Run a hook on a Kubernetes object changes.

Syntax:
```
{
  "configVersion": "v1",
  "kubernetes": [
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
        "labelSelector": {
          "matchLabels": {
            "myLabel": "myLabelValue",
            "someKey": "someValue",
            ...
          },
          "matchExpressions": [
            {
              "key": "env",
              "operator": "In",
              "values": ["production"],
            },
            ...
          ],
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

- `name` is an optional identifier. It is used to distinguish different bindings during runtime. See also [binding context](#binding-context).

- `apiVersion` is an optional group and version of object API. For example, it is `v1` for core objects (Pod, etc.), `rbac.authorization.k8s.io/v1beta1` for ClusterRole and `monitoring.coreos.com/v1` for prometheus-operator.

- `kind` is the type of a monitored Kubernetes resource. This field is required. CRDs are supported, but resource should be registered in cluster before shell-operator starts. This can be checked with `kubectl api-resources` command. You can specify case-insensitive name, kind or short name in this field. For example, to monitor a DaemonSet these forms are valid:

```
"kind": "DaemonSet"
"kind": "Daemonset"
"kind": "daemonsets"
"kind": "DaemonSets"
"kind": "ds"
```

- `watchEvent` — the list of monitored events (Added, Modified, Deleted). By default all events will be monitored. Docs: [Using API](https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes) [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#watchevent-v1-meta).

- `nameSelector` — selector of objects by their name. If this selector is not set, then all objects of specified kind are monitored.

- `labelSelector` — [standard](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#labelselector-v1-meta) selector of objects by labels (examples [of use](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels)).
  If the selector is not set, then all objects of specified kind are monitored.

- `fieldSelector` — selector of objects by their fields, works like `--field-selector=''` flag of `kubectl`. Supported operators are Equals (or `=`, `==`) and NotEquals (or `!=`) and all expressions are combined with AND. Also note that fieldSelector with 'metadata.name' field is mutually exclusive with nameSelector. There are limits on fields, see [Note](#fieldselector).

- `namespace` — filters to choose namespaces. If omitted, events from all namespaces will be monitored.

- `namespace.nameSelector` — this filter can be used to monitor events from objects in a particular list of namespaces.

- `namespace.labelSelector` — this filter works like `labelSelector` but for namespaces and Shell-operator dynamically subscribes to events from matched namespaces.

- `jqFilter` —  an optional parameter that specifies additional event filtering with [jq syntax](https://stedolan.github.io/jq/manual/). The hook will be triggered on Modified event only if filter result is changed after the last event. See example [102-monitor-namespaces](examples/102-monitor-namespaces).

- `allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

Example:
```json
{
  "configVersion": "v1",
  "kubernetes": [
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

#### usage notes

##### default namespace

Unlike `kubectl` you should explicitly define namespace.nameSelector to monitor events from `default` namespace.

```
      "namespace": {
        "nameSelector": ["default"]
      },
```

##### RBAC is required

Shell-operator requires a ServiceAccount with the appropriate [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) permissions. See examples with RBAC: [monitor-pods](examples/101-monitor-pods) and [monitor-namespaces](examples/102-monitor-namespaces).

##### jqFilter

This filter is used to ignore superfluous "Modified" events, not to select objects to subscribe to. For example, if hook is interested in changes of labels, `"jqFilter": ".metadata.labels"` can be used to ignore changes in `.status` or `.metadata.annotations`.

The result of applying filter to the event's object is passed to hook in a binding context file in a `filterResult` field. See [binding context](#binding-context).

##### Added != Object created

Consider that "Added" event is not always equal to "Object created" if labelSelector, fieldSelector or namespace.labelSelector is specified in the `binding`. If objects and/or namespace are updated in Kubernetes, the `binding` may suddenly start matching them, with the "Added" event. The same with "Deleted" event: "Deleted" is not always equal to "Object removed", the object can just move out of scope of selectors.

##### fieldSelector

There is no support of filtering by arbitrary field neither for core resources nor for custom resources (see [issue#53459](https://github.com/kubernetes/kubernetes/issues/53459)). Only `metadata.name` and `metadata.namespace` fields are commonly supported.

However fieldSelector can be useful for some resources with extended set of supported fields:

| kind       | fieldSelector | src url |
|------------|---------------|---------|
| Pod        | spec.nodeName<br>spec.restartPolicy<br>spec.schedulerName<br>spec.serviceAccountName<br>status.phase<br>status.podIP<br>status.nominatedNodeName              | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/core/pod/strategy.go#L219-L230)        |
| Event      | involvedObject.kind<br>involvedObject.namespace<br>involvedObject.name<br>involvedObject.uid<br>involvedObject.apiVersion<br>involvedObject.resourceVersion<br>involvedObject.fieldPath<br>reason<br>source<br>type                | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/core/event/strategy.go#L102-L112)        |
| Secret     | type              | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/core/secret/strategy.go#L128)        |
| Namespace  | status.phase              | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/core/namespace/strategy.go#L163)        |
| ReplicaSet | status.replicas              | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/apps/replicaset/strategy.go#L)        |
| Job        | status.successful              | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/batch/job/strategy.go#L205)        |
| Node       | spec.unschedulable              | [1.16](https://github.com/kubernetes/kubernetes/blob/v1.16.1/pkg/registry/core/node/strategy.go#L204)        |


Example of selecting Pods by 'Running' phase:

```
"kind": "Pod",
"fieldSelector":{
  "matchExpressions":[
    {
      "field":"status.phase",
      "operator":"Equals",
      "value":"Running"
    }
  ]
}
```


## Event triggered execution

When an event associated with a hook is triggered, Shell-operator executes the hook without arguments and sets the following environment variable:

- `BINDING_CONTEXT_PATH` — a path to a temporary file containing data about an event that was triggered (binding context);

### Binding context

When an event associated with a hook is triggered, Shell-operator executes this hook without arguments. The information about the event that led to the hook execution is called the **binding context** and is written to a temporary file. `BINDING_CONTEXT_PATH` environment variable contains the path to this file with JSON-array of structures with the following fields:

- `binding` is a string from the `name` parameter. If the parameter has not been set in the binding configuration, then strings `schedule` or `kubernetes` are used. For a hook executed at startup, this value is always `onStartup`.

There are some extra fields for `kubernetes`-type events:

- `type` - "Synchronization" or "Event". "Synchronization" binding context contains all objects that match selectors in a hook's configuration. "Event" binding context contains a watch event type, an event related object and a jq filter result.

- `watchEvent` — the possible value is one of the values you can pass in `watchEvent` binding parameter: “Added”, “Modified” or “Deleted”. This value is set if type is "Event".
- `object` — the whole object related to the event. It contains the exact copy of the corresponding field in [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#watchevent-v1-meta) response, so it's the object state **at the moment of the event** (not at the moment of the hook execution).
- `filterResult` — the result of jq execution with specified `jqFilter` on the abovementioned object. If `jqFilter` is not specified, then `filterResult` is omitted.
- `objects` — list of existing objects that matches selectors. Each item of this list contains `object` and `filterResult` fields.

#### schedule binding context example

For example, if you have the following binding configuration of a hook:

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

#### kubernetes binding context example

A hook can monitor Pods in all namespaces with this simple configuration:

```json
{
  "configVersion": "v1",
  "kubernetes": [
  {"kind": "Pod"}
  ]
}
```

During startup, hook will be executed with following binding context file:

```
[
  {
    "binding": "kubernetes",
    "type": "Synchronization",
    "objects": [
      {
        "object": {
          "kind": "Pod,
          "metadata":{
            "name":"etcd-...",
            "namespace":"kube-system",
            ...
          },
        }
      },
      {
        "object": {
          "kind": "Pod,
          "metadata":{
            "name":"kube-proxy-...",
            "namespace":"kube-system",
            ...
          },
        }
      },
      ...
    ]
  }
]
```

If pod `pod-321d12` will be added into namespace 'default', then hook will be executed with following binding context file:

```
[
  {
    "binding": "kubernetes",
    "type": "Event",
    "watchEvent": "Added",
    "object": {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {
        "name": "pod-321d12",
        "namespace": "default",
        ...
      },
      "spec": {
        ...
      },
      ...
    }
  }
]
```

## Debugging

The following tools for debugging and fine-tuning of Shell-operator and hooks are available:

- Analysis of logs of a Shell-operator’s pod (enter `kubectl logs -f po/POD_NAME` in terminal),
- The environment variable can be set to `LOG_LEVEL=debug` to include the detailed debugging information into logs,
- You can view the contents of the working queue via the HTTP request `/queue` (`queue` endpoint):
   ```
   kubectl port-forward po/shell-operator 9115:9115
   curl localhost:9115/queue
   ```
