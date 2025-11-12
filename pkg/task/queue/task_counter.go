package queue

import (
	"sync"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/task"
)

const taskCap = 100

type TaskCounter struct {
	mu sync.RWMutex

	queueName      string
	counter        map[string]uint
	reachedCap     map[string]struct{}
	metricStorage  metricsstorage.Storage
	countableTypes map[task.TaskType]struct{}
}

func NewTaskCounter(name string, countableTypes map[task.TaskType]struct{}, metricStorage metricsstorage.Storage) *TaskCounter {
	if metricStorage == nil {
		panic("metricStorage cannot be nil")
	}

	return &TaskCounter{
		queueName:      name,
		counter:        make(map[string]uint, 32),
		reachedCap:     make(map[string]struct{}, 32),
		metricStorage:  metricStorage,
		countableTypes: countableTypes,
	}
}

func (tc *TaskCounter) Add(task task.Task) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if len(tc.countableTypes) > 0 {
		if _, ok := tc.countableTypes[task.GetType()]; !ok {
			return
		}
	}

	id := task.GetCompactionID()

	counter, ok := tc.counter[id]
	if !ok {
		tc.counter[id] = 0
	}

	counter++

	tc.counter[id] = counter

	if counter == taskCap {
		tc.reachedCap[id] = struct{}{}
	}
}

func (tc *TaskCounter) Remove(task task.Task) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if len(tc.countableTypes) > 0 {
		if _, ok := tc.countableTypes[task.GetType()]; !ok {
			return
		}
	}

	id := task.GetCompactionID()

	counter, ok := tc.counter[id]
	if !ok {
		if _, reached := tc.reachedCap[id]; reached {
			delete(tc.reachedCap, id)
		}
		return
	}

	if counter == 0 {
		delete(tc.counter, id)
		if _, reached := tc.reachedCap[id]; reached {
			delete(tc.reachedCap, id)
		}
		return
	}

	counter--

	if counter == 0 {
		delete(tc.counter, id)
	} else {
		tc.counter[id] = counter
	}

	if counter < taskCap {
		if _, reached := tc.reachedCap[id]; reached {
			delete(tc.reachedCap, id)
		}
	}
}

func (tc *TaskCounter) GetReachedCap() map[string]struct{} {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	return tc.reachedCap
}

func (tc *TaskCounter) IsAnyCapReached() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	return len(tc.reachedCap) > 0
}

func (tc *TaskCounter) ResetReachedCap() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.reachedCap = make(map[string]struct{}, 32)
}
