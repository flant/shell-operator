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

// TaskQueueSet is a manager for a set of named queues
type TaskQueueSet struct {
	MainName string

	metricStorage metric.Storage

	ctx    context.Context
	cancel context.CancelFunc

	m      sync.RWMutex
	Queues map[string]*TaskQueue
}

func NewTaskQueueSet() *TaskQueueSet {
	return &TaskQueueSet{
		Queues:   make(map[string]*TaskQueue),
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
	tqs.m.RLock()
	for _, q := range tqs.Queues {
		q.Start(ctx)
	}

	tqs.m.RUnlock()
}

func (tqs *TaskQueueSet) Add(queue *TaskQueue) {
	tqs.m.Lock()
	tqs.Queues[queue.Name] = queue
	tqs.m.Unlock()
}

type QueueOption func(*TaskQueue)

func WithCompactableTypes(taskTypes ...task.TaskType) QueueOption {
	return func(q *TaskQueue) {
		q.compactableTypes = make(map[task.TaskType]struct{}, len(taskTypes))
		for _, taskType := range taskTypes {
			q.compactableTypes[taskType] = struct{}{}
		}
	}
}

func WithCompactionCallback(callback func(compactedTasks []task.Task, targetTask task.Task)) QueueOption {
	return func(q *TaskQueue) {
		q.compactionCallback = callback
	}
}

func WithLogger(logger *log.Logger) QueueOption {
	return func(q *TaskQueue) {
		q.logger = logger
	}
}

func (tqs *TaskQueueSet) NewNamedQueue(name string, handler func(ctx context.Context, t task.Task) TaskResult, opts ...QueueOption) {
	q := NewTasksQueue()
	q.WithName(name)
	q.WithHandler(handler)
	q.WithContext(tqs.ctx)
	q.WithMetricStorage(tqs.metricStorage)

	for _, opt := range opts {
		opt(q)
	}

	if q.logger == nil {
		q.logger = log.NewLogger().Named("task_queue")
	}

	tqs.m.Lock()
	tqs.Queues[name] = q
	tqs.m.Unlock()
}

func (tqs *TaskQueueSet) GetByName(name string) *TaskQueue {
	tqs.m.RLock()
	defer tqs.m.RUnlock()
	ts, exists := tqs.Queues[name]
	if exists {
		return ts
	}
	return nil
}

func (tqs *TaskQueueSet) GetMain() *TaskQueue {
	return tqs.GetByName(tqs.MainName)
}

/*
	taskQueueSet.DoWithLock(func(tqs *TaskQueueSet){
	   tqs.GetMain().Pop()
	})
*/
func (tqs *TaskQueueSet) DoWithLock(fn func(tqs *TaskQueueSet)) {
	tqs.m.Lock()
	defer tqs.m.Unlock()
	if fn != nil {
		fn(tqs)
	}
}

// Iterate run doFn for every task.
func (tqs *TaskQueueSet) Iterate(doFn func(queue *TaskQueue)) {
	if doFn == nil {
		return
	}

	tqs.m.RLock()
	defer tqs.m.RUnlock()
	if len(tqs.Queues) == 0 {
		return
	}

	main := tqs.GetMain()
	if main != nil {
		doFn(main)
	}
	// TODO sort names

	for _, q := range tqs.Queues {
		if q.Name != tqs.MainName {
			doFn(q)
		}
	}
}

func (tqs *TaskQueueSet) Remove(name string) {
	tqs.m.Lock()
	ts, exists := tqs.Queues[name]
	if exists {
		ts.Stop()
	}

	delete(tqs.Queues, name)
	tqs.m.Unlock()
}

func (tqs *TaskQueueSet) WaitStopWithTimeout(timeout time.Duration) {
	checkTick := time.NewTicker(time.Millisecond * 100)
	defer checkTick.Stop()
	timeoutTick := time.NewTicker(timeout)
	defer timeoutTick.Stop()

	for {
		select {
		case <-checkTick.C:
			stopped := true
			tqs.m.RLock()
			for _, q := range tqs.Queues {
				if q.Status != "stop" {
					stopped = false
					break
				}
			}
			tqs.m.RUnlock()
			if stopped {
				return
			}

		case <-timeoutTick.C:
			return
		}
	}
}
