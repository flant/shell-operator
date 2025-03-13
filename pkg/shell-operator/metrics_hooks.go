package shell_operator

import (
	"net/http"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metric"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
)

func (op *ShellOperator) setupHookMetricStorage() {
	metricStorage := metricstorage.NewMetricStorage(op.ctx, app.PrometheusMetricsPrefix, true, op.logger.Named("metric-storage"))

	op.APIServer.RegisterRoute(http.MethodGet, "/metrics/hooks", metricStorage.Handler().ServeHTTP)
	// create new metric storage for hooks
	// register scrape handler
	op.HookMetricStorage = metricStorage
}

// specific metrics for shell-operator HookManager
func registerHookMetrics(metricStorage metric.Storage) {
	// Metrics for enable kubernetes bindings.
	metricStorage.RegisterGauge("{PREFIX}hook_enable_kubernetes_bindings_seconds", map[string]string{"hook": ""})
	metricStorage.RegisterCounter("{PREFIX}hook_enable_kubernetes_bindings_errors_total", map[string]string{"hook": ""})
	metricStorage.RegisterGauge("{PREFIX}hook_enable_kubernetes_bindings_success", map[string]string{"hook": ""})

	// Metrics for hook executions.
	labels := map[string]string{
		"hook":    "",
		"binding": "",
		"queue":   "",
	}
	// Duration of hook execution.
	metricStorage.RegisterHistogram(
		"{PREFIX}hook_run_seconds",
		labels,
		[]float64{
			0.0,
			0.02, 0.05, // 20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
			100, 200, 500, // 100,200,500 seconds
		},
	)

	// System CPU usage.
	metricStorage.RegisterHistogram(
		"{PREFIX}hook_run_user_cpu_seconds",
		labels,
		[]float64{
			0.0,
			0.02, 0.05, // 20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
			100, 200, 500, // 100,200,500 seconds
		},
	)
	// User CPU usage.
	metricStorage.RegisterHistogram(
		"{PREFIX}hook_run_sys_cpu_seconds",
		labels,
		[]float64{
			0.0,
			0.02, 0.05, // 20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
			100, 200, 500, // 100,200,500 seconds
		},
	)
	// Max RSS in bytes.
	metricStorage.RegisterGauge("{PREFIX}hook_run_max_rss_bytes", labels)

	metricStorage.RegisterCounter("{PREFIX}hook_run_errors_total", labels)
	metricStorage.RegisterCounter("{PREFIX}hook_run_allowed_errors_total", labels)
	metricStorage.RegisterCounter("{PREFIX}hook_run_success_total", labels)
	// hook_run task waiting time
	metricStorage.RegisterCounter("{PREFIX}task_wait_in_queue_seconds_total", labels)
}
