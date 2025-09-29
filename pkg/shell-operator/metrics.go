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
	"net/http"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metrics"
)

// setupMetricStorage creates and initializes metrics storage for built-in operator metrics.
// If MetricStorage is already set via options, it uses that; otherwise creates a new one.
func (op *ShellOperator) setupMetricStorage(kubeEventsManagerLabels []string) {
	metricStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
		metricsstorage.WithLogger(op.logger.Named("metric-storage")),
	)

	metrics.RegisterOperatorMetrics(metricStorage, kubeEventsManagerLabels)

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.MetricStorage = metricStorage
}

// setupHookMetricStorage creates and initializes metrics storage for hook metrics.
// If HookMetricStorage is already set via options, it uses that; otherwise creates a new one.
func (op *ShellOperator) setupHookMetricStorage() {
	metricStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
		metricsstorage.WithNewRegistry(),
		metricsstorage.WithLogger(op.logger.Named("metric-storage")),
	)
	op.APIServer.RegisterRoute(http.MethodGet, "/metrics/hooks", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.HookMetricStorage = metricStorage
}
