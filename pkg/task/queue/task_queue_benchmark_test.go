package queue

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/task"
)

// mockTask is a mock implementation of the task.Task interface for benchmarks.
// It's a simplified version to avoid dependencies on other test files.
type mockTaskBench struct {
	Id             string
	Type           task.TaskType
	FailureCount   int
	FailureMessage string
	Metadata       interface{}
	processing     atomic.Bool
}

func newBenchmarkTask() task.Task {
	uuid, _ := uuid.NewV4()
	id := uuid.String()

	return &mockTaskBench{
		Id:   id,
		Type: "BenchmarkTask",
	}
}

func (t *mockTaskBench) GetId() string                      { return t.Id }
func (t *mockTaskBench) GetType() task.TaskType             { return t.Type }
func (t *mockTaskBench) GetQueueName() string               { return "benchmark_queue" }
func (t *mockTaskBench) GetDescription() string             { return fmt.Sprintf("task '%s'", t.Id) }
func (t *mockTaskBench) GetMetadata() interface{}           { return t.Metadata }
func (t *mockTaskBench) UpdateMetadata(m interface{})       { t.Metadata = m }
func (t *mockTaskBench) SetProcessing(val bool)             { t.processing.Store(val) }
func (t *mockTaskBench) IsProcessing() bool                 { return t.processing.Load() }
func (t *mockTaskBench) GetLogLabels() map[string]string    { return map[string]string{"id": t.Id} }
func (t *mockTaskBench) GetFailureCount() int               { return t.FailureCount }
func (t *mockTaskBench) IncrementFailureCount()             { t.FailureCount++ }
func (t *mockTaskBench) GetFailureMessage() string          { return t.FailureMessage }
func (t *mockTaskBench) UpdateFailureMessage(msg string)    { t.FailureMessage = msg }
func (t *mockTaskBench) GetProp(_ string) interface{}       { return nil }
func (t *mockTaskBench) SetProp(_ string, _ interface{})    {}
func (t *mockTaskBench) GetQueuedAt() time.Time             { return time.Now() }
func (t *mockTaskBench) WithQueuedAt(_ time.Time) task.Task { return t }

func (t *mockTaskBench) deepCopy() *mockTaskBench {
	newTask := &mockTaskBench{
		Id:             t.Id,
		Type:           t.Type,
		FailureCount:   t.FailureCount,
		FailureMessage: t.FailureMessage,
		Metadata:       t.Metadata,
	}

	// Copy atomic bool value
	newTask.processing.Store(t.processing.Load())

	return newTask
}

func (t *mockTaskBench) DeepCopyWithNewUUID() task.Task {
	newTask := t.deepCopy()
	newTask.Id = uuid.Must(uuid.NewV4()).String()
	return newTask
}

func (t *mockTaskBench) GetCompactionID() string {
	return t.Id
}

type Queue interface {
	AddLast(tasks ...task.Task)
	AddFirst(tasks ...task.Task)
	GetFirst() task.Task
	RemoveFirst() task.Task
	Get(id string) task.Task
}

func benchmarkAddLast(b *testing.B, queue Queue, size int) {
	for i := 0; i < size; i++ {
		queue.AddLast(newBenchmarkTask())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.AddLast(newBenchmarkTask())
	}
}

func benchmarkAddFirst(b *testing.B, queue Queue, size int) {
	for i := 0; i < size; i++ {
		queue.AddFirst(newBenchmarkTask())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.AddFirst(newBenchmarkTask())
	}
}

func benchmarkRemoveFirst(b *testing.B, queue Queue, size int) {
	for i := 0; i < size; i++ {
		queue.AddLast(newBenchmarkTask())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.RemoveFirst()
	}
}

func benchmarkGetFirst(b *testing.B, queue Queue, size int) {
	for i := 0; i < size; i++ {
		queue.AddLast(newBenchmarkTask())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.GetFirst()
	}
}

func benchmarkGetByID(b *testing.B, queue Queue, size int) {
	uuids := make([]string, 0, size)
	for i := 0; i < size; i++ {
		task := newBenchmarkTask()
		queue.AddLast(task)
		uuids = append(uuids, task.GetId())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queue.Get(uuids[i%size])
	}
}

/* Old code */
func BenchmarkTaskQueueSlice_AddLast_100(b *testing.B) {
	benchmarkAddLast(b, NewTasksQueueSlice(), 100)
}

func BenchmarkTaskQueueSlice_AddLast_1000(b *testing.B) {
	benchmarkAddLast(b, NewTasksQueueSlice(), 1000)
}

func BenchmarkTaskQueueSlice_AddFirst_100(b *testing.B) {
	benchmarkAddFirst(b, NewTasksQueueSlice(), 100)
}

func BenchmarkTaskQueueSlice_AddFirst_1000(b *testing.B) {
	benchmarkAddFirst(b, NewTasksQueueSlice(), 1000)
}

func BenchmarkTaskQueueSlice_RemoveFirst_100(b *testing.B) {
	benchmarkRemoveFirst(b, NewTasksQueueSlice(), 100)
}

func BenchmarkTaskQueueSlice_RemoveFirst_1000(b *testing.B) {
	benchmarkRemoveFirst(b, NewTasksQueueSlice(), 1000)
}

func BenchmarkTaskQueueSlice_GetFirst_100(b *testing.B) {
	benchmarkGetFirst(b, NewTasksQueueSlice(), 100)
}

func BenchmarkTaskQueueSlice_GetFirst_1000(b *testing.B) {
	benchmarkGetFirst(b, NewTasksQueueSlice(), 1000)
}

func BenchmarkTaskQueueSlice_GetByID_100(b *testing.B) {
	benchmarkGetByID(b, NewTasksQueueSlice(), 100)
}

