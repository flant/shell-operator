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

// Global object pools for reducing allocations
var (
	compactionGroupPool = sync.Pool{
		New: func() interface{} {
			return &compactionGroup{
				elementsToMerge: make([]*list.Element, 0, 8),
			}
		},
	}

	contextSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]bindingcontext.BindingContext, 0, 64)
		},
	}

	monitorIDSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 64)
		},
	}

	hookGroupsMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]*compactionGroup, 32)
		},
	}
)

type compactionGroup struct {
	targetElement   *list.Element
	elementsToMerge []*list.Element
	totalContexts   int
	totalMonitorIDs int
}

// reset resets the compaction group for reuse
func (cg *compactionGroup) reset() {
	cg.targetElement = nil
	cg.elementsToMerge = cg.elementsToMerge[:0] // Reuse slice
	cg.totalContexts = 0
	cg.totalMonitorIDs = 0
}

type TaskQueue struct {
	m             sync.RWMutex
	metricStorage metric.Storage
	ctx           context.Context
	cancel        context.CancelFunc

	waitMu         sync.Mutex
	waitInProgress bool
	cancelDelay    bool

	isDirty bool

	taskTypesToMerge map[task.TaskType]struct{}

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

	// Pre-allocated buffers for compaction
	contextBuffer   []bindingcontext.BindingContext
	monitorIDBuffer []string
	groupBuffer     map[string]*compactionGroup
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
		// Pre-allocate buffers
		contextBuffer:   make([]bindingcontext.BindingContext, 0, 128),
		monitorIDBuffer: make([]string, 0, 128),
		groupBuffer:     make(map[string]*compactionGroup, 32),
	}
}

// Pool management methods
func (q *TaskQueue) getCompactionGroup() *compactionGroup {
	return compactionGroupPool.Get().(*compactionGroup)
}

func (q *TaskQueue) putCompactionGroup(cg *compactionGroup) {
	cg.reset()
	compactionGroupPool.Put(cg)
}

func (q *TaskQueue) getContextSlice() []bindingcontext.BindingContext {
	return contextSlicePool.Get().([]bindingcontext.BindingContext)
}

func (q *TaskQueue) putContextSlice(slice []bindingcontext.BindingContext) {
	slice = slice[:0] // Reset slice
	contextSlicePool.Put(slice)
}

func (q *TaskQueue) getMonitorIDSlice() []string {
	return monitorIDSlicePool.Get().([]string)
}

func (q *TaskQueue) putMonitorIDSlice(slice []string) {
	slice = slice[:0] // Reset slice
	monitorIDSlicePool.Put(slice)
}

func (q *TaskQueue) getHookGroupsMap() map[string]*compactionGroup {
	return hookGroupsMapPool.Get().(map[string]*compactionGroup)
}

