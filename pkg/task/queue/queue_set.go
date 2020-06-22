package queue

import (
	"context"
	"sync"
	"time"

	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/flant/shell-operator/pkg/task"
)

// TaskQueueSet is a manager for a set of named queues
type TaskQueueSet struct {
	Queues   map[string]*TaskQueue
	MainName string

	metricStorage *metric_storage.MetricStorage

	m      sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewTaskQueueSet() *TaskQueueSet {
	return &TaskQueueSet{
		Queues: make(map[string]*TaskQueue),
		m:      sync.Mutex{},
	}
}

func (tqs *TaskQueueSet) WithMainName(name string) {
	tqs.MainName = name
}

func (tqs *TaskQueueSet) WithContext(ctx context.Context) {
	tqs.ctx, tqs.cancel = context.WithCancel(ctx)
}

func (tqs *TaskQueueSet) WithMetricStorage(mstor *metric_storage.MetricStorage) {
	tqs.metricStorage = mstor
}

func (tqs *TaskQueueSet) Stop() {
	if tqs.cancel != nil {
		tqs.cancel()
	}
}

func (tqs *TaskQueueSet) StartMain() {
	tqs.GetByName(tqs.MainName).Start()
}

func (tqs *TaskQueueSet) Start() {
	for _, q := range tqs.Queues {
		q.Start()
	}
}

func (tqs *TaskQueueSet) Add(queue *TaskQueue) {
	tqs.Queues[queue.Name] = queue
}

func (tqs *TaskQueueSet) NewNamedQueue(name string, handler func(task.Task) TaskResult) {
	q := NewTasksQueue()
	q.WithName(name)
	q.WithHandler(handler)
	q.WithContext(tqs.ctx)
	q.WithMetricStorage(tqs.metricStorage)
	tqs.Queues[name] = q
}

func (tqs *TaskQueueSet) GetByName(name string) *TaskQueue {
	ts, exists := tqs.Queues[name]
	if exists {
		return ts
	}
	return nil
}

func (tqs *TaskQueueSet) GetMain() *TaskQueue {
	return tqs.GetByName(tqs.MainName)
}

/**
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
	tqs.m.Lock()
	defer tqs.m.Unlock()

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
	ts, exists := tqs.Queues[name]
	if exists {
		ts.Stop()
	}
	tqs.m.Lock()
	defer tqs.m.Unlock()
	delete(tqs.Queues, name)
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
			for _, q := range tqs.Queues {
				if q.Status != "stop" {
					stopped = false
					break
				}
			}
			if stopped {
				return
			}
		case <-timeoutTick.C:
			return
		}
	}
}
