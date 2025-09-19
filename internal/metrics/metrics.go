package metrics

import (
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	TasksQueueActionDurationSeconds  = "{PREFIX}tasks_queue_action_duration_seconds"
	TasksQueueCompactionInQueueTasks = "d8_telemetry_{PREFIX}tasks_queue_compaction_in_queue_tasks"
	TasksQueueCompactionReached      = "d8_telemetry_{PREFIX}tasks_queue_compaction_reached"
)

// specific metrics for shell-operator HookManager
func RegisterHookMetrics(metricStorage metricsstorage.Storage) {
	// Metrics for enable kubernetes bindings.
	_, _ = metricStorage.RegisterGauge("{PREFIX}hook_enable_kubernetes_bindings_seconds", []string{"hook"})
	_, _ = metricStorage.RegisterCounter("{PREFIX}hook_enable_kubernetes_bindings_errors_total", []string{"hook"})
	_, _ = metricStorage.RegisterGauge("{PREFIX}hook_enable_kubernetes_bindings_success", []string{"hook"})

	// Metrics for hook executions.
	labels := []string{
		"hook",
		"binding",
		"queue",
	}
	// Duration of hook execution.
	_, _ = metricStorage.RegisterHistogram(
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
	_, _ = metricStorage.RegisterHistogram(
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
	_, _ = metricStorage.RegisterHistogram(
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
	_, _ = metricStorage.RegisterGauge("{PREFIX}hook_run_max_rss_bytes", labels)

	_, _ = metricStorage.RegisterCounter("{PREFIX}hook_run_errors_total", labels)
	_, _ = metricStorage.RegisterCounter("{PREFIX}hook_run_allowed_errors_total", labels)
	_, _ = metricStorage.RegisterCounter("{PREFIX}hook_run_success_total", labels)
	// hook_run task waiting time
	_, _ = metricStorage.RegisterCounter("{PREFIX}task_wait_in_queue_seconds_total", labels)
}

// RegisterCommonMetrics register base metric
// This function is used in the addon-operator
func RegisterCommonMetrics(metricStorage metricsstorage.Storage) {
	_, _ = metricStorage.RegisterCounter("{PREFIX}live_ticks", []string{})
}

// RegisterTaskQueueMetrics
// This function is used in the addon-operator
func RegisterTaskQueueMetrics(metricStorage metricsstorage.Storage) {
	_, _ = metricStorage.RegisterHistogram(
		TasksQueueActionDurationSeconds,
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

	_, _ = metricStorage.RegisterGauge("{PREFIX}tasks_queue_length", []string{"queue"})
}

// RegisterKubeEventsManagerMetrics registers metrics for kube_event_manager
// This function is used in the addon-operator
func RegisterKubeEventsManagerMetrics(metricStorage metricsstorage.Storage, labels []string) {
	// Count of objects in snapshot for one kubernets bindings.
	_, _ = metricStorage.RegisterGauge("{PREFIX}kube_snapshot_objects", labels)
	// Duration of jqFilter applying.
	_, _ = metricStorage.RegisterHistogram(
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
	_, _ = metricStorage.RegisterHistogram(
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
	_, _ = metricStorage.RegisterCounter("{PREFIX}kubernetes_client_watch_errors_total", []string{"error_type"})
}
