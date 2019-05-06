# Shell-operator metrics

Shell-operator exports Prometheus metrics to the /metrics path. The default port is 9115.

__shell_operator_hook_errors{hook="hook-name"}__

This is the counter of hooks’ execution errors. It only tracks errors of hooks with the disabled allowFailure (i.e. respective key is omitted in the configuration or the allowFailure: false parameter is set). This metric has a “hook” label with the name of a failed hook.

__shell_operator_hook_allowed_errors{hook="hook-name"}__

This is the counter of hooks’ execution errors. It only tracks errors of hooks that are allowed to exit with an error (the parameter allowFailure: true is set in the configuration). The metric has a “hook” label with the name of a failed hook.

__shell_operator_tasks_queue_length__

As a gauge of a working queue length, this metric can be used to warn about stuck hooks. It has no labels.

__shell_operator_live_ticks__

As a counter that increases every 10 seconds, this metric can be used for alerting about a hung Shell-operator. It has no labels.
