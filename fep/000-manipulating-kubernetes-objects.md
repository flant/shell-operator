# Manipulating Kubernetes objects via an opinionated CRUD framework

## Summary

### Goals

Provide an opinionated CRUD framework for Kubernetes' object manipulation.

### Non-goals

Provide direct access to Kubernetes' REST API.

## Manipulations

Let us start with enumerating operationTypes that this framework shall support.
These will implement all common object manipulation patterns.

### Creating

* `CreateOrUpdate` — accept a Kubernetes object.
  It retrieves an object, and if it already exists, computes a JSON Merge Patch and applies it (will not update .status field).
  If it does not exist, we create the object.

Optionally, we might support the following Manipulations:

* `Create` — will fail if an object already exists
* `CreateIfNotExists` — create an object if such an object does not already 
  exist by namespace/name.

These manipulations are potentially dangerous and useless, since they do not reconcile an object in any way.

### Deleting

Mirrors all REST DELETE [options](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection).

* `EnsureDeleted`
* `EnsureDeleteBackground` 
* `EnsureDeleteOrphan`

### Modification (each supports optional status subresource)

#### Considerations

1. **Bandwidth**. Raw `JSONPatch` and `MergePatch` operations will be used when performance of a hook is of utmost importance.
2. **Field ownership and locking**.
   * *Optimistic* operationTypes work well for ensuring object's consistency. An object 
        will be considered as a whole. If there is a conflict, hook's logic will re-run to ensure
        that object will transform to the desired state
   * *Fire and forget*. `FilterPatch` will work great with ensuring consistency of the whole object, but may miss
        some updates to an object if there were `resourceVersion` conflicts.

#### Implementations

##### Optimistic locking

These operationTypes will utilize REST's PUT verb. If an operation fails due to a `resourceVersion` conflict,
shell-operator shall execute a hook again, but not count it as a failure. It should increment an appropriate metric counter
(`{PREFIX}hook_run_conflicts_total`) which will signal the User about a potential bottleneck.

* `FilterPatchOptimistic` — receives an anonymous function that describes object transformations. Only used for Go hooks.

##### Fire and forget

These work similar to their Optimistic-locking counterparts with one crucial difference. If there is a `resourceVersion`
conflict, shell-operator shall not call the hook again. Instead, it will apply jq filter or transformations in the anonymous Go function,
until resource update is successful. With [an exponential backoff](https://pkg.go.dev/k8s.io/client-go@v0.19.4/util/retry#RetryOnConflict)
and upper limit, of course.

* `FilterPatch`

Also, shell-operator should accept raw versions of Kubernetes' API's patch operations:

* `JSONPatch`
* `MergePatch`

These might be used for hooks that are called often and which manipulate objects with constantly incrementing resourceVersions.
They provide weaker consistency than `FilterPatch`. Their use should be discouraged in the documentation.

##### Not implementing

We'll not implement the following patching strategies.

Strategic Merge Patch

1. Does not work on Custom Resources (defined by CRDs).
2. Patch semantics heavily depend on the current API version.

Server-side Apply

1. Too new. Once more controllers and CLI programs switch to properly using managedFields,
   we'll evaluate this strategy (which has a bright future).

# Frameworks

## Go hooks

Still work in progress. They will have access direct to all functions, except FilterPatch will be replaced with JQPatch operationType.

## Shell hooks

As with metrics, we'll utilize a file (with a path passed via an environment variable) and a custom JSON object, which
will describe desired operation.

### Path

Environment variable `$KUBERNETES_OPERATION_PATH`.

### JSON format

* `operationType` — specifies an operation's type.
* `apiVersion` — optional field that specifies object's apiVersion. If not present, we'll deduce an object's preferred
  apiVersion from `discovery` client.
* `kind` — object's Kind.
* `namespace` — object's Namespace. If empty, implies operation on a Cluster-level resource.
* `statusPatch` — optional field. If true, when modifying an object, send requests to `/status` subresource.
* `spec` — operation specification. Can contain a variety of formats that depend on the selected `operationType`.

Example:

```json
{
  "operationType": "JQPatch",
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "namespace": "default",
  "name": "nginx",
  "statusPatch": false,
  "spec": ".spec.replicas = 1"
}
```
