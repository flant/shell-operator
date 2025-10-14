package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/task"
)

const MainQueueName = "main"

// TaskQueueSet is a manager for a set of named queues
type TaskQueueSet struct {
	MainName string

	metricStorage metricsstorage.Storage

	ctx    context.Context
	cancel context.CancelFunc

	Queues *queueStorage

	logger *log.Logger
}

func NewTaskQueueSet() *TaskQueueSet {
	return &TaskQueueSet{
		Queues:   newQueueStorage(),
		MainName: MainQueueName,
		logger:   log.NewLogger().Named("task_queue_set"),
	}
}

func (tqs *TaskQueueSet) WithMainName(name string) {
	tqs.MainName = name
	tqs.logger = tqs.logger.Named(name)
}

func (tqs *TaskQueueSet) WithContext(ctx context.Context) {
	tqs.ctx, tqs.cancel = context.WithCancel(ctx)
}

func (tqs *TaskQueueSet) WithMetricStorage(mstor metricsstorage.Storage) *TaskQueueSet {
	tqs.metricStorage = mstor

	return tqs
}

func (tqs *TaskQueueSet) WithLogger(logger *log.Logger) *TaskQueueSet {
	tqs.logger = logger

	return tqs
}

func (tqs *TaskQueueSet) Stop() {
	if tqs.cancel != nil {
		tqs.cancel()
	}
}

func (tqs *TaskQueueSet) StartMain(ctx context.Context) {
	tqs.GetByName(tqs.MainName).Start(ctx)
}

func (tqs *TaskQueueSet) Start(ctx context.Context) {
	tqs.IterateSnapshot(ctx, func(ctx context.Context, queue *TaskQueue) {
		defer func() {
			if r := recover(); r != nil {
				tqs.logger.Warn("panic recovered in Start", slog.Any("error", r))
			}
		}()

		queue.Start(ctx)
	})
}

// Add register a new queue for TaskQueueSet.
func (tqs *TaskQueueSet) Add(queue *TaskQueue) {
	tqs.Queues.Set(queue.Name, queue)
}

func (tqs *TaskQueueSet) NewNamedQueue(name string, handler func(ctx context.Context, t task.Task) TaskResult, opts ...TaskQueueOption) {
	q := NewTasksQueue(
		name,
		tqs.metricStorage,
		WithHandler(handler),
		WithContext(tqs.ctx),
	)

	for _, opt := range opts {
		opt(q)
	}

	if q.logger == nil {
		q.logger = log.NewLogger().Named("task_queue")
	}

	tqs.Queues.Set(q.Name, q)
}

func (tqs *TaskQueueSet) GetByName(name string) *TaskQueue {
	q, ok := tqs.Queues.Get(name)
	if !ok {
		return nil
	}

	return q
}

func (tqs *TaskQueueSet) GetMain() *TaskQueue {
	return tqs.GetByName(tqs.MainName)
}

func (tqs *TaskQueueSet) AddTailTasks(tasks ...task.Task) {
	tqs.logger.Debug("AddTailTasks: adding tasks to queues", slog.Int("tasksCount", len(tasks)))

	for _, resTask := range tasks {
		q, ok := tqs.Queues.Get(resTask.GetQueueName())
		if ok {
			tqs.logger.Debug("AddTailTasks: adding task to queue",
				slog.String("queueName", resTask.GetQueueName()),
				slog.String("description", resTask.GetDescription()))
			q.AddLast(resTask)

			continue
		}

		log.Error("Possible bug!!! Got task for queue but queue is not created yet.",
			slog.String("queueName", resTask.GetQueueName()),
			slog.String("description", resTask.GetDescription()))
	}

	tqs.logger.Debug("AddTailTasks: adding tasks to queues done")
}

// GetSnapshot returns a snapshot of all queues at the time of the call.
// This is useful for iteration when you need to call methods on the queues
// that might acquire locks, preventing deadlocks.
//
// The returned slice is a snapshot and will not reflect subsequent changes.
// The main queue (tqs.MainName) is always placed first in the list.
func (tqs *TaskQueueSet) GetSnapshot() []*TaskQueue {
	tqs.logger.Debug("GetSnapshot: creating snapshot of queues")

	allQueues := tqs.Queues.List()
	queues := make([]*TaskQueue, 0, len(allQueues))

	// First, add the main queue if it exists
	if mainQueue := tqs.GetMain(); mainQueue != nil {
		queues = append(queues, mainQueue)
	}

	// Then add all other queues
	for _, queue := range allQueues {
		if queue.Name != tqs.MainName {
			queues = append(queues, queue)
		}
	}

	tqs.logger.Debug("GetSnapshot: creating snapshot of queues done")

	return queues
}

// IterateSnapshot creates a snapshot of all queues and iterates over the copy.
// This is safer than Iterate() when you need to call queue methods inside the callback,
// as no locks are held during callback execution.
//
// Note: The snapshot may become stale during iteration if queues are added/removed
// by other goroutines or by the callback itself.
//
// Use this method when:
//   - You need to call queue methods inside the callback (Start, Stop, etc.)
//   - You need to process queues asynchronously
//   - Safety is more important than performance
//
// Memory overhead: O(n) where n is the number of queues.
func (tqs *TaskQueueSet) IterateSnapshot(ctx context.Context, doFn func(ctx context.Context, queue *TaskQueue)) {
	if doFn == nil {
		return
	}

	// Create snapshot under lock (main queue is already first)
	tqs.logger.Warn("IterateSnapshot: creating snapshot of queues")
	snapshot := tqs.GetSnapshot()
	tqs.logger.Warn("IterateSnapshot: creating snapshot of queues done")

	// Execute callbacks without holding any locks
	defer func() {
		if r := recover(); r != nil {
			tqs.logger.Warn("panic recovered in IterateSnapshot", slog.Any("error", r))
		}
	}()

	tqs.logger.Warn("Start iterating snapshot of queues")
	for _, q := range snapshot {
		doFn(ctx, q)
	}
}

func (tqs *TaskQueueSet) Remove(name string) {
	ts, exists := tqs.Queues.Get(name)
	if exists {
		ts.Stop()
	}

	tqs.Queues.Delete(name)
}

func (tqs *TaskQueueSet) WaitStopWithTimeout(timeout time.Duration) {
	checkTick := time.NewTicker(time.Millisecond * 100)
	defer checkTick.Stop()

	timeoutTick := time.NewTicker(timeout)
	defer timeoutTick.Stop()

	for {
		select {
		case <-checkTick.C:
			stopped := func() bool {
				for _, q := range tqs.Queues.List() {
					if q.GetStatusType() != QueueStatusStop {
						return false
					}
				}

				return true
			}()

			if stopped {
				return
			}

		case <-timeoutTick.C:
			return
		}
	}
}
