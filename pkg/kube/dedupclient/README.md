# pkg/kube/dedupclient

Two-part integration of
[`github.com/ldmonster/kubeclient`](https://github.com/ldmonster/kubeclient)
into shell-operator. Both parts can be enabled independently — the one that
moves the RSS needle for typical workloads is the **SnapshotStore**.

| Component        | Type                       | Purpose                                                                                                  | Flag                                |
| ---------------- | -------------------------- | -------------------------------------------------------------------------------------------------------- | ----------------------------------- |
| `Client`         | `*kubeclient.DedupClient` wrapper | Controller-runtime compatible Kubernetes client for hooks/extensions, with its own deduplicated cache. | `--dedup-client-enabled`            |
| `SnapshotStore`  | `*store.DedupStore` wrapper       | Process-wide, reference-counted cache that backs every kube-events-manager monitor's per-object snapshot. **This is what reduces memory.** | `--dedup-client-snapshot-store`     |

For clusters with thousands of similar resources (e.g. templated
`Deployment`s) the upstream store reports **60–90 %** lower cache memory
usage thanks to value interning and subtree deduplication.

## Quick start

```go
import (
    klient "github.com/flant/kube-client/client"
    "github.com/flant/shell-operator/pkg/kube/dedupclient"
)

func newDedup(main *klient.Client, logger *log.Logger) (*dedupclient.Client, error) {
    mapper, _ := main.ToRESTMapper()
    return dedupclient.New(dedupclient.Config{
        RESTConfig: main.RestConfig(),
        RESTMapper: mapper,
        Namespaces: []string{"kube-system", "default"}, // empty = all
        WatchGVKs: []schema.GroupVersionKind{
            {Group: "", Version: "v1", Kind: "Pod"},
            {Group: "apps", Version: "v1", Kind: "Deployment"},
        },
        ReconstructLRUSize: 4096, // 0 disables reconstruction caching
    }, logger)
}
```

`Start(ctx)` spins up the cache run loop in a single dedicated goroutine and
returns immediately. `Shutdown(ctx)` cancels the loop and waits for it to
exit (or for `ctx` to expire).

## How shell-operator wires it up

When `app.Config.DedupClient.Enabled` is `true`,
`AssembleCommonOperatorFromConfig` calls `initDedupClient`, which hands the
main `klient.Client`'s `rest.Config` and `RESTMapper` to
`dedupclient.New`. The resulting `*Client` is stored on
`shell_operator.ShellOperator.DedupClient`, started during `op.Start()`, and
stopped from `op.Shutdown()`.

Configuration knobs (env vars / CLI flags):

| Env var                              | Flag                                 | Meaning                                                      |
| ------------------------------------ | ------------------------------------ | ------------------------------------------------------------ |
| `DEDUP_CLIENT_ENABLED`               | `--dedup-client-enabled`             | Construct the deduplicated client at all.                    |
| `DEDUP_CLIENT_SNAPSHOT_STORE`        | `--dedup-client-snapshot-store`      | Back per-monitor snapshots with the shared dedup store. Independent of `--dedup-client-enabled`. |
| `DEDUP_CLIENT_NAMESPACES`            | `--dedup-client-namespace`           | Comma-separated (env) or repeated (flag) namespace allow-list. Empty = all. |
| `DEDUP_CLIENT_WATCH_GVKS`            | `--dedup-client-watch-gvk`           | GVKs to pre-register, formatted as `<group>/<version>/<kind>` (group empty for core). |
| `DEDUP_CLIENT_RECONSTRUCT_LRU_SIZE`  | `--dedup-client-reconstruct-lru-size`| LRU size for reconstructed Unstructured objects. 0 disables. |
| `DEDUP_CLIENT_GC_INTERVAL`           | `--dedup-client-gc-interval`         | GC interval for unused interned values/subtrees. 0 = upstream default. |

When both features are off the wrappers are **not** constructed at all, so
this integration adds zero runtime overhead to existing deployments.

## SnapshotStore — the memory win

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