func (q *TaskQueue) putHookGroupsMap(m map[string]*compactionGroup) {
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	hookGroupsMapPool.Put(m)
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

func (q *TaskQueue) WithTaskTypesToMerge(taskTypes []task.TaskType) *TaskQueue {
	q.taskTypesToMerge = make(map[task.TaskType]struct{}, len(taskTypes))
	for _, taskType := range taskTypes {
		q.taskTypesToMerge[taskType] = struct{}{}
	}
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
	// fmt.Printf("[TRACE-QUEUE] ADDING FIRST: Adding task %s of type %s to queue '%s'\n", t.GetId(), t.GetType(), q.Name)
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
	// fmt.Printf("[TRACE-QUEUE] ADDING LAST: Adding task %s of type %s to queue '%s'\n", t.GetId(), t.GetType(), q.Name)
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

	element := q.items.PushBack(t)
	q.idIndex[t.GetId()] = element

	// taskType := t.GetType()
	// if _, ok := q.taskTypesToMerge[taskType]; ok {
	// 	q.isDirty = true
	// 	// Only trigger compaction if queue is getting long and we have mergeable tasks
	// 	if q.items.Len() > 100 && q.isDirty {
	// 		q.performGlobalCompaction()
	// 	}
	// }
}

// performGlobalCompaction merges HookRun tasks for the same hook.
// It iterates through the list once, making it an O(N) operation.
// DEV WARNING! Do not use HookMetadataAccessor here. Use only *Accessor interfaces because this method is used from addon-operator.
func (q *TaskQueue) performGlobalCompaction() {
	fmt.Printf("[TRACE-QUEUE] Performing global compaction for queue '%s'\n", q.Name)
	if q.items.Len() < 2 {
		return
	}
	before := q.items.Len()
	var compactedCount int

	// Get objects from pools
	hookGroups := q.getHookGroupsMap()
	defer func() {
		// Return all compaction groups to pool
		for _, group := range hookGroups {
			q.putCompactionGroup(group)
		}
		q.putHookGroupsMap(hookGroups)
	}()

	// First pass: identify groups and calculate sizes
	for e := q.items.Front(); e != nil; e = e.Next() {
		t := e.Value.(task.Task)
		taskType := t.GetType()

		if _, ok := q.taskTypesToMerge[taskType]; !ok {
			continue
		}

		metadata := t.GetMetadata()
		if isNil(metadata) || t.IsProcessing() {
			continue
		}

		hookName := metadata.(task_metadata.HookNameAccessor).GetHookName()
		bindingContext := metadata.(task_metadata.BindingContextAccessor).GetBindingContext()
		monitorIDs := metadata.(task_metadata.MonitorIDAccessor).GetMonitorIDs()

		if group, exists := hookGroups[hookName]; exists {
			// Add to existing group
			group.elementsToMerge = append(group.elementsToMerge, e)
			group.totalContexts += len(bindingContext)
			group.totalMonitorIDs += len(monitorIDs)
		} else {
			// Get new group from pool
			group := q.getCompactionGroup()
			group.targetElement = e
			group.totalContexts = len(bindingContext)
			group.totalMonitorIDs = len(monitorIDs)
			hookGroups[hookName] = group
		}
	}

	// Second pass: merge with pooled slices
	for _, group := range hookGroups {
		if len(group.elementsToMerge) == 0 {
			continue
		}

		targetTask := group.targetElement.Value.(task.Task)
		targetMetadata := targetTask.GetMetadata()

		// Get slices from pools
		newContexts := q.getContextSlice()
		newMonitorIDs := q.getMonitorIDSlice()

		// Ensure capacity
		if cap(newContexts) < group.totalContexts {
			newContexts = make([]bindingcontext.BindingContext, 0, group.totalContexts)
		}
		if cap(newMonitorIDs) < group.totalMonitorIDs {
			newMonitorIDs = make([]string, 0, group.totalMonitorIDs)
		}

		// Add target's contexts first
		newContexts = append(newContexts, targetMetadata.(task_metadata.BindingContextAccessor).GetBindingContext()...)
		newMonitorIDs = append(newMonitorIDs, targetMetadata.(task_metadata.MonitorIDAccessor).GetMonitorIDs()...)

		// Append contexts from other tasks and remove them
		for _, elementToMerge := range group.elementsToMerge {
			taskToMerge := elementToMerge.Value.(task.Task)
			mergeMetadata := taskToMerge.GetMetadata()

			newContexts = append(newContexts, mergeMetadata.(task_metadata.BindingContextAccessor).GetBindingContext()...)
			newMonitorIDs = append(newMonitorIDs, mergeMetadata.(task_metadata.MonitorIDAccessor).GetMonitorIDs()...)

			q.items.Remove(elementToMerge)
			delete(q.idIndex, taskToMerge.GetId())
			compactedCount++
		}

		// Update target task with compacted slices
		compactedContexts := compactBindingContextsOptimized(newContexts)
		withContext := targetMetadata.(task_metadata.BindingContextSetter).SetBindingContext(compactedContexts)
		withContext = withContext.(task_metadata.MonitorIDSetter).SetMonitorIDs(newMonitorIDs)
		targetTask.UpdateMetadata(withContext)

		// Return slices to pools
		q.putContextSlice(newContexts)
		q.putMonitorIDSlice(newMonitorIDs)
	}
	fmt.Printf("[TRACE-QUEUE] Global compaction for queue '%s' completed, compacted %d tasks, before: %d, after: %d\n", q.Name, compactedCount, before, q.items.Len())
}

// compactBindingContexts mimics the logic from shell-operator's CombineBindingContextForHook.
// It removes intermediate states for the same group, keeping only the most recent one.
func compactBindingContextsOptimized(combinedContext []bindingcontext.BindingContext) []bindingcontext.BindingContext {
	if len(combinedContext) < 2 {
		return combinedContext
	}

	// Use the same slice to avoid allocation
	writeIndex := 0
	prevGroup := ""

	for i := 0; i < len(combinedContext); i++ {
		current := combinedContext[i]
		currentGroup := current.Metadata.Group

		// Fast path: empty group always kept
		if currentGroup == "" {
			combinedContext[writeIndex] = current
			writeIndex++
			prevGroup = ""
			continue
		}

		// Fast path: different group always kept
		if currentGroup != prevGroup {
			combinedContext[writeIndex] = current
			writeIndex++
			prevGroup = currentGroup
		}
		// Same group as previous - skip (keep = false)
	}

	return combinedContext[:writeIndex]
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

			// func() {
			// 	q.m.Lock()
			// 	defer q.m.Unlock()

			// 	// Temporarily disable compaction to match old implementation behavior
			// 	// if q.isDirty {
			// 	// 	q.performGlobalCompaction()
			// 	// 	q.isDirty = false
			// 	// }
			// }()

			fmt.Printf("[TRACE-QUEUE] Starting task %s of type %s, queue length %d, queue name %s\n", t.GetId(), t.GetType(), q.Length(), q.Name)

			// set that current task is being processed, so we don't merge it with other tasks
			// t.SetProcessing(true)

			// dump task and a whole queue
			q.debugf("queue %s: tasks after wait %s", q.Name, q.String())
			q.debugf("queue %s: task to handle '%s'", q.Name, t.GetType())

			var nextSleepDelay time.Duration
			q.SetStatus("run first task")
			fmt.Printf("[TRACE-QUEUE] Calling handler for task %s of type %s\n", t.GetId(), t.GetType())
			taskRes := q.Handler(ctx, t)
			fmt.Printf("[TRACE-QUEUE] Handler returned status: %s for task %s of type %s\n", taskRes.Status, t.GetId(), t.GetType())

			// Check Done channel after long-running operation.
			select {
			case <-q.ctx.Done():
				log.Info("queue stopped after task handling", slog.String("name", q.Name))
				q.SetStatus("stop")
				return
			default:
			}

			fmt.Printf("[TRACE-QUEUE] Processing task result status: %s for task %s of type %s\n", taskRes.Status, t.GetId(), t.GetType())
			switch taskRes.Status {
			case Fail:
				fmt.Printf("[TRACE-QUEUE] Task %s of type %s failed, setting processing=false, failure count: %d\n", t.GetId(), t.GetType(), t.GetFailureCount())
				// t.SetProcessing(false)
				// Exponential backoff delay before retry.
				nextSleepDelay = q.ExponentialBackoffFn(t.GetFailureCount())
				t.IncrementFailureCount()
				q.SetStatus(fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String()))
				fmt.Printf("[TRACE-QUEUE] Task %s of type %s will retry after %s delay\n", t.GetId(), t.GetType(), nextSleepDelay.String())
			case Success, Keep:
				fmt.Printf("[TRACE-QUEUE] Task %s of type %s status: %s, processing AfterTasks: %d, HeadTasks: %d, TailTasks: %d\n",
					t.GetId(), t.GetType(), taskRes.Status, len(taskRes.AfterTasks), len(taskRes.HeadTasks), len(taskRes.TailTasks))
				// Insert new tasks right after the current task in reverse order.
				q.withLock(func() {
					for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
						fmt.Printf("[TRACE-QUEUE] Adding AfterTask %s of type %s after task %s\n", taskRes.AfterTasks[i].GetId(), taskRes.AfterTasks[i].GetType(), t.GetId())
						q.addAfter(t.GetId(), taskRes.AfterTasks[i])
					}

					if taskRes.Status == Success {
						fmt.Printf("[TRACE-QUEUE] Removing task %s of type %s from queue (Success)\n", t.GetId(), t.GetType())
						q.remove(t.GetId())
					} else {
						fmt.Printf("[TRACE-QUEUE] Keeping task %s of type %s in queue (Keep)\n", t.GetId(), t.GetType())
					}
					// t.SetProcessing(false) // release processing flag

					// Also, add HeadTasks in reverse order
					// at the start of the queue. The first task in HeadTasks
					// become the new first task in the queue.
					for i := len(taskRes.HeadTasks) - 1; i >= 0; i-- {
						fmt.Printf("[TRACE-QUEUE] Adding HeadTask %s of type %s to front of queue\n", taskRes.HeadTasks[i].GetId(), taskRes.HeadTasks[i].GetType())
						q.addFirst(taskRes.HeadTasks[i])
					}
					// Add tasks to the end of the queue
					for _, newTask := range taskRes.TailTasks {
						fmt.Printf("[TRACE-QUEUE] Adding TailTask %s of type %s to end of queue\n", newTask.GetId(), newTask.GetType())
						q.addLast(newTask)
					}
				})
				q.SetStatus("")
				fmt.Printf("[TRACE-QUEUE] Task %s of type %s processing completed, queue length now: %d\n", t.GetId(), t.GetType(), q.Length())
			case Repeat:
				fmt.Printf("[TRACE-QUEUE] Task %s of type %s will repeat after delay\n", t.GetId(), t.GetType())
				// repeat a current task after a small delay
				// t.SetProcessing(false)
				nextSleepDelay = q.DelayOnRepeat
				q.SetStatus("repeat head task")
			}

			if taskRes.DelayBeforeNextTask != 0 {
				nextSleepDelay = taskRes.DelayBeforeNextTask
				q.SetStatus(fmt.Sprintf("sleep for %s", nextSleepDelay.String()))
				fmt.Printf("[TRACE-QUEUE] Task %s of type %s requested delay: %s\n", t.GetId(), t.GetType(), nextSleepDelay.String())
			}

			sleepDelay = nextSleepDelay
			fmt.Printf("[TRACE-QUEUE] Next sleep delay for queue %s: %s\n", q.Name, sleepDelay.String())

			if taskRes.AfterHandle != nil {
				fmt.Printf("[TRACE-QUEUE] Calling AfterHandle for task %s of type %s\n", t.GetId(), t.GetType())
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
