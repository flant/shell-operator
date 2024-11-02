package queue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/utils/exponential_backoff"
	"github.com/flant/shell-operator/pkg/utils/measure"
)

/*
A working queue (a pipeline) for sequential execution of tasks.

Tasks are added to the tail and executed from the head. Also a task can be pushed
to the head to implement a meta-tasks.

Each task is executed until success. This can be controlled with allowFailure: true
config parameter.

*/

var (
	DefaultWaitLoopCheckInterval    = 125 * time.Millisecond
	DefaultDelayOnQueueIsEmpty      = 250 * time.Millisecond
	DefaultInitialDelayOnFailedTask = 5 * time.Second
	DefaultDelayOnRepeat            = 25 * time.Millisecond
)

type TaskStatus string

const (
	Success TaskStatus = "Success"
	Fail    TaskStatus = "Fail"
	Repeat  TaskStatus = "Repeat"
	Keep    TaskStatus = "Keep"
)

type TaskResult struct {
	Status     TaskStatus
	HeadTasks  []task.Task
	TailTasks  []task.Task
	AfterTasks []task.Task

	DelayBeforeNextTask time.Duration

	AfterHandle func()
}

type TaskQueue struct {
	m             sync.RWMutex
	metricStorage *metric_storage.MetricStorage
	ctx           context.Context
	cancel        context.CancelFunc

	waitMu         sync.Mutex
	waitInProgress bool
	cancelDelay    bool

	items   []task.Task
	started bool // a flag to ignore multiple starts

	// Log debug messages if true.
	debug bool

	Name    string
	Handler func(task.Task) TaskResult
	Status  string

	measureActionFn     func()
	measureActionFnOnce sync.Once

	// Timing settings.
	WaitLoopCheckInterval time.Duration
	DelayOnQueueIsEmpty   time.Duration
	DelayOnRepeat         time.Duration
	ExponentialBackoffFn  func(failureCount int) time.Duration
}

func NewTasksQueue() *TaskQueue {
	return &TaskQueue{
		items: make([]task.Task, 0),
		// Default timings
		WaitLoopCheckInterval: DefaultWaitLoopCheckInterval,
		DelayOnQueueIsEmpty:   DefaultDelayOnQueueIsEmpty,
		DelayOnRepeat:         DefaultDelayOnRepeat,
		ExponentialBackoffFn: func(failureCount int) time.Duration {
			return exponential_backoff.CalculateDelay(DefaultInitialDelayOnFailedTask, failureCount)
		},
	}
}

func (q *TaskQueue) WithContext(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
}

func (q *TaskQueue) WithMetricStorage(mstor *metric_storage.MetricStorage) {
	q.metricStorage = mstor
}

func (q *TaskQueue) WithName(name string) *TaskQueue {
	q.Name = name
	return q
}

func (q *TaskQueue) WithHandler(fn func(task.Task) TaskResult) *TaskQueue {
	q.Handler = fn
	return q
}

// MeasureActionTime is a helper to measure execution time of queue's actions
func (q *TaskQueue) MeasureActionTime(action string) func() {
	q.measureActionFnOnce.Do(func() {
		if os.Getenv("QUEUE_ACTIONS_METRICS") == "no" {
			q.measureActionFn = func() {}
		} else {
			q.measureActionFn = measure.Duration(func(d time.Duration) {
				q.metricStorage.HistogramObserve("{PREFIX}tasks_queue_action_duration_seconds", d.Seconds(), map[string]string{"queue_name": q.Name, "queue_action": action}, nil)
			})
		}
	})
	return q.measureActionFn
}

func (q *TaskQueue) IsEmpty() bool {
	defer q.MeasureActionTime("IsEmpty")()
	q.m.RLock()
	defer q.m.RUnlock()
	return q.isEmpty()
}

func (q *TaskQueue) isEmpty() bool {
	return len(q.items) == 0
}

func (q *TaskQueue) Length() int {
	defer q.MeasureActionTime("Length")()
	q.m.RLock()
	defer q.m.RUnlock()
	return len(q.items)
}

