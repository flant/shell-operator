package queue

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

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
	DelayOnQueueIsEmpty = 250 * time.Millisecond
	DelayOnFailedTask   = 5 * time.Second
	DelayOnRepeat       = 25 * time.Millisecond
)

type TaskResult struct {
	Status     string
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

	items   []task.Task
	started bool // a flag to ignore multiple starts

	Name     string
	Handler  func(task.Task) TaskResult
	HeadLock sync.Mutex
	Status   string

	measureActionFn     func()
	measureActionFnOnce sync.Once
	addHandler          func(task.Task)
	removeHandler       func(task.Task)
}

func NewTasksQueue() *TaskQueue {
	return &TaskQueue{
		m:             sync.RWMutex{},
		items:         make([]task.Task, 0),
		HeadLock:      sync.Mutex{},
		addHandler:    func(_ task.Task) {},
		removeHandler: func(_ task.Task) {},
	}
}

func (q *TaskQueue) WithContext(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
}

func (q *TaskQueue) WithMetricStorage(mstor *metric_storage.MetricStorage) {
	q.metricStorage = mstor
}

func (tq *TaskQueue) WithName(name string) *TaskQueue {
	tq.Name = name
	return tq
}

func (tq *TaskQueue) WithHandler(fn func(task.Task) TaskResult) *TaskQueue {
	tq.Handler = fn
	return tq
}

func (tq *TaskQueue) WithAddHandler(fn func(task.Task)) *TaskQueue {
	tq.addHandler = fn
	return tq
}

func (tq *TaskQueue) WithRemoveHandler(fn func(task.Task)) *TaskQueue {
	tq.removeHandler = fn
	return tq
}

