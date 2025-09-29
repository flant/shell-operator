package queue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/utils/exponential_backoff"
	"github.com/flant/shell-operator/pkg/utils/list"
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

const compactionThreshold = 100

var (
	DefaultWaitLoopCheckInterval    = 125 * time.Millisecond
	DefaultDelayOnQueueIsEmpty      = 250 * time.Millisecond
	DefaultInitialDelayOnFailedTask = 5 * time.Second
	DefaultDelayOnRepeat            = 25 * time.Millisecond
)

// Global object pools for reducing allocations
var (
	compactionGroupPool = sync.Pool{
		New: func() interface{} {
			return &compactionGroup{
				elementsToMerge: make([]*list.Element[task.Task], 0, 8),
			}
		},
	}

	contextSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]bindingcontext.BindingContext, 0, 64)
			return &slice
		},
	}

	monitorIDSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]string, 0, 64)
			return &slice
		},
	}

	hookGroupsMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]*compactionGroup, 32)
		},
	}
)

type compactionGroup struct {
	targetElement   *list.Element[task.Task]
	elementsToMerge []*list.Element[task.Task]
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
	logger *log.Logger

	m             sync.RWMutex
	metricStorage metricsstorage.Storage
	ctx           context.Context
	cancel        context.CancelFunc

	waitMu         sync.Mutex
	waitInProgress bool
	cancelDelay    bool

	items   *list.List[task.Task]
	idIndex map[string]*list.Element[task.Task]

	started bool // a flag to ignore multiple starts

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

	// Compaction
	compactionCallback func(compactedTasks []task.Task, targetTask task.Task)

	compactableTypes map[task.TaskType]struct{}

	queueTasksCounter *TaskCounter

	// Pre-allocated buffers for compaction
	contextBuffer   []bindingcontext.BindingContext
	monitorIDBuffer []string
	groupBuffer     map[string]*compactionGroup
}

// TaskQueueOption defines a functional option for TaskQueue configuration
type TaskQueueOption func(*TaskQueue)

// WithContext sets the context for the TaskQueue
func WithContext(ctx context.Context) TaskQueueOption {
	return func(q *TaskQueue) {
		q.ctx, q.cancel = context.WithCancel(ctx)
	}
}

// WithName sets the name for the TaskQueue
func WithName(name string) TaskQueueOption {
	return func(q *TaskQueue) {
		q.Name = name
	}
}

// WithHandler sets the task handler for the TaskQueue
func WithHandler(fn func(ctx context.Context, t task.Task) TaskResult) TaskQueueOption {
	return func(q *TaskQueue) {
		q.Handler = fn
	}
}

// WithCompactableTypes sets the compactable task types for the TaskQueue
func WithCompactableTypes(taskTypes ...task.TaskType) TaskQueueOption {
	return func(q *TaskQueue) {
		q.compactableTypes = make(map[task.TaskType]struct{}, len(taskTypes))
		for _, taskType := range taskTypes {
			q.compactableTypes[taskType] = struct{}{}
		}
	}
}

func WithCompactionCallback(callback func(compactedTasks []task.Task, targetTask task.Task)) TaskQueueOption {
	return func(q *TaskQueue) {
		q.compactionCallback = callback
	}
}

func WithLogger(logger *log.Logger) TaskQueueOption {
	return func(q *TaskQueue) {
		q.logger = logger
	}
}

// NewTasksQueue creates a new TaskQueue with the provided options
func NewTasksQueue(metricStorage metricsstorage.Storage, opts ...TaskQueueOption) *TaskQueue {
	q := &TaskQueue{
		items:   list.New[task.Task](),
		idIndex: make(map[string]*list.Element[task.Task]),
		// Default timings
		WaitLoopCheckInterval: DefaultWaitLoopCheckInterval,
		DelayOnQueueIsEmpty:   DefaultDelayOnQueueIsEmpty,
		DelayOnRepeat:         DefaultDelayOnRepeat,
		ExponentialBackoffFn: func(failureCount int) time.Duration {
			return exponential_backoff.CalculateDelay(DefaultInitialDelayOnFailedTask, failureCount)
		},
		logger: log.NewNop(),
		// Pre-allocate buffers
		contextBuffer:   make([]bindingcontext.BindingContext, 0, 128),
		monitorIDBuffer: make([]string, 0, 128),
		groupBuffer:     make(map[string]*compactionGroup, 32),
		metricStorage:   metricStorage,
	}

	// Apply all options
	for _, opt := range opts {
		opt(q)
	}

	q.queueTasksCounter = NewTaskCounter(q.Name, q.compactableTypes, q.metricStorage)

	return q
}

