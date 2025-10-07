package queue

import (
	"context"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/task"
)

const MainQueueName = "main"

// queueStorage is a thread-safe storage for task queues with basic Get/Set/Delete operations
type queueStorage struct {
	mu     sync.RWMutex
	queues map[string]*TaskQueue
}

func newQueueStorage() *queueStorage {
	return &queueStorage{
		queues: make(map[string]*TaskQueue),
	}
}

// Get retrieves a queue by name, returns nil if not found
func (qs *queueStorage) Get(name string) (*TaskQueue, bool) {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	queue, exists := qs.queues[name]
	return queue, exists
}

// List retrieves all queues
func (qs *queueStorage) List() []*TaskQueue {
	qs.mu.RLock()
	defer qs.mu.RUnlock()

	queues := make([]*TaskQueue, 0, len(qs.queues))
	for _, queue := range qs.queues {
		queues = append(queues, queue)
	}

	return queues
}

// Set stores a queue with the given name
func (qs *queueStorage) Set(name string, queue *TaskQueue) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.queues[name] = queue
}

// Delete removes a queue by name
func (qs *queueStorage) Delete(name string) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	delete(qs.queues, name)
}

// Len returns the number of tasks in a queue by name
func (qs *queueStorage) Len() int {
	return len(qs.queues)
}

// TaskQueueSet is a manager for a set of named queues
type TaskQueueSet struct {
	MainName string

	metricStorage metric.Storage

	ctx    context.Context
	cancel context.CancelFunc

	m      sync.RWMutex
	Queues *queueStorage
}

func NewTaskQueueSet() *TaskQueueSet {
	return &TaskQueueSet{
		Queues:   newQueueStorage(),
		MainName: MainQueueName,
	}
}

func (tqs *TaskQueueSet) WithMainName(name string) {
	tqs.MainName = name
}

func (tqs *TaskQueueSet) WithContext(ctx context.Context) {
	tqs.ctx, tqs.cancel = context.WithCancel(ctx)
}

func (tqs *TaskQueueSet) WithMetricStorage(mstor metric.Storage) *TaskQueueSet {
	tqs.metricStorage = mstor

	return tqs
}

func (tqs *TaskQueueSet) Stop() {
	tqs.m.RLock()
	if tqs.cancel != nil {
		tqs.cancel()
	}

	tqs.m.RUnlock()
}

func (tqs *TaskQueueSet) StartMain(ctx context.Context) {
	tqs.GetByName(tqs.MainName).Start(ctx)
}

func (tqs *TaskQueueSet) Start(ctx context.Context) {
	tqs.Iterate(ctx, func(ctx context.Context, queue *TaskQueue) {
		queue.Start(ctx)
	})
}

// Add register a new queue for TaskQueueSet.
func (tqs *TaskQueueSet) Add(queue *TaskQueue) {
	tqs.Queues.Set(queue.Name, queue)
}

func (tqs *TaskQueueSet) NewNamedQueue(name string, handler func(ctx context.Context, t task.Task) TaskResult, opts ...TaskQueueOption) {
	q := NewTasksQueue(
		tqs.metricStorage,
		WithName(name),
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

func (tqs *TaskQueueSet) DoWithLock(fn func(tqs *TaskQueueSet)) {
	tqs.m.Lock()
	defer tqs.m.Unlock()

	if fn != nil {
		fn(tqs)
	}
}

// Iterate run doFn for every task.
func (tqs *TaskQueueSet) Iterate(ctx context.Context, doFn func(ctx context.Context, queue *TaskQueue)) {
	if doFn == nil {
		return
	}

	tqs.m.RLock()
	defer tqs.m.RUnlock()
	if tqs.Queues.Len() == 0 {
		return
	}

	main := tqs.GetMain()
	if main != nil {
		doFn(ctx, main)
	}
	// TODO sort names

	for _, q := range tqs.Queues.List() {
		if q.Name != tqs.MainName {
			doFn(ctx, q)
		}
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
				tqs.m.RLock()
				defer tqs.m.RUnlock()

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
