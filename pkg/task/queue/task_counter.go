package queue

import (
	"sync"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/task"
)

const (
	taskCap                    = 100
	compactionMetricsThreshold = 20
)

type TaskCounter struct {
	mu sync.RWMutex

	queueName      string
	counter        map[string]uint
	reachedCap     map[string]struct{}
	hookCounter    map[string]uint // tracks tasks by hook name
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
		hookCounter:    make(map[string]uint, 32),
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

// UpdateHookMetricsFromSnapshot updates metrics for all hooks based on a snapshot of hook counts.
// Only hooks with task count above the threshold are published to avoid metric cardinality explosion.
func (tc *TaskCounter) UpdateHookMetricsFromSnapshot(hookCounts map[string]uint) {
	if tc.metricStorage == nil {
		return
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Clear tracking for hooks that are no longer above threshold
	for hookName := range tc.hookCounter {
		if count, exists := hookCounts[hookName]; !exists || count <= compactionMetricsThreshold {
			// Hook dropped below threshold or disappeared - stop tracking it
			delete(tc.hookCounter, hookName)
		}
	}

	// Update metrics only for hooks above threshold
	for hookName, count := range hookCounts {
		if count > compactionMetricsThreshold {
			// Track and publish metric
			tc.hookCounter[hookName] = count

			labels := map[string]string{
				"queue_name": tc.queueName,
				"hook":       hookName,
			}
			tc.metricStorage.GaugeSet(metrics.TasksQueueCompactionTasksByHook, float64(count), labels)
		}
	}
}
