# Shell-operator metrics

Shell-operator implement prometheus target at /metrics endpoint. Default port is 9115.

## shell_operator_hook_allowed_errors, shell_operator_hook_errors

These metrics are counters of hook execution errors. There is a label `hook` with a name of the erroneous hook. Errors of hooks with `allowFailure: true` setting are in a separate metric to ease alert definitions.

## shell_operator_tasks_queue_length

A gauge with a length of a working queue. Can be used for alerting about long running hooks.


## shell_operator_live_ticks

A counter that increments every 10 seconds. Can be used for alerting about shell-operator malfunctioning.
