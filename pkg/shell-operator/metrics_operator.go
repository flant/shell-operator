package shell_operator

import (
	"net/http"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/flant/shell-operator/internal/metrics"
	"github.com/flant/shell-operator/pkg/app"
)

// setupMetricStorage creates and initializes metrics storage for built-in operator metrics
func (op *ShellOperator) setupMetricStorage(kubeEventsManagerLabels []string) {
	metricStorage := metricsstorage.NewMetricStorage(app.PrometheusMetricsPrefix, metricsstorage.WithLogger(op.logger.Named("metric-storage")))

	registerCommonMetrics(metricStorage)
	registerTaskQueueMetrics(metricStorage)
	registerKubeEventsManagerMetrics(metricStorage, kubeEventsManagerLabels)

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.MetricStorage = metricStorage
}

// registerCommonMetrics register base metric
// This function is used in the addon-operator
func registerCommonMetrics(metricStorage metricsstorage.Storage) {
	metricStorage.RegisterCounter("{PREFIX}live_ticks", []string{})
}

// registerTaskQueueMetrics
// This function is used in the addon-operator
func registerTaskQueueMetrics(metricStorage metricsstorage.Storage) {
	metricStorage.RegisterHistogram(
		metrics.TasksQueueActionDurationSeconds,
		[]string{
			"queue_name",
			"queue_action",
		},
		[]float64{
			0.0,
			0.0001, 0.0002, 0.0005, // 100, 200, 500 microseconds
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
		},
	)

	metricStorage.RegisterGauge("{PREFIX}tasks_queue_length", []string{"queue"})
}

// registerKubeEventsManagerMetrics registers metrics for kube_event_manager
// This function is used in the addon-operator
func registerKubeEventsManagerMetrics(metricStorage metricsstorage.Storage, labels []string) {
	// Count of objects in snapshot for one kubernets bindings.
	metricStorage.RegisterGauge("{PREFIX}kube_snapshot_objects", labels)
	// Duration of jqFilter applying.
	metricStorage.RegisterHistogram(
		"{PREFIX}kube_jq_filter_duration_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, 10, // 1,2,5,10 seconds
		},
	)
	// Duration of handling kubernetes event.
	metricStorage.RegisterHistogram(
		"{PREFIX}kube_event_duration_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, 10, // 1,2,5,10 seconds
		},
	)

	// Count of watch errors.
	metricStorage.RegisterCounter("{PREFIX}kubernetes_client_watch_errors_total", []string{"error_type"})
}
