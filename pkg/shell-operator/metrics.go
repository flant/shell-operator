package shell_operator

import (
	"fmt"
	"net/http"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/internal/metrics"
	"github.com/flant/shell-operator/pkg/app"
)

// setupMetricStorage creates and initializes metrics storage for built-in operator metrics
func (op *ShellOperator) setupMetricStorage(kubeEventsManagerLabels []string) error {
	metricStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
		metricsstorage.WithLogger(op.logger.Named("metric-storage")),
	)

	if err := metrics.RegisterCommonMetrics(metricStorage); err != nil {
		return fmt.Errorf("register common metrics: %w", err)
	}

	if err := metrics.RegisterTaskQueueMetrics(metricStorage); err != nil {
		return fmt.Errorf("register task queue metrics: %w", err)
	}

	if err := metrics.RegisterKubeEventsManagerMetrics(metricStorage, kubeEventsManagerLabels); err != nil {
		return fmt.Errorf("register kube events manager metrics: %w", err)
	}

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.MetricStorage = metricStorage
	return nil
}

// setupHookMetricStorage creates and initializes metrics storage for built-in hooks metrics
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
