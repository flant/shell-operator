package queue

import (
	"context"
	"sync"

	"github.com/flant/shell-operator/pkg/task"
)

// TaskQueueSet is a manager for a set of named queues
type TaskQueueSet struct {
	Queues   map[string]*TaskQueue
	MainName string

	m      sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewTaskQueueSet() *TaskQueueSet {
	return &TaskQueueSet{
		Queues: make(map[string]*TaskQueue, 0),
		m:      sync.Mutex{},
	}
}

func (tqs *TaskQueueSet) WithMainName(name string) {
	tqs.MainName = name
}

func (tqs *TaskQueueSet) WithContext(ctx context.Context) {
	tqs.ctx, tqs.cancel = context.WithCancel(ctx)
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

	doFn(tqs.Queues[tqs.MainName])

	// TODO sort names

	for _, t := range tqs.Queues {
		if t.Name != tqs.MainName {
			doFn(t)
		}
	}
}
