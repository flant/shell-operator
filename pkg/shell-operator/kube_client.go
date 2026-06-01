package shell_operator

import (
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/shell-operator/pkg/kube/dedupclient"
)

// KubeClientConfig holds explicit connection settings for a Kubernetes client,
// decoupling business logic from the global app.* configuration variables.
type KubeClientConfig struct {
	Context      string
	Config       string
	QPS          float32
	Burst        int
	Timeout      time.Duration // zero means no timeout
	MetricPrefix string
}

// DedupClientConfig consolidates the parameters for the singleton
// deduplicated Kubernetes client. Enabled is retained only for deprecated
// configuration compatibility; the singleton is always constructed.
type DedupClientConfig struct {
	Enabled bool

	// Namespaces restricts the cache to this list of namespaces. Empty
	// means "all namespaces".
	Namespaces []string

	// WatchGVKs is a list of GVK strings to pre-register with the cache.
	// Each entry follows the form "<group>/<version>/<kind>"; the group
	// is empty for core resources (e.g. "/v1/Pod"). Malformed entries
	// cause initSingletonKubeClient to return an error so misconfiguration is
	// caught at startup rather than silently ignored.
	WatchGVKs []string

	// ReconstructLRUSize and GCInterval map directly onto the same
	// kubeclient options. Zero means "use the kubeclient default".
	ReconstructLRUSize int
	GCInterval         time.Duration

	// SnapshotStore toggles construction of the process-wide deduplicated
	// SnapshotStore. The store backs per-monitor object caches; turning it
	// on deduplicates monitor snapshots, while Enabled moves informer
	// list/watch storage to the dedup-backed path. Independent of Enabled.
	SnapshotStore bool
}

func initSingletonKubeClient(kubeCfg KubeClientConfig, cfg DedupClientConfig, logger *log.Logger) (*dedupclient.Client, error) {
	gvks, err := parseGVKs(cfg.WatchGVKs)
	if err != nil {
		return nil, fmt.Errorf("initialize singleton kubeclient: parse watched GVKs: %w", err)
	}

	dedupCfg := dedupclient.Config{
		Context:            kubeCfg.Context,
		Config:             kubeCfg.Config,
		QPS:                kubeCfg.QPS,
		Burst:              kubeCfg.Burst,
		Timeout:            kubeCfg.Timeout,
		Namespaces:         cfg.Namespaces,
		WatchGVKs:          gvks,
		ReconstructLRUSize: cfg.ReconstructLRUSize,
		GCInterval:         cfg.GCInterval,
	}

	c, err := dedupclient.New(dedupCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize singleton kubeclient: %w", err)
	}
	return c, nil
}

// initSnapshotStore constructs the optional deduplicated SnapshotStore.
// Returning (nil, nil) when cfg.SnapshotStore is false keeps the assembly
// caller simple — it nil-checks the result and skips the wiring. The
// returned store is a fresh instance (no shared state with previous
// processes) and is safe for concurrent use; the operator passes it to
// KubeEventsManager.WithSnapshotStore so every later-created Monitor
// uses it transparently.
func initSnapshotStore(cfg DedupClientConfig, logger *log.Logger) *dedupclient.SnapshotStore {
	if !cfg.SnapshotStore {
		return nil
	}
	return dedupclient.NewSnapshotStore(logger)
}

// parseGVKs converts a list of "group/version/kind" strings into
// schema.GroupVersionKind values. It accepts an empty group (leading "/")
// for core resources, e.g. "/v1/Pod". An empty input list yields a nil
// slice so kubeclient does not pre-register any GVKs at startup.
func parseGVKs(specs []string) ([]schema.GroupVersionKind, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	out := make([]schema.GroupVersionKind, 0, len(specs))
	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		parts := strings.SplitN(spec, "/", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("expected \"<group>/<version>/<kind>\", got %q", raw)
		}
		version := strings.TrimSpace(parts[1])
		kind := strings.TrimSpace(parts[2])
		if version == "" || kind == "" {
			return nil, fmt.Errorf("version and kind must be non-empty in %q", raw)
		}
		out = append(out, schema.GroupVersionKind{
			Group:   strings.TrimSpace(parts[0]),
			Version: version,
			Kind:    kind,
		})
	}
	return out, nil
}
