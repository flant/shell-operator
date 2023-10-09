package shell_operator

import (
	"fmt"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/metric_storage"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

var (
	defaultMainKubeClientMetricLabels          = map[string]string{"component": "main"}
	defaultObjectPatcherKubeClientMetricLabels = map[string]string{"component": "object_patcher"}
)

// defaultMainKubeClient creates a Kubernetes client for hooks. No timeout specified, because
// timeout will reset connections for Watchers.
func defaultMainKubeClient(metricStorage *metric_storage.MetricStorage, metricLabels map[string]string) klient.Client {
	client := klient.New()
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.KubeClientQps, app.KubeClientBurst)
	client.WithMetricStorage(metricStorage)
	client.WithMetricLabels(utils.DefaultIfEmpty(metricLabels, defaultMainKubeClientMetricLabels))
	return client
}

func initDefaultMainKubeClient(metricStorage *metric_storage.MetricStorage) (klient.Client, error) {
	//nolint:staticcheck
	klient.RegisterKubernetesClientMetrics(metricStorage, defaultMainKubeClientMetricLabels)
	kubeClient := defaultMainKubeClient(metricStorage, defaultMainKubeClientMetricLabels)
	err := kubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize 'main' Kubernetes client: %s\n", err)
	}
	return kubeClient, nil
}

// defaultObjectPatcherKubeClient initializes a Kubernetes client for ObjectPatcher. Timeout is specified here.
func defaultObjectPatcherKubeClient(metricStorage *metric_storage.MetricStorage, metricLabels map[string]string) klient.Client {
	client := klient.New()
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.ObjectPatcherKubeClientQps, app.ObjectPatcherKubeClientBurst)
	client.WithMetricStorage(metricStorage)
	client.WithMetricLabels(utils.DefaultIfEmpty(metricLabels, defaultObjectPatcherKubeClientMetricLabels))
	client.WithTimeout(app.ObjectPatcherKubeClientTimeout)
	return client
}

func initDefaultObjectPatcher(metricStorage *metric_storage.MetricStorage) (*object_patch.ObjectPatcher, error) {
	patcherKubeClient := defaultObjectPatcherKubeClient(metricStorage, defaultObjectPatcherKubeClientMetricLabels)
	err := patcherKubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client for Object patcher: %s\n", err)
	}
	return object_patch.NewObjectPatcher(patcherKubeClient), nil
}