func (q *TaskQueue) getCompactionGroup() *compactionGroup {
	return compactionGroupPool.Get().(*compactionGroup)
}

func (q *TaskQueue) putCompactionGroup(cg *compactionGroup) {
	cg.reset()
	compactionGroupPool.Put(cg)
}

func (q *TaskQueue) getContextSlice() []bindingcontext.BindingContext {
	return *contextSlicePool.Get().(*[]bindingcontext.BindingContext)
}

func (q *TaskQueue) putContextSlice(slice *[]bindingcontext.BindingContext) {
	*slice = (*slice)[:0] // Reset slice
	contextSlicePool.Put(slice)
}

func (q *TaskQueue) getMonitorIDSlice() []string {
	return *monitorIDSlicePool.Get().(*[]string)
}

func (q *TaskQueue) putMonitorIDSlice(slice *[]string) {
	*slice = (*slice)[:0] // Reset slice
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

// MeasureActionTime is a helper to measure execution time of queue's actions
func (q *TaskQueue) MeasureActionTime(action string) func() {
	if q.metricStorage == nil {
		return func() {}
	}

	q.measureActionFnOnce.Do(func() {
		// TODO: remove this metrics switch
		if os.Getenv("QUEUE_ACTIONS_METRICS") == "no" {
			q.measureActionFn = func() {}
		} else {
			q.measureActionFn = measure.Duration(func(d time.Duration) {
				q.metricStorage.HistogramObserve(metrics.TasksQueueActionDurationSeconds, d.Seconds(), map[string]string{"queue_name": q.Name, "queue_action": action}, nil)
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
func (q *TaskQueue) AddFirst(tasks ...task.Task) {
	defer q.MeasureActionTime("AddFirst")()
	q.withLock(func() {
		q.addFirst(tasks...)
	})
}

// addFirst adds new head element.
func (q *TaskQueue) addFirst(tasks ...task.Task) {
	// Also, add tasks in reverse order
	// at the start of the queue. The first task in HeadTasks
	// become the new first task in the queue.
	for i := len(tasks) - 1; i >= 0; i-- {
		element := q.items.PushFront(tasks[i])
		q.idIndex[tasks[i].GetId()] = element

		q.queueTasksCounter.Add(tasks[i])
	}
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
	t := q.items.Remove(element)
	delete(q.idIndex, t.GetId())

	q.queueTasksCounter.Remove(t)

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
	return q.items.Front().Value
}

// AddLast adds new tail element.
func (q *TaskQueue) AddLast(tasks ...task.Task) {
	defer q.MeasureActionTime("AddLast")()
	q.withLock(func() {
		q.addLast(tasks...)
	})
}

// addLast adds a new tail element.
// It implements the merging logic for HookRun tasks by scanning the whole queue.
func (q *TaskQueue) addLast(tasks ...task.Task) {
	for _, t := range tasks {
		q.lazydebug("adding task to queue", func() []any {
			return []any{
				slog.String("queue", q.Name),
				slog.String("task_id", t.GetId()),
				slog.String("task_type", string(t.GetType())),
				slog.String("task_description", t.GetDescription()),
				slog.Int("queue_length_before", q.items.Len()),
			}
		})

		if _, ok := q.idIndex[t.GetId()]; ok {
			q.logger.Warn("task collision detected, unexpected behavior possible", slog.String("queue", q.Name), slog.String("task_id", t.GetId()))
		}

		element := q.items.PushBack(t)
		q.idIndex[t.GetId()] = element
		q.queueTasksCounter.Add(t)

		// TODO: skip compactable check if is already compactable

		taskType := t.GetType()
		if _, ok := q.compactableTypes[taskType]; ok {
			q.lazydebug("task is mergeable, marking queue as dirty", func() []any {
				return []any{
					slog.String("queue", q.Name),
					slog.String("task_id", t.GetId()),
					slog.String("task_type", string(taskType)),
					slog.Int("queue_length", q.items.Len()),
					slog.Bool("queue_is_dirty", q.queueTasksCounter.IsAnyCapReached()),
				}
			})

			// Only trigger compaction if queue is getting long and we have mergeable tasks
			if q.items.Len() > compactionThreshold && q.queueTasksCounter.IsAnyCapReached() {
				q.lazydebug("triggering compaction due to queue length", func() []any {
					return []any{
						slog.String("queue", q.Name),
						slog.Int("queue_length", q.items.Len()),
						slog.Int("compaction_threshold", compactionThreshold),
					}
				})

				currentQueue := q.items.Len()
				q.compaction(q.queueTasksCounter.GetReachedCap())

				q.lazydebug("compaction finished", func() []any {
					return []any{
						slog.String("queue", q.Name),
						slog.Int("queue_length_before", currentQueue),
						slog.Int("queue_length_after", q.items.Len()),
					}
				})
			}
		} else {
			q.lazydebug("task is not mergeable", func() []any {
				return []any{
					slog.String("queue", q.Name),
					slog.String("task_id", t.GetId()),
					slog.String("task_type", string(taskType)),
				}
			})
		}
	}
}

// compaction merges HookRun tasks for the same hook.
// It iterates through the list once, making it an O(N) operation.
// DEV WARNING! Do not use HookMetadataAccessor here. Use only *Accessor interfaces because this method is used from addon-operator.
// TODO: consider compaction only for tasks with one compaction ID (to not affect good tasks)
func (q *TaskQueue) compaction(compactionIDs map[string]struct{}) {
	if q.items.Len() < 2 {
		return
	}

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
		t := e.Value
		taskType := t.GetType()

		if _, ok := q.compactableTypes[taskType]; !ok {
			continue
		}

		if len(compactionIDs) > 0 {
			if _, ok := compactionIDs[t.GetCompactionID()]; !ok {
				continue
			}
		}

		metadata := t.GetMetadata()
		if isNil(metadata) || t.IsProcessing() {
			continue
		}

		hookNameAcessor, ok := metadata.(task_metadata.HookNameAccessor)
		if !ok {
			continue
		}
		bindingContextAcessor, ok := metadata.(task_metadata.BindingContextAccessor)
		if !ok {
			continue
		}
		monitorIDsAcessor, ok := metadata.(task_metadata.MonitorIDAccessor)
		if !ok {
			continue
		}

		hookName := hookNameAcessor.GetHookName()
		bindingContext := bindingContextAcessor.GetBindingContext()
		monitorIDs := monitorIDsAcessor.GetMonitorIDs()

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

		targetTask := group.targetElement.Value
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

		targetBindingContextAcessor, ok := targetMetadata.(task_metadata.BindingContextAccessor)
		if !ok {
			continue
		}
		targetMonitorIDsAcessor, ok := targetMetadata.(task_metadata.MonitorIDAccessor)
		if !ok {
			continue
		}

		// Add target's contexts first
		targetContexts := targetBindingContextAcessor.GetBindingContext()
		targetMonitorIDs := targetMonitorIDsAcessor.GetMonitorIDs()

		newContexts = append(newContexts, targetContexts...)
		newMonitorIDs = append(newMonitorIDs, targetMonitorIDs...)

		// Append contexts from other tasks and remove them
		for _, elementToMerge := range group.elementsToMerge {
			taskToMerge := elementToMerge.Value
			mergeMetadata := taskToMerge.GetMetadata()

			mergeBindingContextAcessor, ok := mergeMetadata.(task_metadata.BindingContextAccessor)
			if !ok {
				continue
			}
			mergeMonitorIDsAcessor, ok := mergeMetadata.(task_metadata.MonitorIDAccessor)
			if !ok {
				continue
			}

			mergeContexts := mergeBindingContextAcessor.GetBindingContext()
			mergeMonitorIDs := mergeMonitorIDsAcessor.GetMonitorIDs()

			newContexts = append(newContexts, mergeContexts...)
			newMonitorIDs = append(newMonitorIDs, mergeMonitorIDs...)

			q.items.Remove(elementToMerge)
			delete(q.idIndex, taskToMerge.GetId())
			q.queueTasksCounter.Remove(taskToMerge)
		}

		// Update target task with compacted slices
		compactedContexts := compactBindingContexts(newContexts)

		// Setters return new metadata, not the same pointer, so we need to update targetTask with the new metadata
		targetBindingContextSetter, ok := targetMetadata.(task_metadata.BindingContextSetter)
		if !ok {
			continue
		}

		withContext := targetBindingContextSetter.SetBindingContext(compactedContexts)

		targetMonitorIDsSetter, ok := withContext.(task_metadata.MonitorIDSetter)
		if !ok {
			continue
		}

		withContext = targetMonitorIDsSetter.SetMonitorIDs(newMonitorIDs)
		targetTask.UpdateMetadata(withContext)

		// Call compaction callback if set
		if q.compactionCallback != nil && len(group.elementsToMerge) > 0 {
			compactedTasks := make([]task.Task, 0, len(group.elementsToMerge))
			for _, elementToMerge := range group.elementsToMerge {
				compactedTasks = append(compactedTasks, elementToMerge.Value)
			}
			q.compactionCallback(compactedTasks, targetTask)
		}

		// Return slices to pools
		q.putContextSlice(&newContexts)
		q.putMonitorIDSlice(&newMonitorIDs)
	}

	q.queueTasksCounter.ResetReachedCap()
}

// compactBindingContexts mimics the logic from shell-operator's CombineBindingContextForHook.
// It removes intermediate states for the same group, keeping only the most recent one.
func compactBindingContexts(combinedContext []bindingcontext.BindingContext) []bindingcontext.BindingContext {
	if len(combinedContext) < 2 {
		return combinedContext
	}

	// Count ungrouped contexts and estimate result size
	ungroupedCount := 0
	groupCount := 0
	for _, bc := range combinedContext {
		if bc.Metadata.Group == "" {
			ungroupedCount++
		} else {
			groupCount++
		}
	}

	// If no grouped contexts, return as is
	if groupCount == 0 {
		return combinedContext
	}

	// Use a single pass approach with a map to track the last occurrence of each group
	lastIndexForGroup := make(map[string]int, groupCount)
	ungroupedIndices := make([]int, 0, ungroupedCount)

	for i, bc := range combinedContext {
		group := bc.Metadata.Group
		if group == "" {
			ungroupedIndices = append(ungroupedIndices, i)
		} else {
			lastIndexForGroup[group] = i
		}
	}

	// Pre-allocate result slice with exact capacity
	resultSize := len(ungroupedIndices) + len(lastIndexForGroup)
	result := make([]bindingcontext.BindingContext, 0, resultSize)

	// Add ungrouped contexts first (preserving their order)
	for _, idx := range ungroupedIndices {
		result = append(result, combinedContext[idx])
	}

	// Add the last occurrence of each group
	for _, idx := range lastIndexForGroup {
		result = append(result, combinedContext[idx])
	}

	return result
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
	t := q.items.Remove(element)
	delete(q.idIndex, t.GetId())

	q.queueTasksCounter.Remove(t)

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

	return q.items.Back().Value
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
		return element.Value
	}

	return nil
}

// AddAfter inserts a task after the task with specified id.
func (q *TaskQueue) AddAfter(id string, tasks ...task.Task) {
	defer q.MeasureActionTime("AddAfter")()
	q.withLock(func() {
		q.addAfter(id, tasks...)
	})
}

// addAfter inserts a task after the task with specified id.
func (q *TaskQueue) addAfter(id string, tasks ...task.Task) {
	if element, ok := q.idIndex[id]; ok {
		// Insert new tasks right after the id task in reverse order.
		for i := len(tasks) - 1; i >= 0; i-- {
			newElement := q.items.InsertAfter(tasks[i], element)
			q.idIndex[tasks[i].GetId()] = newElement

			q.queueTasksCounter.Add(tasks[i])
		}
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

		q.queueTasksCounter.Add(newTask)
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
		t := q.items.Remove(element)
		delete(q.idIndex, id)

		q.queueTasksCounter.Remove(t)

		return t
	}
	return nil
}

func (q *TaskQueue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
}

// lazydebug evaluates args only if debug log is enabled.
// It is used to avoid unnecessary allocations when logging is disabled.
// Queue MUST remain fast and not allocate memory when logging is disabled.
func (q *TaskQueue) lazydebug(msg string, argsFn func() []any) {
	if q.logger != nil && q.logger.Enabled(context.Background(), slog.LevelDebug) {
		q.logger.Debug(msg, argsFn()...)
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
			q.logger.Debug("queue: wait for task", slog.String("queue", q.Name), slog.Duration("sleep_delay", sleepDelay))
			t := q.waitForTask(sleepDelay)
			if t == nil {
				q.SetStatus("stop")
				q.logger.Info("queue stopped", slog.String("name", q.Name))
				return
			}

			q.withLock(func() {
				if q.queueTasksCounter.IsAnyCapReached() {
					q.lazydebug("triggering compaction before task processing", func() []any {
						return []any{slog.String("queue", q.Name), slog.String("task_id", t.GetId()), slog.String("task_type", string(t.GetType())), slog.Int("queue_length", q.items.Len())}
					})

					q.compaction(q.queueTasksCounter.GetReachedCap())

					q.lazydebug("compaction completed, queue no longer dirty", func() []any {
						return []any{slog.String("queue", q.Name), slog.Int("queue_length_after", q.items.Len())}
					})
				}
			})

			// set that current task is being processed, so we don't merge it with other tasks
			t.SetProcessing(true)

			// use lazydebug because it dumps whole queue and task
			q.lazydebug("queue tasks after wait", func() []any {
				return []any{
					slog.String("queue", q.Name),
					slog.String("tasks", q.String()),
				}
			})

			q.lazydebug("queue task to handle", func() []any {
				return []any{
					slog.String("queue", q.Name),
					slog.String("task_type", string(t.GetType())),
				}
			})

			var nextSleepDelay time.Duration
			q.SetStatus("run first task")
			taskRes := q.Handler(ctx, t)

			// Check Done channel after long-running operation.
			select {
			case <-q.ctx.Done():
				q.logger.Info("queue stopped after task handling", slog.String("name", q.Name))
				q.SetStatus("stop")
				return
			default:
			}

			switch taskRes.Status {
			case Success, Keep:
				// Insert new tasks right after the current task in reverse order.
				q.withLock(func() {
					q.addAfter(t.GetId(), taskRes.GetAfterTasks()...)

					if taskRes.Status == Success {
						q.remove(t.GetId())
					}
					t.SetProcessing(false) // release processing flag

					// Also, add HeadTasks in reverse order
					// at the start of the queue. The first task in HeadTasks
					// become the new first task in the queue.
					q.addFirst(taskRes.GetHeadTasks()...)

					// Add tasks to the end of the queue
					q.addLast(taskRes.GetTailTasks()...)
				})
				q.SetStatus("")
			case Fail:
				if len(taskRes.GetAfterTasks()) > 0 || len(taskRes.GetHeadTasks()) > 0 || len(taskRes.GetTailTasks()) > 0 {
					q.logger.Warn("result is fail, cannot process tasks in result",
						slog.Int("after_task_count", len(taskRes.GetAfterTasks())),
						slog.Int("head_task_count", len(taskRes.GetHeadTasks())),
						slog.Int("tail_task_count", len(taskRes.GetTailTasks())))
				}

				t.SetProcessing(false)
				// Exponential backoff delay before retry.
				nextSleepDelay = q.ExponentialBackoffFn(t.GetFailureCount())
				t.IncrementFailureCount()
				q.SetStatus(fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String()))
			case Repeat:
				if len(taskRes.GetAfterTasks()) > 0 || len(taskRes.GetHeadTasks()) > 0 || len(taskRes.GetTailTasks()) > 0 {
					q.logger.Warn("result is repeat, cannot process tasks in result",
						slog.Int("after_task_count", len(taskRes.GetAfterTasks())),
						slog.Int("head_task_count", len(taskRes.GetHeadTasks())),
						slog.Int("tail_task_count", len(taskRes.GetTailTasks())))
				}

				// repeat a current task after a small delay
				t.SetProcessing(false)
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
			// use lazydebug because it dumps whole queue
			q.lazydebug("queue: tasks after handle", func() []any {
				return []any{slog.String("queue", q.Name), slog.String("tasks", q.String())}
			})
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
			doFn(e.Value)
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
			t := current.Value
			if !filterFn(t) {
				q.items.Remove(current)
				delete(q.idIndex, t.GetId())
				q.queueTasksCounter.Remove(t)
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