// MeasureActionTime is a helper to measure execution time of queue's actions
func (q *TaskQueue) MeasureActionTime(action string) func() {
	q.measureActionFnOnce.Do(func() {
		if os.Getenv("QUEUE_ACTIONS_METRICS") == "no" {
			q.measureActionFn = func() {}
		} else {
			q.measureActionFn = measure.Duration(func(d time.Duration) {
				q.metricStorage.HistogramObserve("{PREFIX}tasks_queue_action_duration_seconds", d.Seconds(), map[string]string{"queue_name": q.Name, "queue_action": action})
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
	q.addHandler(t)
	q.m.Lock()
	q.items = append([]task.Task{t}, q.items...)
	q.m.Unlock()
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueue) RemoveFirst() (t task.Task) {
	defer q.MeasureActionTime("RemoveFirst")()
	q.m.Lock()
	if q.isEmpty() {
		q.m.Unlock()
		return
	}
	t = q.items[0]
	q.items = q.items[1:]
	q.m.Unlock()
	q.removeHandler(t)
	return t
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

// AddFirst adds new tail element.
func (q *TaskQueue) AddLast(t task.Task) {
	defer q.MeasureActionTime("AddLast")()
	q.addHandler(t)
	q.m.Lock()
	q.items = append(q.items, t)
	q.m.Unlock()
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueue) RemoveLast() (t task.Task) {
	defer q.MeasureActionTime("RemoveLast")()
	q.m.Lock()
	if q.isEmpty() {
		q.m.Unlock()
		return
	}
	t = q.items[len(q.items)-1]
	if len(q.items) == 1 {
		q.items = make([]task.Task, 0)
	} else {
		q.items = q.items[:len(q.items)-1]
	}
	q.m.Unlock()
	q.removeHandler(t)
	return t
}

// GetLast returns a tail element.
func (q *TaskQueue) GetLast() task.Task {
	defer q.MeasureActionTime("GetLast")()
	q.m.RLock()
	defer q.m.RUnlock()
	if q.isEmpty() {
		return nil
	}
	return q.items[len(q.items)-1]
}

// Get returns a task by id.
func (q *TaskQueue) Get(id string) task.Task {
	defer q.MeasureActionTime("Get")()
	q.m.RLock()
	defer q.m.RUnlock()
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
	q.addHandler(newTask)

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
	q.addHandler(newTask)

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

	q.m.Lock()
	defer q.m.Unlock()
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
	q.removeHandler(t)
	return t
}

func (q *TaskQueue) DoWithHeadLock(fn func(tasksQueue *TaskQueue)) {
	defer q.MeasureActionTime("DoWithHeadLock")()
	q.HeadLock.Lock()
	defer q.HeadLock.Unlock()
	if fn != nil {
		fn(q)
	}
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
	go func() {
		q.Status = ""
		var sleepDelay time.Duration
		for {
			log.Debugf("queue %s: wait for task, delay %d", q.Name, sleepDelay)
			var t = q.waitForTask(sleepDelay)
			if t == nil {
				q.Status = "stop"
				log.Infof("queue '%s' stopped", q.Name)
				return
			}

			// dump task and a whole queue
			log.Debugf("queue %s: get task %s", q.Name, t.GetType())
			log.Debugf("queue %s: tasks after wait %s", q.Name, q.String())

			// Now the task can be handled!
			if q.Handler == nil {
				continue
			}
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
			case "Fail":
				// Exponential backoff delay before retry.
				nextSleepDelay = exponential_backoff.CalculateDelay(DelayOnFailedTask, t.GetFailureCount())
				t.IncrementFailureCount()
				q.Status = fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String())
			case "Success":
				// Insert new tasks right after the current task in reverse order.
				for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
					q.AddAfter(t.GetId(), taskRes.AfterTasks[i])
				}
				q.DoWithHeadLock(func(q *TaskQueue) {
					// Current head task is handled, remove it.
					q.Remove(t.GetId())
					// Also, add HeadTasks in reverse order
					// at the start of the queue. The first task in HeadTasks
					// become the new first task in the queue.
					for i := len(taskRes.HeadTasks) - 1; i >= 0; i-- {
						q.AddFirst(taskRes.HeadTasks[i])
					}
				})
				// Add tasks to the end of the queue
				for _, newTask := range taskRes.TailTasks {
					q.AddLast(newTask)
				}
				q.Status = ""
			case "Repeat":
				// repeat a current task after a small delay
				nextSleepDelay = DelayOnRepeat
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

			// dump queue
			log.Debugf("queue %s: tasks after handle %s", q.Name, q.String())
		}
	}()
	q.started = true
}

// waitForTask returns a task that can be processed or a nil if context is canceled.
// sleepDelay is used to sleep before check a task, e.g. in case of failed previous task.
// If queue is empty, than it will be checked every DelayOnQueueIsEmpty.
func (q *TaskQueue) waitForTask(sleepDelay time.Duration) task.Task {
	// Check Done channel to be able to stop queue
	select {
	case <-q.ctx.Done():
		return nil
	default:
	}

	// Wait for non empty queue or closed Done channel
	origStatus := q.Status
	waitBegin := time.Now()
	for {
		// Skip this loop if sleep is not needed and there is a task to process.
		if !q.IsEmpty() && sleepDelay == 0 {
			return q.GetFirst()
			//break
		}
		//log.WithField("operator.component", "taskRunner").
		//	Debug("Task queue is empty. Will sleep now.")
		// Sleep for sleepDelay in case of failure and then sleep for DelayOnQueueIsEmpty until queue is empty.
		newDelay := DelayOnQueueIsEmpty
		if sleepDelay != 0 {
			newDelay = sleepDelay
		}
		delayTicker := time.NewTicker(newDelay)
		secondTicker := time.NewTicker(time.Second)
		var stop = false
		for {
			select {
			case <-delayTicker.C:
				// reset sleepDelay
				sleepDelay = 0
				stop = true
			case <-q.ctx.Done():
				return nil
			case <-secondTicker.C:
				waitSeconds := time.Since(waitBegin).Truncate(time.Second).String()
				if sleepDelay == 0 {
					q.Status = fmt.Sprintf("waiting for task %s", waitSeconds)
				} else {
					q.Status = fmt.Sprintf("%s (elapsed %s)", origStatus, waitSeconds)
				}
			}
			if stop {
				break
			}
		}
		delayTicker.Stop()
		secondTicker.Stop()
	}
}

// Iterate run doFn for every task.
func (q *TaskQueue) Iterate(doFn func(task.Task)) {
	if doFn == nil {
		return
	}

	defer q.MeasureActionTime("Iterate")()

	q.m.RLock()
	defer q.m.RUnlock()
	for _, t := range q.items {
		doFn(t)
	}
}

// Filter run filterFn on every task and remove each with false result.
func (q *TaskQueue) Filter(filterFn func(task.Task) bool) {
	if filterFn == nil {
		return
	}

	defer q.MeasureActionTime("Filter")()

	q.m.Lock()
	defer q.m.Unlock()
	var newItems = make([]task.Task, 0)
	for _, t := range q.items {
		if filterFn(t) {
			newItems = append(newItems, t)
		}
	}
	q.items = newItems
}

// TODO define mapping method with QueueAction to insert, modify and delete tasks.

// Dump tasks in queue to one line
func (q *TaskQueue) String() string {
	var buf strings.Builder
	var index int
	var qLen = q.Length()
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
