package queue

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/task"
)

func DumpTaskIds(q *TaskQueue) string {
	var buf bytes.Buffer
	var index int
	q.Iterate(func(t task.Task) {
		buf.WriteString(fmt.Sprintf("%d: %s\n", index, t.GetId()))
		index++
	})
	return buf.String()
}

func Test_TasksQueue_Remove(t *testing.T) {
	g := NewWithT(t)

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		assert.Equal(t, metric, "{PREFIX}tasks_queue_action_duration_seconds")
		assert.NotZero(t, value)
		assert.Equal(t, map[string]string{
			"queue_action": "AddFirst",
			"queue_name":   "",
		}, labels)
		assert.Nil(t, buckets)
	})

	q := NewTasksQueue().WithMetricStorage(metricStorage)

	// Remove just one element
	Task := &task.BaseTask{Id: "First one"}
	q.AddFirst(Task)
	g.Expect(q.Length()).To(Equal(1))
	q.Remove("First one")
	g.Expect(q.Length()).To(Equal(0))

	// Remove element in the middle
	for i := 0; i < 5; i++ {
		Task := &task.BaseTask{Id: fmt.Sprintf("task_%02d", i)}
		q.AddFirst(Task)
	}
	g.Expect(q.Length()).To(Equal(5))
	q.Remove("task_02")
	g.Expect(q.Length()).To(Equal(4))

	idsDump := DumpTaskIds(q)

	g.Expect(idsDump).To(And(
		ContainSubstring("task_00"),
		ContainSubstring("task_01"),
		ContainSubstring("task_03"),
		ContainSubstring("task_04"),
	))

	// Remove last element
	q.Remove("task_04")
	g.Expect(q.Length()).To(Equal(3))

	idsDump = DumpTaskIds(q)

	g.Expect(idsDump).To(And(
		ContainSubstring("task_00"),
		ContainSubstring("task_01"),
		ContainSubstring("task_03"),
	))

	// Remove first element by id
	q.Remove("task_00")
	g.Expect(q.Length()).To(Equal(2))

	idsDump = DumpTaskIds(q)

	g.Expect(idsDump).To(And(
		ContainSubstring("task_01"),
		ContainSubstring("task_03"),
	))
}

func Test_ExponentialBackoff(t *testing.T) {
	g := NewWithT(t)

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		assert.Equal(t, metric, "{PREFIX}tasks_queue_action_duration_seconds")
		assert.NotZero(t, value)
		assert.Equal(t, map[string]string{
			"queue_action": "AddFirst",
			"queue_name":   "test-queue",
		}, labels)
		assert.Nil(t, buckets)
	})

	// Init and prefill queue.
	q := NewTasksQueue().WithMetricStorage(metricStorage)
	q.WithContext(context.TODO())
	q.WithName("test-queue")
	// Since we don't want the test to run for too long, we don't
	// want to use lengthy times.
	q.WaitLoopCheckInterval = 5 * time.Millisecond // default is 125ms
	q.DelayOnQueueIsEmpty = 5 * time.Millisecond   // default is 250ms
	q.DelayOnRepeat = 5 * time.Millisecond         // default is 25ms
	// Add one task.
	Task := &task.BaseTask{Id: "First one"}
	q.AddFirst(Task)

	// Set handler to fail 10 times and catch timestamps for each task execution.
	runsAt := make([]time.Time, 0)
	failureCounts := make([]int, 0)
	const fails = 10
	failsCount := fails
	queueStopCh := make(chan struct{}, 1)
	q.WithHandler(func(_ context.Context, t task.Task) TaskResult {
		var res TaskResult
		runsAt = append(runsAt, time.Now())
		failureCounts = append(failureCounts, t.GetFailureCount())
		if failsCount > 0 {
			res.Status = Fail
			failsCount--

			return res
		}

		res.Status = Success
		res.AfterHandle = func() {
			close(queueStopCh)
		}

		return res
	})

	// Set exponential backoff to the constant delay just to wait more than DelayOnQueueIsEmpty.
	// It is a test of delaying between task runs, not a test of exponential distribution.
	mockExponentialDelay := 30 * time.Millisecond
	q.ExponentialBackoffFn = func(_ int) time.Duration {
		return mockExponentialDelay
	}

	q.Start(context.TODO())

	// Expect taskHandler returns Success result.
	g.Eventually(queueStopCh, "5s", "20ms").Should(BeClosed(), "Should handle first task in queue successfully")

	// Expect taskHandler called 'fails' times.
	g.Expect(Task.GetFailureCount()).Should(Equal(fails), "task should fail %d times.", fails)

	prev := failureCounts[0]
	for i := 1; i < len(failureCounts); i++ {
		cur := failureCounts[i]
		g.Expect(cur > prev).Should(BeTrue(), "taskHandler should receive task with growing FailureCount. Got %d after %d", cur, prev)
	}

	// Expect mean delay is greater than mocked delay.
	mean, _ := calculateMeanDelay(runsAt)
	g.Expect(mean).Should(BeNumerically(">", mockExponentialDelay),
		"mean delay of %d fails should be more than %s, got %s. Check exponential delaying not broken in Start or waitForTask.",
		fails, mockExponentialDelay.String(), mean.Truncate(100*time.Microsecond).String())
}

