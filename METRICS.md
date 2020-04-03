# Shell-operator metrics

Shell-operator exports Prometheus metrics to the `/metrics` path. The default port is 9115.

## Metrics

* `shell_operator_hook_errors{hook="hook-name"}` – this is the counter of hooks’ execution errors. It only tracks errors of hooks with the disabled `allowFailure` (i.e. respective key is omitted in the configuration or the `allowFailure: false` parameter is set). This metric has a “hook” label with the name of a failed hook.
* `shell_operator_hook_allowed_errors{hook="hook-name"}` – this is the counter of hooks’ execution errors. It only tracks errors of hooks that are allowed to exit with an error (the parameter `allowFailure: true` is set in the configuration). The metric has a “hook” label with the name of a failed hook.
* `shell_operator_tasks_queue_length` – a gauge showing the length of the working queue. This metric can be used to warn about stuck hooks. It has no labels.
* `shell_operator_live_ticks` – a counter that increases every 10 seconds. This metric can be used for alerting about an unhealthy Shell-operator. It has no labels.

## Custom metrics

Hooks can export metrics by writing a set of operation on JSON format into $METRICS_PATH file.

Operation to increase a counter:

```json
{"name":"metric_name","add":1,"labels":{"label1":"value1"}}
```

Operation to set a value for a gauge:

```json
{"name":"metric_name","set":33,"labels":{"label1":"value1"}}
```

Labels are not required, but Shell-operator adds a `hook` label.

Several metrics can be expored at once. For example, this script will create 2 metrics:

```
echo '{"name":"hook_metric_count","add":1,"labels":{"label1":"value1"}}' >> $METRICS_PATH
echo '{"name":"hook_metrics_items","add":1,"labels":{"label1":"value1"}}' >> $METRICS_PATH
```
