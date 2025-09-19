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

// Package metrics provides centralized metric names and registration functions for shell-operator.
// All metric names use constants to ensure consistency and prevent typos.
// The {PREFIX} placeholder is replaced by the metrics storage with the appropriate prefix.
package metrics

import (
	"fmt"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

// Metric name constants organized by functional area.
// Each constant represents a unique metric name used throughout shell-operator.
const (
	// ============================================================================
	// Common Metrics
	// ============================================================================
	// LiveTicks is a counter that increases every 10 seconds to indicate shell-operator is alive
	LiveTicks = "{PREFIX}live_ticks"

	// ============================================================================
	// Task Queue Metrics
	// ============================================================================
	// TasksQueueActionDurationSeconds measures task queue operation durations
	TasksQueueActionDurationSeconds = "{PREFIX}tasks_queue_action_duration_seconds"
	// TasksQueueLength shows the current length of the task queue
	TasksQueueLength = "{PREFIX}tasks_queue_length"
	// TasksQueueCompactionInQueueTasks tracks telemetry for queue compaction
	TasksQueueCompactionInQueueTasks = "d8_telemetry_{PREFIX}tasks_queue_compaction_in_queue_tasks"
	// TasksQueueCompactionReached tracks when queue compaction is reached
	TasksQueueCompactionReached = "d8_telemetry_{PREFIX}tasks_queue_compaction_reached"

	// ============================================================================
	// Hook Execution Metrics
	// ============================================================================
	// Kubernetes Bindings
	HookEnableKubernetesBindingsSeconds     = "{PREFIX}hook_enable_kubernetes_bindings_seconds"
	HookEnableKubernetesBindingsErrorsTotal = "{PREFIX}hook_enable_kubernetes_bindings_errors_total"
	HookEnableKubernetesBindingsSuccess     = "{PREFIX}hook_enable_kubernetes_bindings_success"

	// Hook Runtime Metrics
	HookRunSeconds        = "{PREFIX}hook_run_seconds"
	HookRunUserCPUSeconds = "{PREFIX}hook_run_user_cpu_seconds"
	HookRunSysCPUSeconds  = "{PREFIX}hook_run_sys_cpu_seconds"
	HookRunMaxRSSBytes    = "{PREFIX}hook_run_max_rss_bytes"

	// Hook Execution Results
	HookRunErrorsTotal        = "{PREFIX}hook_run_errors_total"
	HookRunAllowedErrorsTotal = "{PREFIX}hook_run_allowed_errors_total"
	HookRunSuccessTotal       = "{PREFIX}hook_run_success_total"

	// Task Queue Wait Time
	TaskWaitInQueueSecondsTotal = "{PREFIX}task_wait_in_queue_seconds_total"

	// ============================================================================
	// Kubernetes Events Manager Metrics
	// ============================================================================
	// KubeSnapshotObjects counts cached objects for particular bindings
	KubeSnapshotObjects = "{PREFIX}kube_snapshot_objects"
	// KubeJqFilterDurationSeconds measures jq filter execution time
	KubeJqFilterDurationSeconds = "{PREFIX}kube_jq_filter_duration_seconds"
	// KubeEventDurationSeconds measures Kubernetes event handling time
	KubeEventDurationSeconds = "{PREFIX}kube_event_duration_seconds"
	// KubernetesClientWatchErrorsTotal counts Kubernetes client watch errors
	KubernetesClientWatchErrorsTotal = "{PREFIX}kubernetes_client_watch_errors_total"
)

// ============================================================================
// Registration Functions
// ============================================================================

// RegisterHookMetrics registers all hook-related metrics with the provided storage.
// This includes metrics for hook execution, Kubernetes bindings, and resource usage.
// Used specifically by shell-operator's HookManager.
func RegisterHookMetrics(metricStorage metricsstorage.Storage) error {
	// Register Kubernetes bindings metrics
	if err := registerKubernetesBindingsMetrics(metricStorage); err != nil {
		return fmt.Errorf("register kubernetes bindings metrics: %w", err)
	}

	// Register hook execution metrics
	if err := registerHookExecutionMetrics(metricStorage); err != nil {
		return fmt.Errorf("register hook execution metrics: %w", err)
	}

	return nil
}

// registerKubernetesBindingsMetrics registers metrics related to Kubernetes bindings setup
func registerKubernetesBindingsMetrics(metricStorage metricsstorage.Storage) error {
	hookLabels := []string{"hook"}

	_, err := metricStorage.RegisterGauge(
		HookEnableKubernetesBindingsSeconds, hookLabels,
		options.WithHelp("Duration of enabling kubernetes bindings in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookEnableKubernetesBindingsSeconds, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookEnableKubernetesBindingsErrorsTotal, hookLabels,
		options.WithHelp("Counter of failed attempts to start Kubernetes informers for a hook"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookEnableKubernetesBindingsErrorsTotal, err)
	}

	_, err = metricStorage.RegisterGauge(
		HookEnableKubernetesBindingsSuccess, hookLabels,
		options.WithHelp("Gauge indicating successful start of Kubernetes informers (0.0 = not started, 1.0 = started)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookEnableKubernetesBindingsSuccess, err)
	}

	return nil
}

// registerHookExecutionMetrics registers metrics related to hook execution and resource usage
func registerHookExecutionMetrics(metricStorage metricsstorage.Storage) error {
	// Common labels for hook execution metrics
	executionLabels := []string{"hook", "binding", "queue"}

	// Standard histogram buckets for timing metrics (microseconds to minutes)
	timingBuckets := []float64{
		0.0,
		0.02, 0.05, // 20,50 milliseconds
		0.1, 0.2, 0.5, // 100,200,500 milliseconds
		1, 2, 5, // 1,2,5 seconds
		10, 20, 50, // 10,20,50 seconds
		100, 200, 500, // 100,200,500 seconds
	}

	// Register execution duration metric
	_, err := metricStorage.RegisterHistogram(
		HookRunSeconds, executionLabels, timingBuckets,
		options.WithHelp("Histogram of hook execution times in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunSeconds, err)
	}

	// Register CPU usage metrics
	_, err = metricStorage.RegisterHistogram(
		HookRunUserCPUSeconds, executionLabels, timingBuckets,
		options.WithHelp("Histogram of hook user CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunUserCPUSeconds, err)
	}

	_, err = metricStorage.RegisterHistogram(
		HookRunSysCPUSeconds, executionLabels, timingBuckets,
		options.WithHelp("Histogram of hook system CPU usage in seconds"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunSysCPUSeconds, err)
	}
	// Register memory usage metric
	_, err = metricStorage.RegisterGauge(
		HookRunMaxRSSBytes, executionLabels,
		options.WithHelp("Gauge of maximum resident set size used by hook in bytes"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunMaxRSSBytes, err)
	}

	// Register execution result counters
	_, err = metricStorage.RegisterCounter(
		HookRunErrorsTotal, executionLabels,
		options.WithHelp("Counter of hook execution errors (allowFailure: false)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookRunAllowedErrorsTotal, executionLabels,
		options.WithHelp("Counter of hook execution errors that are allowed to fail (allowFailure: true)"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunAllowedErrorsTotal, err)
	}

	_, err = metricStorage.RegisterCounter(
		HookRunSuccessTotal, executionLabels,
		options.WithHelp("Counter of successful hook executions"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", HookRunSuccessTotal, err)
	}

	// Register queue wait time metric
	_, err = metricStorage.RegisterCounter(
		TaskWaitInQueueSecondsTotal, executionLabels,
		options.WithHelp("Counter of seconds that hooks waited in queue before execution"),
	)
	if err != nil {
		return fmt.Errorf("can not register %s: %w", TaskWaitInQueueSecondsTotal, err)
	}

	return nil
}

// RegisterCommonMetrics registers base metrics used across shell-operator and addon-operator.
// These are fundamental metrics that indicate the health and activity of the operator.
func RegisterCommonMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterCounter(
		LiveTicks, []string{},
		options.WithHelp("Counter that increases every 10 seconds to indicate shell-operator is alive"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", LiveTicks, err)
	}

	return nil
}

// RegisterTaskQueueMetrics registers metrics related to task queue operations and performance.
// Used by both shell-operator and addon-operator to monitor queue health and performance.
func RegisterTaskQueueMetrics(metricStorage metricsstorage.Storage) error {
	// Task queue operation labels
	queueActionLabels := []string{"queue_name", "queue_action"}
	queueLabels := []string{"queue"}

	// High-resolution buckets for queue operations (microseconds to milliseconds)
	queueOperationBuckets := []float64{
		0.0,
		0.0001, 0.0002, 0.0005, // 100, 200, 500 microseconds
		0.001, 0.002, 0.005, // 1,2,5 milliseconds
		0.01, 0.02, 0.05, // 10,20,50 milliseconds
		0.1, 0.2, 0.5, // 100,200,500 milliseconds
	}

	// Register queue operation duration metric
	_, err := metricStorage.RegisterHistogram(
		TasksQueueActionDurationSeconds, queueActionLabels, queueOperationBuckets,
		options.WithHelp("Histogram of task queue operation durations in seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", TasksQueueActionDurationSeconds, err)
	}

	// Register queue length metric
	_, err = metricStorage.RegisterGauge(
		TasksQueueLength, queueLabels,
		options.WithHelp("Gauge showing the length of the task queue"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", TasksQueueLength, err)
	}

	return nil
}

// RegisterKubeEventsManagerMetrics registers metrics for Kubernetes events manager.
// These metrics monitor Kubernetes API interactions, object caching, and event processing.
// Used by both shell-operator and addon-operator.
func RegisterKubeEventsManagerMetrics(metricStorage metricsstorage.Storage, labels []string) error {
	// Error type labels for watch error tracking
	errorTypeLabels := []string{"error_type"}

	// Buckets for Kubernetes operation timing (milliseconds to seconds)
	kubeOperationBuckets := []float64{
		0.0,
		0.001, 0.002, 0.005, // 1,2,5 milliseconds
		0.01, 0.02, 0.05, // 10,20,50 milliseconds
		0.1, 0.2, 0.5, // 100,200,500 milliseconds
		1, 2, 5, 10, // 1,2,5,10 seconds
	}

	// Register snapshot objects gauge
	_, err := metricStorage.RegisterGauge(
		KubeSnapshotObjects, labels,
		options.WithHelp("Gauge with count of cached objects (snapshot) for particular binding"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", KubeSnapshotObjects, err)
	}

	// Register jq filter duration histogram
	_, err = metricStorage.RegisterHistogram(
		KubeJqFilterDurationSeconds, labels, kubeOperationBuckets,
		options.WithHelp("Histogram of jq filter execution duration in seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", KubeJqFilterDurationSeconds, err)
	}

	// Register Kubernetes event handling duration histogram
	_, err = metricStorage.RegisterHistogram(
		KubeEventDurationSeconds, labels, kubeOperationBuckets,
		options.WithHelp("Histogram of Kubernetes event handling duration in seconds"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", KubeEventDurationSeconds, err)
	}

	// Register watch errors counter
	_, err = metricStorage.RegisterCounter(
		KubernetesClientWatchErrorsTotal, errorTypeLabels,
		options.WithHelp("Counter of Kubernetes client watch errors by error type"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", KubernetesClientWatchErrorsTotal, err)
	}

	return nil
}
