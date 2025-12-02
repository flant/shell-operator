package queue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/task"
)

func Test_TaskQueueList_Requeue(t *testing.T) {
	g := NewWithT(t)

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		// Optional: only validate if it's the AddLast action
		if labels["queue_action"] == "AddLast" {
			assert.Equal(t, metric, metrics.TasksQueueActionDurationSeconds)
			assert.NotZero(t, value)
		}
	})
	metricStorage.GaugeSetMock.Optional().Set(func(_ string, _ float64, _ map[string]string) {
		// Optional: accept any gauge set calls
	})
	metricStorage.CounterAddMock.Optional().Set(func(_ string, _ float64, _ map[string]string) {
		// Optional: accept any counter add calls
	})

	// A channel to control when RequeueTask can finish.
	requeueTaskCanFinish := make(chan struct{})

	// A channel to signal that RequeueTask has finished.
	requeueTaskFinished := make(chan struct{})

	// Store execution order.
	executionOrder := make([]string, 0)
	mu := &sync.Mutex{}

	// Create a new task queue
	q := NewTasksQueue(
		"requeue-test-queue",
		metricStorage,
		WithContext(context.Background()),
	)

	q.WaitLoopCheckInterval = 5 * time.Millisecond
	q.DelayOnQueueIsEmpty = 5 * time.Millisecond
	q.DelayOnRepeat = 5 * time.Millisecond

	// Define the handler for tasks
	q.Handler = func(_ context.Context, tsk task.Task) TaskResult {
		mu.Lock()
		executionOrder = append(executionOrder, tsk.GetId())
		mu.Unlock()

		if tsk.GetId() == "RequeueTask" {
			// If there are other tasks in the queue, move this task to the end.
			if q.Length() > 1 {
				res := TaskResult{Status: Success}
				res.tailTasks = append(res.tailTasks, tsk)

				return res
			}

			// If no other tasks, wait for the signal to finish.
			<-requeueTaskCanFinish
			close(requeueTaskFinished)
			return TaskResult{Status: Success}
		}

		// For simple tasks, just succeed.
		return TaskResult{Status: Success}
	}

	// Add the "requeue" task first.
	requeueTask := task.BaseTask{Id: "RequeueTask", Type: task_metadata.HookRun}
	q.AddLast(&requeueTask)

	// Add a few simple tasks.
	for i := 0; i < 3; i++ {
		simpleTask := task.BaseTask{Id: fmt.Sprintf("SimpleTask-%d", i), Type: task_metadata.HookRun}
		q.AddLast(&simpleTask)
	}

	g.Expect(q.Length()).To(Equal(4))

	// Start processing the queue in a separate goroutine.
	go q.Start(context.Background())
	defer q.Stop()

	// Wait until all simple tasks are processed.
	g.Eventually(func() int {
		mu.Lock()
		defer mu.Unlock()
		count := 0
		for _, id := range executionOrder {
			if id != "RequeueTask" {
				count++
			}
		}
		return count
	}, "5s", "10ms").Should(Equal(3), "All simple tasks should run")

	// Verify the order of execution so far.
	// The RequeueTask should have been processed and moved to the back.
	mu.Lock()
	// The first task should be the RequeueTask.
	g.Expect(executionOrder[0]).To(Equal("RequeueTask"))
	// The next 3 tasks should be SimpleTasks
	g.Expect(executionOrder[1:4]).To(Equal([]string{"SimpleTask-0", "SimpleTask-1", "SimpleTask-2"}))
	mu.Unlock()

	// Allow the RequeueTask to finish.
	close(requeueTaskCanFinish)

	// Wait for the RequeueTask to finish.
	g.Eventually(requeueTaskFinished, "5s", "10ms").Should(BeClosed())

	// Check final execution order.
	mu.Lock()
	g.Expect(len(executionOrder)).To(BeNumerically(">=", 5))
	// Last executed task must be RequeueTask
	g.Expect(executionOrder[len(executionOrder)-1]).To(Equal("RequeueTask"))
	mu.Unlock()

	g.Expect(q.Length()).To(Equal(0))
}
