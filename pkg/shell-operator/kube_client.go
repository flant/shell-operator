// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shell_operator

import (
	"fmt"

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

// defaultMainKubeClient creates a Kubernetes client for hooks. No timeout specified, because
// timeout will reset connections for Watchers.
func defaultMainKubeClient(metricStorage metricsstorage.Storage, metricLabels map[string]string, logger *log.Logger) *klient.Client {
	client := klient.New(klient.WithLogger(logger))
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.KubeClientQps, app.KubeClientBurst)
	client.WithMetricStorage(metric.NewMetricsAdapter(metricStorage, logger.Named("kube-client-metrics-adapter")))
	client.WithMetricLabels(utils.DefaultIfEmpty(metricLabels, defaultMainKubeClientMetricLabels))
	return client
}

func initDefaultMainKubeClient(metricStorage metricsstorage.Storage, logger *log.Logger) (*klient.Client, error) {
	//nolint:staticcheck
	klient.RegisterKubernetesClientMetrics(metric.NewMetricsAdapter(metricStorage, logger.Named("kube-client-metrics-adapter")), defaultMainKubeClientMetricLabels)
	kubeClient := defaultMainKubeClient(metricStorage, defaultMainKubeClientMetricLabels, logger.Named("main-kube-client"))
	err := kubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize 'main' Kubernetes client: %s\n", err)
	}
	return kubeClient, nil
}

// defaultObjectPatcherKubeClient initializes a Kubernetes client for ObjectPatcher. Timeout is specified here.
func defaultObjectPatcherKubeClient(metricStorage metricsstorage.Storage, metricLabels map[string]string, logger *log.Logger) *klient.Client {
	client := klient.New(klient.WithLogger(logger))
	client.WithContextName(app.KubeContext)
	client.WithConfigPath(app.KubeConfig)
	client.WithRateLimiterSettings(app.ObjectPatcherKubeClientQps, app.ObjectPatcherKubeClientBurst)
	client.WithMetricStorage(metric.NewMetricsAdapter(metricStorage, logger.Named("kube-client-metrics-adapter")))
	client.WithMetricLabels(utils.DefaultIfEmpty(metricLabels, defaultObjectPatcherKubeClientMetricLabels))
	client.WithTimeout(app.ObjectPatcherKubeClientTimeout)
	return client
}

func initDefaultObjectPatcher(metricStorage metricsstorage.Storage, logger *log.Logger) (*objectpatch.ObjectPatcher, error) {
	patcherKubeClient := defaultObjectPatcherKubeClient(metricStorage, defaultObjectPatcherKubeClientMetricLabels, logger.Named("object-patcher-kube-client"))
	err := patcherKubeClient.Init()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client for Object patcher: %s\n", err)
	}
	return objectpatch.NewObjectPatcher(patcherKubeClient, logger), nil
}
