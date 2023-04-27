package shell_operator

import (
	"fmt"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/metric_storage"
)

var (
	DefaultMainKubeClientMetricLabels          = map[string]string{"component": "main"}
	DefaultObjectPatcherKubeClientMetricLabels = map[string]string{"component": "object_patcher"}
)

func DefaultIfEmpty(m map[string]string, def map[string]string) map[string]string {
	if len(m) == 0 {
		return def
	}
	return m
}

// DefaultMainKubeClient creates a Kubernetes client for hooks. No timeout specified, because
// timeout will reset connections for Watchers.
func DefaultMainKubeClient(metricStorage *metric_storage.MetricStorage, metricLabels map[string]string) klient.Client {
	client := klient.New()
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.KubeClientQps, app.KubeClientBurst)
	client.WithMetricStorage(metricStorage)
	client.WithMetricLabels(DefaultIfEmpty(metricLabels, DefaultMainKubeClientMetricLabels))
	return client
}

func InitDefaultMainKubeClient(metricStorage *metric_storage.MetricStorage) (klient.Client, error) {
	//nolint:staticcheck
	klient.RegisterKubernetesClientMetrics(metricStorage, DefaultMainKubeClientMetricLabels)
	kubeClient := DefaultMainKubeClient(metricStorage, DefaultMainKubeClientMetricLabels)
	err := kubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize 'main' Kubernetes client: %s\n", err)
	}
	return kubeClient, nil
}

// DefaultObjectPatcherKubeClient initializes a Kubernetes client for ObjectPatcher. Timeout is specified here.
func DefaultObjectPatcherKubeClient(metricStorage *metric_storage.MetricStorage, metricLabels map[string]string) klient.Client {
	client := klient.New()
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.ObjectPatcherKubeClientQps, app.ObjectPatcherKubeClientBurst)
	client.WithMetricStorage(metricStorage)
	client.WithMetricLabels(DefaultIfEmpty(metricLabels, DefaultObjectPatcherKubeClientMetricLabels))
	client.WithTimeout(app.ObjectPatcherKubeClientTimeout)
	return client
}

func InitDefaultObjectPatcher(metricStorage *metric_storage.MetricStorage) (*object_patch.ObjectPatcher, error) {
	patcherKubeClient := DefaultObjectPatcherKubeClient(metricStorage, DefaultObjectPatcherKubeClientMetricLabels)
	err := patcherKubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client for Object patcher: %s\n", err)
	}
	return object_patch.NewObjectPatcher(patcherKubeClient), nil
}
