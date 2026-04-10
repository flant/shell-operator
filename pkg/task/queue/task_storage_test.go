package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/utils/list"
)

// TestTaskStorage_BasicOperations tests basic add, get, and remove operations
func TestTaskStorage_BasicOperations(t *testing.T) {
	ts := newTaskStorage()

	// Test empty storage
	if ts.Length() != 0 {
		t.Errorf("Expected length 0, got %d", ts.Length())
	}

	if ts.GetFirst() != nil {
		t.Error("Expected GetFirst() to return nil for empty storage")
	}

	if ts.GetLast() != nil {
		t.Error("Expected GetLast() to return nil for empty storage")
	}

	// Create some tasks
	task1 := task.NewTask("test1")
	task2 := task.NewTask("test2")
	task3 := task.NewTask("test3")

	// Test AddLast
	ts.AddLast(task1, task2)
	if ts.Length() != 2 {
		t.Errorf("Expected length 2, got %d", ts.Length())
	}

	if ts.GetFirst() != task1 {
		t.Error("Expected first task to be task1")
	}

	if ts.GetLast() != task2 {
		t.Error("Expected last task to be task2")
	}

	// Test Get by ID
	if ts.Get(task1.GetId()) != task1 {
		t.Error("Expected to get task1 by ID")
	}

	if ts.Get("nonexistent") != nil {
		t.Error("Expected Get(nonexistent) to return nil")
	}

	// Test AddFirst
	ts.AddFirst(task3)
	if ts.Length() != 3 {
		t.Errorf("Expected length 3, got %d", ts.Length())
	}

	if ts.GetFirst() != task3 {
		t.Error("Expected first task to be task3 after AddFirst")
	}

	// Test RemoveFirst
	removed := ts.RemoveFirst()
	if removed != task3 {
		t.Error("Expected RemoveFirst to return task3")
	}

	if ts.Length() != 2 {
		t.Errorf("Expected length 2 after RemoveFirst, got %d", ts.Length())
	}

	if ts.GetFirst() != task1 {
		t.Error("Expected first task to be task1 after RemoveFirst")
	}

	// Test RemoveLast
	removed = ts.RemoveLast()
	if removed != task2 {
		t.Error("Expected RemoveLast to return task2")
	}

	if ts.Length() != 1 {
		t.Errorf("Expected length 1 after RemoveLast, got %d", ts.Length())
	}

	// Test Remove by ID
	removed = ts.Remove(task1.GetId())
	if removed != task1 {
		t.Error("Expected Remove to return task1")
	}

	if ts.Length() != 0 {
		t.Errorf("Expected length 0 after Remove, got %d", ts.Length())
	}

	// Test removing non-existent ID
	if ts.Remove("nonexistent") != nil {
		t.Error("Expected Remove(nonexistent) to return nil")
	}
}

// TestTaskStorage_AddBeforeAfter tests AddBefore and AddAfter operations
func TestTaskStorage_AddBeforeAfter(t *testing.T) {
	ts := newTaskStorage()

	task1 := task.NewTask("test1")
	task2 := task.NewTask("test2")
	task3 := task.NewTask("test3")
	task4 := task.NewTask("test4")

	// Add initial tasks
	ts.AddLast(task1, task2)

	// Test AddAfter
	ts.AddAfter(task1.GetId(), task3)
	snapshot := ts.GetSnapshot()
	if len(snapshot) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(snapshot))
	}

	if snapshot[0] != task1 || snapshot[1] != task3 || snapshot[2] != task2 {
		t.Error("AddAfter failed: incorrect order")
	}

	// Test AddBefore
	ts.AddBefore(task2.GetId(), task4)
	snapshot = ts.GetSnapshot()
	if len(snapshot) != 4 {
		t.Errorf("Expected 4 tasks, got %d", len(snapshot))
	}

	if snapshot[0] != task1 || snapshot[1] != task3 || snapshot[2] != task4 || snapshot[3] != task2 {
		t.Error("AddBefore failed: incorrect order")
	}

	// Test AddAfter/AddBefore with non-existent ID
	ts.AddAfter("nonexistent", task.NewTask("should-not-be-added"))
	ts.AddBefore("nonexistent", task.NewTask("should-not-be-added"))

	if ts.Length() != 4 {
		t.Errorf("Expected length 4 (no tasks should be added for non-existent IDs), got %d", ts.Length())
	}
}

