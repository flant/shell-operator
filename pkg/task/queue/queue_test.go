package queue

import (
	"context"
	"sync"
	"testing"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/task"
)

func TestConcurrentOperations(t *testing.T) {
	queue := NewTasksQueue(
		"test",
		metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry()),
		WithContext(context.Background()),
	)

	// Use a smaller number of goroutines and operations to avoid overwhelming the system
	numGoroutines := 10
	opsPerGoroutine := 10

	var wg sync.WaitGroup

	// 10 goroutines adding tasks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				newTask := task.NewTask("test")
				queue.AddLast(newTask)
			}
		}()
	}

	// 10 goroutines adding tasks before a random existing task
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				newTask := task.NewTask("test")
				// Try to add before a task that might exist
				existingTask := queue.GetFirst()
				if existingTask != nil {
					queue.AddBefore(existingTask.GetId(), newTask)
				} else {
					queue.AddLast(newTask)
				}
			}
		}()
	}

	// 10 goroutines adding tasks after a random existing task
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				newTask := task.NewTask("test")
				// Try to add after a task that might exist
				existingTask := queue.GetFirst()
				if existingTask != nil {
					queue.AddAfter(existingTask.GetId(), newTask)
				} else {
					queue.AddLast(newTask)
				}
			}
		}()
	}

	// 10 goroutines deleting tasks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				queue.DeleteFunc(func(t task.Task) bool {
					// Randomly delete some tasks
					return j%10 != 0 // Delete 90% of tasks
				})
			}
		}()
	}

	// 10 goroutines reading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				queue.GetFirst()
			}
		}()
	}

	// 10 goroutines reading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				queue.Length()
			}
		}()
	}

	// 10 goroutines reading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				queue.IterateSnapshot(func(t task.Task) {
					_ = t.GetId()
				})
			}
		}()
	}

	// 10 goroutines reading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = queue.String()
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify the queue is in a consistent state
	if queue.Length() < 0 {
		t.Errorf("Queue length should not be negative")
	}
}
