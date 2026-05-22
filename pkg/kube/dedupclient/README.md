# pkg/kube/dedupclient

Thin shell-operator wrapper around
[`github.com/ldmonster/kubeclient`](https://github.com/ldmonster/kubeclient) — a
controller-runtime compatible Kubernetes client whose cache is backed by a
deduplicated, value-interned object store. For clusters with thousands of
similar resources (e.g. templated `Deployment`s) the upstream library reports
**60–90 %** lower cache memory usage compared to a standard informer cache.

This package keeps the upstream dependency contained inside one shell-operator
package: every other consumer interacts with the wrapper's small surface
(`Client`, `Config`, `Start`, `Shutdown`, `EnsureInformer`, `WaitForCacheSync`)
or with the embedded `sigs.k8s.io/controller-runtime/pkg/client.Client`.

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
| `DEDUP_CLIENT_NAMESPACES`            | `--dedup-client-namespace`           | Comma-separated (env) or repeated (flag) namespace allow-list. Empty = all. |
| `DEDUP_CLIENT_WATCH_GVKS`            | `--dedup-client-watch-gvk`           | GVKs to pre-register, formatted as `<group>/<version>/<kind>` (group empty for core). |
| `DEDUP_CLIENT_RECONSTRUCT_LRU_SIZE`  | `--dedup-client-reconstruct-lru-size`| LRU size for reconstructed Unstructured objects. 0 disables. |
| `DEDUP_CLIENT_GC_INTERVAL`           | `--dedup-client-gc-interval`         | GC interval for unused interned values/subtrees. 0 = upstream default. |

When the feature is off the wrapper is **not** constructed at all, so this
integration adds zero runtime overhead to existing deployments.

## Debug endpoint

Once registered (automatically in `bootstrap.go`), the debug server exposes:

```
GET /dedup-client/status.{json|yaml|text}
```

The response indicates whether the client is configured and a best-effort hint
about cache sync state.
