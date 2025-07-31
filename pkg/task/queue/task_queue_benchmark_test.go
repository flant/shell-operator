package queue

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
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

func newBenchmarkTask(id int, hookRun bool) task.Task {
	if !hookRun {
		return &mockTaskBench{
			Id:   "task-" + strconv.Itoa(id),
			Type: "BenchmarkTask",
		}
	}
	return &mockTaskBench{
		Id:   "task-" + strconv.Itoa(id),
		Type: task_metadata.HookRun,
		Metadata: task_metadata.HookMetadata{
			HookName: "benchmark_hook",
			BindingContext: []bindingcontext.BindingContext{
				{Binding: fmt.Sprintf("bc_for_%d", id)},
			},
		},
	}
}

func (t *mockTaskBench) GetId() string                             { return t.Id }
func (t *mockTaskBench) GetType() task.TaskType                    { return t.Type }
func (t *mockTaskBench) GetQueueName() string                      { return "benchmark_queue" }
func (t *mockTaskBench) GetDescription() string                    { return fmt.Sprintf("task '%s'", t.Id) }
func (t *mockTaskBench) GetMetadata() interface{}                  { return t.Metadata }
func (t *mockTaskBench) UpdateMetadata(m interface{})              { t.Metadata = m }
func (t *mockTaskBench) SetProcessing(val bool)                    { t.processing.Store(val) }
func (t *mockTaskBench) IsProcessing() bool                        { return t.processing.Load() }
func (t *mockTaskBench) GetLogLabels() map[string]string           { return map[string]string{"id": t.Id} }
func (t *mockTaskBench) GetFailureCount() int                      { return t.FailureCount }
func (t *mockTaskBench) IncrementFailureCount()                    { t.FailureCount++ }
func (t *mockTaskBench) GetFailureMessage() string                 { return t.FailureMessage }
func (t *mockTaskBench) UpdateFailureMessage(msg string)           { t.FailureMessage = msg }
func (t *mockTaskBench) GetProp(key string) interface{}            { return nil }
func (t *mockTaskBench) SetProp(key string, value interface{})     {}
func (t *mockTaskBench) GetQueuedAt() time.Time                    { return time.Now() }
func (t *mockTaskBench) WithQueuedAt(queuedAt time.Time) task.Task { return t }

// --- Benchmarks for TaskQueue (slice-based) ---

func benchmarkTaskQueueAddLast(b *testing.B, size int) {
	q := NewTasksQueueOLD()
	for i := 0; i < size; i++ {
		q.AddLast(newBenchmarkTask(i, true))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.AddLast(newBenchmarkTask(size+i, true))
	}
}

func benchmarkTaskQueueAddFirst(b *testing.B, size int) {
	q := NewTasksQueueOLD()
	for i := 0; i < size; i++ {
		q.AddLast(newBenchmarkTask(i, true))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.AddFirst(newBenchmarkTask(size+i, true))
	}
}

func benchmarkTaskQueueRemoveFirst(b *testing.B, size int) {
	b.StopTimer()
	q := NewTasksQueueOLD()
	for i := 0; i < size; i++ {
		q.AddLast(newBenchmarkTask(i, false))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if q.Length() == 0 {
			b.StopTimer()
			for j := 0; j < size; j++ {
				q.AddLast(newBenchmarkTask(j, false))
			}
			b.StartTimer()
		}
		q.RemoveFirst()
	}
}

func BenchmarkTaskQueue_AddLast_100(b *testing.B)      { benchmarkTaskQueueAddLast(b, 100) }
func BenchmarkTaskQueue_AddLast_1000(b *testing.B)     { benchmarkTaskQueueAddLast(b, 1000) }
func BenchmarkTaskQueue_AddFirst_100(b *testing.B)     { benchmarkTaskQueueAddFirst(b, 100) }
func BenchmarkTaskQueue_AddFirst_1000(b *testing.B)    { benchmarkTaskQueueAddFirst(b, 1000) }
func BenchmarkTaskQueue_RemoveFirst_100(b *testing.B)  { benchmarkTaskQueueRemoveFirst(b, 100) }
func BenchmarkTaskQueue_RemoveFirst_1000(b *testing.B) { benchmarkTaskQueueRemoveFirst(b, 1000) }

