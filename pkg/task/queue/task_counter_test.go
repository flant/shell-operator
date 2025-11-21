package queue

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
)

func TestTaskCounterUpdateHookMetricsFromSnapshot(t *testing.T) {
	metricStorage := metric.NewStorageMock(t)

	type gaugeCall struct {
		metric string
		value  float64
		labels map[string]string
	}

	var (
		mu    sync.Mutex
		calls []gaugeCall
	)

	metricStorage.GaugeSetMock.Set(func(metric string, value float64, labels map[string]string) {
		cloned := make(map[string]string, len(labels))
		for k, v := range labels {
			cloned[k] = v
		}

		mu.Lock()
		calls = append(calls, gaugeCall{
			metric: metric,
			value:  value,
			labels: cloned,
		})
		mu.Unlock()
	})

	tc := NewTaskCounter("main", nil, metricStorage)

	// Simulate initial state with hooks above threshold by setting up a snapshot
	initialSnapshot := map[string]uint{
		"hook1": 26,
		"hook2": 31,
		"hook3": 51,
	}
	tc.UpdateHookMetricsFromSnapshot(initialSnapshot)

	mu.Lock()
	calls = nil // Clear previous calls
	mu.Unlock()

	// Update with new snapshot where:
	// - hook1 still has high count (25 tasks)
	// - hook2 dropped below threshold (15 tasks) - should not be published
	// - hook3 is completely gone (0 tasks in new snapshot) - should not be published
	// - hook4 is new (30 tasks)
	newSnapshot := map[string]uint{
		"hook1": 25,
		"hook2": 15,
		"hook4": 30,
	}

	tc.UpdateHookMetricsFromSnapshot(newSnapshot)

	mu.Lock()
	defer mu.Unlock()

	// Verify that metrics were set correctly
	require.NotEmpty(t, calls)

	// Build a map of last call for each hook
	lastCallByHook := make(map[string]gaugeCall)
	for _, call := range calls {
		if call.metric == metrics.TasksQueueCompactionTasksByHook {
			hook := call.labels["hook"]
			lastCallByHook[hook] = call
		}
	}

	// hook1: should have value 25
	require.Contains(t, lastCallByHook, "hook1")
	require.Equal(t, float64(25), lastCallByHook["hook1"].value)
	require.Equal(t, "main", lastCallByHook["hook1"].labels["queue_name"])

	// hook2: should NOT be published (below threshold)
	require.NotContains(t, lastCallByHook, "hook2")

	// hook3: should NOT be published (removed from snapshot and was above threshold before)
	require.NotContains(t, lastCallByHook, "hook3")

	// hook4: should have value 30
	require.Contains(t, lastCallByHook, "hook4")
	require.Equal(t, float64(30), lastCallByHook["hook4"].value)
	require.Equal(t, "main", lastCallByHook["hook4"].labels["queue_name"])
}
