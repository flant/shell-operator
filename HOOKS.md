# Hooks

A hook is an executable file that Shell-operator runs when some event occurs. It can be a script or a compiled program written in any programming language. For illustrative purposes, we will use bash scripts. An example with a hook in the form of a Python script is available here: [002-startup-python](examples/002-startup-python).

The hook receives the data and returns the result via files. Paths to files are passed to the hook via environment variables.

## Shell-operator lifecycle

At startup Shell-operator initializes the hooks:

- The recursive search for hook files is performed in the hooks directory. You can specify it with `--hooks-dir` command-line argument or with the `SHELL_OPERATOR_HOOKS_DIR` environment variable (the default path is `/hooks`).
  - Every executable file found in the path is considered a hook.
- Found hooks are sorted alphabetically according to the directories’ and hooks’ names. Then they are executed with the `--config` flag to get bindings to events in YAML or JSON format.
- If hook's configuration is successful, the working queue named "main" is filled with `onStartup` hooks.
- Then, the "main" queue is filled with `kubernetes` hooks with `Synchronization` [binding context](#binding-context) type, so that each hook receives all existing objects described in hook's configuration.
- After executing `kubernetes` hook with `Synchronization` binding context, Shell-operator starts a monitor of Kubernetes events according to configured `kubernetes` binding.
  - Each monitor stores a *snapshot* — a refreshable list of all Kubernetes objects that match a binding definition.

Next, the main cycle is started:

- Event handler adds hooks to the named queues on events:
  - `kubernetes` hooks are added to the queue when desired [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#watchevent-v1-meta) is received from Kubernetes,
  - `schedule` hooks are added according to the schedule,
  - `kubernetes` and `schedule` hooks are added to the "main" queue or the named queue if `queue` field was specified.

- Each named queue has its queue handler which executes hooks strictly sequentially. If hook fails with an error (non-zero exit code), Shell-operator restarts it (every 5 seconds) until it succeeds. In case of an erroneous execution of a hook, when other events occur, a queue will be filled with new tasks, but their execution will be blocked until the failing hook succeeds.
  - You can change this behavior for a specific hook by adding `allowFailure: true` to the binding configuration (not available for `onStartup` hooks).

- Each hook is executed with a binding context, that describes an already occurred event:
  - `kubernetes` hook receives `Event` binding context with an object related to the event.
  - `schedule` hook receives a name of triggered schedule binding.
  
- If there is a sequence of hook executions in a queue, then hook is executed once with array of binding contexts.
  - If binding contains `group` key, then a sequence of binding context with similar `group` key is compacted into one binding context.

- Several metrics are available for monitoring the activity of the queues and hooks: queues size, number of execution errors for specific hooks, etc. See [METRICS](METRICS.md) for more details.

## Hook configuration

Shell-operator runs the hook with the `--config` flag. In response, the hook should print its event binding configuration to stdout. The response can be in YAML format:

```yaml
configVersion: v1
onStartup: ORDER,
schedule:
- {SCHEDULE_PARAMETERS}
- {SCHEDULE_PARAMETERS}
kubernetes:
- {KUBERNETES_PARAMETERS}
- {KUBERNETES_PARAMETERS}
kubernetesValidating:
- {VALIDATING_PARAMETERS}
- {VALIDATING_PARAMETERS}
settings:
  SETTINGS_PARAMETERS
```

or in JSON format:

```yaml
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
  ],
  "kubernetesValidating": [
    {VALIDATING_PARAMETERS},
    {VALIDATING_PARAMETERS}
  ],
  "settings": {SETTINGS_PARAMETERS}
}
```

`configVersion` field specifies a version of configuration schema. The latest schema version is **v1** and it is described below.

Event binding is an event type (one of "onStartup", "schedule", "kubernetes" or "kubernetesValidating") plus parameters required for a subscription.

### onStartup

Use this binding type to execute a hook at the Shell-operator’s startup.

Syntax:

```yaml
configVersion: v1
onStartup: ORDER
```

Parameters:

`ORDER` — an integer value that specifies an execution order. "OnStartup" hooks will be sorted by this value and then alphabetically by file name.

### schedule

Scheduled execution. You can bind a hook to any number of schedules.

#### Syntax

```yaml
configVersion: v1

schedule:

- crontab: "*/5 * * * *"
  allowFailure: true|false

- name: "Every 20 minutes"
  crontab: "*/20 * * * *"
  allowFailure: true|false

- name: "every 10 seconds",
  crontab: "*/10 * * * * *"
  allowFailure: true|false
  queue: "every-ten"
  includeSnapshotsFrom: "monitor-pods"

- name: "every minute"
  crontab: "* * * * *"
  allowFailure: true|false
  group: "pods"
  ...
```

#### Parameters

- `name` — is an optional identifier. It is used to distinguish between multiple schedules during runtime. For more information see [binding context](#binding-context).

- `crontab` – is a mandatory schedule with a regular crontab syntax with 5 fields. 6 fields style crontab also supported, for more information see [documentation on robfig/cron.v2 library](https://godoc.org/gopkg.in/robfig/cron.v2).

- `allowFailure` — if ‘true’, Shell-operator skips the hook execution errors. If ‘false’ or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

- `queue` — a name of a separate queue. It can be used to execute long-running hooks in parallel with other hooks.

- `includeSnapshotsFrom` — a list of names of `kubernetes` bindings. When specified, all monitored objects will be added to the binding context in a `snapshots` field.

- `group` — a key that define a group of `schedule` and `kubernetes` bindings. See [grouping](#an-example-of-a-binding-context-with-group).

### kubernetes

Run a hook on a Kubernetes object changes.

#### Syntax

```yaml
configVersion: v1
kubernetes:
- name: "Monitor pods in cache tier"
  apiVersion: v1
  kind: Pod  # required
  executeHookOnEvent: [ "Added", "Modified", "Deleted" ]
  executeHookOnSynchronization: true|false # default is true
  keepFullObjectsInMemory: true|false # default is true
  nameSelector:
    matchNames:
    - pod-0
    - pod-1
  labelSelector:
    matchLabels:
      myLabel: myLabelValue
      someKey: someValue
    matchExpressions:
    - key: "tier"
      operator: "In"
      values: ["cache"]
    # - ...
  fieldSelector:
    matchExpressions:
    - field: "status.phase"
      operator: "Equals"
      value: "Pending"
    # - ...
  namespace:
    nameSelector:
      matchNames: ["somenamespace", "proj-production", "proj-stage"]
    labelSelector:
      matchLabels:
        myLabel: "myLabelValue"
        someKey: "someValue"
      matchExpressions:
      - key: "env"
        operator: "In"
        values: ["production"]
      # - ...
  jqFilter: ".metadata.labels"
  includeSnapshotsFrom:
  - "Monitor pods in cache tier"
  - "monitor Pods"
  - ...
  allowFailure: true|false  # default is false
  queue: "cache-pods"
  group: "pods"

- name: "monitor Pods"
  kind: "pod"
  # ...
```

#### Parameters

- `name` is an optional identifier. It is used to distinguish different bindings during runtime. See also [binding context](#binding-context).

- `apiVersion` is an optional group and version of object API. For example, it is `v1` for core objects (Pod, etc.), `rbac.authorization.k8s.io/v1beta1` for ClusterRole and `monitoring.coreos.com/v1` for prometheus-operator.

- `kind` is the type of a monitored Kubernetes resource. This field is required. CRDs are supported, but the resource should be registered in the cluster before Shell-operator starts. This can be checked with `kubectl api-resources` command. You can specify a case-insensitive name, kind or short name in this field. For example, to monitor a DaemonSet these forms are valid:

  ```text
  "kind": "DaemonSet"
  "kind": "Daemonset"
  "kind": "daemonsets"
  "kind": "DaemonSets"
  "kind": "ds"
  ```

- `executeHookOnEvent` — the list of events which led to a hook's execution. By default, all events are used to execute a hook: "Added", "Modified" and "Deleted". Docs: [Using API](https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes) [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#watchevent-v1-meta). Empty array can be used to prevent hook execution, it is useful when binding is used only to define a snapshot.

- `executeHookOnSynchronization` — if `false`, Shell-operator skips the hook execution with Synchronization binding context. See [binding context](#binding-context).

- `nameSelector` — selector of objects by their name. If this selector is not set, then all objects of a specified Kind are monitored.

- `labelSelector` — [standard](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#labelselector-v1-meta) selector of objects by labels (examples [of use](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels)).
  If the selector is not set, then all objects of a specified kind are monitored.

- `fieldSelector` — selector of objects by their fields, works like `--field-selector=''` flag of `kubectl`. Supported operators are Equals (or `=`, `==`) and NotEquals (or `!=`) and all expressions are combined with AND. Also, note that fieldSelector with 'metadata.name' the field is mutually exclusive with nameSelector. There are limits on fields, see [Note](#fieldselector).

- `namespace` — filters to choose namespaces. If omitted, events from all namespaces will be monitored.

- `namespace.nameSelector` — this filter can be used to monitor events from objects in a particular list of namespaces.

- `namespace.labelSelector` — this filter works like `labelSelector` but for namespaces and Shell-operator dynamically subscribes to events from matched namespaces.

- `jqFilter` —  an optional parameter that specifies event **filtering** using [jq syntax](https://stedolan.github.io/jq/manual/). The hook will be triggered on the "Modified" event only if the filter result is *changed* after the last event. See example [102-monitor-namespaces](examples/102-monitor-namespaces).

- `allowFailure` — if `true`, Shell-operator skips the hook execution errors. If `false` or the parameter is not set, the hook is restarted after a 5 seconds delay in case of an error.

- `queue` — a name of a separate queue. It can be used to execute long-running hooks in parallel with hooks in the "main" queue.

- `includeSnapshotsFrom` — an array of names of `kubernetes` bindings in a hook. When specified, a list of monitored objects from that bindings will be added to the binding context in a `snapshots` field. Self-include is also possible.

- `keepFullObjectsInMemory` — if not set or `true`, dumps of Kubernetes resources are cached for this binding, and the snapshot includes them as `object` fields. Set to `false` if the hook does not rely on full objects to reduce the memory footprint.

- `group` — a key that define a group of `schedule` and `kubernetes` bindings. See [grouping](#an-example-of-a-binding-context-with-group).

#### Example

```yaml
configVersion: v1
kubernetes:
# Trigger on labels changes of Pods with myLabel:myLabelValue in any namespace
- name: "label-changes-of-mylabel-pods"
  kind: pod
  executeHookOnEvent: ["Modified"]
  labelSelector:
    matchLabels:
      myLabel: "myLabelValue"
  namespace:
    nameSelector: ["default"]
  jqFilter: .metadata.labels
  allowFailure: true
  includeSnapshotsFrom: ["label-changes-of-mylabel-pods"]
```

This hook configuration will execute hook on each change in labels of pods labeled with `myLabel=myLabelValue` in "default" namespace. The binding context will contain all pods with `myLabel=myLabelValue` from "default" namespace.

#### Notes

##### default Namespace

Unlike `kubectl` you should explicitly define `namespace.nameSelector` to monitor events from `default` namespace.

```yaml
  namespace:
    nameSelector: ["default"]
```

##### RBAC is required

Shell-operator requires a ServiceAccount with the appropriate [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) permissions. See examples with RBAC: [monitor-pods](examples/101-monitor-pods) and [monitor-namespaces](examples/102-monitor-namespaces).

##### jqFilter

This filter is used to *ignore* superfluous "Modified" events, *and* to *exclude* object from event subscription. For example, if the hook should track changes of object's labels, `jqFilter: ".metadata.labels"` can be used to ignore changes in other properties (`.status`,`.metadata.annotations`, etc.).

The result of applying the filter to the event's object is passed to the hook in a `filterResult` field of a [binding context](#binding-context).

You can use `JQ_LIBRARY_PATH` environment variable to set a path with `jq` modules.

##### Added != Object created

Consider that the "Added" event is not always equal to "Object created" if `labelSelector`, `fieldSelector` or `namespace.labelSelector` is specified in the `binding`. If objects and/or namespace are updated in Kubernetes, the `binding` may suddenly start matching them, with the "Added" event. The same with "Deleted" event: "Deleted" is not always equal to "Object removed", the object can just move out of a scope of selectors.

##### fieldSelector

There is no support for filtering by arbitrary field neither for core resources nor for custom resources (see [issue#53459](https://github.com/kubernetes/kubernetes/issues/53459)). Only `metadata.name` and `metadata.namespace` fields are commonly supported.

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

```yaml
kind: Pod
fieldSelector:
  matchExpressions:
  - field: "status.phase"
    operator: Equals
    value: Running
```

##### fieldSelector and labelSelector expressions are ANDed

Objects should match all expressions defined in `fieldSelector` and `labelSelector`, so, for example, multiple `fieldSelector` expressions with `metadata.name` field and different values will not match any object.

### kubernetesValidating

Use a hook as handler for [ValidationWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers).

See syntax and parameters in [BINDING_VALIDATING.md](BINDING_VALIDATING.md)

### kubernetesCustomResourceConversion

Use a hook as handler for [custom resource conversion](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning).

See syntax and parameters in [BINDING_CONVERSION.md](BINDING_CONVERSION.md)

## Binding context

When an event associated with a hook is triggered, Shell-operator executes the hook without arguments. The information about the event that led to the hook execution is called the **binding context** and is written in JSON format to a temporary file. The path to this file is available to hook via environment variable `BINDING_CONTEXT_PATH`.

Temporary files have unique names to prevent collisions between queues and are deleted after the hook run.

Binging context is a JSON-array of structures with the following fields:

- `binding` — a string from the `name` or `group` parameters. If these parameters has not been set in the binding configuration, then strings "schedule" or "kubernetes" are used. For a hook executed at startup, this value is always "onStartup".
- `type` — "Schedule" for `schedule` bindings. "Synchronization" or "Event" for `kubernetes` bindings. "Group" if `group` is defined.

The hook receives "Event"-type binding context on Kubernetes event and it contains more fields:
- `watchEvent` — the possible value is one of the values you can use with `executeHookOnEvent` parameter: "Added", "Modified" or "Deleted".
- `object` — a JSON dump of the full object related to the event. It contains an exact copy of the corresponding field in [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#watchevent-v1-meta) response, so it's the object state **at the moment of the event** (not at the moment of the hook execution).
- `filterResult` — the result of `jq` execution with specified `jqFilter` on the above mentioned object. If `jqFilter` is not specified, then `filterResult` is omitted.

The hook receives existed objects on startup for each binding with "Synchronization"-type binding context:
- `objects` — a list of existing objects that match selectors in binding configuration. Each item of this list contains `object` and `filterResult` fields. The state of items is actual **for the moment of the hook execution**. If the list is empty, the value of `objects` is an empty array.

If `group` or `includeSnapshotsFrom` are defined, the hook receives binding context with additional field:
- `snapshots` — a map that contains an up-to-date lists of objects for each binding name from `includeSnapshotsFrom` or for each `kubernetes` binding with a similar `group`. If `includeSnapshotsFrom` list is empty, the field is omitted.

### `onStartup` binding context example

Hook with this configuration:

```yaml
configVersion: v1
onStartup: 1
```

will be executed with this binding context at startup:

```json
[{"binding": "onStartup"}]
```

### `schedule` binding context example

For example, if you have the following configuration in a hook:

```yaml
configVersion: v1
schedule:
- name: incremental
  crontab: "0 2 */3 * * *"
  allowFailure: true
```

then at 12:02, it will be executed with the following binding context:

```json
[{ "binding": "incremental", "type":"Schedule"}]
```

### `kubernetes` binding context example

A hook can monitor Pods in all namespaces with this simple configuration:

```yaml
configVersion: v1
kubernetes:
- kind: Pod
```

#### "Synchronization" binding context

During startup, the hook receives all existing objects with "Synchronization"-type binding context:

```yaml
[
  {
    "binding": "kubernetes",
    "type": "Synchronization",
    "objects": [
      {
        "object": {
          "kind": "Pod",
          "metadata":{
            "name":"etcd-...",
            "namespace":"kube-system",
            ...
          },
        }
      },
      {
        "object": {
          "kind": "Pod",
          "metadata": {
            "name": "kube-proxy-...",
            "namespace": "kube-system",
            ...
          },
        }
      },
      ...
    ]
  }
]
```

> Note: hook execution at startup with "Synchronization" binding context can be turned off with `executeHookOnSynchronization: false`

#### "Event" binding context

If pod `pod-321d12` is then added into namespace 'default', then the hook will be executed with the "Event"-type binding context:

```yaml
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

### Snapshots

"Event"-type binding context contains an object state at the moment of the event. Actual objects' state for the moment of the execution can be received in a form of _Snapshots_.

Shell-operator maintains an up-to-date list of objects for each `kubernetes` binding. `schedule` and `kubernetes` bindings can be configured to receive these lists via `includeSnapshotsFrom` parameter. Also, there is a `group` parameter to automatically receive all snapshots from multiple bindings and to deduplicate executions.

Snapshot is a JSON array of Kubernetes objects and corresponding jqFilter results. To access the snapshot during the hook execution, there is a map `snapshots` in the binding context. The key of this map is a binding name, and the value is the snapshot.

`snapshots` example:

```yaml
[
  { "binding": ...,
    "snapshots": {
      "binding-name-1": [ 
        {
          "object": {
            "kind": "Pod",
            "metadata": {
              "name": "etcd-...",
              "namespace": "kube-system",
              ...
            },
          },
          "filterResult": { ... },
        },
        ...
      ]
    }}]
```

- `object` — a JSON dump of Kubernetes object.
- `filterResult` — a JSON result of applying `jqFilter` to the Kubernetes object.

Keeping dumps for `object` fields can take a lot of memory. There is a parameter `keepFullObjectsInMemory: false` to disable full dumps.
 
Note that disabling full objects make sense only if `jqFilter` is defined, as it disables full objects in `snapshots` field, `objects` field of "Synchronization" binding context and `object` field of "Event" binding context.

For example, this binding configuration will execute hook with empty items in `objects` field of "Synchronization" binding context:
 
```yaml
kubernetes:
- name: pods
  kinds: Pod   
  keepFullObjectsInMemory: false
```

### Snapshots example

To illustrate the `includeSnapshotsFrom` parameter, consider the hook that reacts to changes of labels of all Pods and requires the content of the ConfigMap named "settings-for-my-hook". There is also a schedule to do periodic checks:

```yaml
configVersion: v1
schedule:
- name: periodic-checking
  crontab: "0 */3 * * *"
  includeSnapshotsFrom: ["monitor-pods", "cm"]
kubernetes:
- name: configmap-content
  kind: ConfigMap
  nameSelector:
    matchNames: ["settings-for-my-hook"]
  executeHookOnSynchronization: false
  executeHookOnEvent: []
- name: monitor-pods
  kind: Pod
  jqFilter: '.metadata.labels'
  includeSnapshotsFrom: ["cm"]
```

This hook will not be executed for events related to the binding "configmap-content". `executeHookOnSynchronization: false` accompanied by `executeHookOnEvent: []` defines a "snapshot-only" binding. This is one of the techniques to reduce the number of `kubectl` invocations.

#### "Synchronization" binding context with snapshots

During startup, the hook will be executed with the "Synchronization" binding context with `snapshots` and `objects`:

```yaml
[
  {
    "binding": "monitor-pods",
    "type": "Synchronization",
    "objects": [
      {
        "object": {
          "kind": "Pod",
          "metadata": {
            "name": "etcd-...",
            "namespace": "kube-system",
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
          "kind": "Pod",
          "metadata": {
            "name": "kube-proxy-...",
            "namespace": "kube-system",
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
      "configmap-content": [
        {
          "object": {
            "kind": "ConfigMap",
            "metadata": {"name": "settings-for-my-hook", ... },
            "data": {"field1": ... }
          }
        }
      ]
    }
  }
]
```

#### "Event" binding context with snapshots

If pod `pod-321d12` is then added into the "default" namespace, then the hook will be executed with the "Event" binding context with `object` and `filterResult` fields:

```yaml
[
  {
    "binding": "monitor-pods",
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
      "configmap-content": [
        {
          "object": {
            "kind": "ConfigMap",
            "metadata": {"name": "settings-for-my-hook", ... },
            "data": {"field1": ... }
          }
        }
      ]
    }
  }
]
```

#### "Schedule" binding context with snapshots

Every 3 hours, the hook will be executed with the binding context that include 2 snapshots ("monitor-pods" and "configmap-content"):

```yaml
[
  {
    "binding": "periodic-checking",
    "type": "Schedule",
    "snapshots": {
      "monitor-pods": [
        {
          "object": {
            "kind": "Pod",
            "metadata": {
              "name": "etcd-...",
              "namespace": "kube-system",
              ...
            },
          },
          "filterResult": { ... },
        },
        ...
      ],
      "configmap-content": [
        {
          "object": {
            "kind": "ConfigMap",
            "metadata": {"name": "settings-for-my-hook", ... },
            "data": {"field1": ... }
          }
        }
      ]
    }
  }
]
```

### Binding context of grouped bindings

`group` parameter defines a named group of bindings. Group is used when the source of the event is not important, and data in snapshots is enough for the hook. When binding with `group` is triggered with the event, the hook receives snapshots from all `kubernetes` bindings with the same `group` name.

Adjacent tasks for `kubernetes` and `schedule` bindings with the same `group` and `queue` are "compacted", and the hook is executed only once. So it is wise to use the same `queue` for all hooks in a group. This "compaction" mechanism is not available for `kubernetesValidating` and `kubernetesCustomResourceConversion` bindings as they're not queued.

`executeHookOnSynchronization`, `executeHookOnEvent` and `keepFullObjectsInMemory` can be used  with `group`. Their effects are as described above for non-grouped bindings.

`group` parameter is compatible with `includeSnapshotsFrom` parameter. `includeSnapshotsFrom` can be used to include additional snapshots into binding context.

Binding context for group contains:
- `binding` field with the group name.
- `type` field with the value "Group".
- `snapshots` field if there is at least one `kubernetes` binding in the group or `includeSnapshotsFrom` is not empty.

### Group binding context example

Consider the hook that is executed on changes of labels of all Pods, changes in ConfigMap's data and also on schedule:

```yaml
configVersion: v1
schedule:
- name: periodic-checking
  crontab: "0 */3 * * *"
  group: "pods"
kubernetes:
- name: monitor-pods
  apiVersion: v1
  kind: Pod
  jqFilter: '.metadata.labels'
  group: "pods"
- name: configmap-content
  apiVersion: v1
  kind: ConfigMap
  nameSelector:
    matchNames: ["settings-for-my-hook"]
  jqFilter: '.data'
  group: "pods"

```

#### binding context for grouped bindings

Grouped bindings is used when only the occurrence of an event is important. So, the hook receives actual state of Pods and the ConfigMap on every of these events:

- During startup.
- A new Pod is added.
- The Pod is deleted.
- Labels of the Pod are changed.
- ConfigMap/settings-for-my-hook is deleted.
- ConfigMap/settings-for-my-hook is added.
- Data field is changed in ConfigMap/settings-for-my-hook.
- Every 3 hours.

Binding context for these events will be the same:

```yaml
[
  {
    "binding": "pods",
    "type": "Group",
    "snapshots": {
      "monitor-pods": [
        {
          "object": {
            "kind": "Pod",
            "metadata": {
              "name": "etcd-...",
              "namespace": "kube-system",
              ...
            },
          },
          "filterResult": { ... },
        },
        ...
      ],
      "configmap-content": [
        {
          "object": {
            "kind": "ConfigMap",
            "metadata": {
              "name": "etcd-...",
              "namespace": "kube-system",
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

### settings

An optional block with hook-level settings.

#### Syntax

```yaml
configVersion: v1
settings:
  executionMinInterval: 3s
  executionBurst: 1
```

#### Parameters

- `executionMinInterval` defines a minimum time between hook executions.
- `executionBurst` a number of allowed executions during a period.

#### Execution rate

`executionMinInterval` and `executionBurst` are parameters for "token bucket" algorithm. These parameters are used to throttle hook executions and wait for more events in the queue. It is wise to use a separate queue for bindings in such a hook, as a hook with execution rate settings and with default ("main") queue can hold the execution of other hooks.

#### Example

```yaml
configVersion: v1
kubernetes:
- name: "all-pods-in-ns"
  kind: pod
  executeHookOnEvent: ["Modified"]
  namespace:
    nameSelector: ["default"]
  queue: handle-pods-queue
settings:
  executionMinInterval: 3s
  executionBurst: 1
```

If the Shell-operator will receive a lot of events for the "all-pods-in-ns" binding, the hook will be executed no more than once in 3 seconds.