// AddFirst adds new head element.
func (q *TaskQueue) AddFirst(t task.Task) {
	defer q.MeasureActionTime("AddFirst")()
	q.withLock(func() {
		q.addFirst(t)
	})
}

// addFirst adds new head element.
func (q *TaskQueue) addFirst(t task.Task) {
	q.items = append([]task.Task{t}, q.items...)
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueue) RemoveFirst() (t task.Task) {
	defer q.MeasureActionTime("RemoveFirst")()
	q.withLock(func() {
		t = q.removeFirst()
	})
	return t
}

// removeFirst deletes a head element, so head is moved.
func (q *TaskQueue) removeFirst() (t task.Task) {
	if q.isEmpty() {
		return
	}
	t = q.items[0]
	q.items = q.items[1:]
	return
}

// GetFirst returns a head element.
func (q *TaskQueue) GetFirst() task.Task {
	defer q.MeasureActionTime("GetFirst")()
	q.m.RLock()
	defer q.m.RUnlock()
	if q.isEmpty() {
		return nil
	}
	return q.items[0]
}

// AddLast adds new tail element.
func (q *TaskQueue) AddLast(t task.Task) {
	defer q.MeasureActionTime("AddLast")()
	q.withLock(func() {
		q.addLast(t)
	})
}

// addFirst adds new tail element.
func (q *TaskQueue) addLast(t task.Task) {
	q.items = append(q.items, t)
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueue) RemoveLast() (t task.Task) {
	defer q.MeasureActionTime("RemoveLast")()
	q.withLock(func() {
		t = q.removeLast()
	})
	return t
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueue) removeLast() (t task.Task) {
	if q.isEmpty() {
		return nil
	}
	t = q.items[len(q.items)-1]
	if len(q.items) == 1 {
		q.items = make([]task.Task, 0)
	} else {
		q.items = q.items[:len(q.items)-1]
	}
	return t
}

// GetLast returns a tail element.
func (q *TaskQueue) GetLast() (t task.Task) {
	defer q.MeasureActionTime("GetLast")()
	q.withRLock(func() {
		t = q.getLast()
	})
	return t
}

// GetLast returns a tail element.
func (q *TaskQueue) getLast() (t task.Task) {
	if q.isEmpty() {
		return nil
	}
	return q.items[len(q.items)-1]
}

// Get returns a task by id.
func (q *TaskQueue) Get(id string) (t task.Task) {
	defer q.MeasureActionTime("Get")()
	q.withRLock(func() {
		t = q.get(id)
	})
	return t
}

// Get returns a task by id.
func (q *TaskQueue) get(id string) (t task.Task) {
	for _, t := range q.items {
		if t.GetId() == id {
			return t
		}
	}
	return nil
}

// AddAfter inserts a task after the task with specified id.
func (q *TaskQueue) AddAfter(id string, newTask task.Task) {
	defer q.MeasureActionTime("AddAfter")()
	q.withLock(func() {
		q.addAfter(id, newTask)
	})
}

// addAfter inserts a task after the task with specified id.
func (q *TaskQueue) addAfter(id string, newTask task.Task) {
	newItems := make([]task.Task, len(q.items)+1)

	idFound := false
	for i, t := range q.items {
		if !idFound {
			// copy task while id not found
			newItems[i] = t
			if t.GetId() == id {
				idFound = true
				// when id is found, inject new task after task with equal id
				newItems[i+1] = newTask
			}
		} else {
			// when id is found, copy other tasks to i+1 position
			newItems[i+1] = t
		}
	}

	q.items = newItems
}

// AddBefore inserts a task before the task with specified id.
func (q *TaskQueue) AddBefore(id string, newTask task.Task) {
	defer q.MeasureActionTime("AddBefore")()
	q.withLock(func() {
		q.addBefore(id, newTask)
	})
}

// addBefore inserts a task before the task with specified id.
func (q *TaskQueue) addBefore(id string, newTask task.Task) {
	newItems := make([]task.Task, len(q.items)+1)

	idFound := false
	for i, t := range q.items {
		if !idFound {
			if t.GetId() != id {
				// copy task while id not found
				newItems[i] = t
			} else {
				idFound = true
				// when id is found, inject newTask to a current position
				// and copy current task to i+1 position
				newItems[i] = newTask
				newItems[i+1] = t
			}
		} else {
			// when id is found, copy other taskы to i+1 position
			newItems[i+1] = t
		}
	}

	q.items = newItems
}