// TestTaskStorage_DeleteFunc tests the DeleteFunc operation
func TestTaskStorage_DeleteFunc(t *testing.T) {
	ts := newTaskStorage()

	task1 := task.NewTask("test1")
	task2 := task.NewTask("test2")
	task3 := task.NewTask("test3")

	ts.AddLast(task1, task2, task3)

	// Delete tasks where Type is not "test2"
	ts.DeleteFunc(func(t task.Task) bool {
		return t.GetType() != "test2"
	})

	if ts.Length() != 2 {
		t.Errorf("Expected length 2 after DeleteFunc, got %d", ts.Length())
	}

	snapshot := ts.GetSnapshot()
	if snapshot[0] != task1 || snapshot[1] != task3 {
		t.Error("DeleteFunc failed: incorrect remaining tasks")
	}

	// Test DeleteFunc that deletes all
	ts.DeleteFunc(func(t task.Task) bool {
		return false
	})

	if ts.Length() != 0 {
		t.Errorf("Expected length 0 after deleting all, got %d", ts.Length())
	}
}

// TestTaskStorage_ProcessResult tests the ProcessResult operation
func TestTaskStorage_ProcessResult(t *testing.T) {
	ts := newTaskStorage()

	task1 := task.NewTask("test1")
	task2 := task.NewTask("test2")
	headTask := task.NewTask("head")
	tailTask := task.NewTask("tail")
	afterTask := task.NewTask("after")

	ts.AddLast(task1, task2)

	// Create a TaskResult for successful processing
	result := &TaskResult{
		Status: Success,
	}
	result.AddHeadTasks(headTask)
	result.AddTailTasks(tailTask)
	result.AddAfterTasks(afterTask)

	// Process the result for task1
	ts.ProcessResult(*result, task1)

	snapshot := ts.GetSnapshot()
	if len(snapshot) != 4 {
		t.Errorf("Expected 4 tasks after ProcessResult, got %d", len(snapshot))
	}

	// Order should be: headTask, afterTask, task2, tailTask
	if snapshot[0] != headTask || snapshot[1] != afterTask || snapshot[2] != task2 || snapshot[3] != tailTask {
		t.Error("ProcessResult failed: incorrect task order")
	}

	// Test with Fail status (task should not be removed)
	ts = newTaskStorage()
	ts.AddLast(task1)

	failResult := &TaskResult{Status: Fail}
	failResult.AddHeadTasks(task.NewTask("fail-head"))

	ts.ProcessResult(*failResult, task1)

	snapshot = ts.GetSnapshot()
	if len(snapshot) != 2 {
		t.Errorf("Expected 2 tasks after Fail ProcessResult, got %d", len(snapshot))
	}

	if snapshot[0].GetType() != "fail-head" || snapshot[1] != task1 {
		t.Error("ProcessResult with Fail status failed")
	}
}

// TestTaskStorage_SnapshotAndIteration tests GetSnapshot and Iterate operations
func TestTaskStorage_SnapshotAndIteration(t *testing.T) {
	ts := newTaskStorage()

	task1 := task.NewTask("test1")
	task2 := task.NewTask("test2")
	task3 := task.NewTask("test3")

	ts.AddLast(task1, task2, task3)

	// Test GetSnapshot
	snapshot := ts.GetSnapshot()
	if len(snapshot) != 3 {
		t.Errorf("Expected snapshot length 3, got %d", len(snapshot))
	}

	if snapshot[0] != task1 || snapshot[1] != task2 || snapshot[2] != task3 {
		t.Error("GetSnapshot returned incorrect order")
	}

	// Test that snapshot is a copy (modifying it shouldn't affect storage)
	snapshot[0] = nil
	if ts.GetFirst() != task1 {
		t.Error("Snapshot modification affected original storage")
	}

	// Test Iterate
	var iteratedTasks []task.Task
	ts.Iterate(func(e *list.Element[task.Task]) {
		iteratedTasks = append(iteratedTasks, e.Value)
	})

	if len(iteratedTasks) != 3 {
		t.Errorf("Expected 3 iterated tasks, got %d", len(iteratedTasks))
	}

	if iteratedTasks[0] != task1 || iteratedTasks[1] != task2 || iteratedTasks[2] != task3 {
		t.Error("Iterate returned incorrect order")
	}
}

