# Running shell-operator

## Configuration

### Run in a cluster

- Build image from flant/shell-operator:v1.0.0-beta.7, copy your hooks in /hooks directory,
- Apply RBAC manifests,
- Apply Pod or Deployment manifest with built image.

More detailed explanation is available in [README](README.md#quickstart), also see [examples](/examples).

### Run outside of a cluster

- Setup kube context,
- Prepare hooks directory,
- Run shell-operator with context and path to hooks directory.

It is not recommended for production but can be useful while debugging hooks. A scenario can be like this:

```
# Start local cluster
kind create cluster

# Start shell-operator from outside the cluster
shell-operator start --hooks-dir $(pwd)/hooks --tmp-dir $(pwd)/tmp --log-type color

```

### Environment variables and flags

You can configure the operator with the following environment variables and cli flags:

| CLI flag | Env-Variable name | Default | Description |
|---|---|---|---|
| --hooks-dir | SHELL_OPERATOR_HOOKS_DIR | `""` | A path to a hooks file structure |
| --tmp-dir | SHELL_OPERATOR_TMP_DIR | `"/tmp/shell-operator"` | A path to store temporary files with data for hooks |
| --listen-address | SHELL_OPERATOR_LISTEN_ADDRESS | `"0.0.0.0"` | Address to use for HTTP serving. |
| --listen-port | SHELL_OPERATOR_LISTEN_PORT | `"9115"` | Port to use for HTTP serving. |
| --prometheus-metrics-prefix | SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX | `"shell_operator_"` | A prefix for metrics names. |
| --kube-context | KUBE_CONTEXT | `""` | The name of the kubeconfig context to use. (as a `--context` flag of kubectl) |
| --kube-config | KUBE_CONFIG | `""` | Path to the kubeconfig file. (as a `$KUBECONFIG` for kubectl) |
| --kube-client-qps | KUBE_CLIENT_QPS | `5` | QPS for rate limiter of k8s.io/client-go |
| --kube-client-burst | KUBE_CLIENT_BURST | `10` | burst for rate limiter of k8s.io/client-go |
| --jq-library-path | JQ_LIBRARY_PATH | `""` | Prepend directory to the search list for jq modules (works as `jq -L`). |
| n/a | JQ_EXEC | `""` | Set to `yes` to use jq as executable — it is more for **developing purposes**. |
| --log-level | LOG_LEVEL | `"info"` | Logging level: `debug`, `info`, `error`. |
| --log-type | LOG_TYPE | `"text"` | Logging formatter type: `json`, `text` or `color`. |
| --log-no-time | LOG_NO_TIME | `false` | Disable timestamp logging if flag is present. Useful when output is redirected to logging system that already adds timestamps. |
| --debug-keep-tmp-files | DEBUG_KEEP_TMP_FILES | `"no"` | Set to `yes` to keep files in $SHELL_OPERATOR_TMP_DIR for debugging purposes. Note that it can generate many files. |
| --debug-unix-socket | DEBUG_UNIX_SOCKET | `"/var/run/shell-operator/debug.socket"` | Path to the unix socket file for debugging purposes. |


## Debug

The following tools for debugging and fine-tuning of Shell-operator and hooks are available:

- Analysis of logs of a Shell-operator’s pod (enter `kubectl logs -f po/POD_NAME` in terminal),
- The environment variable can be set to `LOG_LEVEL=debug` to include the detailed debugging information into logs,
- You can view the contents of the working queues with cli command from inside a Pod:
   ```
   kubectl exec -ti po/shell-operator /bin/bash
   shell-operator queue list
   ```