// Remove finds element by id and deletes it.
func (q *TaskQueue) Remove(id string) (t task.Task) {
	defer q.MeasureActionTime("Remove")()

	q.withLock(func() {
		t = q.remove(id)
	})
	if t == nil {
		return
	}
	return t
}

func (q *TaskQueue) remove(id string) (t task.Task) {
	delId := -1
	for i, item := range q.items {
		if item.GetId() == id {
			delId = i
			break
		}
	}
	if delId == -1 {
		return nil
	}
	t = q.items[delId]
	q.items = append(q.items[:delId], q.items[delId+1:]...)
	return t
}

func (q *TaskQueue) SetDebug(debug bool) {
	q.debug = debug
}

func (q *TaskQueue) debugf(format string, args ...interface{}) {
	if !q.debug {
		return
	}
	log.Debugf(format, args...)
}

func (q *TaskQueue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
}

func (q *TaskQueue) Start() {
	if q.started {
		return
	}

	if q.Handler == nil {
		log.Errorf("queue %s: should set handler before start", q.Name)
		q.Status = "no handler set"
		return
	}

	go func() {
		q.Status = ""
		var sleepDelay time.Duration
		for {
			q.debugf("queue %s: wait for task, delay %d", q.Name, sleepDelay)
			t := q.waitForTask(sleepDelay)
			if t == nil {
				q.Status = "stop"
				log.Infof("queue '%s' stopped", q.Name)
				return
			}

			// dump task and a whole queue
			q.debugf("queue %s: tasks after wait %s", q.Name, q.String())
			q.debugf("queue %s: task to handle '%s'", q.Name, t.GetType())

			// Now the task can be handled!
			var nextSleepDelay time.Duration
			q.Status = "run first task"
			taskRes := q.Handler(t)

			// Check Done channel after long running operation.
			select {
			case <-q.ctx.Done():
				log.Infof("queue '%s' stopped after task handling", q.Name)
				q.Status = "stop"
				return
			default:
			}

			switch taskRes.Status {
			case Fail:
				// Exponential backoff delay before retry.
				nextSleepDelay = q.ExponentialBackoffFn(t.GetFailureCount())
				t.IncrementFailureCount()
				q.Status = fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String())
			case Success, Keep:
				// Insert new tasks right after the current task in reverse order.
				q.withLock(func() {
					for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
						q.addAfter(t.GetId(), taskRes.AfterTasks[i])
					}
					// Remove current task on success.
					if taskRes.Status == Success {
						q.remove(t.GetId())
					}
					// Also, add HeadTasks in reverse order
					// at the start of the queue. The first task in HeadTasks
					// become the new first task in the queue.
					for i := len(taskRes.HeadTasks) - 1; i >= 0; i-- {
						q.addFirst(taskRes.HeadTasks[i])
					}
					// Add tasks to the end of the queue
					for _, newTask := range taskRes.TailTasks {
						q.addLast(newTask)
					}
				})
				q.Status = ""
			case Repeat:
				// repeat a current task after a small delay
				nextSleepDelay = q.DelayOnRepeat
				q.Status = "repeat head task"
			}

			if taskRes.DelayBeforeNextTask != 0 {
				nextSleepDelay = taskRes.DelayBeforeNextTask
				q.Status = fmt.Sprintf("sleep for %s", nextSleepDelay.String())
			}

			sleepDelay = nextSleepDelay

			if taskRes.AfterHandle != nil {
				taskRes.AfterHandle()
			}

			q.debugf("queue %s: tasks after handle %s", q.Name, q.String())
		}
	}()
	q.started = true
}