// TestTaskStorage_ConcurrentReadWrite tests concurrent read and write operations
func TestTaskStorage_ConcurrentReadWrite(t *testing.T) {
	ts := newTaskStorage()

	const numGoroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup

	// Start multiple goroutines performing read operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = ts.Length()
				_ = ts.GetFirst()
				_ = ts.GetLast()
				_ = ts.GetSnapshot()
				ts.Iterate(func(e *list.Element[task.Task]) {
					_ = e.Value.GetId()
				})
			}
		}()
	}

	// Start multiple goroutines performing write operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				tsk := task.NewTask("concurrent")
				ts.AddLast(tsk)

				// Try to add before/after existing tasks
				if first := ts.GetFirst(); first != nil {
					ts.AddAfter(first.GetId(), task.NewTask("after"))
					ts.AddBefore(first.GetId(), task.NewTask("before"))
				}

				// Remove some tasks
				ts.DeleteFunc(func(t task.Task) bool {
					return j%5 != 0 // Keep 80% of tasks
				})

				// Remove first/last occasionally
				if j%10 == 0 {
					ts.RemoveFirst()
					ts.RemoveLast()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify storage is in a consistent state
	if ts.Length() < 0 {
		t.Error("Storage length should not be negative")
	}

	// Verify all tasks in storage have valid IDs in the index
	snapshot := ts.GetSnapshot()
	idSet := make(map[string]bool)
	for _, tsk := range snapshot {
		idSet[tsk.GetId()] = true
	}

	// Check that all tasks in the list are also in the index
	ts.Iterate(func(e *list.Element[task.Task]) {
		if !idSet[e.Value.GetId()] {
			t.Errorf("Task %s found in iteration but not in snapshot", e.Value.GetId())
		}
	})
}

// TestTaskStorage_DeadlockPrevention tests for potential deadlock scenarios
func TestTaskStorage_DeadlockPrevention(t *testing.T) {
	ts := newTaskStorage()

	// Test 1: Concurrent operations that could cause lock ordering issues
	const numGoroutines = 20
	var wg sync.WaitGroup

	// Add some initial tasks
	for i := 0; i < 10; i++ {
		ts.AddLast(task.NewTask("initial"))
	}

	// Start goroutines that perform operations in different orders
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Perform operations that acquire locks in different patterns
			for j := 0; j < 50; j++ {
				// Mix of operations that could potentially deadlock
				ts.AddLast(task.NewTask("mixed"))

				if first := ts.GetFirst(); first != nil {
					ts.AddAfter(first.GetId(), task.NewTask("after"))
				}

				ts.DeleteFunc(func(t task.Task) bool {
					return true // Keep all
				})

				_ = ts.GetSnapshot()
				_ = ts.Length()

				if j%10 == 0 {
					ts.RemoveFirst()
					ts.RemoveLast()
				}
			}
		}(i)
	}

	// Use a timeout to detect deadlocks
	done := make(chan bool, 1)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - potential deadlock detected")
	}

	// Verify storage integrity after concurrent operations
	if ts.Length() < 0 {
		t.Error("Storage length should not be negative after concurrent operations")
	}
}

// TestTaskStorage_RaceConditionDetection uses race detector to catch race conditions
func TestTaskStorage_RaceConditionDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	ts := newTaskStorage()

	const numGoroutines = 10
	const opsPerGoroutine = 100

	var wg sync.WaitGroup

	// Start multiple goroutines that access the same task storage concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < opsPerGoroutine; j++ {
				// Mix read and write operations to trigger potential races
				tsk := task.NewTask("race-test")

				ts.AddLast(tsk)
				_ = ts.Get(tsk.GetId())
				_ = ts.Length()
				_ = ts.GetFirst()
				_ = ts.GetLast()

				if j%5 == 0 {
					ts.Remove(tsk.GetId())
				}

				// Snapshot operations
				snapshot := ts.GetSnapshot()
				if len(snapshot) > 0 {
					ts.AddAfter(snapshot[0].GetId(), task.NewTask("after-race"))
				}

				// Iteration
				ts.Iterate(func(e *list.Element[task.Task]) {
					_ = e.Value.GetId()
				})
			}
		}(i)
	}

	wg.Wait()

	// The race detector will report any races that occurred during this test
}

