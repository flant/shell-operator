# pkg/kube/dedupclient

Two-part integration of
[`github.com/ldmonster/kubeclient`](https://github.com/ldmonster/kubeclient)
into shell-operator. Both parts can be enabled independently and target
different memory holders in the Kubernetes binding path.

| Component        | Type                       | Purpose                                                                                                  | Flag                                |
| ---------------- | -------------------------- | -------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| `Client`         | `*kubeclient.DedupClient` singleton facade | The only Kubernetes client shell-operator constructs. It exposes controller-runtime, dynamic, typed, discovery, and RESTMapper APIs, and backs kube-events-manager resource informers with dedup stores. | always on |
| `SnapshotStore`  | `*store.DedupStore` wrapper       | Process-wide, reference-counted cache that backs every kube-events-manager monitor's per-object snapshot. | `--dedup-client-snapshot-store`     |

For clusters with thousands of similar resources (e.g. templated
`Deployment`s) the upstream store reports **60–90 %** lower cache memory
usage thanks to value interning and subtree deduplication.

## Quick start

```go
client, err := dedupclient.New(dedupclient.Config{
    Context:    "kind-dev",          // optional kubeconfig context
    Config:     "/path/to/kubeconfig", // optional kubeconfig path
    QPS:        20,
    Burst:      40,
    Namespaces: []string{"kube-system", "default"}, // empty = all
    WatchGVKs: []schema.GroupVersionKind{
        {Group: "", Version: "v1", Kind: "Pod"},
        {Group: "apps", Version: "v1", Kind: "Deployment"},
    },
    ReconstructLRUSize: 4096, // 0 disables reconstruction caching
}, logger)
```

`Start(ctx)` spins up the cache run loop in a single dedicated goroutine and
returns immediately. `Shutdown(ctx)` cancels the loop and waits for it to
exit (or for `ctx` to expire).

## How shell-operator wires it up

`AssembleCommonOperatorFromConfig` always constructs one `*dedupclient.Client`
unless a library caller has already injected one with
`shell_operator.WithKubeClient(...)`. The resulting singleton is stored on
`shell_operator.ShellOperator.KubeClient`, started during `op.Start()`, and
stopped from `op.Shutdown()`. Kubernetes binding resource informers use the
singleton's dedup-backed informer factory; namespace label-selector informers
still use the singleton's typed client-go namespace interface.

Configuration knobs (env vars / CLI flags):

| Env var                              | Flag                                 | Meaning                                                      |
| ------------------------------------ | ------------------------------------ | ------------------------------------------------------------ |
| `DEDUP_CLIENT_ENABLED`               | `--dedup-client-enabled`             | Deprecated no-op; the singleton dedup client is always constructed. |
| `DEDUP_CLIENT_SNAPSHOT_STORE`        | `--dedup-client-snapshot-store`      | Back per-monitor snapshots with the shared dedup store. |
| `DEDUP_CLIENT_NAMESPACES`            | `--dedup-client-namespace`           | Comma-separated (env) or repeated (flag) namespace allow-list. Empty = all. |
| `DEDUP_CLIENT_WATCH_GVKS`            | `--dedup-client-watch-gvk`           | GVKs to pre-register, formatted as `<group>/<version>/<kind>` (group empty for core). |
| `DEDUP_CLIENT_RECONSTRUCT_LRU_SIZE`  | `--dedup-client-reconstruct-lru-size`| LRU size for reconstructed Unstructured objects. 0 disables. |
| `DEDUP_CLIENT_GC_INTERVAL`           | `--dedup-client-gc-interval`         | GC interval for unused interned values/subtrees. 0 = upstream default. |

The singleton client is always constructed. Optional knobs only tune cache
scope, reconstruction, GC, and monitor snapshot storage.

## SnapshotStore — monitor snapshot memory

`SnapshotStore` plugs into shell-operator's `pkg/kube_events_manager` so that
every monitor's `cachedObjects` map stops holding `*Unstructured` pointers
and instead stores `(resourceId → store.ObjectKey)` references into a
process-wide, reference-counted dedup store.

What changes when the flag is on (`MonitorConfig.KeepFullObjectsInMemory == true`):

- Each `resourceInformer` calls `SnapshotStore.Acquire(ownerID, key, obj)` on
  initial-list and on Add/Modified events. The store de-duplicates field
  values and subtrees across every object it holds.
- The per-monitor `*ObjectAndFilterResult` keeps `Object == nil`; the
  authoritative body lives once in the store.
- `monitor.Snapshot()` reconstitutes `Object` lazily by calling
  `SnapshotStore.Get(key)`. Reconstitution is a fresh allocation per call,
  which trades a small CPU cost for the memory drop.
- On informer shutdown, all keys held by that informer are released. The
  underlying object is removed from the store only when the last owner
  releases it, so overlapping watches are correctly handled.

When `KeepFullObjectsInMemory == false`, the existing "no full body kept"
path takes precedence and the store is bypassed for that monitor — there is
no benefit to deduplicating bodies you've already chosen to discard.

### When does it actually save memory?

The win scales with two factors:

1. **Cross-factory duplication.** Each unique
   `(GVR, namespace, fieldSelector, labelSelector)` gets its own client-go
   informer cache today. When several monitors observe overlapping object
   sets through *different* selectors, every cache holds its own copy. Once
   `SnapshotStore` is on, the bodies converge to a single deduplicated copy
   regardless of how many monitors observe them.
2. **Intra-object subtree duplication.** Even within a single GVR, similar
   objects share substantial structure — e.g. a thousand Pods generated
   from one template share `securityContext`, `tolerations`, `resources`,
   and most label/annotation keys. Value interning + subtree dedup encode
   those shared parts once.

If your hooks rarely call `Snapshot()` on each event the CPU cost of
reconstruction is negligible; if they do (and operate on huge snapshots),
benchmark before turning it on.

## Debug endpoint

Once registered (automatically in `bootstrap.go`), the debug server exposes:

```
GET /dedup-client/status.{json|yaml|text}
```

The response carries the status of both components:

```json
{
  "client":        { "enabled": true,  "cacheSyncedHint": false },
  "snapshotStore": { "enabled": true,  "liveObjects": 1284, "totalAcquires": 5012, "totalReleases": 3728, "totalDeletes": 211 }
}
```

Each component reports `enabled: false` with a clear `reason` when its flag
is not set, so liveness probes can distinguish "not configured" from
"errored".
