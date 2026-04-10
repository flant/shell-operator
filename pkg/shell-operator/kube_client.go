package shell_operator

import (
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	klient "github.com/flant/kube-client/client"
	pkg "github.com/flant/shell-operator/pkg"
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
	Context string
	Config  string
	QPS     float32
	Burst   int
	Timeout time.Duration // zero means no timeout
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
	return client
}

func initDefaultMainKubeClient(kubeCfg KubeClientConfig, metricStorage metricsstorage.Storage, logger *log.Logger) (*klient.Client, error) {
	cfg := kubeCfg
	//nolint:staticcheck
	klient.RegisterKubernetesClientMetrics(metric.NewMetricsAdapter(metricStorage, logger.Named("kube-client-metrics-adapter")), defaultMainKubeClientMetricLabels)
	kubeClient := defaultMainKubeClient(cfg, metricStorage, defaultMainKubeClientMetricLabels, logger.Named("main-kube-client"))
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
	if cfg.Timeout > 0 {
		client.WithTimeout(cfg.Timeout)
	}
	return client
}

func initDefaultObjectPatcher(kubeCfg KubeClientConfig, metricStorage metricsstorage.Storage, logger *log.Logger) (*objectpatch.ObjectPatcher, error) {
	cfg := kubeCfg
	patcherKubeClient := defaultObjectPatcherKubeClient(cfg, metricStorage, defaultObjectPatcherKubeClientMetricLabels, logger.Named("object-patcher-kube-client"))
	err := patcherKubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client for Object patcher: %s\n", err)
	}
	return objectpatch.NewObjectPatcher(patcherKubeClient, logger), nil
}
