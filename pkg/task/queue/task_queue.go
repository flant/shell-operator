package queue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sort"
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
	metricStorage metric.Storage
	ctx           context.Context
	cancel        context.CancelFunc

	waitMu         sync.Mutex
	waitInProgress bool
	cancelDelay    bool

	isDirty          bool
	taskTypesToMerge map[task.TaskType]struct{}

	items   []task.Task
	started bool // a flag to ignore multiple starts

	// Log debug messages if true.
	debug bool

	Name    string
	Handler func(ctx context.Context, t task.Task) TaskResult
	Status  string

	// Callback for task compaction events
	CompactionCallback func(compactedTasks []task.Task, targetTask task.Task)

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

func (q *TaskQueue) WithMetricStorage(mstor metric.Storage) *TaskQueue {
	q.metricStorage = mstor

	return q
}

func (q *TaskQueue) WithTaskTypesToMerge(taskTypes []task.TaskType) *TaskQueue {
	q.taskTypesToMerge = make(map[task.TaskType]struct{}, len(taskTypes))
	for _, taskType := range taskTypes {
		q.taskTypesToMerge[taskType] = struct{}{}
	}
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

func (q *TaskQueue) WithCompactionCallback(callback func(compactedTasks []task.Task, targetTask task.Task)) *TaskQueue {
	q.CompactionCallback = callback
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
	isEmpty := q.isEmpty()

	return isEmpty
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

	t := q.items[0]
	q.items = q.items[1:]

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
	task := q.items[0]
	return task
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
	taskType := t.GetType()

	if _, ok := q.taskTypesToMerge[taskType]; ok {
		q.isDirty = true
	}

	if q.isDirty && len(q.items) > 100 {
		q.performGlobalCompaction()
	}
}

// performGlobalCompaction merges HookRun tasks for the same hook.
// DEV WARNING! Do not use HookMetadataAccessor here. Use only *Accessor interfaces because this method is used from addon-operator.
func (q *TaskQueue) performGlobalCompaction() {
	if len(q.items) == 0 {
		return
	}

	// Предварительно выделяем память для результата
	result := make([]task.Task, 0, len(q.items))

	hookGroups := make(map[string][]int, 10) // hookName -> []indices
	var hookOrder []string

	for i, task := range q.items {
		if _, ok := q.taskTypesToMerge[task.GetType()]; !ok {
			result = append(result, task)
			continue
		}
		hm := task.GetMetadata()
		if isNil(hm) || task.IsProcessing() {
			result = append(result, task) // Nil metadata и processing задачи сразу в результат
			continue
		}

		// Safety check to ensure we can access hook name
		hookNameAccessor, ok := hm.(task_metadata.HookNameAccessor)
		if !ok {
			result = append(result, task) // Cannot access hook name, skip compaction
			continue
		}
		hookName := hookNameAccessor.GetHookName()
		if _, exists := hookGroups[hookName]; !exists {
			hookOrder = append(hookOrder, hookName)
		}
		hookGroups[hookName] = append(hookGroups[hookName], i)
	}

	// Обрабатываем группы хуков - O(N) в худшем случае
	for _, hookName := range hookOrder {
		indices := hookGroups[hookName]

		if len(indices) == 1 {
			// Только одна задача - добавляем как есть
			result = append(result, q.items[indices[0]])
			continue
		}

		// Находим задачу с минимальным индексом как целевую
		minIndex := indices[0]
		for _, idx := range indices {
			if idx < minIndex {
				minIndex = idx
			}
		}

		// Safety check to ensure minIndex is valid
		if minIndex < 0 || minIndex >= len(q.items) {
			continue
		}

		targetTask := q.items[minIndex]
		targetHm := targetTask.GetMetadata()
		if targetHm == nil {
			continue
		}

		// Safety checks for type assertions
		bindingContextAccessor, ok := targetHm.(task_metadata.BindingContextAccessor)
		if !ok {
			continue
		}
		monitorIDAccessor, ok := targetHm.(task_metadata.MonitorIDAccessor)
		if !ok {
			continue
		}

		contexts := bindingContextAccessor.GetBindingContext()
		monitorIDs := monitorIDAccessor.GetMonitorIDs()
		// Предварительно вычисляем общий размер
		totalContexts := len(contexts)
		totalMonitorIDs := len(monitorIDs)

		for _, idx := range indices {
			if idx == minIndex {
				continue // Пропускаем целевую задачу
			}
			existingHm := q.items[idx].GetMetadata()
			if existingHm != nil {
				if bindingContextAccessor, ok := existingHm.(task_metadata.BindingContextAccessor); ok {
					totalContexts += len(bindingContextAccessor.GetBindingContext())
				}
				if monitorIDAccessor, ok := existingHm.(task_metadata.MonitorIDAccessor); ok {
					totalMonitorIDs += len(monitorIDAccessor.GetMonitorIDs())
				}
			}
		}

		// Создаем новые слайсы с правильным размером
		// Safety check to ensure we don't create negative-sized slices
		if totalContexts < 0 {
			totalContexts = 0
		}
		if totalMonitorIDs < 0 {
			totalMonitorIDs = 0
		}
		newContexts := make([]bindingcontext.BindingContext, totalContexts)
		newMonitorIDs := make([]string, totalMonitorIDs)

		// Копируем контексты целевой задачи
		if len(contexts) > 0 && len(newContexts) > 0 {
			copySize := len(contexts)
			if copySize > len(newContexts) {
				copySize = len(newContexts)
			}
			copy(newContexts[:copySize], contexts[:copySize])
		}
		if len(monitorIDs) > 0 && len(newMonitorIDs) > 0 {
			copySize := len(monitorIDs)
			if copySize > len(newMonitorIDs) {
				copySize = len(newMonitorIDs)
			}
			copy(newMonitorIDs[:copySize], monitorIDs[:copySize])
		}

		// Копируем контексты от остальных задач
		contextIndex := len(contexts)
		monitorIndex := len(monitorIDs)

		for _, idx := range indices {
			if idx == minIndex {
				continue
			}
			// Safety check to ensure idx is valid
			if idx < 0 || idx >= len(q.items) {
				continue
			}
			existingHm := q.items[idx].GetMetadata()
			if existingHm == nil {
				continue
			}

			// Safety checks for type assertions
			bindingContextAccessor, ok := existingHm.(task_metadata.BindingContextAccessor)
			if !ok {
				continue
			}
			monitorIDAccessor, ok := existingHm.(task_metadata.MonitorIDAccessor)
			if !ok {
				continue
			}

			existingContexts := bindingContextAccessor.GetBindingContext()
			existingMonitorIDs := monitorIDAccessor.GetMonitorIDs()

			if len(existingContexts) > 0 && contextIndex < len(newContexts) {
				// Safety check to ensure we don't exceed slice bounds
				remainingSpace := len(newContexts) - contextIndex
				if remainingSpace > 0 {
					copySize := len(existingContexts)
					if copySize > remainingSpace {
						copySize = remainingSpace
					}
					copy(newContexts[contextIndex:contextIndex+copySize], existingContexts[:copySize])
				}
			}
			contextIndex += len(existingContexts)

			if len(existingMonitorIDs) > 0 && monitorIndex < len(newMonitorIDs) {
				// Safety check to ensure we don't exceed slice bounds
				remainingSpace := len(newMonitorIDs) - monitorIndex
				if remainingSpace > 0 {
					copySize := len(existingMonitorIDs)
					if copySize > remainingSpace {
						copySize = remainingSpace
					}
					copy(newMonitorIDs[monitorIndex:monitorIndex+copySize], existingMonitorIDs[:copySize])
				}
			}
			monitorIndex += len(existingMonitorIDs)

		}

		// Обновляем метаданные
		bindingContextSetter, ok := targetHm.(task_metadata.BindingContextSetter)
		if !ok {
			continue
		}
		withContext := bindingContextSetter.SetBindingContext(compactBindingContextsOptimized(newContexts))

		monitorIDSetter, ok := withContext.(task_metadata.MonitorIDSetter)
		if !ok {
			continue
		}
		withContext = monitorIDSetter.SetMonitorIDs(newMonitorIDs)
		targetTask.UpdateMetadata(withContext)

		// Просто добавляем в конец, потом отсортируем
		result = append(result, targetTask)

		// Call compaction callback if set
		if q.CompactionCallback != nil && len(indices) > 1 {
			compactedTasks := make([]task.Task, 0, len(indices)-1)
			for _, idx := range indices {
				if idx != minIndex {
					compactedTasks = append(compactedTasks, q.items[idx])
				}
			}
			q.CompactionCallback(compactedTasks, targetTask)
		}
	}

	positionMap := make(map[task.Task]int, len(q.items))
	for i, task := range q.items {
		positionMap[task] = i
	}

	sort.Slice(result, func(i, j int) bool {
		posI := positionMap[result[i]]
		posJ := positionMap[result[j]]
		return posI < posJ
	})

	q.items = result
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

	// Safety check to prevent slice bounds panic
	if writeIndex <= 0 {
		return []bindingcontext.BindingContext{}
	}
	if writeIndex > len(combinedContext) {
		writeIndex = len(combinedContext)
	}

	result := combinedContext[:writeIndex]
	return result
}

// compactionGroup represents a group of tasks that can be merged
type compactionGroup struct {
	targetIndex     int
	indicesToMerge  []int
	totalContexts   int
	totalMonitorIDs int
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

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueue) removeLast() task.Task {
	if q.isEmpty() {
		return nil
	}

	t := q.items[len(q.items)-1]
	if len(q.items) == 1 {
		q.items = make([]task.Task, 0)
	} else {
		q.items = q.items[:len(q.items)-1]
	}

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

// GetLast returns a tail element.
func (q *TaskQueue) getLast() task.Task {
	if q.isEmpty() {
		return nil
	}

	return q.items[len(q.items)-1]
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

// Get returns a task by id.
func (q *TaskQueue) get(id string) task.Task {
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

	if !idFound {
		newItems[len(q.items)] = newTask
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
func (q *TaskQueue) Remove(id string) task.Task {
	defer q.MeasureActionTime("Remove")()
	var t task.Task

	q.withLock(func() {
		t = q.remove(id)
	})

	return t
}

func (q *TaskQueue) remove(id string) task.Task {
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

	t := q.items[delId]
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

			// dump task and a whole queue
			q.debugf("queue %s: tasks after wait %s", q.Name, q.String())
			q.debugf("queue %s: task to handle '%s'", q.Name, t.GetType())

			// compact queue if it's dirty
			q.withLock(func() {
				if q.isDirty {
					q.performGlobalCompaction()
				}
			})

			// set that current task is being processed, so we don't merge it with other tasks
			t.SetProcessing(true)

			// Now the task can be handled!
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
				// Reset processing flag for failed task
				t.SetProcessing(false)
				// Exponential backoff delay before retry.
				nextSleepDelay = q.ExponentialBackoffFn(t.GetFailureCount())
				t.IncrementFailureCount()
				q.SetStatus(fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String()))
			case Success, Keep:
				// Insert new tasks right after the current task in reverse order.
				q.withLock(func() {
					for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
						q.addAfter(t.GetId(), taskRes.AfterTasks[i])
					}
					// Remove current task on success.
					if taskRes.Status == Success {
						q.remove(t.GetId())
					} else {
						// Reset processing flag for kept task
						t.SetProcessing(false)
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
				// Reset processing flag for repeated task
				t.SetProcessing(false)
				// repeat a current task after a small delay
				nextSleepDelay = q.DelayOnRepeat
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

func isNil(v interface{}) bool {
	if v == nil {
		return true
	}

	// Use reflection to check if the interface contains a nil concrete value
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
