# Shell-operator metrics

Shell-operator exports Prometheus metrics to the `/metrics` path. The default port is 9115.

## Metrics

* `shell_operator_hook_run_seconds{hook="", binding="", queue=""}` — a histogram with hook execution times. "hook" label is a name of the hook, "binding" is a binding name from configuration, "queue" is a queue name where hook is queued.
* `shell_operator_hook_run_errors_total{hook="hook-name", binding="", queue=""}` — this is the counter of hooks’ execution errors. It only tracks errors of hooks with the disabled `allowFailure` (i.e. respective key is omitted in the configuration or the `allowFailure: false` parameter is set). This metric has a "hook" label with the name of a failed hook.
* `shell_operator_hook_run_allowed_errors_total{hook="hook-name", binding="", queue=""}` — this is the counter of hooks’ execution errors. It only tracks errors of hooks that are allowed to exit with an error (the parameter `allowFailure: true` is set in the configuration). The metric has a "hook" label with the name of a failed hook.
* `shell_operator_hook_run_success_total{hook="hook-name", binding="", queue=""}` — this is the counter of hooks’ success execution. The metric has a "hook" label with the name of a succeeded hook.
* `shell_operator_hook_enable_kubernetes_bindings_success{hook=""}` — this gauge have two values: 0.0 if Kubernetes informers are not started and 1.0 if Kubernetes informers are successfully started for a hook.   
* `shell_operator_hook_enable_kubernetes_bindings_errors_total{hook=""}` — a counter of failed attempts to start Kubernetes informers for a hook. 
* `shell_operator_hook_enable_kubernetes_bindings_seconds{hook=""}` — a gauge with time of Kubernetes informers start.

* `shell_operator_tasks_queue_length{queue=""}` — a gauge showing the length of the working queue. This metric can be used to warn about stuck hooks. It has the "queue" label with the queue name.

* `shell_operator_task_wait_in_queue_seconds_total{hook="", binding="", queue=""}` — a counter with seconds that the task to run a hook elapsed in the queue.

* `shell_operator_live_ticks` — a counter that increases every 10 seconds. This metric can be used for alerting about an unhealthy Shell-operator. It has no labels.

* `shell_operator_kube_jq_filter_duration_seconds{hook="", binding="", queue=""}` — a histogram with jq filter timings.

* `shell_operator_kube_event_duration_seconds{hook="", binding="", queue=""}` — a histogram with kube event handling timings.

* `shell_operator_kube_snapshot_objects{hook="", binding="", queue=""}` — a gauge with count of cached objects (the snapshot) for particular binding.

* `shell_operator_kube_snapshot_bytes{hook="", binding="", queue=""}` — a gauge with size in bytes of cached objects for particular binding. Each cached object contains a Kubernetes object and/or result of jqFilter depending on the binding configuration. The size is a sum of the length of Kubernetes object in JSON format and the length of jqFilter‘s result in JSON format.

* `shell_operator_kubernetes_client_request_result_total` — a counter of requests made by kubernetes/client-go library. 

* `shell_operator_kubernetes_client_request_latency_seconds` — a histogram with latency of requests made by kubernetes/client-go library. 

* `shell_operator_tasks_queue_action_duration_seconds{queue_name="", queue_action=""}` — a histogram with measurements of low level queue operations. Use QUEUE_ACTIONS_METRICS="no" to disable this metric.

* `shell_operator_hook_run_sys_cpu_seconds{hook="", binding="", queue=""}` — a histogram with system cpu seconds.
* `shell_operator_hook_run_user_cpu_seconds{hook="", binding="", queue=""}` — a histogram with user cpu seconds.
* `shell_operator_hook_run_max_rss_bytes{hook="", binding="", queue=""}` — a gauge with maximum resident set size used in bytes.

## Custom metrics

Hooks can export metrics by writing a set of operations in JSON format into $METRICS_PATH file.

Operation to register a counter and increase its value:

```json
{"name":"metric_name","action":"add","value":1,"labels":{"label1":"value1"}}
```

Operation to register a gauge and set its value:

```json
{"name":"metric_name","action":"set","value":33,"labels":{"label1":"value1"}}
```

Operation to register a histogram and observe a duration (not yet supported for grouped metrics):

```json
{"name":"metric_name","action":"observe","value":42, "buckets": [1,2,5,10,20,50,100,200,500], "labels":{"label1":"value1"}}
```

Labels are not required, but Shell-operator adds a `hook` label with a path to a hook script relative to hooks directory.

Several metrics can be exported at once. For example, this script will create 2 metrics:

```
echo '{"name":"hook_metric_count","action":"add","value":1,"labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"name":"hook_metrics_items","action":"add","value":1,"labels":{"label1":"value1"}}' >> $METRICS_PATH
```

The metric name is used as-is, so several hooks can export same metric name. It is responsibility of hooks‘ developer to maintain consistent label cardinality.

There are fields "add" and "set" that can be used as shortcuts for action and value. This feature may be deprecated in future releases.

```
{"name":"metric_name","add":1,"labels":{"label1":"value1"}}
```

