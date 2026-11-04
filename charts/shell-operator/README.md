# shell-operator

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v1.21.4](https://img.shields.io/badge/AppVersion-v1.21.4-informational?style=flat-square)

Run event-driven shell (or any executable) hooks in a Kubernetes cluster.

The chart runs a single shell-operator replica (it has no leader election, so the Deployment uses the `Recreate` strategy) as a non-root user with a read-only root filesystem. Hooks can come from three places:

- `hooks.files`: hook files written inline in values and mounted from a ConfigMap. Good for small shell hooks.
- `hooks.existingConfigMap`: a ConfigMap you manage yourself.
- your own image built `FROM ghcr.io/flant/shell-operator` with hooks under `/hooks`. Set `image.repository` and `image.tag` and leave both options above empty.

ConfigMap keys cannot contain `/`, so hooks that need a directory tree (or extra binaries) belong in a custom image.

## Install

```console
helm install shell-operator ./charts/shell-operator \
  --namespace shell-operator --create-namespace \
  -f my-values.yaml
```

## Example

Log every namespace that is created or deleted:

```yaml
rbac:
  rules:
    - apiGroups: [""]
      resources: ["namespaces"]
      verbs: ["get", "list", "watch"]
hooks:
  files:
    namespace-hook.sh: |
      #!/usr/bin/env bash
      if [[ $1 == "--config" ]]; then
        cat <<CONFIG
      configVersion: v1
      kubernetes:
      - apiVersion: v1
        kind: Namespace
        executeHookOnEvent: ["Added", "Deleted"]
        executeHookOnSynchronization: false
      CONFIG
      else
        type=$(jq -r '.[0].type' "$BINDING_CONTEXT_PATH")
        name=$(jq -r '.[0].object.metadata.name' "$BINDING_CONTEXT_PATH")
        echo "Namespace $name event: $type"
      fi
```

## RBAC

`rbac.rules` is empty by default: list what your hooks subscribe to or change. The rules are bound with a ClusterRole, or with a Role in the release namespace when `rbac.clusterWide=false`. When a webhook is enabled the chart adds the rule shell-operator needs to manage the ValidatingWebhookConfiguration or to patch CRDs.

## Webhooks

Hooks can serve [validating](https://flant.github.io/shell-operator/BINDING_VALIDATING.html) and [conversion](https://flant.github.io/shell-operator/BINDING_CONVERSION.html) webhooks. Enable them and point the chart at a Secret holding `tls.crt`, `tls.key` and `ca.crt`, such as one issued by cert-manager. The certificate must be valid for `<fullname>-validating.<namespace>.svc` or `<fullname>-conversion.<namespace>.svc`.

```yaml
webhooks:
  validating:
    enabled: true
    certSecretName: shell-operator-validating-tls
```

## Requirements

Kubernetes: `>=1.25.0-0`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity rules. |
| commonLabels | object | `{}` | Extra labels added to every resource. |
| env | list | `[]` | Extra environment variables, see https://flant.github.io/shell-operator/RUNNING.html for the full list. |
| envFrom | list | `[]` | Environment from ConfigMaps or Secrets. |
| extraVolumeMounts | list | `[]` | Extra volume mounts. |
| extraVolumes | list | `[]` | Extra volumes. |
| fullnameOverride | string | `""` | Overrides the fully qualified resource name. |
| hooks.defaultMode | int | `365` | File mode of the mounted hook files. Hooks must be executable. |
| hooks.existingConfigMap | string | `""` | Mount an existing ConfigMap with hook files instead of `hooks.files`. |
| hooks.files | object | `{}` | Hook files rendered into a ConfigMap and mounted at `hooks.mountPath`. Keys are file names, values are the file contents. |
| hooks.mountPath | string | `"/hooks"` | Directory shell-operator loads hooks from. Leave as is when hooks are baked into the image. |
| image.digest | string | `""` | Image digest. Takes precedence over the tag when set. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy. |
| image.repository | string | `"ghcr.io/flant/shell-operator"` | Image repository. Use your own image when hooks are baked into it. |
| image.tag | string | `""` | Image tag. Defaults to the chart appVersion. |
| imagePullSecrets | list | `[]` | Secrets used to pull the image. |
| logLevel | string | `"info"` | Log level: debug, info or error. |
| logType | string | `"json"` | Log format: json, text or color. |
| metrics.port | int | `9115` | Port that serves /metrics and /metrics/hooks. |
| metrics.service.annotations | object | `{}` | Service annotations. |
| metrics.service.enabled | bool | `true` | Create a Service for the metrics port. |
| metrics.serviceMonitor.enabled | bool | `false` | Create a Prometheus Operator ServiceMonitor. |
| metrics.serviceMonitor.interval | string | `"30s"` | Scrape interval. |
| metrics.serviceMonitor.labels | object | `{}` | Extra labels, for example the one your Prometheus selects on. |
| metrics.serviceMonitor.scrapeTimeout | string | `"10s"` | Scrape timeout. |
| nameOverride | string | `""` | Overrides the chart name used in resource names and labels. |
| nodeSelector | object | `{}` | Node selector. |
| podAnnotations | object | `{}` | Extra annotations added to the pod. |
| podLabels | object | `{}` | Extra labels added to the pod. |
| podSecurityContext | object | `{"fsGroup":65534,"runAsGroup":65534,"runAsNonRoot":true,"runAsUser":65534,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod level security context. |
| priorityClassName | string | `""` | Priority class of the pod. |
| probes.liveness.enabled | bool | `true` | Enable the liveness probe on the metrics port. |
| probes.liveness.failureThreshold | int | `3` | Failures before the container is restarted. |
| probes.liveness.periodSeconds | int | `10` | Seconds between checks. |
| probes.readiness.enabled | bool | `true` | Enable the readiness probe on the metrics port. |
| probes.readiness.failureThreshold | int | `3` | Failures before the pod is marked unready. |
| probes.readiness.periodSeconds | int | `10` | Seconds between checks. |
| rbac.clusterWide | bool | `true` | Use a ClusterRole and ClusterRoleBinding. Set to false to grant the rules only in the release namespace with a Role and RoleBinding. |
| rbac.create | bool | `true` | Create the RBAC objects that bind `rbac.rules` to the ServiceAccount. |
| rbac.rules | list | `[]` | Rules the hooks need, for example get/list/watch on the kinds they subscribe to. The chart adds the webhook rules on its own. |
| resources | object | `{"limits":{"memory":"256Mi"},"requests":{"cpu":"50m","memory":"64Mi"}}` | CPU and memory for the operator and the hooks it runs. |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container level security context. |
| serviceAccount.annotations | object | `{}` | ServiceAccount annotations. |
| serviceAccount.create | bool | `true` | Create a ServiceAccount. |
| serviceAccount.name | string | `""` | ServiceAccount name. Generated when empty. |
| terminationGracePeriodSeconds | int | `30` | Grace period for running hooks to finish on shutdown. |
| tmpVolumeSizeLimit | string | `"256Mi"` | Size limit of the writable emptyDir mounted at /tmp. shell-operator keeps binding contexts there. |
| tolerations | list | `[]` | Tolerations. |
| webhooks.conversion.certSecretName | string | `""` | Secret with tls.crt, tls.key and ca.crt. |
| webhooks.conversion.enabled | bool | `false` | Serve CRD conversion webhooks declared by hooks. shell-operator patches the CRDs itself. |
| webhooks.conversion.port | int | `9681` | Container port of the HTTPS server. |
| webhooks.validating.certSecretName | string | `""` | Secret with tls.crt, tls.key and ca.crt, for example issued by cert-manager. |
| webhooks.validating.configurationName | string | `""` | Name of the ValidatingWebhookConfiguration. Defaults to the release fullname. |
| webhooks.validating.enabled | bool | `false` | Serve validating webhooks declared by hooks. shell-operator creates the ValidatingWebhookConfiguration itself. |
| webhooks.validating.failurePolicy | string | `"Fail"` | Default failure policy: Fail or Ignore. |
| webhooks.validating.port | int | `9680` | Container port of the HTTPS server. |
