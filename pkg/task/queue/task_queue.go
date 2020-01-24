package queue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/task"
)

/*
A working queue (a pipeline) for sequential execution of tasks.

Tasks are added to the tail and executed from the head. Also a task can be pushed
to the head to implement a meta-tasks.

Each task is executed until success. This can be controlled with allowFailure: true
config parameter.

*/

var (
	DelayOnQueueIsEmpty = 3 * time.Second
	DelayOnFailedTask   = 5 * time.Second
)

type QueueWatcher interface {
	QueueChangeCallback()
}

type TaskResult struct {
	Status     string
	HeadTasks  []task.Task
	TailTasks  []task.Task
	AfterTasks []task.Task

	DelayBeforeNextTask time.Duration
}

type TaskQueue struct {
	m              sync.Mutex
	items          []task.Task
	changesEnabled bool           // turns on and off callbacks execution
	changesCount   int            // counts changes when callbacks turned off
	queueWatchers  []QueueWatcher // callbacks to be executed on items change
	ctx            context.Context
	cancel         context.CancelFunc
	started        bool // indicator to ignore multiple starts

	Name     string
	Handler  func(task.Task) TaskResult
	HeadLock sync.Mutex
}

func NewTasksQueue() *TaskQueue {
	return &TaskQueue{
		m:              sync.Mutex{},
		items:          make([]task.Task, 0),
		changesCount:   0,
		changesEnabled: false,
		queueWatchers:  make([]QueueWatcher, 0),

		HeadLock: sync.Mutex{},
	}
}

func (q *TaskQueue) IsEmpty() bool {
	q.m.Lock()
	defer q.m.Unlock()
	return q.isEmpty()
}

func (q *TaskQueue) isEmpty() bool {
	return len(q.items) == 0
}

func (q *TaskQueue) Length() int {
	q.m.Lock()
	defer q.m.Unlock()
	return len(q.items)
}

// AddFirst adds new head element.
func (q *TaskQueue) AddFirst(t task.Task) {
	q.m.Lock()
	q.items = append([]task.Task{t}, q.items...)
	q.m.Unlock()
	q.queueChanged()
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueue) RemoveFirst() (t task.Task) {
	q.m.Lock()
	if q.isEmpty() {
		q.m.Unlock()
		return
	}
	t = q.items[0]
	q.items = q.items[1:]
	q.m.Unlock()
	q.queueChanged()
	return t
}

// GetFirst returns a head element.
func (q *TaskQueue) GetFirst() task.Task {
	q.m.Lock()
	defer q.m.Unlock()
	if q.isEmpty() {
		return nil
	}
	return q.items[0]
}

// AddFirst adds new tail element.
func (q *TaskQueue) AddLast(task task.Task) {
	q.m.Lock()
	q.items = append(q.items, task)
	q.m.Unlock()
	q.queueChanged()
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueue) RemoveLast() (t task.Task) {
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
	q.queueChanged()
	return t
}

// GetLast returns a tail element.
func (q *TaskQueue) GetLast() task.Task {
	q.m.Lock()
	defer q.m.Unlock()
	if q.isEmpty() {
		return nil
	}
	return q.items[len(q.items)-1]
}

// Get returns a task by id.
func (q *TaskQueue) Get(id string) task.Task {
	q.m.Lock()
	defer q.m.Unlock()
	for _, t := range q.items {
		if t.GetId() == id {
			return t
		}
	}
	return nil
}

//
func (q *TaskQueue) AddAfter(id string, newTask task.Task) {
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
	return
}

//
func (q *TaskQueue) AddBefore(id string, newTask task.Task) {
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
	return
}

//func (q *TaskQueue) AddBefore(id string, newTask task.Task) {
//	return
//}

// Remove finds element by id and deletes it.
func (q *TaskQueue) Remove(id string) (t task.Task) {
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
	return t
}

func (tq *TaskQueue) WithName(name string) *TaskQueue {
	tq.Name = name
	return tq
}

func (tq *TaskQueue) WithHandler(fn func(task.Task) TaskResult) *TaskQueue {
	tq.Handler = fn
	return tq
}

