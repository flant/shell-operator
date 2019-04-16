# Shell-operator metrics

Shell-operator implement prometheus target at /metrics endpoint. Default port is 9115.


__shell_operator_hook_errors{hook="hook-name"}__

A counter of hook execution errors. It counts errors for hooks with disabled `allowFailure` (no key in configuration or with explicit `allowFailure: false`).
This metric has label `hook` with a name of erroneous hook. 


__shell_operator_hook_allowed_errors{hook="hook-name"}__

A counter of hook execution errors. It counts errors for hooks that allowed to fail (`allowFailure: true`               ).
This metric has label `hook` with a name of erroneous hook. 


__shell_operator_tasks_queue_length__

A gauge with a length of a working queue. Can be used for alerting about long running hooks. This metric has no labels.


__shell_operator_live_ticks__

A counter that increments every 10 seconds. Can be used for alerting about shell-operator malfunctioning. This metric has no labels.
