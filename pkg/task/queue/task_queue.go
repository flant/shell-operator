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

const DefaultDebounceDuration = 50 * time.Millisecond

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

type TaskQueueOLD struct {
	m             sync.RWMutex
	metricStorage metric.Storage
	ctx           context.Context
	cancel        context.CancelFunc

	waitMu         sync.Mutex
	waitInProgress bool
	cancelDelay    bool

	items           []task.Task
	compactionTimer *time.Timer // Timer to debounce compaction.
	head            int
	size            int
	idIndex         map[string]int

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

func NewTasksQueueOLD() *TaskQueueOLD {
	return &TaskQueueOLD{
		items:   make([]task.Task, 0),
		idIndex: make(map[string]int),
		// Default timings
		WaitLoopCheckInterval: DefaultWaitLoopCheckInterval,
		DelayOnQueueIsEmpty:   DefaultDelayOnQueueIsEmpty,
		DelayOnRepeat:         DefaultDelayOnRepeat,
		ExponentialBackoffFn: func(failureCount int) time.Duration {
			return exponential_backoff.CalculateDelay(DefaultInitialDelayOnFailedTask, failureCount)
		},
	}
}

func (q *TaskQueueOLD) WithContext(ctx context.Context) {
	q.ctx, q.cancel = context.WithCancel(ctx)
}

func (q *TaskQueueOLD) WithMetricStorage(mstor metric.Storage) *TaskQueueOLD {
	q.metricStorage = mstor

	return q
}

func (q *TaskQueueOLD) WithName(name string) *TaskQueueOLD {
	q.Name = name
	return q
}

func (q *TaskQueueOLD) WithHandler(fn func(ctx context.Context, t task.Task) TaskResult) *TaskQueueOLD {
	q.Handler = fn
	return q
}

// MeasureActionTime is a helper to measure execution time of queue's actions
func (q *TaskQueueOLD) MeasureActionTime(action string) func() {
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

func (q *TaskQueueOLD) GetStatus() string {
	defer q.MeasureActionTime("GetStatus")()
	q.m.RLock()
	defer q.m.RUnlock()
	return q.Status
}

func (q *TaskQueueOLD) SetStatus(status string) {
	q.m.Lock()
	q.Status = status
	q.m.Unlock()
}

func (q *TaskQueueOLD) IsEmpty() bool {
	defer q.MeasureActionTime("IsEmpty")()
	q.m.RLock()
	defer q.m.RUnlock()
	return q.isEmpty()
}

func (q *TaskQueueOLD) isEmpty() bool {
	return len(q.items) == 0
}

func (q *TaskQueueOLD) Length() int {
	defer q.MeasureActionTime("Length")()
	q.m.RLock()
	defer q.m.RUnlock()
	return len(q.items)
}

// AddFirst adds new head element.
func (q *TaskQueueOLD) AddFirst(t task.Task) {
	defer q.MeasureActionTime("AddFirst")()
	q.withLock(func() {
		q.addFirst(t)
	})
}

// addFirst adds new head element.
func (q *TaskQueueOLD) addFirst(t task.Task) {
	q.items = append([]task.Task{t}, q.items...)
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueueOLD) RemoveFirst() task.Task {
	defer q.MeasureActionTime("RemoveFirst")()
	var t task.Task

	q.withLock(func() {
		t = q.removeFirst()
	})

	return t
}

// removeFirst deletes a head element, so head is moved.
func (q *TaskQueueOLD) removeFirst() task.Task {
	if q.isEmpty() {
		return nil
	}

	t := q.items[0]
	q.items = q.items[1:]

	return t
}

// GetFirst returns a head element.
func (q *TaskQueueOLD) GetFirst() task.Task {
	defer q.MeasureActionTime("GetFirst")()
	q.m.RLock()
	defer q.m.RUnlock()
	if q.isEmpty() {
		return nil
	}
	return q.items[0]
}

// AddLast adds new tail element.
func (q *TaskQueueOLD) AddLast(t task.Task) {
	defer q.MeasureActionTime("AddLast")()
	q.withLock(func() {
		q.addLast(t)
	})
}

// addLast adds a new tail element.
// It implements the merging logic for HookRun tasks by scanning the whole queue.
func (q *TaskQueueOLD) addLast(t task.Task) {
	hm := task_metadata.HookMetadataAccessor(t)
	// The task is not a hook task if it has no HookMetadata.
	if isNil(hm) {
		q.items = append(q.items, t)
		return
	}

	// For hook tasks, always add and then trigger debounced compaction.
	q.items = append(q.items, t)

	// Debounce compaction.
	if q.compactionTimer != nil {
		q.compactionTimer.Stop()
	}

	q.compactionTimer = time.AfterFunc(DefaultDebounceDuration, func() {
		q.m.Lock()
		defer q.m.Unlock()
		q.performGlobalCompaction()
	})
}

// performGlobalCompaction merges hook tasks.
func (q *TaskQueueOLD) performGlobalCompaction() {
	if len(q.items) < 2 {
		return
	}

	// Предварительно выделяем память для результата
	result := make([]task.Task, 0, len(q.items))

	// Map для быстрого поиска групп хуков
	hookGroups := make(map[string][]int, 10) // hookName -> []indices
	var hookOrder []string

	// Один проход: собираем индексы задач по хукам - O(N)
	for i, task := range q.items {
		metadata := task.GetMetadata()

		if isNil(metadata) {
			// This is not a hook task, add it to the result as is.
			result = append(result, task)
			continue
		}

		if task.IsProcessing() {
			result = append(result, task) // Do not compact tasks that are being processed.
			continue
		}

		hookName := metadata.(task_metadata.HookNameAccessor).GetHookName()
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

		targetTask := q.items[minIndex]
		targetHm := task_metadata.HookMetadataAccessor(targetTask)

		// Предварительно вычисляем общий размер
		totalContexts := len(targetHm.BindingContext)
		totalMonitorIDs := len(targetHm.MonitorIDs)

		for _, idx := range indices {
			if idx == minIndex {
				continue // Пропускаем целевую задачу
			}
			existingHm := task_metadata.HookMetadataAccessor(q.items[idx])
			totalContexts += len(existingHm.BindingContext)
			totalMonitorIDs += len(existingHm.MonitorIDs)
		}

		// Создаем новые слайсы с правильным размером
		newContexts := make([]bindingcontext.BindingContext, totalContexts)
		newMonitorIDs := make([]string, totalMonitorIDs)

		// Копируем контексты целевой задачи
		copy(newContexts, targetHm.BindingContext)
		copy(newMonitorIDs, targetHm.MonitorIDs)

		// Копируем контексты от остальных задач
		contextIndex := len(targetHm.BindingContext)
		monitorIndex := len(targetHm.MonitorIDs)

		for _, idx := range indices {
			if idx == minIndex {
				continue
			}
			existingHm := task_metadata.HookMetadataAccessor(q.items[idx])

			copy(newContexts[contextIndex:], existingHm.BindingContext)
			contextIndex += len(existingHm.BindingContext)

			copy(newMonitorIDs[monitorIndex:], existingHm.MonitorIDs)
			monitorIndex += len(existingHm.MonitorIDs)

			// fmt.Printf("[TRACE-QUEUE] Compacting task %s for hook '%s' into task %s\n",
			// 	q.items[idx].GetId(), hookName, targetTask.GetId())
		}

		// Обновляем метаданные
		targetHm.BindingContext = newContexts
		targetHm.MonitorIDs = newMonitorIDs
		targetTask.UpdateMetadata(targetHm)

		// Просто добавляем в конец, потом отсортируем
		result = append(result, targetTask)

		// fmt.Printf("[TRACE-QUEUE] Global compaction: merged %d tasks for hook '%s' into task %s\n",
		// 	len(indices), hookName, targetTask.GetId())
	}

	// Сортируем результат по исходным позициям для сохранения порядка
	// Создаем map для быстрого поиска позиций
	positionMap := make(map[task.Task]int, len(q.items))
	for i, task := range q.items {
		positionMap[task] = i
	}

	sort.Slice(result, func(i, j int) bool {
		posI := positionMap[result[i]]
		posJ := positionMap[result[j]]
		return posI < posJ
	})

	// Заменяем очередь
	// originalSize := len(q.items)
	q.items = result
	// compactedSize := len(q.items)

	// fmt.Printf("[TRACE-QUEUE] Global compaction complete: %d tasks -> %d tasks (compressed %d tasks)\n",
	// 	originalSize, compactedSize, originalSize-compactedSize)
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueueOLD) RemoveLast() task.Task {
	defer q.MeasureActionTime("RemoveLast")()
	var t task.Task

	q.withLock(func() {
		t = q.removeLast()
	})

	return t
}

// RemoveLast deletes a tail element, so tail is moved.
func (q *TaskQueueOLD) removeLast() task.Task {
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
func (q *TaskQueueOLD) GetLast() task.Task {
	defer q.MeasureActionTime("GetLast")()
	var t task.Task

	q.withRLock(func() {
		t = q.getLast()
	})

	return t
}

// GetLast returns a tail element.
func (q *TaskQueueOLD) getLast() task.Task {
	if q.isEmpty() {
		return nil
	}

	return q.items[len(q.items)-1]
}

// Get returns a task by id.
func (q *TaskQueueOLD) Get(id string) task.Task {
	defer q.MeasureActionTime("Get")()
	var t task.Task

	q.withRLock(func() {
		t = q.get(id)
	})

	return t
}

// Get returns a task by id.
func (q *TaskQueueOLD) get(id string) task.Task {
	for _, t := range q.items {
		if t.GetId() == id {
			return t
		}
	}

	return nil
}

// AddAfter inserts a task after the task with specified id.
func (q *TaskQueueOLD) AddAfter(id string, newTask task.Task) {
	defer q.MeasureActionTime("AddAfter")()
	q.withLock(func() {
		q.addAfter(id, newTask)
	})
}

// addAfter inserts a task after the task with specified id.
func (q *TaskQueueOLD) addAfter(id string, newTask task.Task) {
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
func (q *TaskQueueOLD) AddBefore(id string, newTask task.Task) {
	defer q.MeasureActionTime("AddBefore")()
	q.withLock(func() {
		q.addBefore(id, newTask)
	})
}

// addBefore inserts a task before the task with specified id.
func (q *TaskQueueOLD) addBefore(id string, newTask task.Task) {
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
func (q *TaskQueueOLD) Remove(id string) task.Task {
	defer q.MeasureActionTime("Remove")()
	var t task.Task

	q.withLock(func() {
		t = q.remove(id)
	})

	return t
}

func (q *TaskQueueOLD) remove(id string) task.Task {
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

func (q *TaskQueueOLD) SetDebug(debug bool) {
	q.debug = debug
}

func (q *TaskQueueOLD) debugf(format string, args ...interface{}) {
	if !q.debug {
		return
	}
	log.Debug("DEBUG", fmt.Sprintf(format, args...))
}

func (q *TaskQueueOLD) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
}

func (q *TaskQueueOLD) Start(ctx context.Context) {
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

			log.Info(
				"Starting task",
				slog.String("task", t.GetDescription()),
				slog.Int("queue_length", q.Length()),
				slog.String("queue_name", q.Name),
			)

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
					for i := len(taskRes.AfterTasks) - 1; i >= 0; i-- {
						q.addAfter(t.GetId(), taskRes.AfterTasks[i])
					}
					// Remove current task on success.
					if taskRes.Status == Success {
						q.remove(t.GetId())
					}

					t.SetProcessing(false)
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
func (q *TaskQueueOLD) waitForTask(sleepDelay time.Duration) task.Task {
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
func (q *TaskQueueOLD) CancelTaskDelay() {
	q.waitMu.Lock()
	if q.waitInProgress {
		q.cancelDelay = true
	}
	q.waitMu.Unlock()
}

// Iterate run doFn for every task.
func (q *TaskQueueOLD) Iterate(doFn func(task.Task)) {
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
func (q *TaskQueueOLD) Filter(filterFn func(task.Task) bool) {
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
func (q *TaskQueueOLD) String() string {
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

func (q *TaskQueueOLD) withLock(fn func()) {
	q.m.Lock()
	fn()
	q.m.Unlock()
}

func (q *TaskQueueOLD) withRLock(fn func()) {
	q.m.RLock()
	fn()
	q.m.RUnlock()
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}
