package shell_operator

import (
	"net/http"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/internal/metrics"
	"github.com/flant/shell-operator/pkg/app"
)

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

// setupMetricStorage creates and initializes metrics storage for built-in operator metrics
func (op *ShellOperator) setupMetricStorage(kubeEventsManagerLabels []string) {
	metricStorage := metricsstorage.NewMetricStorage(
		metricsstorage.WithPrefix(app.PrometheusMetricsPrefix),
		metricsstorage.WithLogger(op.logger.Named("metric-storage")),
	)

	metrics.RegisterCommonMetrics(metricStorage)
	metrics.RegisterTaskQueueMetrics(metricStorage)
	metrics.RegisterKubeEventsManagerMetrics(metricStorage, kubeEventsManagerLabels)

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.MetricStorage = metricStorage
}