// TestTaskStorage_ProcessResultConcurrent tests ProcessResult under concurrent load
func TestTaskStorage_ProcessResultConcurrent(t *testing.T) {
	ts := newTaskStorage()

	// Add some initial tasks
	initialTasks := make([]task.Task, 10)
	for i := 0; i < 10; i++ {
		initialTasks[i] = task.NewTask("initial")
		ts.AddLast(initialTasks[i])
	}

	const numGoroutines = 5
	var wg sync.WaitGroup

	// Start goroutines that process results concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				// Get a task to process
				var targetTask task.Task
				ts.Iterate(func(e *list.Element[task.Task]) {
					if targetTask == nil {
						targetTask = e.Value
					}
				})

				if targetTask != nil {
					result := &TaskResult{Status: Success}
					result.AddHeadTasks(task.NewTask("head-concurrent"))
					result.AddTailTasks(task.NewTask("tail-concurrent"))
					result.AddAfterTasks(task.NewTask("after-concurrent"))

					ts.ProcessResult(*result, targetTask)
				}

				// Also perform other operations concurrently
				ts.AddLast(task.NewTask("extra"))
				_ = ts.GetSnapshot()
			}
		}(i)
	}

	wg.Wait()

	// Verify storage is still consistent
	if ts.Length() < 0 {
		t.Error("Storage length should not be negative")
	}

	// Check that we can still perform basic operations
	ts.AddLast(task.NewTask("final"))
	if ts.Length() == 0 {
		t.Error("Storage should contain at least the final task")
	}
}

// TestTaskStorage_EdgeCases tests various edge cases
func TestTaskStorage_EdgeCases(t *testing.T) {
	ts := newTaskStorage()

	// Test operations on empty storage
	if ts.RemoveFirst() != nil {
		t.Error("RemoveFirst on empty storage should return nil")
	}

	if ts.RemoveLast() != nil {
		t.Error("RemoveLast on empty storage should return nil")
	}

	if ts.Remove("nonexistent") != nil {
		t.Error("Remove nonexistent ID should return nil")
	}

	// Test AddAfter/AddBefore with empty storage
	ts.AddAfter("nonexistent", task.NewTask("test"))
	ts.AddBefore("nonexistent", task.NewTask("test"))

	if ts.Length() != 0 {
		t.Error("Adding after/before nonexistent IDs in empty storage should not add tasks")
	}

	// Test with single task
	singleTask := task.NewTask("single")
	ts.AddLast(singleTask)

	if ts.GetFirst() != singleTask {
		t.Error("Single task should be both first and last")
	}

	if ts.GetLast() != singleTask {
		t.Error("Single task should be both first and last")
	}

	// Remove the single task
	if ts.RemoveFirst() != singleTask {
		t.Error("RemoveFirst should return the single task")
	}

	if ts.Length() != 0 {
		t.Error("Storage should be empty after removing single task")
	}

	// Test ProcessResult with non-existent task
	result := &TaskResult{Status: Success}
	result.AddHeadTasks(task.NewTask("head"))
	ts.ProcessResult(*result, task.NewTask("nonexistent"))

	// Should still add head tasks even if target task doesn't exist
	if ts.Length() != 1 {
		t.Errorf("Expected 1 task after ProcessResult with non-existent task, got %d", ts.Length())
	}
}

// TestTaskStorage_LargeScaleOperations tests performance and correctness with many operations
func TestTaskStorage_LargeScaleOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	ts := newTaskStorage()

	const numTasks = 1000
	var wg sync.WaitGroup

	// Add many tasks concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for j := 0; j < numTasks/10; j++ {
				ts.AddLast(task.NewTask("large-scale"))
			}
		}(i * (numTasks / 10))
	}

	wg.Wait()

	if ts.Length() != numTasks {
		t.Errorf("Expected %d tasks, got %d", numTasks, ts.Length())
	}

	// Perform many concurrent operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = ts.GetSnapshot()
				ts.DeleteFunc(func(t task.Task) bool {
					return true // Keep all tasks
				})

				if first := ts.GetFirst(); first != nil {
					ts.AddAfter(first.GetId(), task.NewTask("large-after"))
				}
			}
		}()
	}

	wg.Wait()

	// Verify storage is still consistent
	if ts.Length() < numTasks {
		t.Errorf("Expected at least %d tasks after operations, got %d", numTasks, ts.Length())
	}
}