// --- Benchmarks for TaskQueueList (list-based) ---

func benchmarkTaskQueueListAddLast(b *testing.B, size int) {
	q := NewTasksQueue()
	for i := 0; i < size; i++ {
		q.AddLast(newBenchmarkTask(i, true))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.AddLast(newBenchmarkTask(size+i, true))
	}
}

func benchmarkTaskQueueListAddFirst(b *testing.B, size int) {
	q := NewTasksQueue()
	for i := 0; i < size; i++ {
		q.AddLast(newBenchmarkTask(i, true))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.AddFirst(newBenchmarkTask(size+i, true))
	}
}

func benchmarkTaskQueueListRemoveFirst(b *testing.B, size int) {
	b.StopTimer()
	q := NewTasksQueue()
	for i := 0; i < size; i++ {
		q.AddLast(newBenchmarkTask(i, false))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if q.Length() == 0 {
			b.StopTimer()
			for j := 0; j < size; j++ {
				q.AddLast(newBenchmarkTask(j, false))
			}
			b.StartTimer()
		}
		q.RemoveFirst()
	}
}

func BenchmarkTaskQueueList_AddLast_100(b *testing.B)     { benchmarkTaskQueueListAddLast(b, 100) }
func BenchmarkTaskQueueList_AddLast_1000(b *testing.B)    { benchmarkTaskQueueListAddLast(b, 1000) }
func BenchmarkTaskQueueList_AddFirst_100(b *testing.B)    { benchmarkTaskQueueListAddFirst(b, 100) }
func BenchmarkTaskQueueList_AddFirst_1000(b *testing.B)   { benchmarkTaskQueueListAddFirst(b, 1000) }
func BenchmarkTaskQueueList_RemoveFirst_100(b *testing.B) { benchmarkTaskQueueListRemoveFirst(b, 100) }
func BenchmarkTaskQueueList_RemoveFirst_1000(b *testing.B) {
	benchmarkTaskQueueListRemoveFirst(b, 1000)
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

// Benchmark for original TaskQueue (slice-based) compaction
func benchmarkTaskQueueCompaction(b *testing.B, size int) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		q := NewTasksQueueOLD()
		tasks := createCompactionBenchmarkData(b, size)
		q.items = tasks // Directly set items to avoid compaction during setup
		triggerTask := newCompactionHookTask(size, "hook-a")
		b.StartTimer()
		q.addLast(triggerTask) // This triggers performGlobalCompaction
	}
}

func BenchmarkTaskQueue_Compaction_100(b *testing.B)  { benchmarkTaskQueueCompaction(b, 100) }
func BenchmarkTaskQueue_Compaction_500(b *testing.B)  { benchmarkTaskQueueCompaction(b, 500) }
func BenchmarkTaskQueue_Compaction_1000(b *testing.B) { benchmarkTaskQueueCompaction(b, 1000) }

// Benchmark for new TaskQueueList (list-based) compaction
func benchmarkTaskQueueListCompaction(b *testing.B, size int) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		q := NewTasksQueue()
		tasks := createCompactionBenchmarkData(b, size)
		// Setup queue without triggering compaction
		for _, t := range tasks {
			element := q.items.PushBack(t)
			q.idIndex[t.GetId()] = element
		}
		triggerTask := newCompactionHookTask(size, "hook-a")
		b.StartTimer()
		q.addLast(triggerTask) // This triggers performGlobalCompaction
	}
}

func BenchmarkTaskQueueList_Compaction_100(b *testing.B)  { benchmarkTaskQueueListCompaction(b, 100) }
func BenchmarkTaskQueueList_Compaction_500(b *testing.B)  { benchmarkTaskQueueListCompaction(b, 500) }
func BenchmarkTaskQueueList_Compaction_1000(b *testing.B) { benchmarkTaskQueueListCompaction(b, 1000) }
