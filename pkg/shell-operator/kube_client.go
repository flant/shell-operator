package shell_operator

import (
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/app"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/metric"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

var (
	defaultMainKubeClientMetricLabels          = map[string]string{"component": "main"}
	defaultObjectPatcherKubeClientMetricLabels = map[string]string{"component": "object_patcher"}
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

func initDefaultMainKubeClient(metricStorage metricsstorage.Storage, logger *log.Logger) (*klient.Client, error) {
	cfg := KubeClientConfig{
		Context: app.KubeContext,
		Config:  app.KubeConfig,
		QPS:     app.KubeClientQps,
		Burst:   app.KubeClientBurst,
	}
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

func initDefaultObjectPatcher(metricStorage metricsstorage.Storage, logger *log.Logger) (*objectpatch.ObjectPatcher, error) {
	cfg := KubeClientConfig{
		Context: app.KubeContext,
		Config:  app.KubeConfig,
		QPS:     app.ObjectPatcherKubeClientQps,
		Burst:   app.ObjectPatcherKubeClientBurst,
		Timeout: app.ObjectPatcherKubeClientTimeout,
	}
	patcherKubeClient := defaultObjectPatcherKubeClient(cfg, metricStorage, defaultObjectPatcherKubeClientMetricLabels, logger.Named("object-patcher-kube-client"))
	err := patcherKubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client for Object patcher: %s\n", err)
	}
	return objectpatch.NewObjectPatcher(patcherKubeClient, logger), nil
}
