# Custom metrics

Hooks can export metrics by writing a set of operations in JSON format into $METRICS_PATH file.

Operation to register a counter and increase its value:

```json
{"name":"metric_name","action":"add","value":1,"labels":{"label1":"value1"}}
```

Operation to register a gauge and set its value:

```json
{"name":"metric_name","action":"set","value":33,"labels":{"label1":"value1"}}
```

Operation to register a histogram and observe a duration:

```json
{"name":"metric_name","action":"observe","value":42, "buckets": [1,2,5,10,20,50], "labels":{"label1":"value1"}}
```

Labels are not required, but Shell-operator adds a `hook` label with a path to a hook script relative to hooks directory.

Several metrics can be exported at once. For example, this script will create 2 metrics:

```sh
echo '{"name":"hook_metric_count","action":"add","value":1,"labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"name":"hook_metrics_items","action":"add","value":1,"labels":{"label1":"value1"}}' >> $METRICS_PATH
```

The metric name is used as-is, so several hooks can export the same metric name. It is advisable for a hooks‘ developer to maintain consistent label cardinality.

There are fields "add" and "set" that can be used as shortcuts for action and value. This feature may be deprecated in future releases.

```
{"name":"metric_name","add":1,"labels":{"label1":"value1"}}
```

Note that there is no mechanism to expire this kind of metrics except the shell-operator restart. It is the default behavior of prometheus-client.

### Grouped metrics

The common cause to expire a metric is a removed object. It means that the object is no longer in the snapshot, and the hook can't identify the metric that should be expired.

To solve this, use the "group" field in metric operations. When Shell-operator receives operations with the "group" field, it expires previous metrics with the same group and applies new metric values. This grouping works across hooks and label values.

```sh
echo '{"group":"group1", "name":"hook_metric_count",  "action":"add", "value":1, "labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"group":"group1", "name":"hook_metrics_items", "action":"add", "value":1, "labels":{"label1":"value1"}}' >> $METRICS_PATH
```

To expire all metrics in a group, use action "expire":

```
{"group":"group_name_1", "action":"expire"}
```

**WARNING**: "observe" is currently an unsupported _action_ for grouped metrics

### Example

`hook1.sh` returns these metrics:

```sh
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"pod"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"replicaset"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"hook_metric", "action":"add", "value":1, "labels":{"kind":"deployment"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"hook1_special_metric", "action":"set", "value":12, "labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"group":"hook1", "name":"common_metric", "action":"set", "value":300, "labels":{"source":"source3"}}' >> $METRICS_PATH
echo '{"name":"common_metric", "action":"set", "value":100, "labels":{"source":"source1"}}' >> $METRICS_PATH
```

`hook2.sh` returns these metrics:

```sh
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

```sh
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

```sh
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
