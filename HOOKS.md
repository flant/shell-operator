# Hooks

A hook is an executable file that Shell-operator runs when some event occurs. It can be a script or a compiled program written in any programming language. For illustrative purposes, we will use bash scripts. An example with a hook in the form of a Python script is available here: [002-startup-python](examples/002-startup-python).

The hook receives the data and returns the result via files. Paths to files are passed to the hook via environment variables.

## Shell-operator lifecycle

At startup Shell-operator initializes the hooks:

- The recursive search for hook files is performed in the hooks directory. You can specify it with `--hooks-dir` command-line argument or with the `SHELL_OPERATOR_HOOKS_DIR` environment variable (the default path is `/hooks`).
  - Every executable file found in the path is considered a hook.

- The found hooks are sorted alphabetically according to the directories’ and hooks’ names. Then they are executed with the `--config` flag to get bindings to events in YAML or JSON format.

- If hook's configuration is successful, the working queue named "main" is filled with `onStartup` hooks.

- Then, the "main" queue is filled with `kubernetes` hooks with `Synchronization` [binding context](#binding-context) type, so that each hook receives all existing objects described in hook's configuration.

- After executing `kubernetes` hook with `Synchronization` binding context, Shell-operator starts a monitor of Kubernetes events according to configured `kubernetes` binding.
  - Each monitor stores a *snapshot* — a refreshable list of all Kubernetes objects that match a binding definition.

Next the main cycle is started:

- Event handler adds hooks to the named queues on events:
  - `kubernetes` hooks are added to the queue when desired [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#watchevent-v1-meta) is received from Kubernetes,
  - `schedule` hooks are added according to the schedule,
  - `kubernetes` and `schedule` hooks are added to the "main" queue or to the named queue if `queue` field was specified.

- Each named queue has its own queue handler which executes hooks strictly sequentially. If hook fails with an error (non-zero exit code), Shell-operator restarts it (every 5 seconds) until succeeds. In case of an erroneous execution of some hook, when other events occur, the queue will be filled with new tasks, but their execution will be blocked until the failing hook succeeds.
  - You can change this behavior for a specific hook by adding `allowFailure: true` to the binding configuration (not available for `onStartup` hooks).
  
- Each hook is executed with a binding context, that describes an already occurred event: 
  - `kubernetes` hook receives `Event` binding context with object related to the event.
  - `schedule` hook receives a name of triggered schedule binding. 

- Several metrics are available for monitoring the activity of the queues and hooks: queues size, number of execution errors for specific hooks, etc. See [METRICS](METRICS.md) for more details.

## Hook configuration

Shell-operator runs the hook with the `--config` flag. In response, the hook should print its event binding configuration to stdout. The response can be in JSON format:

```
{
  "configVersion": "v1",
  "onStartup": STARTUP_ORDER,
  "schedule": [
    {SCHEDULE_PARAMETERS},
    {SCHEDULE_PARAMETERS}
  ],
  "kubernetes": [
       {KUBERNETES_PARAMETERS},
       {KUBERNETES_PARAMETERS}
  ]
}
```

or in YAML format:

```
configVersion: v1
onStartup: ORDER,
schedule:
- {SCHEDULE_PARAMETERS}
- {SCHEDULE_PARAMETERS}
kubernetes: 
- {KUBERNETES_PARAMETERS}
- {KUBERNETES_PARAMETERS}
```

`configVersion` field specifies a version of configuration schema. The latest schema version is "v1" and it is described below.

Event binding is an event type ("onStartup", "schedule" or "kubernetes") plus parameters required for a subscription.

### onStartup

Use this binding type to execute a hook at the Shell-operator’ startup.

Syntax:
```
{
  "configVersion": "v1",
  "onStartup": ORDER
}
```

Parameters:

`ORDER` — an integer value that specifies an execution order. When added to the "main" queue, the hooks will be sorted by this value and then alphabetically by file name.


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

`queue` — a name of a separate queue. It can be used to execute long-running hooks in parallel with other hooks.

`includeSnapshotsFrom` — a list of names of `kubernetes` bindings. When specified, all monitored objects will be added to the binding context.

### kubernetes

Run a hook on a Kubernetes object changes.

Syntax:
```
{
  "configVersion": "v1",
  "kubernetes": [
    {
      "name": "Monitor pods in cache tier",
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
      "includeSnapshotsFrom": ["Monitor pods in cache tier", "another-binding-name", ...],
      "allowFailure": true|false,
      "queue": "cache-pods",

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

- `jqFilter` —  an optional parameter that specifies *event filtering* using [jq syntax](https://stedolan.github.io/jq/manual/). The hook will be triggered on Modified event only if the filter result is changed after the last event. See example [102-monitor-namespaces](examples/102-monitor-namespaces).

- `allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

- `queue` — a name of a separate queue. It can be used to execute long-running hooks in parallel with hooks in "main" queue.

- `includeSnapshotsFrom` — an array of names of `kubernetes` bindings in a hook. When specified, a list of monitored objects from that bindings will be added to the binding context. Self-include is also possible.

YAML example:
```yaml
configVersion: v1
kubernetes:
# Trigger on labels changes of Pods with myLabel:myLabelValue in any namespace
- name: "label-changes-of-mylabel-pods"
  kind: pod
  watchEvent: ["Modified"]
  labelSelector:
    matchLabels:
      myLabel: "myLabelValue"
  namespace:
    nameSelector: ["default"]
  jqFilter: .metadata.labels
  allowFailure: true
  includeSnapshotsFrom: ["label-changes-of-mylabel-pods"]
```

This hook configuration will execute hook on each change in labels of pods labeled with myLabel=myLabelValue in namespace "default". The binding context will contain all pods with myLabel=myLabelValue from namespace "default".

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

This filter is used to ignore superfluous "Modified" events, and not to select objects to subscribe to. For example, if the hook is interested in changes of labels, `"jqFilter": ".metadata.labels"` can be used to ignore changes in `.status` or `.metadata.annotations`.

The result of applying the filter to the event's object is passed to the hook in a `filterResult` field of a [binding context](#binding-context).

You can use JQ_LIBRARY_PATH to set a path with jq modules. Also, Shell-operator uses jq release 1.6 so you can check your filters with a binary of that version.

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

##### fieldSelector and labelSelector expressions are ANDed

Objects should match all expressions defined in `fieldSelector` and `labelSelector`, so, for example, multiple `fieldSelector` expressions with "metadata.name" field and different values will not match any object.

## Binding context

When an event associated with a hook is triggered, Shell-operator executes the hook without arguments. The information about the event that led to the hook execution is called the **binding context** and is written in JSON format to a temporary file. The path of this file is available to hook via environment variable `BINDING_CONTEXT_PATH`.

Temporary files have unique names to prevent collisions between queues and are deleted after the hook run.

 Binging context is a JSON-array of structures with the following fields:

- `binding` is a string from the `name` parameter. If the parameter has not been set in the binding configuration, then strings `schedule` or `kubernetes` are used. For a hook executed at startup, this value is always `onStartup`.

- `snapshots` — binding context for `kubernetes` and `schedule` hooks contains a `snapshots` field if `includeSnapshotsFrom` is defined. `snapshots` object contains a list of objects for each binding name from `includeSnapshotsFrom`. If the list is empty, the named field is omitted.

There are some extra fields for `kubernetes`-type events:

- `type` - "Synchronization" or "Event". "Synchronization" binding context contains all objects that match selectors in a hook's configuration. "Event" binding context contains a watch event type, an event related object and a jq filter result.

- `watchEvent` — the possible value is one of the values you can pass in `watchEvent` binding parameter: “Added”, “Modified” or “Deleted”. This value is set if type is "Event".
- `object` — the whole object related to the event. It contains the exact copy of the corresponding field in [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#watchevent-v1-meta) response, so it's the object state **at the moment of the event** (not at the moment of the hook execution).
- `filterResult` — the result of jq execution with specified `jqFilter` on the abovementioned object. If `jqFilter` is not specified, then `filterResult` is omitted.
- `objects` — a list of existing objects that match selectors. Each item of this list contains `object` and `filterResult` fields. If the list is empty, the value of `objects` is an empty array.

#### onStartup binding context example

```json
[{ "binding": "onStartup"}]
```

#### schedule binding context example

For example, if you have the following configuration in a hook:

```yaml
configVersion: v1
schedule:
- name: incremental
  crontab: "0 2 */3 * * *"
  allowFailure: true
```

... then at 12:02 it will be executed with the following binding context:

```json
[{ "binding": "incremental"}]
```

#### kubernetes binding context example

A hook can monitor Pods in all namespaces with this simple configuration:

```yaml
configVersion: v1
kubernetes:
- kind: Pod
```

During startup, the hook will be executed with "Synchronization" binding context:

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

If pod `pod-321d12` will be added into namespace 'default', then hook will be executed with "Event" binding context:

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

#### example of a binding context with included snapshots and jqFilter result

Consider the hook that monitors changes of labels of all Pods and do something interesting on schedule:

```yaml
configVersion: v1
schedule:
- name: incremental
  crontab: "0 2 */3 * * *"
  includeSnapshotsFrom: ["monitor-pods"]
kubernetes:
- name: monitor-pods
  kind: Pod
  jqFilter: '.metadata.labels'
  includeSnapshotsFrom: ["monitor-pods"]
```

During startup, the hook will be executed with "Synchronization" binding context with "snapshots" JSON-object:

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
            "labels": { ... },
            ...
          },
        },
        "filterResult": {
          "label1": "value",
          ...
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
        },
        "filterResult": {
          "label1": "value",
          ...
        }
      },
      ...
    ],
    "snapshots": {
      "monitor-pods": [
        {
          "object": {
            "kind": "Pod,
            "metadata":{
              "name":"etcd-...",
              "namespace":"kube-system",
              ...
            },
          },
          "filterResult": { ... },
        },
        ...        
      ]
    }
  }
]
```

If pod `pod-321d12` will be added into namespace 'default', then hook will be executed with "Event" binding context with `object` and `filterResult` fields:

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
    },
    "filterResult": { ... },
    "snapshots": {
      "monitor-pods": [
        {
          "object": {
            "kind": "Pod,
            "metadata":{
              "name":"etcd-...",
              "namespace":"kube-system",
              ...
            },
          },
          "filterResult": { ... },
        },
        ...        
      ]
    }
  }
]
```

And at 12:02 it will be executed with the following binding context:

```
[
  {
    "binding": "incremental",
    "snapshots": {
      "monitor-pods": [
        {
          "object": {
            "kind": "Pod,
            "metadata":{
              "name":"etcd-...",
              "namespace":"kube-system",
              ...
            },
          },
          "filterResult": { ... },
        },
        ...        
      ]
    }
  }
]
```
