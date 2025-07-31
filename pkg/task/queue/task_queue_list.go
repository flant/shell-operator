package queue

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/metric"
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

This implementation uses container/list for O(1) queue operations and a map for O(1) task lookup by ID.
*/

type TaskQueue struct {
	m             sync.RWMutex
	metricStorage metric.Storage
	ctx           context.Context
	cancel        context.CancelFunc

	waitMu         sync.Mutex
	waitInProgress bool
	cancelDelay    bool

	items   *list.List
	idIndex map[string]*list.Element

	started bool // a flag to ignore multiple starts

	// Log debug messages if true.
	debug bool

	Name    string
	Handler func(ctx context.Context, t task.Task) TaskResult
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
		items:   list.New(),
		idIndex: make(map[string]*list.Element),
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

func (q *TaskQueue) WithMetricStorage(mstor metric.Storage) *TaskQueue {
	q.metricStorage = mstor

	return q
}

func (q *TaskQueue) WithName(name string) *TaskQueue {
	q.Name = name
	return q
}

func (q *TaskQueue) WithHandler(fn func(ctx context.Context, t task.Task) TaskResult) *TaskQueue {
	q.Handler = fn
	return q
}

// MeasureActionTime is a helper to measure execution time of queue's actions
func (q *TaskQueue) MeasureActionTime(action string) func() {
	if q.metricStorage == nil {
		return func() {}
	}

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

func (q *TaskQueue) GetStatus() string {
	defer q.MeasureActionTime("GetStatus")()
	q.m.RLock()
	defer q.m.RUnlock()
	return q.Status
}

func (q *TaskQueue) SetStatus(status string) {
	q.m.Lock()
	q.Status = status
	q.m.Unlock()
}

func (q *TaskQueue) IsEmpty() bool {
	defer q.MeasureActionTime("IsEmpty")()
	q.m.RLock()
	defer q.m.RUnlock()
	return q.isEmpty()
}

func (q *TaskQueue) isEmpty() bool {
	return q.items.Len() == 0
}

func (q *TaskQueue) Length() int {
	defer q.MeasureActionTime("Length")()
	q.m.RLock()
	defer q.m.RUnlock()
	return q.items.Len()
}

// AddFirst adds new head element.
func (q *TaskQueue) AddFirst(t task.Task) {
	fmt.Printf("[TRACE-QUEUE] ADDING FIRST: Adding task %s of type %s to queue '%s'\n", t.GetId(), t.GetType(), q.Name)
	defer q.MeasureActionTime("AddFirst")()
	q.withLock(func() {
		q.addFirst(t)
	})
}

// addFirst adds new head element.
func (q *TaskQueue) addFirst(t task.Task) {
	element := q.items.PushFront(t)
	q.idIndex[t.GetId()] = element
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueue) RemoveFirst() task.Task {
	defer q.MeasureActionTime("RemoveFirst")()
	var t task.Task

	q.withLock(func() {
		t = q.removeFirst()
	})

	return t
}

// removeFirst deletes a head element, so head is moved.
func (q *TaskQueue) removeFirst() task.Task {
	if q.isEmpty() {
		return nil
	}

	element := q.items.Front()
	t := q.items.Remove(element).(task.Task)
	delete(q.idIndex, t.GetId())
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
	return q.items.Front().Value.(task.Task)
}

// AddLast adds new tail element.
func (q *TaskQueue) AddLast(t task.Task) {
	fmt.Printf("[TRACE-QUEUE] ADDING LAST: Adding task %s of type %s to queue '%s'\n", t.GetId(), t.GetType(), q.Name)
	defer q.MeasureActionTime("AddLast")()
	q.withLock(func() {
		q.addLast(t)
	})
}

// addLast adds a new tail element.
// It implements the merging logic for HookRun tasks by scanning the whole queue.
func (q *TaskQueue) addLast(t task.Task) {
	// Global compaction strategy:
	// When any task is added, perform a full queue compaction for ALL hooks.
	// This ensures the queue is always in the most compact state possible.

	if t.GetType() != task_metadata.HookRun {
		fmt.Printf("[TRACE-QUEUE] Added non-mergeable task %s of type %s to queue '%s'\n", t.GetId(), t.GetType(), q.Name)
		element := q.items.PushBack(t)
		q.idIndex[t.GetId()] = element
		return
	}

	// Add the new task temporarily to perform global compaction
	element := q.items.PushBack(t)
	q.idIndex[t.GetId()] = element

	// Perform global compaction
	// q.performGlobalCompaction()
}

// performGlobalCompaction merges HookRun tasks for the same hook.
// It iterates through the list once, making it an O(N) operation.
func (q *TaskQueue) performGlobalCompaction() {
	if q.items.Len() < 2 {
		return
	}

	type compactionGroup struct {
		targetElement   *list.Element
		elementsToMerge []*list.Element
		totalContexts   int
		totalMonitorIDs int
	}

	hookGroups := make(map[string]*compactionGroup)
	var compactedCount int

	// First pass: identify groups and calculate sizes
	for e := q.items.Front(); e != nil; e = e.Next() {
		t := e.Value.(task.Task)
		if t.GetType() != task_metadata.HookRun {
			continue
		}

		hm := task_metadata.HookMetadataAccessor(t)
		if isNil(hm) || t.IsProcessing() {
			continue
		}
		hookName := hm.HookName

		if group, exists := hookGroups[hookName]; exists {
			// Add to existing group
			group.elementsToMerge = append(group.elementsToMerge, e)
			group.totalContexts += len(hm.BindingContext)
			group.totalMonitorIDs += len(hm.MonitorIDs)
		} else {
			// Create a new group
			hookGroups[hookName] = &compactionGroup{
				targetElement:   e,
				elementsToMerge: []*list.Element{},
				totalContexts:   len(hm.BindingContext),
				totalMonitorIDs: len(hm.MonitorIDs),
			}
		}
	}

	// Second pass: merge with pre-allocated slices
	for hookName, group := range hookGroups {
		if len(group.elementsToMerge) == 0 {
			continue
		}

		targetTask := group.targetElement.Value.(task.Task)
		targetHm := task_metadata.HookMetadataAccessor(targetTask)

		// Pre-allocate new slices with the final calculated size
		newContexts := make([]bindingcontext.BindingContext, 0, group.totalContexts)
		newMonitorIDs := make([]string, 0, group.totalMonitorIDs)

		// Add target's contexts first
		newContexts = append(newContexts, targetHm.BindingContext...)
		newMonitorIDs = append(newMonitorIDs, targetHm.MonitorIDs...)

		// Append contexts from other tasks and remove them
		for _, elementToMerge := range group.elementsToMerge {
			taskToMerge := elementToMerge.Value.(task.Task)
			hmToMerge := task_metadata.HookMetadataAccessor(taskToMerge)

			newContexts = append(newContexts, hmToMerge.BindingContext...)
			newMonitorIDs = append(newMonitorIDs, hmToMerge.MonitorIDs...)

			fmt.Printf("[TRACE-QUEUE] Compacting task %s for hook '%s' into task %s\n",
				taskToMerge.GetId(), hookName, targetTask.GetId())

			q.items.Remove(elementToMerge)
			delete(q.idIndex, taskToMerge.GetId())
			compactedCount++
		}

		// Update target task with new, perfectly sized slices
		targetHm.BindingContext = newContexts
		targetHm.MonitorIDs = newMonitorIDs
		targetTask.UpdateMetadata(targetHm)
	}

	if compactedCount > 0 {
		fmt.Printf("[TRACE-QUEUE] Global compaction complete: merged %d tasks\n", compactedCount)
	}
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueue) RemoveLast() task.Task {
	defer q.MeasureActionTime("RemoveLast")()
	var t task.Task

	q.withLock(func() {
		t = q.removeLast()
	})

	return t
}

// removeLast deletes a tail element, so tail is moved.
func (q *TaskQueue) removeLast() task.Task {
	if q.isEmpty() {
		return nil
	}

	element := q.items.Back()
	t := q.items.Remove(element).(task.Task)
	delete(q.idIndex, t.GetId())

	return t
}

// GetLast returns a tail element.
func (q *TaskQueue) GetLast() task.Task {
	defer q.MeasureActionTime("GetLast")()
	var t task.Task

	q.withRLock(func() {
		t = q.getLast()
	})

	return t
}

// getLast returns a tail element.
func (q *TaskQueue) getLast() task.Task {
	if q.isEmpty() {
		return nil
	}

	return q.items.Back().Value.(task.Task)
}

// Get returns a task by id.
func (q *TaskQueue) Get(id string) task.Task {
	defer q.MeasureActionTime("Get")()
	var t task.Task

	q.withRLock(func() {
		t = q.get(id)
	})

	return t
}

// get returns a task by id.
func (q *TaskQueue) get(id string) task.Task {
	if element, ok := q.idIndex[id]; ok {
		return element.Value.(task.Task)
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
	if element, ok := q.idIndex[id]; ok {
		newElement := q.items.InsertAfter(newTask, element)
		q.idIndex[newTask.GetId()] = newElement
	}
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
	if element, ok := q.idIndex[id]; ok {
		newElement := q.items.InsertBefore(newTask, element)
		q.idIndex[newTask.GetId()] = newElement
	}
}

// Remove finds element by id and deletes it.
func (q *TaskQueue) Remove(id string) task.Task {
	defer q.MeasureActionTime("Remove")()
	var t task.Task

	q.withLock(func() {
		t = q.remove(id)
	})

	return t
}

func (q *TaskQueue) remove(id string) task.Task {
	if element, ok := q.idIndex[id]; ok {
		t := q.items.Remove(element).(task.Task)
		delete(q.idIndex, id)
		return t
	}
	return nil
}

func (q *TaskQueue) SetDebug(debug bool) {
	q.debug = debug
}

func (q *TaskQueue) debugf(format string, args ...interface{}) {
	if !q.debug {
		return
	}
	log.Debug("DEBUG", fmt.Sprintf(format, args...))
}

func (q *TaskQueue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
}

func (q *TaskQueue) Start(ctx context.Context) {
	if q.started {
		return
	}

	if q.Handler == nil {
		log.Error("should set handler before start in queue", slog.String("name", q.Name))
		q.SetStatus("no handler set")
		return
	}

	go func() {
		q.SetStatus("")
		var sleepDelay time.Duration
		for {
			q.debugf("queue %s: wait for task, delay %d", q.Name, sleepDelay)
			t := q.waitForTask(sleepDelay)
			if t == nil {
				q.SetStatus("stop")
				log.Info("queue stopped", slog.String("name", q.Name))
				return
			}

			fmt.Printf("[TRACE-QUEUE] Starting task %s of type %s, queue length %d, queue name %s\n", t.GetId(), t.GetType(), q.Length(), q.Name)

			// set that current task is being processed, so we don't merge it with other tasks
			t.SetProcessing(true)

			// dump task and a whole queue
			q.debugf("queue %s: tasks after wait %s", q.Name, q.String())
			q.debugf("queue %s: task to handle '%s'", q.Name, t.GetType())

			var nextSleepDelay time.Duration
			q.SetStatus("run first task")
			taskRes := q.Handler(ctx, t)

			// Check Done channel after long-running operation.
			select {
			case <-q.ctx.Done():
				log.Info("queue stopped after task handling", slog.String("name", q.Name))
				q.SetStatus("stop")
				return
			default:
			}

			switch taskRes.Status {
			case Fail:
				t.SetProcessing(false)
				// Exponential backoff delay before retry.
				nextSleepDelay = q.ExponentialBackoffFn(t.GetFailureCount())
				t.IncrementFailureCount()
				q.SetStatus(fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String()))
			case Success, Keep:
				// Insert new tasks right after the current task in reverse order.
				q.withLock(func() {
					if taskRes.Status == Success {
						// Remove current task on success.
						// Note: it is safe to use t.GetId() because we have a lock.
						// The task t is the one we just processed.
						q.remove(t.GetId())
					}
					t.SetProcessing(false) // release processing flag

					for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
						q.addAfter(t.GetId(), taskRes.AfterTasks[i])
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
				q.SetStatus("")
			case Repeat:
				// repeat a current task after a small delay
				nextSleepDelay = q.DelayOnRepeat
				t.SetProcessing(false)
				q.SetStatus("repeat head task")
			}

			if taskRes.DelayBeforeNextTask != 0 {
				nextSleepDelay = taskRes.DelayBeforeNextTask
				q.SetStatus(fmt.Sprintf("sleep for %s", nextSleepDelay.String()))
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

	origStatus := q.GetStatus()

	defer func() {
		checkTicker.Stop()
		q.waitMu.Lock()
		q.waitInProgress = false
		q.cancelDelay = false
		q.waitMu.Unlock()
		q.SetStatus(origStatus)
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
		waitTime := time.Since(waitBegin).Truncate(time.Second)
		if sleepDelay == 0 {
			q.SetStatus(fmt.Sprintf("waiting for task %s", waitTime.String()))
		} else {
			delay := sleepDelay.Truncate(time.Second)
			q.SetStatus(fmt.Sprintf("%s (%s left of %s delay)", origStatus, (delay - waitTime).String(), delay.String()))
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
		for e := q.items.Front(); e != nil; e = e.Next() {
			doFn(e.Value.(task.Task))
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
		for e := q.items.Front(); e != nil; {
			current := e
			e = e.Next()
			t := current.Value.(task.Task)
			if !filterFn(t) {
				q.items.Remove(current)
				delete(q.idIndex, t.GetId())
			}
		}
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