// waitForTask returns a task that can be processed or a nil if context is canceled.
// sleepDelay is used to sleep before check a task, e.g. in case of failed previous task.
// If queue is empty, then it will be checked every DelayOnQueueIsEmpty.
func (q *TaskQueue) waitForTask(sleepDelay time.Duration) task.Task {
	// Check Done channel.
	select {
	case <-q.ctx.Done():
		return nil
	default:
	}

	// Shortcut: return the first task if the queue is not empty and delay is not required.
	if !q.IsEmpty() && sleepDelay == 0 {
		return q.GetFirst()
	}

	// Initialize wait settings.
	waitBegin := time.Now()
	waitUntil := q.DelayOnQueueIsEmpty
	if sleepDelay != 0 {
		waitUntil = sleepDelay
	}

	checkTicker := time.NewTicker(q.WaitLoopCheckInterval)
	q.waitMu.Lock()
	q.waitInProgress = true
	q.cancelDelay = false
	q.waitMu.Unlock()

	origStatus := q.Status

	defer func() {
		checkTicker.Stop()
		q.waitMu.Lock()
		q.waitInProgress = false
		q.cancelDelay = false
		q.waitMu.Unlock()
		q.Status = origStatus
	}()

	// Wait for the queued task with some delay.
	// Every tick increases the 'elapsed' counter until it outgrows the waitUntil value.
	// Or, delay can be canceled to handle new head task immediately.
	for {
		checkTask := false
		select {
		case <-q.ctx.Done():
			// Queue is stopped.
			return nil
		case <-checkTicker.C:
			// Check and update waitUntil.
			elapsed := time.Since(waitBegin)

			q.waitMu.Lock()
			if q.cancelDelay {
				// Reset waitUntil to check task immediately.
				waitUntil = elapsed
			}
			q.waitMu.Unlock()

			// Wait loop is done or canceled: break select to check for the head task.
			if elapsed >= waitUntil {
				// Increase waitUntil to wait on the next iteration and go check for the head task.
				checkTask = true
			}
		}

		// Break the for-loop to see if the head task can be returned.
		if checkTask {
			if q.IsEmpty() {
				// No task to return: increase wait time.
				waitUntil += q.DelayOnQueueIsEmpty
			} else {
				return q.GetFirst()
			}
		}

		// Wait loop still in progress: update queue status.
		waitSeconds := time.Since(waitBegin).Truncate(time.Second).String()
		if sleepDelay == 0 {
			q.Status = fmt.Sprintf("waiting for task %s", waitSeconds)
		} else {
			delay := sleepDelay.Truncate(time.Second).String()
			q.Status = fmt.Sprintf("%s (%s left of %s delay)", origStatus, waitSeconds, delay)
		}
	}
}

// CancelTaskDelay breaks wait loop. Useful to break the possible long sleep delay.
func (q *TaskQueue) CancelTaskDelay() {
	q.waitMu.Lock()
	if q.waitInProgress {
		q.cancelDelay = true
	}
	q.waitMu.Unlock()
}

// Iterate run doFn for every task.
func (q *TaskQueue) Iterate(doFn func(task.Task)) {
	if doFn == nil {
		return
	}

	defer q.MeasureActionTime("Iterate")()

	q.withRLock(func() {
		for _, t := range q.items {
			doFn(t)
		}
	})
}

// Filter run filterFn on every task and remove each with false result.
func (q *TaskQueue) Filter(filterFn func(task.Task) bool) {
	if filterFn == nil {
		return
	}

	defer q.MeasureActionTime("Filter")()

	q.withLock(func() {
		newItems := make([]task.Task, 0)
		for _, t := range q.items {
			if filterFn(t) {
				newItems = append(newItems, t)
			}
		}
		q.items = newItems
	})
}

// TODO define mapping method with QueueAction to insert, modify and delete tasks.

// Dump tasks in queue to one line
func (q *TaskQueue) String() string {
	var buf strings.Builder
	var index int
	qLen := q.Length()
	q.Iterate(func(t task.Task) {
		buf.WriteString(fmt.Sprintf("[%s,id=%10.10s]", t.GetDescription(), t.GetId()))
		index++
		if index == qLen {
			return
		}
		buf.WriteString(", ")
	})

	return buf.String()
}

func (q *TaskQueue) withLock(fn func()) {
	q.m.Lock()
	fn()
	q.m.Unlock()
}

func (q *TaskQueue) withRLock(fn func()) {
	q.m.RLock()
	fn()
	q.m.RUnlock()
}
