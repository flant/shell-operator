package queue

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/task"
)

// mockTask is a mock implementation of the task.Task interface for testing.
type mockTask struct {
	Id             string
	Type           task.TaskType
	LogLabels      map[string]string
	FailureCount   int
	FailureMessage string
	QueueName      string
	QueuedAt       time.Time

	Props map[string]interface{}

	Metadata interface{}

	processing atomic.Bool
}

func newMockTask(id, hookName string, taskType task.TaskType) *mockTask {
	return &mockTask{
		Id:   id,
		Type: taskType,
		Metadata: task_metadata.HookMetadata{
			Group:    "group",
			HookName: hookName,
			BindingContext: []bindingcontext.BindingContext{
				{
					Metadata: struct {
						Version             string
						BindingType         types.BindingType
						JqFilter            string
						IncludeSnapshots    []string
						IncludeAllSnapshots bool
						Group               string
					}{
						Group: "group",
					},
					Binding: fmt.Sprintf("bc_for_%s", id),
				},
			},
		},
	}
}

func newHookTask(id, hookName string) *mockTask {
	return newMockTask(id, hookName, task_metadata.HookRun)
}

func newServiceTask(id string) *mockTask {
	return newMockTask(id, "", "Service")
}

func (t *mockTask) GetId() string {
	return t.Id
}

func (t *mockTask) GetType() task.TaskType {
	return t.Type
}

func (t *mockTask) GetQueueName() string {
	return "main"
}

func (t *mockTask) GetDescription() string {
	return fmt.Sprintf("task '%s'", t.Id)
}

func (t *mockTask) GetMetadata() interface{} {
	if t.Type != task_metadata.HookRun {
		return nil
	}
	return t.Metadata
}

func (t *mockTask) UpdateMetadata(m interface{}) {
	t.Metadata = m
}

func (t *mockTask) SetProcessing(val bool) {
	t.processing.Store(val)
}

func (t *mockTask) IsProcessing() bool {
	return t.processing.Load()
}

func (t *mockTask) GetLogLabels() map[string]string {
	return map[string]string{"id": t.Id}
}

func (t *mockTask) GetFailureCount() int {
	return t.FailureCount
}

func (t *mockTask) GetFailureMessage() string {
	return t.FailureMessage
}

func (t *mockTask) UpdateFailureMessage(msg string) {
	t.FailureMessage = msg
}

func (t *mockTask) GetProp(key string) interface{} {
	return t.Props[key]
}

func (t *mockTask) SetProp(key string, value interface{}) {
	t.Props[key] = value
}

func (t *mockTask) GetQueuedAt() time.Time {
	return t.QueuedAt
}

func (t *mockTask) WithQueuedAt(queuedAt time.Time) task.Task {
	t.QueuedAt = queuedAt
	return t
}

func (t *mockTask) IncrementFailureCount() {
	t.FailureCount++
}

func (t *mockTask) deepCopy() *mockTask {
	newTask := &mockTask{
		Id:             t.Id,
		Type:           t.Type,
		FailureCount:   t.FailureCount,
		FailureMessage: t.FailureMessage,
		QueueName:      t.QueueName,
		QueuedAt:       t.QueuedAt,
		Metadata:       t.Metadata,
	}

	// Deep copy LogLabels
	newTask.LogLabels = make(map[string]string)
	for k, v := range t.LogLabels {
		newTask.LogLabels[k] = v
	}

	// Deep copy Props
	newTask.Props = make(map[string]interface{})
	for k, v := range t.Props {
		newTask.Props[k] = v
	}

	// Copy atomic bool value
	newTask.processing.Store(t.processing.Load())

	return newTask
}

func (t *mockTask) DeepCopyWithNewUUID() task.Task {
	newTask := t.deepCopy()
	newTask.Id = uuid.Must(uuid.NewV4()).String()
	newTask.LogLabels["task.id"] = newTask.Id
	return newTask
}

func (t *mockTask) GetCompactionID() string {
	return t.Id
}

// newTestMetricStorage creates a metric storage mock with all required methods stubbed
func newTestMetricStorage(t *testing.T, expectCompaction bool) *metric.StorageMock {
	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(_ string, _ float64, _ map[string]string, _ []float64) {
	})
	// CounterAdd is only called during actual compaction
	if expectCompaction {
		metricStorage.CounterAddMock.Set(func(_ string, _ float64, _ map[string]string) {
		})
	}
	// Note: GaugeSet is no longer called during compaction tests since metrics
	// are updated asynchronously in a separate goroutine that runs with Start()
	return metricStorage
}

