package queue

// import (
// 	"sync"
// 	"testing"

// 	"github.com/stretchr/testify/require"

// 	"github.com/flant/shell-operator/pkg/hook/task_metadata"
// 	"github.com/flant/shell-operator/pkg/metric"
// 	"github.com/flant/shell-operator/pkg/metrics"
// 	"github.com/flant/shell-operator/pkg/task"
// )

// func TestTaskCounterRemoveUsesQueueName(t *testing.T) {
// 	metricStorage := metric.NewStorageMock(t)

// 	type gaugeCall struct {
// 		metric string
// 		value  float64
// 		labels map[string]string
// 	}

// 	var (
// 		mu    sync.Mutex
// 		calls []gaugeCall
// 	)

// 	metricStorage.GaugeSetMock.Set(func(metric string, value float64, labels map[string]string) {
// 		cloned := make(map[string]string, len(labels))
// 		for k, v := range labels {
// 			cloned[k] = v
// 		}

// 		mu.Lock()
// 		calls = append(calls, gaugeCall{
// 			metric: metric,
// 			value:  value,
// 			labels: cloned,
// 		})
// 		mu.Unlock()
// 	})

// 	tc := NewTaskCounter("main", nil, metricStorage)

// 	testTask := task.NewTask(task_metadata.HookRun).
// 		WithCompactionID("test-hook")

// 	tc.Add(testTask)
// 	tc.Remove(testTask)

// 	mu.Lock()
// 	require.NotEmpty(t, calls)
// 	lastCall := calls[len(calls)-1]
// 	mu.Unlock()

// 	require.Equal(t, metrics.TasksQueueCompactionInQueueTasks, lastCall.metric)
// 	require.Equal(t, float64(0), lastCall.value)
// 	require.Equal(t, "main", lastCall.labels["queue_name"])
// 	require.Equal(t, "test-hook", lastCall.labels["task_id"])
// }

// func TestTaskCounterRemoveClearsReachedCap(t *testing.T) {
// 	metricStorage := metric.NewStorageMock(t)

// 	var (
// 		mu            sync.Mutex
// 		reachedValues []float64
// 	)

// 	metricStorage.GaugeSetMock.Set(func(metric string, value float64, labels map[string]string) {
// 		if metric == metrics.TasksQueueCompactionReached {
// 			mu.Lock()
// 			reachedValues = append(reachedValues, value)
// 			mu.Unlock()
// 		}
// 	})

// 	tc := NewTaskCounter("main", nil, metricStorage)

// 	testTask := task.NewTask(task_metadata.HookRun).
// 		WithCompactionID("test-hook")

// 	for i := 0; i < taskCap; i++ {
// 		tc.Add(testTask)
// 	}

// 	require.True(t, tc.IsAnyCapReached())

// 	tc.Remove(testTask)

// 	require.False(t, tc.IsAnyCapReached())

// 	mu.Lock()
// 	require.NotEmpty(t, reachedValues)
// 	require.Equal(t, float64(0), reachedValues[len(reachedValues)-1])
// 	mu.Unlock()
// }
