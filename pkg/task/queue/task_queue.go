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
	fmt.Printf("[TRACE-QUEUE] IsEmpty: queue=%s, isEmpty=%v, length=%d\n", q.Name, isEmpty, len(q.items))
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
	fmt.Printf("[TRACE-QUEUE] AddFirst: queue=%s, task=%s, type=%s, id=%s\n", q.Name, t.GetDescription(), t.GetType(), t.GetId())
	defer q.MeasureActionTime("AddFirst")()
	q.withLock(func() {
		q.addFirst(t)
	})
	fmt.Printf("[TRACE-QUEUE] AddFirst: queue=%s, new length=%d\n", q.Name, len(q.items))
}

// addFirst adds new head element.
func (q *TaskQueue) addFirst(t task.Task) {
	fmt.Printf("[TRACE-QUEUE] addFirst: queue=%s, before length=%d\n", q.Name, len(q.items))
	q.items = append([]task.Task{t}, q.items...)
	fmt.Printf("[TRACE-QUEUE] addFirst: queue=%s, after length=%d\n", q.Name, len(q.items))
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueue) RemoveFirst() task.Task {
	fmt.Printf("[TRACE-QUEUE] RemoveFirst: queue=%s, before length=%d\n", q.Name, len(q.items))
	defer q.MeasureActionTime("RemoveFirst")()
	var t task.Task

	q.withLock(func() {
		t = q.removeFirst()
	})

	if t != nil {
		fmt.Printf("[TRACE-QUEUE] RemoveFirst: queue=%s, removed task=%s, type=%s, id=%s, after length=%d\n", q.Name, t.GetDescription(), t.GetType(), t.GetId(), len(q.items))
	} else {
		fmt.Printf("[TRACE-QUEUE] RemoveFirst: queue=%s, no task to remove, after length=%d\n", q.Name, len(q.items))
	}

	return t
}

// removeFirst deletes a head element, so head is moved.
func (q *TaskQueue) removeFirst() task.Task {
	fmt.Printf("[TRACE-QUEUE] removeFirst: queue=%s, isEmpty=%v, length=%d\n", q.Name, q.isEmpty(), len(q.items))
	if q.isEmpty() {
		return nil
	}

	t := q.items[0]
	fmt.Printf("[TRACE-QUEUE] removeFirst: queue=%s, removing task=%s, type=%s, id=%s\n", q.Name, t.GetDescription(), t.GetType(), t.GetId())
	q.items = q.items[1:]
	fmt.Printf("[TRACE-QUEUE] removeFirst: queue=%s, after removal length=%d\n", q.Name, len(q.items))

	return t
}

// GetFirst returns a head element.
func (q *TaskQueue) GetFirst() task.Task {
	defer q.MeasureActionTime("GetFirst")()
	q.m.RLock()
	defer q.m.RUnlock()
	if q.isEmpty() {
		fmt.Printf("[TRACE-QUEUE] GetFirst: queue=%s, queue is empty, returning nil\n", q.Name)
		return nil
	}
	task := q.items[0]
	fmt.Printf("[TRACE-QUEUE] GetFirst: queue=%s, returning task=%s, type=%s, id=%s\n", q.Name, task.GetDescription(), task.GetType(), task.GetId())
	return task
}

// AddLast adds new tail element.
func (q *TaskQueue) AddLast(t task.Task) {
	fmt.Printf("[TRACE-QUEUE] AddLast: queue=%s, task=%s, type=%s, id=%s\n", q.Name, t.GetDescription(), t.GetType(), t.GetId())
	defer q.MeasureActionTime("AddLast")()
	q.withLock(func() {
		q.addLast(t)
	})
	fmt.Printf("[TRACE-QUEUE] AddLast: queue=%s, new length=%d, isDirty=%v\n", q.Name, len(q.items), q.isDirty)
}