func TestTaskQueueList_AddLast_GreedyMerge(t *testing.T) {
	tests := []struct {
		name             string
		initialQueue     []task.Task
		taskToAdd        task.Task
		expectedIDs      []string
		expectedBCs      map[string]string // map[taskID] -> expected number of binding contexts
		expectCompaction bool              // whether compaction is expected to happen
	}{
		{
			name:             "Simple merge into last task",
			initialQueue:     []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:        newHookTask("h1_B", "hook-1"),
			expectedIDs:      []string{"h1_A"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_B"},
			expectCompaction: true,
		},
		{
			name:             "No merge for different hook",
			initialQueue:     []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:        newHookTask("h2_B", "hook-2"),
			expectedIDs:      []string{"h1_A", "h2_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_A", "h2_B": "bc_for_h2_B"},
			expectCompaction: false,
		},
		{
			name:             "Greedy merge over a different hook task",
			initialQueue:     []task.Task{newHookTask("h1_A", "hook-1"), newHookTask("h2_B", "hook-2")},
			taskToAdd:        newHookTask("h1_C", "hook-1"),
			expectedIDs:      []string{"h1_A", "h2_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_C", "h2_B": "bc_for_h2_B"},
			expectCompaction: true,
		},
		{
			name: "Do not merge into a processing task, add new",
			initialQueue: []task.Task{func() task.Task {
				t := newHookTask("h1_A", "hook-1")
				t.SetProcessing(true)
				return t
			}()},
			taskToAdd:        newHookTask("h1_B", "hook-1"),
			expectedIDs:      []string{"h1_A", "h1_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_A", "h1_B": "bc_for_h1_B"},
			expectCompaction: false,
		},
		{
			name: "Merge into the second pile, not the processing one",
			initialQueue: []task.Task{
				func() task.Task {
					t := newHookTask("h1_A", "hook-1")
					t.SetProcessing(true)
					return t
				}(),
				newHookTask("h1_B", "hook-1"),
			},
			taskToAdd:        newHookTask("h1_C", "hook-1"),
			expectedIDs:      []string{"h1_A", "h1_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_A", "h1_B": "bc_for_h1_C"},
			expectCompaction: true,
		},
		{
			name: "Merge over a processing task of the same kind",
			initialQueue: []task.Task{
				newHookTask("h1_A", "hook-1"),
				func() task.Task {
					t := newHookTask("h1_B", "hook-1")
					t.SetProcessing(true)
					return t
				}(),
			},
			taskToAdd:        newHookTask("h1_C", "hook-1"),
			expectedIDs:      []string{"h1_A", "h1_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_C", "h1_B": "bc_for_h1_B"},
			expectCompaction: true,
		},
		{
			name:             "Add service task, no merge",
			initialQueue:     []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:        newServiceTask("service_B"),
			expectedIDs:      []string{"h1_A", "service_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_A"},
			expectCompaction: false,
		},
		{
			name:             "Merge hook task over a service task",
			initialQueue:     []task.Task{newHookTask("h1_A", "hook-1"), newServiceTask("service_B")},
			taskToAdd:        newHookTask("h1_C", "hook-1"),
			expectedIDs:      []string{"h1_A", "service_B"},
			expectedBCs:      map[string]string{"h1_A": "bc_for_h1_C"},
			expectCompaction: true,
		},
		{
			name: "Greedy merge should compact the entire queue",
			initialQueue: []task.Task{
				func() task.Task {
					t := newHookTask("h1_A", "hook-1")
					t.SetProcessing(true)
					return t
				}(),
				newHookTask("h1_B", "hook-1"),
				newServiceTask("service-1"),
				newHookTask("h2_A", "hook-2"),
				newHookTask("h1_C", "hook-1"),
				newHookTask("h1_D", "hook-1"),
				newHookTask("h2_B", "hook-2"),
			},
			taskToAdd:   newHookTask("h1_E", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B", "service-1", "h2_A"},
			expectedBCs: map[string]string{
				"h1_A": "bc_for_h1_A", // because it's processing
				"h1_B": "bc_for_h1_E", // own (dropped) + h1_C (dropped) + h1_D (dropped) + h1_E (latest kept)
				"h2_A": "bc_for_h2_B", // own (dropped) + h2_B (latest kept)
			},
			expectCompaction: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricStorage := newTestMetricStorage(t, tt.expectCompaction)

			q := NewTasksQueue("test", metricStorage, WithCompactableTypes(task_metadata.HookRun))

			for _, task := range tt.initialQueue {
				q.AddLast(task)
			}

			q.AddLast(tt.taskToAdd)

			q.compaction(nil)
			// Verify IDs and order
			finalIDs := make([]string, 0, q.Length())
			q.IterateSnapshot(func(t task.Task) {
				finalIDs = append(finalIDs, t.GetId())
			})

			assert.Equal(t, tt.expectedIDs, finalIDs, "Task IDs and order should match expected")
			q.IterateSnapshot(func(task task.Task) {
				if mt, ok := task.(*mockTask); ok && mt.GetType() == task_metadata.HookRun {
					hm := task_metadata.HookMetadataAccessor(mt)
					require.NotNil(t, hm, "HookMetadataAccessor should not return nil for hook task")
					assert.Equal(t, tt.expectedBCs[mt.GetId()], hm.BindingContext[0].Binding, "BindingContext for task %s should match", mt.GetId())
				}
			})
		})
	}
}
