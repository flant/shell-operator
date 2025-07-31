package queue

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	metadata "github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/task"
)

// mockTask is a mock implementation of the task.Task interface for testing.
type mockTask struct {
	Id             string
	Type           task.TaskType
	LogLabels      map[string]string
	FailureCount   int // Failed executions count
	FailureMessage string
	QueueName      string
	QueuedAt       time.Time

	Props map[string]interface{}

	lock     sync.RWMutex
	Metadata interface{}

	processing atomic.Bool
}

func newMockTask(id, hookName string, taskType task.TaskType) *mockTask {
	return &mockTask{
		Id:   id,
		Type: taskType,
		Metadata: metadata.HookMetadata{
			HookName: hookName,
			BindingContext: []bindingcontext.BindingContext{
				{
					Binding: fmt.Sprintf("bc_for_%s", id),
				},
			},
		},
	}
}

func newHookTask(id, hookName string) *mockTask {
	return newMockTask(id, hookName, metadata.HookRun)
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
	if t.Type != metadata.HookRun {
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

func TestTaskQueue_AddLast_GreedyMerge(t *testing.T) {
	tests := []struct {
		name         string
		initialQueue []task.Task
		taskToAdd    task.Task
		expectedIDs  []string
		expectedBCs  map[string]int // map[taskID] -> expected number of binding contexts
	}{
		{
			name:         "Simple merge into last task",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:    newHookTask("h1_B", "hook-1"),
			expectedIDs:  []string{"h1_A"},
			expectedBCs:  map[string]int{"h1_A": 2},
		},
		{
			name:         "No merge for different hook",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:    newHookTask("h2_B", "hook-2"),
			expectedIDs:  []string{"h1_A", "h2_B"},
			expectedBCs:  map[string]int{"h1_A": 1, "h2_B": 1},
		},
		{
			name:         "Greedy merge over a different hook task",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1"), newHookTask("h2_B", "hook-2")},
			taskToAdd:    newHookTask("h1_C", "hook-1"),
			expectedIDs:  []string{"h1_A", "h2_B"},
			expectedBCs:  map[string]int{"h1_A": 2, "h2_B": 1},
		},
		{
			name: "Do not merge into a processing task, add new",
			initialQueue: []task.Task{func() task.Task {
				t := newHookTask("h1_A", "hook-1")
				t.SetProcessing(true)
				return t
			}()},
			taskToAdd:   newHookTask("h1_B", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B"},
			expectedBCs: map[string]int{"h1_A": 1, "h1_B": 1},
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
			taskToAdd:   newHookTask("h1_C", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B"},
			expectedBCs: map[string]int{"h1_A": 1, "h1_B": 2},
		},
		{
			name: "Greedy merge over a processing task of the same kind",
			initialQueue: []task.Task{
				newHookTask("h1_A", "hook-1"),
				func() task.Task {
					t := newHookTask("h1_B", "hook-1")
					t.SetProcessing(true)
					return t
				}(),
			},
			taskToAdd:   newHookTask("h1_C", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B"},
			expectedBCs: map[string]int{"h1_A": 2, "h1_B": 1},
		},
		{
			name:         "Add service task, no merge",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:    newServiceTask("service_B"),
			expectedIDs:  []string{"h1_A", "service_B"},
			expectedBCs:  map[string]int{"h1_A": 1},
		},
		{
			name:         "Merge hook task over a service task",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1"), newServiceTask("service_B")},
			taskToAdd:    newHookTask("h1_C", "hook-1"),
			expectedIDs:  []string{"h1_A", "service_B"},
			expectedBCs:  map[string]int{"h1_A": 2},
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
			expectedBCs: map[string]int{
				"h1_A": 1, // because it's processing
				"h1_B": 4, // own (1) + h1_C (1) + h1_D (1) + h1_E (1)
				"h2_A": 2, // own (1) + h2_B (1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &TaskQueueOLD{
				Name:  "test_queue",
				m:     sync.RWMutex{},
				items: tt.initialQueue,
			}

			q.addLast(tt.taskToAdd)

			// Verify IDs and order
			finalIDs := make([]string, len(q.items))
			for i, item := range q.items {
				finalIDs[i] = item.GetId()
			}
			assert.Equal(t, tt.expectedIDs, finalIDs, "Task IDs and order should match expected")

			// Verify binding context counts
			for _, item := range q.items {
				if mt, ok := item.(*mockTask); ok && mt.GetType() == metadata.HookRun {
					expectedCount, found := tt.expectedBCs[mt.GetId()]
					require.True(t, found, "Task ID %s should be in expectedBCs map", mt.GetId())
					hm := task_metadata.HookMetadataAccessor(mt)
					require.NotNil(t, hm, "HookMetadataAccessor should not return nil for hook task")
					assert.Len(t, hm.BindingContext, expectedCount, "BindingContext count for task %s should match", mt.GetId())
				}
			}
		})
	}
}

func TestTaskQueueList_AddLast_GreedyMerge(t *testing.T) {
	tests := []struct {
		name         string
		initialQueue []task.Task
		taskToAdd    task.Task
		expectedIDs  []string
		expectedBCs  map[string]int // map[taskID] -> expected number of binding contexts
	}{
		{
			name:         "Simple merge into last task",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:    newHookTask("h1_B", "hook-1"),
			expectedIDs:  []string{"h1_A"},
			expectedBCs:  map[string]int{"h1_A": 2},
		},
		{
			name:         "No merge for different hook",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:    newHookTask("h2_B", "hook-2"),
			expectedIDs:  []string{"h1_A", "h2_B"},
			expectedBCs:  map[string]int{"h1_A": 1, "h2_B": 1},
		},
		{
			name:         "Greedy merge over a different hook task",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1"), newHookTask("h2_B", "hook-2")},
			taskToAdd:    newHookTask("h1_C", "hook-1"),
			expectedIDs:  []string{"h1_A", "h2_B"},
			expectedBCs:  map[string]int{"h1_A": 2, "h2_B": 1},
		},
		{
			name: "Do not merge into a processing task, add new",
			initialQueue: []task.Task{func() task.Task {
				t := newHookTask("h1_A", "hook-1")
				t.SetProcessing(true)
				return t
			}()},
			taskToAdd:   newHookTask("h1_B", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B"},
			expectedBCs: map[string]int{"h1_A": 1, "h1_B": 1},
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
			taskToAdd:   newHookTask("h1_C", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B"},
			expectedBCs: map[string]int{"h1_A": 1, "h1_B": 2},
		},
		{
			name: "Greedy merge over a processing task of the same kind",
			initialQueue: []task.Task{
				newHookTask("h1_A", "hook-1"),
				func() task.Task {
					t := newHookTask("h1_B", "hook-1")
					t.SetProcessing(true)
					return t
				}(),
			},
			taskToAdd:   newHookTask("h1_C", "hook-1"),
			expectedIDs: []string{"h1_A", "h1_B"},
			expectedBCs: map[string]int{"h1_A": 2, "h1_B": 1},
		},
		{
			name:         "Add service task, no merge",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1")},
			taskToAdd:    newServiceTask("service_B"),
			expectedIDs:  []string{"h1_A", "service_B"},
			expectedBCs:  map[string]int{"h1_A": 1},
		},
		{
			name:         "Merge hook task over a service task",
			initialQueue: []task.Task{newHookTask("h1_A", "hook-1"), newServiceTask("service_B")},
			taskToAdd:    newHookTask("h1_C", "hook-1"),
			expectedIDs:  []string{"h1_A", "service_B"},
			expectedBCs:  map[string]int{"h1_A": 2},
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
			expectedBCs: map[string]int{
				"h1_A": 1, // because it's processing
				"h1_B": 4, // own (1) + h1_C (1) + h1_D (1) + h1_E (1)
				"h2_A": 2, // own (1) + h2_B (1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewTasksQueue()
			q.WithName("test_queue")
			for _, task := range tt.initialQueue {
				q.addLast(task)
			}
			// Reset compaction logging for the actual test
			q.performGlobalCompaction() // Initial compaction before adding the new task

			q.addLast(tt.taskToAdd)

			// Verify IDs and order
			finalIDs := make([]string, 0, q.items.Len())
			for e := q.items.Front(); e != nil; e = e.Next() {
				finalIDs = append(finalIDs, e.Value.(task.Task).GetId())
			}
			assert.Equal(t, tt.expectedIDs, finalIDs, "Task IDs and order should match expected")

			// Verify binding context counts
			for e := q.items.Front(); e != nil; e = e.Next() {
				item := e.Value.(task.Task)
				if mt, ok := item.(*mockTask); ok && mt.GetType() == metadata.HookRun {
					expectedCount, found := tt.expectedBCs[mt.GetId()]
					require.True(t, found, "Task ID %s should be in expectedBCs map", mt.GetId())
					hm := task_metadata.HookMetadataAccessor(mt)
					require.NotNil(t, hm, "HookMetadataAccessor should not return nil for hook task")
					assert.Len(t, hm.BindingContext, expectedCount, "BindingContext count for task %s should match", mt.GetId())
				}
			}
		})
	}
}
