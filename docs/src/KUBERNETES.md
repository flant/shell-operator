# Modifying Kubernetes objects

You can delegate Kubernetes object manipulation to the shell-operator.

To do this, you have to write one or more JSON or YAML documents describing operation type and its parameters to a file.
List of possible operations and corresponding JSON specifications can be found below.

The path to the file is found in the `$KUBERNETES_PATCH_PATH` environment variable.

## Operations

### Create

* `operation` — specifies an operation's type.
    * `CreateOrUpdate` — accept a Kubernetes object.
      It retrieves an object, and if it already exists, computes a JSON Merge Patch and applies it (will not update .status field).
      If it does not exist, we create the object.
    * `Create` — will fail if an object already exists
    * `CreateIfNotExists` — create an object if such an object does not already
      exist by namespace/name.
* `object` — full object specification including "apiVersion", "kind" and all necessary metadata. Can be a normal JSON or YAML object or a stringified JSON or YAML object.

#### Example

```json
{
  "operation": "CreateOrUpdate",
  "object": {
    "apiVersion": "apps/v1",
    "kind": "DaemonSet",
    "metadata": {
      "name": "flannel",
      "namespace": "d8-flannel"
    },
    "spec": {
      "selector": {
        "matchLabels": {
          "app": "flannel"
        }
      },
      "template": {
        "metadata": {
          "labels": {
            "app": "flannel",
            "tier": "node"
          }
        },
        "spec": {
          "containers": [
            {
              "args": [
                "--ip-masq",
                "--kube-subnet-mgr"
              ],
              "image": "flannel:v0.11",
              "name": "kube-flannel",
              "securityContext": {
                "privileged": true
              }
            }
          ],
          "hostNetwork": true,
          "imagePullSecrets": [
            {
              "name": "registry"
            }
          ],
          "terminationGracePeriodSeconds": 5
        }
      },
      "updateStrategy": {
        "type": "RollingUpdate"
      }
    }
  }
}
```

```yaml
operation: Create
object:
  apiVersion: v1
  kind: ConfigMap
  metadata:
    namespace: default
    name: testcm
  data: ...
```

```yaml
operation: Create
object: |
  {"apiVersion":"v1", "kind":"ConfigMap",
   "metadata":{"namespace":"default","name":"testcm"},
   "data":{"foo": "bar"}}
```

### Delete

* `operation` — specifies an operation's type. Deletion types map directly to Kubernetes
  DELETE's [`propagationPolicy`][controller-gc].
  * `Delete` — foreground deletion. Hook will block the queue until the referenced object and all its descendants are deleted.
  * `DeleteInBackground` — will delete the referenced object immediately. All its descendants will be removed by Kubernetes'
    garbage collector.
  * `DeleteNonCascading` — will delete the referenced object immediately, and orphan all its descendants.
* `apiVersion` — optional field that specifies object's apiVersion. If not present, we'll use preferred apiVersion
  for the given kind.
* `kind` — object's Kind.
* `namespace` — object's namespace. If empty, implies operation on a cluster-level resource.
* `name` — object's name.
* `subresource` — a subresource name if subresource is to be transformed. For example, `status`.

#### Example

```json
{
  "operation": "Delete",
  "kind": "Pod",
  "namespace": "default",
  "name": "nginx"
}
```

### Patch

Use `JQPatch` for almost everything. Consider using `MergePatch` or `JSONPatch` if you are attempting to modify 
rapidly changing object, for example `status` field with many concurrent changes (and incrementing `resourceVersion`).

Be careful, when updating a `.status` field. If a `/status` subresource is enabled on a resource,
it'll ignore updates to the `.status` field if you haven't specified `subresource: status` in the operation spec.
More info [here][spec-and-status].

#### JQPatch

* `operation` — specifies an operation's type.
* `apiVersion` — optional field that specifies object's apiVersion. If not present, we'll use preferred apiVersion
  for the given kind.
* `kind` — object's Kind.
* `namespace` — object's Namespace. If empty, implies operation on a Cluster-level resource.
* `name` — object's name.
* `jqFilter` — describes transformations to perform on an object.
* `subresource` — a subresource name if subresource is to be transformed. For example, `status`.
##### Example

```json
{
  "operation": "JQPatch",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "jqFilter": ".spec.replicas = 1"
}
```

#### MergePatch

* `operation` — specifies an operation's type.
* `apiVersion` — optional field that specifies object's apiVersion. If not present, we'll use preferred apiVersion
  for the given kind.
* `kind` — object's Kind.
* `namespace` — object's Namespace. If empty, implies operation on a Cluster-level resource.
* `name` — object's name.
* `mergePatch` — describes transformations to perform on an object. Can be a normal JSON or YAML object or a stringified JSON or YAML object.
* `subresource` — e.g., `status`.
* `ignoreMissingObject` — set to true to ignore error when patching non existent object.

##### Example

```json
{
  "operation": "MergePatch",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "mergePatch": {
    "spec": {
      "replicas": 1
    }
  }
}
```

```yaml
operation: MergePatch
kind: Deployment
namespace: default
name: nginx
ignoreMissingObject: true
mergePatch: |
  spec:
    replicas: 1
```

#### JSONPatch

* `operation` — specifies an operation's type.
* `apiVersion` — optional field that specifies object's apiVersion. If not present, we'll use preferred apiVersion
  for the given kind.
* `kind` — object's Kind.
* `namespace` — object's Namespace. If empty, implies operation on a Cluster-level resource.
* `name` — object's name.
* `jsonPatch` — describes transformations to perform on an object. Can be a normal JSON or YAML array or a stringified JSON or YAML array.
* `subresource` — a subresource name if subresource is to be transformed. For example, `status`.
* `ignoreMissingObject` — set to true to ignore error when patching non existent object.

##### Example

```json
{
  "operation": "JSONPatch",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "jsonPatch": [
    {"op": "replace", "path":  "/spec/replicas", "value":  1}
  ]
}
```

```json
{
  "operation": "JSONPatch",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "jsonPatch": "[
    {\"op\": \"replace\", \"path\":  \"/spec/replicas\", \"value\":  1}
  ]"
}
```

[controller-gc]: https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/
[spec-and-status]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
