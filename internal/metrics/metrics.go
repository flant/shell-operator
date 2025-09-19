package metrics

import (
	"fmt"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

const (
	// Task queue metrics
	TasksQueueActionDurationSeconds  = "{PREFIX}tasks_queue_action_duration_seconds"
	TasksQueueCompactionInQueueTasks = "d8_telemetry_{PREFIX}tasks_queue_compaction_in_queue_tasks"
	TasksQueueCompactionReached      = "d8_telemetry_{PREFIX}tasks_queue_compaction_reached"
	TasksQueueLength                 = "{PREFIX}tasks_queue_length"

	// Hook execution metrics
	HookEnableKubernetesBindingsSeconds     = "{PREFIX}hook_enable_kubernetes_bindings_seconds"
	HookEnableKubernetesBindingsErrorsTotal = "{PREFIX}hook_enable_kubernetes_bindings_errors_total"
	HookEnableKubernetesBindingsSuccess     = "{PREFIX}hook_enable_kubernetes_bindings_success"
	HookRunSeconds                          = "{PREFIX}hook_run_seconds"
	HookRunUserCPUSeconds                   = "{PREFIX}hook_run_user_cpu_seconds"
	HookRunSysCPUSeconds                    = "{PREFIX}hook_run_sys_cpu_seconds"
	HookRunMaxRSSBytes                      = "{PREFIX}hook_run_max_rss_bytes"
	HookRunErrorsTotal                      = "{PREFIX}hook_run_errors_total"
	HookRunAllowedErrorsTotal               = "{PREFIX}hook_run_allowed_errors_total"
	HookRunSuccessTotal                     = "{PREFIX}hook_run_success_total"
	TaskWaitInQueueSecondsTotal             = "{PREFIX}task_wait_in_queue_seconds_total"

	// Common metrics
	LiveTicks = "{PREFIX}live_ticks"

	// Kubernetes events manager metrics
	KubeSnapshotObjects              = "{PREFIX}kube_snapshot_objects"
	KubeJqFilterDurationSeconds      = "{PREFIX}kube_jq_filter_duration_seconds"
	KubeEventDurationSeconds         = "{PREFIX}kube_event_duration_seconds"
	KubernetesClientWatchErrorsTotal = "{PREFIX}kubernetes_client_watch_errors_total"
)

// specific metrics for shell-operator HookManager
func RegisterHookMetrics(metricStorage metricsstorage.Storage) error {
	// Metrics for enable kubernetes bindings.
	_, err := metricStorage.RegisterGauge(
		HookEnableKubernetesBindingsSeconds, []string{"hook"},
		options.WithHelp("Duration of enabling kubernetes bindings in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookEnableKubernetesBindingsSeconds, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookEnableKubernetesBindingsErrorsTotal, []string{"hook"},
		options.WithHelp("Counter of failed attempts to start Kubernetes informers for a hook"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookEnableKubernetesBindingsErrorsTotal, err)
	}

	_, err = metricStorage.RegisterGauge(
		HookEnableKubernetesBindingsSuccess, []string{"hook"},
		options.WithHelp("Gauge indicating successful start of Kubernetes informers (0.0 = not started, 1.0 = started)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookEnableKubernetesBindingsSuccess, err)
	}

	// Metrics for hook executions.
	labels := []string{
		"hook",
		"binding",
		"queue",
	}
	// Duration of hook execution.
	_, err = metricStorage.RegisterHistogram(
		HookRunSeconds,
		labels,
		[]float64{
			0.0,
			0.02, 0.05, // 20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
			100, 200, 500, // 100,200,500 seconds
		},
		options.WithHelp("Histogram of hook execution times in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunSeconds, err)
	}

	// User CPU usage.
	_, err = metricStorage.RegisterHistogram(
		HookRunUserCPUSeconds,
		labels,
		[]float64{
			0.0,
			0.02, 0.05, // 20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
			100, 200, 500, // 100,200,500 seconds
		},
		options.WithHelp("Histogram of hook user CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunUserCPUSeconds, err)
	}
	// System CPU usage.
	_, err = metricStorage.RegisterHistogram(
		HookRunSysCPUSeconds,
		labels,
		[]float64{
			0.0,
			0.02, 0.05, // 20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, // 1,2,5 seconds
			10, 20, 50, // 10,20,50 seconds
			100, 200, 500, // 100,200,500 seconds
		},
		options.WithHelp("Histogram of hook system CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunSysCPUSeconds, err)
	}
	// Max RSS in bytes.
	_, err = metricStorage.RegisterGauge(
		HookRunMaxRSSBytes, labels,
		options.WithHelp("Gauge of maximum resident set size used by hook in bytes"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunMaxRSSBytes, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookRunErrorsTotal, labels,
		options.WithHelp("Counter of hook execution errors (allowFailure: false)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookRunAllowedErrorsTotal, labels,
		options.WithHelp("Counter of hook execution errors that are allowed to fail (allowFailure: true)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunAllowedErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookRunSuccessTotal, labels,
		options.WithHelp("Counter of successful hook executions"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunSuccessTotal, err)
	}

	// hook_run task waiting time
	_, err = metricStorage.RegisterCounter(
		TaskWaitInQueueSecondsTotal, labels,
		options.WithHelp("Counter of seconds that hooks waited in queue before execution"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", TaskWaitInQueueSecondsTotal, err)
	}

	return nil
}

// RegisterCommonMetrics register base metric
// This function is used in the addon-operator
func RegisterCommonMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterCounter(
		LiveTicks, []string{},
		options.WithHelp("Counter that increases every 10 seconds to indicate shell-operator is alive"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", LiveTicks, err)
	}

	return nil
}

// RegisterTaskQueueMetrics
// This function is used in the addon-operator
func RegisterTaskQueueMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterHistogram(
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
		options.WithHelp("Histogram of task queue operation durations in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", TasksQueueActionDurationSeconds, err)
	}

	_, err = metricStorage.RegisterGauge(
		TasksQueueLength, []string{"queue"},
		options.WithHelp("Gauge showing the length of the task queue"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", TasksQueueLength, err)
	}

	return nil
}

// RegisterKubeEventsManagerMetrics registers metrics for kube_event_manager
// This function is used in the addon-operator
func RegisterKubeEventsManagerMetrics(metricStorage metricsstorage.Storage, labels []string) error {
	// Count of objects in snapshot for one kubernetes binding.
	_, err := metricStorage.RegisterGauge(
		KubeSnapshotObjects, labels,
		options.WithHelp("Gauge with count of cached objects (snapshot) for particular binding"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", KubeSnapshotObjects, err)
	}

	// Duration of jqFilter applying.
	_, err = metricStorage.RegisterHistogram(
		KubeJqFilterDurationSeconds,
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, 10, // 1,2,5,10 seconds
		},
		options.WithHelp("Histogram of jq filter execution duration in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", KubeJqFilterDurationSeconds, err)
	}

	// Duration of handling kubernetes event.
	_, err = metricStorage.RegisterHistogram(
		KubeEventDurationSeconds,
		labels,
		[]float64{
			0.0,
			0.001, 0.002, 0.005, // 1,2,5 milliseconds
			0.01, 0.02, 0.05, // 10,20,50 milliseconds
			0.1, 0.2, 0.5, // 100,200,500 milliseconds
			1, 2, 5, 10, // 1,2,5,10 seconds
		},
		options.WithHelp("Histogram of Kubernetes event handling duration in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", KubeEventDurationSeconds, err)
	}

	// Count of watch errors.
	_, err = metricStorage.RegisterCounter(
		KubernetesClientWatchErrorsTotal, []string{"error_type"},
		options.WithHelp("Counter of Kubernetes client watch errors by error type"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", KubernetesClientWatchErrorsTotal, err)
	}

	return nil
}
