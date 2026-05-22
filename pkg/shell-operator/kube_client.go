package shell_operator

import (
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"k8s.io/apimachinery/pkg/runtime/schema"

	klient "github.com/flant/kube-client/client"
	pkg "github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/kube/dedupclient"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/metric"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

var (
	defaultMainKubeClientMetricLabels          = map[string]string{pkg.MetricKeyComponent: "main"}
	defaultObjectPatcherKubeClientMetricLabels = map[string]string{pkg.MetricKeyComponent: "object_patcher"}
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

// defaultMainKubeClient creates a Kubernetes client for hooks. No timeout specified, because
// timeout will reset connections for Watchers.
func defaultMainKubeClient(cfg KubeClientConfig, metricStorage metricsstorage.Storage, metricLabels map[string]string, logger *log.Logger) *klient.Client {
	client := klient.New(klient.WithLogger(logger))
	client.WithContextName(cfg.Context)
	client.WithConfigPath(cfg.Config)
	client.WithRateLimiterSettings(cfg.QPS, cfg.Burst)
	client.WithMetricStorage(metric.NewMetricsAdapter(metricStorage, logger.Named("kube-client-metrics-adapter")))
	client.WithMetricLabels(utils.DefaultIfEmpty(metricLabels, defaultMainKubeClientMetricLabels))
	client.WithMetricPrefix(cfg.MetricPrefix)

	return client
}

func initDefaultMainKubeClient(kubeCfg KubeClientConfig, metricStorage metricsstorage.Storage, logger *log.Logger) (*klient.Client, error) {
	//nolint:staticcheck
	kubeClient := defaultMainKubeClient(kubeCfg, metricStorage, defaultMainKubeClientMetricLabels, logger.Named("main-kube-client"))
	err := kubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize 'main' Kubernetes client: %s\n", err)
	}
	return kubeClient, nil
}

// defaultObjectPatcherKubeClient initializes a Kubernetes client for ObjectPatcher. Timeout is specified here.
func defaultObjectPatcherKubeClient(cfg KubeClientConfig, metricStorage metricsstorage.Storage, metricLabels map[string]string, logger *log.Logger) *klient.Client {
	client := klient.New(klient.WithLogger(logger))
	client.WithContextName(cfg.Context)
	client.WithConfigPath(cfg.Config)
	client.WithRateLimiterSettings(cfg.QPS, cfg.Burst)
	client.WithMetricStorage(metric.NewMetricsAdapter(metricStorage, logger.Named("kube-client-metrics-adapter")))
	client.WithMetricLabels(utils.DefaultIfEmpty(metricLabels, defaultObjectPatcherKubeClientMetricLabels))
	client.WithMetricPrefix(cfg.MetricPrefix)
	if cfg.Timeout > 0 {
		client.WithTimeout(cfg.Timeout)
	}
	return client
}

func initDefaultObjectPatcher(kubeCfg KubeClientConfig, metricStorage metricsstorage.Storage, logger *log.Logger) (*objectpatch.ObjectPatcher, error) {
	patcherKubeClient := defaultObjectPatcherKubeClient(kubeCfg, metricStorage, defaultObjectPatcherKubeClientMetricLabels, logger.Named("object-patcher-kube-client"))
	err := patcherKubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client for Object patcher: %s\n", err)
	}
	return objectpatch.NewObjectPatcher(patcherKubeClient, logger), nil
}

// DedupClientConfig consolidates the parameters needed to build the optional
// deduplicated kubeclient on top of an already initialised *klient.Client. It
// mirrors app.Config.DedupClient but stays decoupled from the app package so
// addon-operator and other library consumers can populate it directly.
type DedupClientConfig struct {
	// Enabled toggles construction of the deduplicated client. When false,
	// initDedupClient returns (nil, nil) and the operator runs as before.
	Enabled bool

	// Namespaces restricts the cache to this list of namespaces. Empty
	// means "all namespaces".
	Namespaces []string

	// WatchGVKs is a list of GVK strings to pre-register with the cache.
	// Each entry follows the form "<group>/<version>/<kind>"; the group
	// is empty for core resources (e.g. "/v1/Pod"). Malformed entries
	// cause initDedupClient to return an error so misconfiguration is
	// caught at startup rather than silently ignored.
	WatchGVKs []string

	// ReconstructLRUSize and GCInterval map directly onto the same
	// kubeclient options. Zero means "use the kubeclient default".
	ReconstructLRUSize int
	GCInterval         time.Duration
}

// initDedupClient constructs an optional deduplicated kubeclient using the
// rest.Config and RESTMapper exposed by an already-initialised KubeClient.
// Returning (nil, nil) when cfg.Enabled is false keeps the call site at the
// assembly layer simple — it only has to nil-check the result.
func initDedupClient(kubeClient *klient.Client, cfg DedupClientConfig, logger *log.Logger) (*dedupclient.Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if kubeClient == nil {
		return nil, fmt.Errorf("initialize dedup kubeclient: main kube client is nil")
	}

	restCfg := kubeClient.RestConfig()
	if restCfg == nil {
		return nil, fmt.Errorf("initialize dedup kubeclient: main kube client has no rest.Config (is it initialised?)")
	}

	mapper, mapperErr := kubeClient.ToRESTMapper()
	if mapperErr != nil {
		// Fall through with a nil mapper; kubeclient will use its built-in
		// default. The error is logged so misconfiguration is visible but
		// non-fatal — many operators run fine with the default mapper.
		logger.Warn("could not derive RESTMapper from main kube client; "+
			"dedup kubeclient will fall back to the default in-memory mapper",
			log.Err(mapperErr))
		mapper = nil
	}

	gvks, err := parseGVKs(cfg.WatchGVKs)
	if err != nil {
		return nil, fmt.Errorf("initialize dedup kubeclient: parse watched GVKs: %w", err)
	}

	dedupCfg := dedupclient.Config{
		RESTConfig:         restCfg,
		RESTMapper:         mapper,
		Namespaces:         cfg.Namespaces,
		WatchGVKs:          gvks,
		ReconstructLRUSize: cfg.ReconstructLRUSize,
		GCInterval:         cfg.GCInterval,
	}

	c, err := dedupclient.New(dedupCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize dedup kubeclient: %w", err)
	}
	return c, nil
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