func BenchmarkTaskQueueSlice_GetByID_1000(b *testing.B) {
	benchmarkGetByID(b, NewTasksQueueSlice(), 1000)
}

/* New code */

func newBenchmarkTasksQueue(b *testing.B) *TaskQueue {
	metricStorage := metric.NewStorageMock(b)
	metricStorage.HistogramObserveMock.Set(func(_ string, _ float64, _ map[string]string, _ []float64) {
		// skip any observe hit
	})
	metricStorage.GaugeSetMock.Set(func(_ string, _ float64, _ map[string]string) {
	})

	return NewTasksQueue("test", metricStorage)
}

func BenchmarkTaskQueue_AddLast_100(b *testing.B) {
	benchmarkAddLast(b, newBenchmarkTasksQueue(b), 100)
}

func BenchmarkTaskQueue_AddLast_1000(b *testing.B) {
	benchmarkAddLast(b, newBenchmarkTasksQueue(b), 1000)
}

func BenchmarkTaskQueue_AddFirst_100(b *testing.B) {
	benchmarkAddFirst(b, newBenchmarkTasksQueue(b), 100)
}

func BenchmarkTaskQueue_AddFirst_1000(b *testing.B) {
	benchmarkAddFirst(b, newBenchmarkTasksQueue(b), 1000)
}

func BenchmarkTaskQueue_RemoveFirst_100(b *testing.B) {
	benchmarkRemoveFirst(b, newBenchmarkTasksQueue(b), 100)
}

func BenchmarkTaskQueue_RemoveFirst_1000(b *testing.B) {
	benchmarkRemoveFirst(b, newBenchmarkTasksQueue(b), 1000)
}

func BenchmarkTaskQueue_GetFirst_100(b *testing.B) {
	benchmarkGetFirst(b, newBenchmarkTasksQueue(b), 100)
}

func BenchmarkTaskQueue_GetFirst_1000(b *testing.B) {
	benchmarkGetFirst(b, newBenchmarkTasksQueue(b), 1000)
}

func BenchmarkTaskQueue_GetByID_100(b *testing.B) {
	benchmarkGetByID(b, newBenchmarkTasksQueue(b), 100)
}

func BenchmarkTaskQueue_GetByID_1000(b *testing.B) {
	benchmarkGetByID(b, newBenchmarkTasksQueue(b), 1000)
}

// --- Compaction Benchmarks ---

// mockTaskForCompaction is a more realistic mock for compaction tests.
type mockTaskForCompaction struct {
	mockTaskBench
}

func (t *mockTaskForCompaction) GetType() task.TaskType {
	return task_metadata.HookRun
}

func newCompactionHookTask(id int, hookName string) task.Task {
	t := &mockTaskForCompaction{}
	t.Id = "task-" + strconv.Itoa(id)
	t.Metadata = task_metadata.HookMetadata{
		HookName: hookName,
		BindingContext: []bindingcontext.BindingContext{
			{Binding: fmt.Sprintf("bc_for_%d", id)},
		},
	}
	return t
}

func createCompactionBenchmarkData(b *testing.B, size int) []task.Task {
	b.Helper()
	tasks := make([]task.Task, 0, size)
	// Create a mix of hooks for a realistic test
	hookNames := []string{"hook-a", "hook-b", "hook-c", "hook-d", "hook-e"}
	for i := 0; i < size; i++ {
		hookName := hookNames[i%len(hookNames)]
		t := newCompactionHookTask(i, hookName)
		// Mark some tasks as processing
		if i%20 == 0 {
			t.SetProcessing(true)
		}
		tasks = append(tasks, t)
	}
	return tasks
}

func benchmarkTaskQueueCompaction(b *testing.B, size int) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		metricStorage := metric.NewStorageMock(b)
		metricStorage.GaugeSetMock.Set(func(_ string, _ float64, _ map[string]string) {
		})

		q := NewTasksQueue("test", metricStorage, WithCompactableTypes(task_metadata.HookRun))
		tasks := createCompactionBenchmarkData(b, size)
		// Setup queue without triggering compaction
		for _, t := range tasks {
			q.storage.AddLast(t)
		}

		b.StartTimer()
		q.compaction(nil)
	}
}

func benchmarkTaskQueueSliceCompaction(b *testing.B, size int) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		q := NewTasksQueueSlice()
		q.WithCompactableTypes([]task.TaskType{task_metadata.HookRun})
		tasks := createCompactionBenchmarkData(b, size)
		// Setup queue without triggering compaction
		q.items = append(q.items, tasks...)

		b.StartTimer()
		q.compaction()
	}
}

/* Old code */
func BenchmarkTaskQueueSlice_Compaction_10(b *testing.B) {
	benchmarkTaskQueueSliceCompaction(b, 10)
}

func BenchmarkTaskQueueSlice_Compaction_100(b *testing.B) {
	benchmarkTaskQueueSliceCompaction(b, 100)
}

func BenchmarkTaskQueueSlice_Compaction_500(b *testing.B) {
	benchmarkTaskQueueSliceCompaction(b, 500)
}

func BenchmarkTaskQueueSlice_Compaction_1000(b *testing.B) {
	benchmarkTaskQueueSliceCompaction(b, 1000)
}

/* New code */
func BenchmarkTaskQueue_Compaction_10(b *testing.B) {
	benchmarkTaskQueueCompaction(b, 10)
}

func BenchmarkTaskQueue_Compaction_100(b *testing.B) {
	benchmarkTaskQueueCompaction(b, 100)
}

func BenchmarkTaskQueue_Compaction_500(b *testing.B) {
	benchmarkTaskQueueCompaction(b, 500)
}

func BenchmarkTaskQueue_Compaction_1000(b *testing.B) {
	benchmarkTaskQueueCompaction(b, 1000)
}