// addFirst adds new tail element.
func (q *TaskQueue) addLast(t task.Task) {
	fmt.Printf("[TRACE-QUEUE] addLast: queue=%s, before length=%d\n", q.Name, len(q.items))
	q.items = append(q.items, t)
	fmt.Printf("[TRACE-QUEUE] addLast: queue=%s, after append length=%d\n", q.Name, len(q.items))

	taskType := t.GetType()
	fmt.Printf("[TRACE-QUEUE] addLast: queue=%s, taskType=%s, isMergeable=%v\n", q.Name, taskType, q.taskTypesToMerge != nil)
	if _, ok := q.taskTypesToMerge[taskType]; ok {
		fmt.Printf("[TRACE-QUEUE] addLast: queue=%s, setting isDirty=true, current length=%d\n", q.Name, len(q.items))
		// Only trigger compaction if queue is getting long and we have mergeable tasks
		fmt.Printf("[TRACE-QUEUE] addLast: queue=%s, triggering compaction, length=%d\n", q.Name, len(q.items))
		q.performGlobalCompaction()
	}
}

// performGlobalCompaction merges HookRun tasks for the same hook.
// DEV WARNING! Do not use HookMetadataAccessor here. Use only *Accessor interfaces because this method is used from addon-operator.
func (q *TaskQueue) performGlobalCompaction() {
	fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, start, length=%d\n", q.Name, len(q.items))
	if len(q.items) == 0 {
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, empty queue, skipping\n", q.Name)
		return
	}

	// Предварительно выделяем память для результата
	result := make([]task.Task, 0, len(q.items))

	hookGroups := make(map[string][]int, 10) // hookName -> []indices
	var hookOrder []string
	fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, starting grouping phase\n", q.Name)

	for i, task := range q.items {
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, processing task[%d]=%s, type=%s, id=%s\n", q.Name, i, task.GetDescription(), task.GetType(), task.GetId())
		if _, ok := q.taskTypesToMerge[task.GetType()]; !ok {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, task[%d] not mergeable, adding to result\n", q.Name, i)
			result = append(result, task)
			continue
		}
		hm := task.GetMetadata()
		if isNil(hm) || task.IsProcessing() {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, task[%d] has nil metadata or is processing, adding to result\n", q.Name, i)
			result = append(result, task) // Nil metadata и processing задачи сразу в результат
			continue
		}

		// Safety check to ensure we can access hook name
		hookNameAccessor, ok := hm.(task_metadata.HookNameAccessor)
		if !ok {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, task[%d] cannot access hook name, adding to result\n", q.Name, i)
			result = append(result, task) // Cannot access hook name, skip compaction
			continue
		}
		hookName := hookNameAccessor.GetHookName()
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, task[%d] hookName=%s\n", q.Name, i, hookName)
		if _, exists := hookGroups[hookName]; !exists {
			hookOrder = append(hookOrder, hookName)
		}
		hookGroups[hookName] = append(hookGroups[hookName], i)
	}

	// Обрабатываем группы хуков - O(N) в худшем случае
	fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, processing hook groups, total groups=%d\n", q.Name, len(hookOrder))
	for _, hookName := range hookOrder {
		indices := hookGroups[hookName]
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, processing hook=%s, indices=%v\n", q.Name, hookName, indices)

		if len(indices) == 1 {
			// Только одна задача - добавляем как есть
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s has only 1 task, adding as-is\n", q.Name, hookName)
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
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task index=%d\n", q.Name, hookName, minIndex)

		// Safety check to ensure minIndex is valid
		if minIndex < 0 || minIndex >= len(q.items) {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s invalid minIndex=%d, queue length=%d, skipping\n", q.Name, hookName, minIndex, len(q.items))
			continue
		}

		targetTask := q.items[minIndex]
		targetHm := targetTask.GetMetadata()
		if targetHm == nil {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task has nil metadata, skipping\n", q.Name, hookName)
			continue
		}

		// Safety checks for type assertions
		bindingContextAccessor, ok := targetHm.(task_metadata.BindingContextAccessor)
		if !ok {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task cannot access binding context, skipping\n", q.Name, hookName)
			continue
		}
		monitorIDAccessor, ok := targetHm.(task_metadata.MonitorIDAccessor)
		if !ok {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task cannot access monitor IDs, skipping\n", q.Name, hookName)
			continue
		}

		contexts := bindingContextAccessor.GetBindingContext()
		monitorIDs := monitorIDAccessor.GetMonitorIDs()
		// Предварительно вычисляем общий размер
		totalContexts := len(contexts)
		totalMonitorIDs := len(monitorIDs)
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task has contexts=%d, monitorIDs=%d\n", q.Name, hookName, totalContexts, totalMonitorIDs)

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
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s calculated total contexts=%d, total monitorIDs=%d\n", q.Name, hookName, totalContexts, totalMonitorIDs)

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
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s created slices: contexts=%d, monitorIDs=%d\n", q.Name, hookName, len(newContexts), len(newMonitorIDs))

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
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s starting merge from index: context=%d, monitor=%d\n", q.Name, hookName, contextIndex, monitorIndex)

		for _, idx := range indices {
			if idx == minIndex {
				continue
			}
			// Safety check to ensure idx is valid
			if idx < 0 || idx >= len(q.items) {
				fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s invalid index=%d, queue length=%d, skipping\n", q.Name, hookName, idx, len(q.items))
				continue
			}
			existingHm := q.items[idx].GetMetadata()
			if existingHm == nil {
				fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s task[%d] has nil metadata, skipping\n", q.Name, hookName, idx)
				continue
			}

			// Safety checks for type assertions
			bindingContextAccessor, ok := existingHm.(task_metadata.BindingContextAccessor)
			if !ok {
				fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s task[%d] cannot access binding context, skipping\n", q.Name, hookName, idx)
				continue
			}
			monitorIDAccessor, ok := existingHm.(task_metadata.MonitorIDAccessor)
			if !ok {
				fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s task[%d] cannot access monitor IDs, skipping\n", q.Name, hookName, idx)
				continue
			}

			existingContexts := bindingContextAccessor.GetBindingContext()
			existingMonitorIDs := monitorIDAccessor.GetMonitorIDs()
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s task[%d] has contexts=%d, monitorIDs=%d\n", q.Name, hookName, idx, len(existingContexts), len(existingMonitorIDs))

			if len(existingContexts) > 0 && contextIndex < len(newContexts) {
				// Safety check to ensure we don't exceed slice bounds
				remainingSpace := len(newContexts) - contextIndex
				if remainingSpace > 0 {
					copySize := len(existingContexts)
					if copySize > remainingSpace {
						copySize = remainingSpace
					}
					copy(newContexts[contextIndex:contextIndex+copySize], existingContexts[:copySize])
					fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s task[%d] copied %d contexts to index %d\n", q.Name, hookName, idx, copySize, contextIndex)
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
					fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s task[%d] copied %d monitorIDs to index %d\n", q.Name, hookName, idx, copySize, monitorIndex)
				}
			}
			monitorIndex += len(existingMonitorIDs)

			fmt.Printf("[TRACE-QUEUE] Compacting task %s for hook '%s' into task %s\n",
				q.items[idx].GetId(), hookName, targetTask.GetId())
		}

		// Обновляем метаданные
		bindingContextSetter, ok := targetHm.(task_metadata.BindingContextSetter)
		if !ok {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task cannot set binding context, skipping\n", q.Name, hookName)
			continue
		}
		withContext := bindingContextSetter.SetBindingContext(compactBindingContextsOptimized(newContexts))

		monitorIDSetter, ok := withContext.(task_metadata.MonitorIDSetter)
		if !ok {
			fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s target task cannot set monitor IDs, skipping\n", q.Name, hookName)
			continue
		}
		withContext = monitorIDSetter.SetMonitorIDs(newMonitorIDs)
		targetTask.UpdateMetadata(withContext)
		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s updated target task metadata\n", q.Name, hookName)

		// Просто добавляем в конец, потом отсортируем
		result = append(result, targetTask)

		fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, hook=%s merged %d tasks into task %s\n",
			q.Name, hookName, len(indices), targetTask.GetId())
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

	fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, final result length=%d\n", q.Name, len(result))
	q.items = result
	fmt.Printf("[TRACE-QUEUE] performGlobalCompaction: queue=%s, compaction completed, new length=%d\n", q.Name, len(q.items))
}

// compactBindingContexts mimics the logic from shell-operator's CombineBindingContextForHook.
// It removes intermediate states for the same group, keeping only the most recent one.
func compactBindingContextsOptimized(combinedContext []bindingcontext.BindingContext) []bindingcontext.BindingContext {
	fmt.Printf("[TRACE-QUEUE] compactBindingContextsOptimized: input length=%d\n", len(combinedContext))
	if len(combinedContext) < 2 {
		fmt.Printf("[TRACE-QUEUE] compactBindingContextsOptimized: no compaction needed, returning as-is\n")
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
	fmt.Printf("[TRACE-QUEUE] compactBindingContextsOptimized: writeIndex=%d, input length=%d\n", writeIndex, len(combinedContext))

	// Safety check to prevent slice bounds panic
	if writeIndex <= 0 {
		fmt.Printf("[TRACE-QUEUE] compactBindingContextsOptimized: writeIndex <= 0, returning empty slice\n")
		return []bindingcontext.BindingContext{}
	}
	if writeIndex > len(combinedContext) {
		writeIndex = len(combinedContext)
	}

	result := combinedContext[:writeIndex]
	fmt.Printf("[TRACE-QUEUE] compactBindingContextsOptimized: returning length=%d\n", len(result))
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
	fmt.Printf("[TRACE-QUEUE] addAfter: queue=%s, inserting task=%s, type=%s, id=%s after id=%s\n", q.Name, newTask.GetDescription(), newTask.GetType(), newTask.GetId(), id)
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
				fmt.Printf("[TRACE-QUEUE] addAfter: queue=%s, found target id=%s at index=%d, inserted new task at index=%d\n", q.Name, id, i, i+1)
			}
		} else {
			// when id is found, copy other tasks to i+1 position
			newItems[i+1] = t
		}
	}

	if !idFound {
		fmt.Printf("[TRACE-QUEUE] addAfter: queue=%s, target id=%s not found, appending to end\n", q.Name, id)
		newItems[len(q.items)] = newTask
	}

	fmt.Printf("[TRACE-QUEUE] addAfter: queue=%s, new length=%d\n", q.Name, len(newItems))
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
	fmt.Printf("[TRACE-QUEUE] remove: queue=%s, looking for id=%s, current length=%d\n", q.Name, id, len(q.items))
	delId := -1
	for i, item := range q.items {
		if item.GetId() == id {
			delId = i
			break
		}
	}
	if delId == -1 {
		fmt.Printf("[TRACE-QUEUE] remove: queue=%s, id=%s not found\n", q.Name, id)
		return nil
	}

	t := q.items[delId]
	fmt.Printf("[TRACE-QUEUE] remove: queue=%s, removing task=%s, type=%s, id=%s at index=%d\n", q.Name, t.GetDescription(), t.GetType(), t.GetId(), delId)
	q.items = append(q.items[:delId], q.items[delId+1:]...)
	fmt.Printf("[TRACE-QUEUE] remove: queue=%s, after removal length=%d\n", q.Name, len(q.items))

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
	fmt.Printf("[TRACE-QUEUE] Start: queue=%s, started=%v\n", q.Name, q.started)
	if q.started {
		fmt.Printf("[TRACE-QUEUE] Start: queue=%s, already started, returning\n", q.Name)
		return
	}

	if q.Handler == nil {
		fmt.Printf("[TRACE-QUEUE] Start: queue=%s, no handler set, error\n", q.Name)
		log.Error("should set handler before start in queue", slog.String("name", q.Name))
		q.SetStatus("no handler set")
		return
	}

	go func() {
		fmt.Printf("[TRACE-QUEUE] Start: queue=%s, starting goroutine\n", q.Name)
		q.SetStatus("")
		var sleepDelay time.Duration
		for {
			q.debugf("queue %s: wait for task, delay %d", q.Name, sleepDelay)
			t := q.waitForTask(sleepDelay)
			if t == nil {
				fmt.Printf("[TRACE-QUEUE] Start: queue=%s, no task returned, stopping\n", q.Name)
				q.SetStatus("stop")
				log.Info("queue stopped", slog.String("name", q.Name))
				return
			}

			// dump task and a whole queue
			q.debugf("queue %s: tasks after wait %s", q.Name, q.String())
			q.debugf("queue %s: task to handle '%s'", q.Name, t.GetType())

			// set that current task is being processed, so we don't merge it with other tasks
			fmt.Printf("[TRACE-QUEUE] Start: queue=%s, handling task=%s, type=%s, id=%s\n", q.Name, t.GetDescription(), t.GetType(), t.GetId())
			t.SetProcessing(true)

			// Now the task can be handled!
			var nextSleepDelay time.Duration
			q.SetStatus("run first task")
			fmt.Printf("[TRACE-QUEUE] Start: queue=%s, calling handler for task=%s\n", q.Name, t.GetId())
			taskRes := q.Handler(ctx, t)
			fmt.Printf("[TRACE-QUEUE] Start: queue=%s, handler returned status=%s\n", q.Name, taskRes.Status)

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
				fmt.Printf("[TRACE-QUEUE] Start: queue=%s, task failed, failureCount=%d\n", q.Name, t.GetFailureCount())
				// Reset processing flag for failed task
				t.SetProcessing(false)
				// Exponential backoff delay before retry.
				nextSleepDelay = q.ExponentialBackoffFn(t.GetFailureCount())
				t.IncrementFailureCount()
				q.SetStatus(fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String()))
			case Success, Keep:
				fmt.Printf("[TRACE-QUEUE] Start: queue=%s, task %s, headTasks=%d, tailTasks=%d, afterTasks=%d\n", q.Name, taskRes.Status, len(taskRes.HeadTasks), len(taskRes.TailTasks), len(taskRes.AfterTasks))
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
				fmt.Printf("[TRACE-QUEUE] Start: queue=%s, task repeat\n", q.Name)
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

			fmt.Printf("[TRACE-QUEUE] Start: queue=%s, next sleep delay=%s\n", q.Name, sleepDelay.String())
			q.debugf("queue %s: tasks after handle %s", q.Name, q.String())
		}
	}()
	fmt.Printf("[TRACE-QUEUE] Start: queue=%s, goroutine started, setting started=true\n", q.Name)
	q.started = true
}

// waitForTask returns a task that can be processed or a nil if context is canceled.
// sleepDelay is used to sleep before check a task, e.g. in case of failed previous task.
// If queue is empty, then it will be checked every DelayOnQueueIsEmpty.
func (q *TaskQueue) waitForTask(sleepDelay time.Duration) task.Task {
	fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, sleepDelay=%s\n", q.Name, sleepDelay.String())
	// Check Done channel.
	select {
	case <-q.ctx.Done():
		fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, context done, returning nil\n", q.Name)
		return nil
	default:
	}

	// Shortcut: return the first task if the queue is not empty and delay is not required.
	if !q.IsEmpty() && sleepDelay == 0 {
		fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, shortcut - queue not empty and no delay, returning first task\n", q.Name)
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
	fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, starting wait loop, waitUntil=%s\n", q.Name, waitUntil.String())
	for {
		checkTask := false
		select {
		case <-q.ctx.Done():
			// Queue is stopped.
			fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, context done in wait loop, returning nil\n", q.Name)
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
				fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, queue empty, increasing wait time\n", q.Name)
				waitUntil += q.DelayOnQueueIsEmpty
			} else {
				fmt.Printf("[TRACE-QUEUE] waitForTask: queue=%s, returning first task\n", q.Name)
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
