package queue

import (
	"context"
	"testing"
	"time"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/flant/shell-operator/pkg/task"
)

func TestConcurrentOperations(t *testing.T) {
	queue := NewTasksQueue(
		"test",
		metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry()),
		WithContext(context.Background()),
	)

	newTask := task.NewTask("test")

	// 100 goroutines adding tasks
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.AddLast(newTask)
			}
		}()
	}

	// 100 goroutines adding tasks
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.AddBefore(newTask.Id, newTask)
			}
		}()
	}

	// 100 goroutines adding tasks
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.AddAfter(newTask.Id, newTask)
			}
		}()
	}

	// 100 goroutines adding tasks
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.DeleteFunc(func(t task.Task) bool {
					return t.GetId() == newTask.Id
				})
			}
		}()
	}

	// 100 goroutines reading
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.GetFirst()
			}
		}()
	}

	// 100 goroutines reading
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.Length()
			}
		}()
	}

	// 100 goroutines reading
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				queue.IterateSnapshot(func(t task.Task) {
					_ = t.GetId()
				})
			}
		}()
	}

	// 100 goroutines reading
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				_ = queue.String()
			}
		}()
	}

	time.Sleep(5 * time.Second)
}