func (tq *TaskQueue) DoWithHeadLock(fn func(tasksQueue *TaskQueue)) {
	tq.HeadLock.Lock()
	defer tq.HeadLock.Unlock()
	if fn != nil {
		fn(tq)
	}
}

func (q *TaskQueue) WithContext(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
}

func (q *TaskQueue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
}

func (q *TaskQueue) Start() {
	go func() {
		var sleepDelay time.Duration
		for {
			log.Debugf("queue %s: wait for task, delay %d", q.Name, sleepDelay)
			var t = q.waitForTask(sleepDelay)
			if t == nil {
				log.Debugf("queue %s: got nil task, stop queue", q.Name)
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
			taskRes := q.Handler(t)
			switch taskRes.Status {
			case "Fail":
				t.IncrementFailureCount()
				// delay before retry
				nextSleepDelay = DelayOnFailedTask
			case "Success":
				// add tasks after current task in reverse order
				for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
					q.AddAfter(t.GetId(), taskRes.AfterTasks[i])
				}
				// Add tasks to the end of the queue
				for _, newTask := range taskRes.TailTasks {
					q.AddLast(newTask)
				}
				// Remove current task and add tasks to the head
				q.DoWithHeadLock(func(q *TaskQueue) {
					q.Remove(t.GetId())
					for _, newTask := range taskRes.HeadTasks {
						q.AddFirst(newTask)
					}
				})
			}
			if taskRes.DelayBeforeNextTask != 0 {
				nextSleepDelay = taskRes.DelayBeforeNextTask
			}

			sleepDelay = nextSleepDelay

			// dump queue
			log.Debugf("queue %s: tasks after handle %s", q.Name, q.String())
		}
	}()
}

// waitForTask returns a task that can be processed or a nil if context is canceled.
// sleepDelay is used to sleep between tasks, e.g. in case of failed task.
// If queue is empty, than it will be checked every DelayOnQueueIsEmpty.
func (q *TaskQueue) waitForTask(sleepDelay time.Duration) task.Task {
	// Wait for non empty queue or closed Done channel
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
		select {
		case <-q.ctx.Done():
			return nil
		case <-delayTicker.C:
			delayTicker.Stop()
			// reset sleepDelay
			sleepDelay = 0
			break
		}
	}
}

// Iterate run doFn for every task.
func (q *TaskQueue) Iterate(doFn func(task.Task)) {
	if doFn == nil {
		return
	}
	q.m.Lock()
	defer q.m.Unlock()
	for _, t := range q.items {
		doFn(t)
	}
}

// Filter run filterFn on every task and remove each with false result.
func (q *TaskQueue) Filter(filterFn func(task.Task) bool) {
	if filterFn == nil {
		return
	}
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

// Watcher functions

// AddWatcher adds queue watcher.
func (q *TaskQueue) AddWatcher(queueWatcher QueueWatcher) {
	q.queueWatchers = append(q.queueWatchers, queueWatcher)
}

// queueChanged must be called every time the queue is changed.
func (q *TaskQueue) queueChanged() {
	// TODO
	// if q.changesSuspended { return }

	if len(q.queueWatchers) == 0 {
		return
	}

	if q.changesEnabled {
		for _, watcher := range q.queueWatchers {
			watcher.QueueChangeCallback()
		}
	} else {
		q.changesCount++
	}
	// TODO
	// Send changes signal asynchronously.
	// go func(){
	// for _, watcher := range q.watchers {
	//   watcher.Ch() <- true
	// }
	// }()
}

// TODO
//func (q *TaskQueue) DoWithSuspendedWatchers(action func (Task)) {
//
//
//}

// Включить вызов QueueChangeCallback при каждом изменении
// В паре с ChangesDisabled могут быть использованы, чтобы
// производить массовые изменения. Если runCallbackOnPreviousChanges true,
// то будет вызвана QueueChangeCallback
func (q *TaskQueue) ChangesEnable(runCallbackOnPreviousChanges bool) {
	q.changesEnabled = true
	if runCallbackOnPreviousChanges && q.changesCount > 0 {
		q.changesCount = 0
		q.queueChanged()
	}
}

func (q *TaskQueue) ChangesDisable() {
	q.changesEnabled = false
	q.changesCount = 0
}

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
