package shell_operator

import "github.com/flant/shell-operator/pkg/metric_storage"

func RegisterShellOperatorMetrics(metricStorage *metric_storage.MetricStorage) {
	RegisterCommonMetrics(metricStorage)
	RegisterTaskQueueMetrics(metricStorage)
	RegisterKubeEventsManagerMetrics(metricStorage, map[string]string{
		"hook":    "",
		"binding": "",
		"queue":   "",
	})
	RegisterHookMetrics(metricStorage)
}

func RegisterCommonMetrics(metricStorage *metric_storage.MetricStorage) {
	metricStorage.RegisterCounter("{PREFIX}live_ticks", map[string]string{})
}

func RegisterTaskQueueMetrics(metricStorage *metric_storage.MetricStorage) {
	metricStorage.RegisterHistogramWithBuckets(
		"{PREFIX}tasks_queue_action_duration_seconds",
		map[string]string{
			"queue_name":   "",
			"queue_action": "",
		},
		[]float64{
			0.0,
			0.000001, 0.000002, 0.000005, // 1,2,5 microseconds
			0.00001, 0.00002, 0.00005, // 10,20,50 microsends
			0.0001, 0.0002, 0.0005, // 100, 200, 500 microseconds
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
		},
	)

	metricStorage.RegisterGauge("{PREFIX}tasks_queue_length", map[string]string{"queue": ""})
}

// metrics for kube_event_manager
func RegisterKubeEventsManagerMetrics(metricStorage *metric_storage.MetricStorage, labels map[string]string) {
	// Count of objects in snapshot for one kubernets bindings.
	metricStorage.RegisterGauge("{PREFIX}kube_snapshot_objects", labels)
	// Size of snapshot in JSON format.
	metricStorage.RegisterGauge("{PREFIX}kube_snapshot_bytes", labels)
	// Duration of jqFilter applying.
	metricStorage.RegisterHistogramWithBuckets(
		"{PREFIX}kube_jq_filter_duration_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
		},
	)
	// Duration of handling kubernetes event.
	metricStorage.RegisterHistogramWithBuckets(
		"{PREFIX}kube_event_duration_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
		},
	)
}

// Shell-operator specific metrics
func RegisterHookMetrics(metricStorage *metric_storage.MetricStorage) {
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
	metricStorage.RegisterHistogramWithBuckets(
		"{PREFIX}hook_run_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
		},
	)

	// System CPU usage.
	metricStorage.RegisterHistogramWithBuckets(
		"{PREFIX}hook_run_user_cpu_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
		},
	)
	// User CPU usage.
	metricStorage.RegisterHistogramWithBuckets(
		"{PREFIX}hook_run_sys_cpu_seconds",
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, // 10 seconds
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