Note that there is no mechanism to expire this kind of metrics except the shell-operator restart. It is the default behavior of prometheus-client.

### Grouped metrics

The common cause to expire a metric is a removed object. It means that the object is no longer in the snapshot, and the hook can't identify the metric that should be expired.

To solve this, use the "group" field in metric operations. When Shell-operator receives operations with the "group" field, it expires previous metrics with the same group and applies new metric values. This grouping works across hooks and label values.

```
echo '{"group":"group1", "name":"hook_metric_count",  "action":"add", "value":1, "labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"group":"group1", "name":"hook_metrics_items", "action":"add", "value":1, "labels":{"label1":"value1"}}' >> $METRICS_PATH
```

To expire all metrics in a group, use action "expire":

```
{"group":"group_name_1", "action":"expire"}
```

### Example

`hook1.sh` returns these metrics:

```
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"pod"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"replicaset"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"deployment"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"hook1_special_metric", "action":"set", "value":12, "labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"common_metric", "action":"set", "value":300, "labels":{"source":"source3"}}' >> $METRICS_PATH
echo '{"name":"common_metric", "action":"set", "value":100, "labels":{"source":"source1"}}' >> $METRICS_PATH
```

`hook2.sh` returns these metrics:

```
echo '{"group":"hook2", "name":"hook_metric","action":"add", "value":1, "labels":{"kind":"configmap"}}' >> $METRICS_PATH
echo '{"group":"hook2", "name":"hook_metric","action":"add", "value":1, "labels":{"kind":"secret"}}' >> $METRICS_PATH
echo '{"group":"hook2", "name":"hook2_special_metric", "action":"set", "value":42}' >> $METRICS_PATH
echo '{"name":"common_metric", "action":"set", "value":200, "labels":{"source":"source2"}}' >> $METRICS_PATH
```

Prometheus scrapes these metrics:

```
# HELP hook_metric hook_metric
# TYPE hook_metric counter
hook_metric{hook="hook1.sh", kind="pod"} 1 -------------------+---------- group:hook1
hook_metric{hook="hook1.sh", kind="replicaset"} 1 ------------+
hook_metric{hook="hook1.sh", kind="deployment"} 1 ------------+
hook_metric{hook="hook2.sh", kind="configmap"} 1  ------------|-------+-- group:hook2
hook_metric{hook="hook2.sh", kind="secret"} 1 ----------------|-------+
# HELP hook1_special_metric hook1_special_metric              |       |
# TYPE hook1_special_metric gauge                             |       |
hook1_special_metric{hook="hook1.sh", label1="value1"} 12 ----+       |
# HELP hook2_special_metric hook2_special_metric              |       |
# TYPE hook2_special_metric gauge                             |       |
hook2_special_metric{hook="hook2.sh"} 42 ---------------------|-------'
# HELP common_metric common_metric                            |
# TYPE common_metric gauge                                    |
common_metric{hook="hook1.sh", source="source3"} 300 ---------'
common_metric{hook="hook1.sh", source="source1"} 100 ---------------+---- no group
common_metric{hook="hook2.sh", source="source2"} 200 ---------------'
```

On next execution of `hook1.sh` values for `hook_metric{kind="replicaset"}`, `hook_metric{kind="deployment"}`, `common_metric{source="source3"}` and `hook1_special_metric` are expired and hook returns only one metric:

```
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"pod"}}' >> $METRICS_PATH
```

Shell-operator expires previous values for group "hook1" and updates value for `hook_metric{hook="hook1.sh", kind="pod"}`. Values for group `hook2` and `common_metric` without group are left intact. Now Prometheus scrapes these metrics:

```
# HELP hook_metric hook_metric
# TYPE hook_metric counter
hook_metric{hook="hook1.sh", kind="pod"} 2 --------------- group:hook1
hook_metric{hook="hook2.sh", kind="configmap"} 1 ----+---- group:hook2
hook_metric{hook="hook2.sh", kind="secret"} 1 -------+
# HELP hook2_special_metric hook2_special_metric     |
# TYPE hook2_special_metric gauge                    |
hook2_special_metric{hook="hook2.sh"} 42 ------------'
# HELP common_metric common_metric
# TYPE common_metric gauge
common_metric{hook="hook1.sh", source="source1"} 100 --+-- no group
common_metric{hook="hook2.sh", source="source2"} 200 --'
```

Next execution of `hook2.sh` expires all metrics in group 'hook2':

```
echo '{"group":"hook2", "action":"expire"}' >> $METRICS_PATH
```

Shell-operator expires previous values for group "hook2" but leaves `common_metrics` for "hook2.sh" as is. Now Prometheus scrapes these metrics:

```
# HELP hook_metric hook_metric
# TYPE hook_metric counter
hook_metric{hook="hook1.sh", kind="pod"} 2 --------------- group:hook1
# HELP common_metric common_metric
# TYPE common_metric gauge
common_metric{hook="hook1.sh", source="source1"} 100 --+-- no group
common_metric{hook="hook2.sh", source="source2"} 200 --'
```
