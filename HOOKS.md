# HOOKs

A hook is a script in any language you prefer with an added events configuration code. Shell-operator will execute the script in a Kubernetes cluster on events you configured.

> A corresponding runtime environment must be added into the image to execute a hook.

Every hook must have an argument `--config`. Getting `--config` as an argument, hook should return to `stdout` its [bindings configuration](#bindings) as a JSON structure.

When one or more events the hook is bound are triggered, Shell-operator executes the hook without arguments but with the following environment variables set:
- BINDING_CONTEXT_PATH — [execution context](#execution-context), contains a path to a JSON file with information about an event that was triggered.
- WORKING_DIR — contains a path to a hooks directory. It can be helpful when you use shared libraries in your hooks.

## How hooks run

Shell-operator searches and runs hooks going through the following steps:
- Recursively searches hooks in a working directory. A working directory is `"/hooks"` by default and can be specified wiht a WORKING_DIR environment variable or with a `--working-dir` argument (overrides WORKING_DIR):
    - directories starting from the dot symbol are excluded
    - directories and files are sorted alphabetically
    - every **executable** file not starting from dot symbol counts as a hook.
- Executes hooks to get hooks configuration:
    - hooks are executed in the order found (see above)
    - each hook is executed in the hook directory in which it is located with the `--config` argument and the WORKING_DIR environment variable set
    - hook returns to `stdout` a JSON array of its [bindings configuration](#bindings).
- Executes hooks depending on the schedule and Kubernetes events:
    - if there is more than one hook for a triggered event, hooks executed sequentially in the same order they have been found.
- Export [metrics](METRICS.md) to Prometheus.

Shell operator executes each hook one by one (without concurrency), and executes each hook until it finishes. On that note, you may have a question — "What happens if one of the hooks fails?".

### What happens if hook fails?

If hook fails (exit with exit status > 0) Shell-operator does the following (by default):
- writes the corresponding message to stdout (use e.g. `kubectl logs` to see it)
- increases the `shell_operator_hook_errors` counter
- waits for 3 seconds
- tries to execute the hook again...

If you want Shell-operator to skip failed hook, you can use the `allowFailure: yes` option for `schedule` and `onKubernetesEvent` binding types.

## Hook configuration

Hook must return to stdout its binding configuration when the hook is executed with the `--config` argument. Binding configuration is the following JSON structure:

```
{
  "BINDING_TYPE": "BINDING_PARAMETERS",
  ...
  "BINDING_TYPE": "BINDING_PARAMETERS",
  "BINDING_TYPE": "BINDING_PARAMETERS"
}
```

Where:
- BINDING_NAME is one of `onStartup`, `schedule`, `onKubernetesEvent` binding types.
- BINDING_PARAMETERS is a value for binding or a JSON array with binding parameters (learn the [bindings](#bindings) section below)

### Bindings

There are three types of bindings:
- onStartup
- schedule
- onKubernetesEvent

#### onStartup

This type of binding tells Shell-operator to execute hook once at Shell-operator startup. The `onStartup` binding requires only a single parameter — a hook execution order.

Syntax:
```
{
  "onStartup": ORDER
}
```

#### schedule

This type of binding tells Shell-operator to execute a hook on a specified schedule or schedules. You can bind one hook for any number of schedules. The `schedule` binding requires a JSON array of event descriptors.

Syntax:
```
{
  "schedule": [
    {
      "name": "EVENT_NAME",
      "crontab": CRON_EXPRESSION,
      "allowFailure": true|false,
    },
    {
      "crontab": CRON_EXPRESSION,
      "allowFailure": true|false,
    },
    ...
  ]
}
```

The `name` is an optional parameter. It specifies a binding name, which hook can get in the execution time from the [execution context](#execution-context)

The `CRON_EXPRESSION` is a crontab syntax with 6 values separated by spaces, each of them can be:
- a number
- a set of numbers separated by commas
- a range, — two numbers separated by a hyphen
- an asterisk `*` or a slash `/`
- [predefined values](https://godoc.org/github.com/robfig/cron#hdr-Predefined_schedules)
- [intervals](https://godoc.org/github.com/robfig/cron#hdr-Intervals)

Syntax:
```
* * * * * *
| | | | | |
| | | | | +---- Day of the Week   (range: 1-7, 1 standing for Monday)
| | | | +------ Month of the Year (range: 1-12)
| | | +-------- Day of the Month  (range: 1-31)
| | +---------- Hour              (range: 0-23)
| +------------ Minute            (range: 0-59)
+-------------- Second            (range: 0-59)
```

The `allowFailure` — is an optional field, false by default. When true, Shell-operator will not try to re-execute hook if it fails and will skip it.

#### onKubernetesEvent

This type of binding tells Shell-operator to execute hook on a specified event from a Kubernetes cluster.

The `onKubernetesEvent` binding requires a JSON array of event descriptors. Each event descriptor assumes tracking object events of only one kind (see below).

Syntax:
```
{
  "onKubernetesEvent": [
    {
      "name": "EVENT_NAME",
      "kind": RESOURCE,
      "event": EVENTS_LIST,
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
        "matchNames": ["somenamespace", "bush-production", "bush-stage"],
        "any": true|false,
      },
      "jqFilter": ".metadata.labels",
      "allowFailure": true|false,
      "disableDebug": true|false,
    },
    ...
  ]
}
```

Where:
- `name` — is an optional parameter, specifies a binding name, which hook can get in the execution time from the [execution context](#execution-context).
- `kind` — is a kind of a Kubernetes object, events of which is required to bind. Case-insensitive. Can be one of the following:
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
- `event` — is an optional parameter, by default equals to `["add", "update", "delete"]`. Specifies an array of Kubernetes events to bind, can be any combination of:
    - `add` — an event of adding a Kubernetes resource of a specified `kind`
    - `update` — an event of updating a Kubernetes resource of a specified `kind`
    - `delete` — an event of deleting a Kubernetes resource of a specified `kind`.
- `selector` — is an optional parameter, specifies the condition for selecting a subset of objects. The `selector.matchLabels` is a `{key,value}` Kubernetes filter by lable/selector. The `selector.matchExpressions` is a standard Kubernetes list of pod selector requirements (learn more about Kubernetes labels and selectors [here](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels)). If you don't specify a `selector`, all objects of this kind are assumed.
- `namespaceSelector` — is an optional parameter, by defaul equals to `namespaceSelector.any=true`. Specifies a filter for namespaces for selecting objects. `namespaceSelector.matchNames` can contain an array of namespaces for selecting objects.
- `jqFilter` — is an optional parameter, specifies additional event filter. Since a Kubernetes object has many events during its life, binding on the `update` event may give you a lot of events that you may not need. The `jqFilter` option allows binding hook only on necessary events because Shell-operator will apply defined in `jqFilter` option filter before binding to objects.
    - Suppose you want to execute a hook only when labels of the specified object have changed. Then you can set `"jqFilter":".metadata.labels"` and the hook will be executed only on labels of the object have changed.
- `disableDebug` — is an optional field, disables debug messages for a specified event when true.
- `allowFailure` — is an optional field, false by default. When true, Shell-operator will not try to re-execute hook if it fails and will skip it.

Example:
```
{
  "onKubernetesEvent": [
    {
      "name": "myLabelValue pods",
      "kind": "pod",
      "event": ["add", "delete"],
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

> Note, than Shell-operator Deployment must have the serviceAccount with the appropriate rights to monitor resources in a Kubernetes cluster.

## Execution context

As hook can have more than one bindings, it should identify a binding (or in other words — an event) which was triggered. When a hook is executing, the BINDING_CONTEXT_PATH environment variable contains a path to an execution context — a JSON file with information about the binding that was triggered.

The execution context contains the following fields:
- `binding` — contains the name of the binding type or a `name` value from the binding configuration (if set)
- `resourceNamespace`, `resourceKind`, `resourceName`, `resourceEvent` — corresponding values for identifying a resource on an `onKubernetesEvent` binding.

For example, you have the following hook binding configuration:
```json
{
  "schedule": [
    {
      "name": "incremental",
      "crontab": "* 2 */3 * * *",
      "allowFailure": true
    },
    {
      "crontab": "* 59 23 * * *",
      "allowFailure": false
    }
  ]
}
```

At 12:02 this hook will be executed with the following context:
```json
[{ "binding": "incremental"}]
```

At 23:59 this hook will be executed with the following context:
```json
[{ "binding": "schedule"}]
```

## Debug

For debugging you can use the following:
- get logs of a Shell-operator pod by using `kubectl logs <POD_NAME>`
- get a hook queue content in the `/tmp/shell-operator-queue` file of a Shell-operator pod
- get a hook queue content by executing CURL to the `POD_IP:9115/queue` (e.g. from a Shell-operator pod itself).