func calculateMeanDelay(in []time.Time) (time.Duration, []int64) {
	var sum int64

	// Calculate deltas from timestamps.
	prev := in[0].UnixNano()
	deltas := make([]int64, 0, len(in)-1)
	for i := 1; i < len(in); i++ {
		delta := in[i].UnixNano() - prev
		prev = in[i].UnixNano()
		deltas = append(deltas, delta)
		sum += delta
	}
	mean := time.Duration(sum / int64(len(deltas)))

	return mean, deltas
}

func Test_CancelDelay(t *testing.T) {
	g := NewWithT(t)

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		assert.Equal(t, metric, "{PREFIX}tasks_queue_action_duration_seconds")
		assert.NotZero(t, value)
		assert.Equal(t, map[string]string{
			"queue_action": "AddFirst",
			"queue_name":   "test-queue",
		}, labels)
		assert.Nil(t, buckets)
	})

	// Init and prefill queue.
	q := NewTasksQueue().WithMetricStorage(metricStorage)
	q.WithContext(context.TODO())
	q.WithName("test-queue")
	// Since we don't want the test to run for too long, we don't
	// want to use lengthy times.
	q.WaitLoopCheckInterval = 5 * time.Millisecond // default is 125ms
	q.DelayOnQueueIsEmpty = 5 * time.Millisecond   // default is 250ms
	q.DelayOnRepeat = 5 * time.Millisecond         // default is 25ms
	// Add 'always fail' task.
	ErrTask := &task.BaseTask{Id: "erroneous"}
	q.AddFirst(ErrTask)

	HealingTask := &task.BaseTask{Id: "healing"}

	// Set handler to always return fail for task "erroneous", and return success for other tasks.
	// Catch time between "erroneous" task handling and "healing" task handling.
	startedAt := time.Now()
	endedAt := startedAt
	delayStartsCh := make(chan struct{}, 1)
	healingDoneCh := make(chan struct{}, 1)
	q.WithHandler(func(_ context.Context, t task.Task) TaskResult {
		var res TaskResult
		if t.GetId() == ErrTask.GetId() {
			res.Status = Fail
			// Close chan after first delay.
			if t.GetFailureCount() == 1 {
				res.AfterHandle = func() {
					close(delayStartsCh)
				}
			}

			return res
		}

		if t.GetId() == HealingTask.GetId() {
			endedAt = time.Now()
			res.AfterHandle = func() {
				close(healingDoneCh)
			}
		}

		res.Status = Success

		return res
	})

	// Set exponential backoff to the constant delay just to wait more than DelayOnQueueIsEmpty.
	// It is a test of delaying between task runs, not a test of exponential distribution.
	mockExponentialDelay := 150 * time.Millisecond
	q.ExponentialBackoffFn = func(_ int) time.Duration {
		return mockExponentialDelay
	}

	// Start handling 'erroneous' task.
	q.Start(context.TODO())

	// Expect taskHandler returns Success result.
	g.Eventually(delayStartsCh, "5s", "20ms").Should(BeClosed(), "Should handle failed task and starts a delay")

	// Add healing task and cancel delay in parallel.
	go func() {
		q.AddFirst(HealingTask)
		q.CancelTaskDelay()
	}()

	// Expect queue handles 'healing' task.
	g.Eventually(healingDoneCh, "5s", "20ms").Should(BeClosed(), "Should handle 'healing' task")

	elapsed := endedAt.Sub(startedAt).Truncate(100 * time.Microsecond)

	// Expect elapsed is less than mocked delay.
	g.Expect(elapsed).Should(BeNumerically(">", mockExponentialDelay),
		"Should delay after failed task. Got delay of %s, expect more than %s. Check delay for failed task not broken in Start or waitForTask.",
		elapsed.String(), mockExponentialDelay.String())
	g.Expect(elapsed).Should(BeNumerically("<", 2*mockExponentialDelay),
		"Should stop delaying after CancelTaskDelay call. Got delay of %s, expect less than %s. Check cancel delay not broken in Start or waitForTask.",
		elapsed.String(), (2 * mockExponentialDelay).String())
}
