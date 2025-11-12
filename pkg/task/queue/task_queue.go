package queue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

	waitInProgress atomic.Bool
	cancelDelay    atomic.Bool

	storage *TaskStorage

	started atomic.Bool // a flag to ignore multiple starts

	Name    string
	Handler func(ctx context.Context, t task.Task) TaskResult
	status  *TaskQueueStatus

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

	// Hook tasks counter for metrics (tracks tasks by hook name)
	hookTasksCounter map[string]uint

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
		q.logger = q.logger.Named(q.Name)
	}
}

// NewTasksQueue creates a new TaskQueue with the provided options
func NewTasksQueue(name string, metricStorage metricsstorage.Storage, opts ...TaskQueueOption) *TaskQueue {
	q := &TaskQueue{
		Name:    name,
		storage: newTaskStorage(),
		// Default timings
		WaitLoopCheckInterval: DefaultWaitLoopCheckInterval,
		DelayOnQueueIsEmpty:   DefaultDelayOnQueueIsEmpty,
		DelayOnRepeat:         DefaultDelayOnRepeat,
		ExponentialBackoffFn: func(failureCount int) time.Duration {
			return exponential_backoff.CalculateDelay(DefaultInitialDelayOnFailedTask, failureCount)
		},
		logger: log.NewLogger().Named("task_queue").Named(name),
		// Pre-allocate buffers
		contextBuffer:    make([]bindingcontext.BindingContext, 0, 128),
		monitorIDBuffer:  make([]string, 0, 128),
		groupBuffer:      make(map[string]*compactionGroup, 32),
		metricStorage:    metricStorage,
		status:           NewTaskQueueStatus(),
		hookTasksCounter: make(map[string]uint, 32),
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

// getHookName extracts hook name from task metadata if available
func (q *TaskQueue) getHookName(t task.Task) string {
	metadata := t.GetMetadata()
	if metadata == nil {
		return ""
	}

	hookNameAccessor, ok := metadata.(task_metadata.HookNameAccessor)
	if !ok {
		return ""
	}

	return hookNameAccessor.GetHookName()
}

// updateHookTasksCounter updates the hook tasks counter and metrics
func (q *TaskQueue) updateHookTasksCounter(hookName string, delta int) {
	if hookName == "" || q.metricStorage == nil {
		return
	}

	q.hookTasksCounter[hookName] = uint(int(q.hookTasksCounter[hookName]) + delta)

	count := q.hookTasksCounter[hookName]

	labels := map[string]string{
		"queue_name": q.Name,
		"hook":       hookName,
	}

	// Only update metric if count > 20, or set to 0 if count <= 20
	if count > 20 {
		q.metricStorage.GaugeSet(metrics.TasksQueueCompactionTasksByHook, float64(count), labels)
	} else {
		// Set to 0 to remove from metrics when count drops below threshold
		if count == 0 {
			delete(q.hookTasksCounter, hookName)
		}
		q.metricStorage.GaugeSet(metrics.TasksQueueCompactionTasksByHook, 0, labels)
	}
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

	return q.status.Get()
}

func (q *TaskQueue) GetStatusType() QueueStatusType {
	return q.status.GetType()
}

func (q *TaskQueue) SetStatus(statusType QueueStatusType) {
	q.status.Set(statusType)
}

func (q *TaskQueue) SetStatusText(text string) {
	q.status.SetText(text)
}

func (q *TaskQueue) Length() int {
	defer q.MeasureActionTime("Length")()

	return q.storage.Length()
}

// AddFirst adds new head element.
func (q *TaskQueue) AddFirst(tasks ...task.Task) {
	defer q.MeasureActionTime("AddFirst")()

	q.storage.AddFirst(tasks...)

	// Update queueTasksCounter for each added task
	for _, t := range tasks {
		q.queueTasksCounter.Add(t)

		// Update hook tasks counter
		hookName := q.getHookName(t)
		if hookName != "" {
			q.updateHookTasksCounter(hookName, 1)
		}
	}
}

// RemoveFirst deletes a head element, so head is moved.
func (q *TaskQueue) RemoveFirst() task.Task {
	defer q.MeasureActionTime("RemoveFirst")()

	t := q.storage.RemoveFirst()
	if t != nil {
		q.queueTasksCounter.Remove(t)

		// Update hook tasks counter
		hookName := q.getHookName(t)
		if hookName != "" {
			q.updateHookTasksCounter(hookName, -1)
		}
	}

	return t
}

// GetFirst returns a head element.
func (q *TaskQueue) GetFirst() task.Task {
	defer q.MeasureActionTime("GetFirst")()

	return q.storage.GetFirst()
}

// addLast adds a new tail element.
// It implements the merging logic for HookRun tasks by scanning the whole queue.
func (q *TaskQueue) AddLast(tasks ...task.Task) {
	defer q.MeasureActionTime("AddLast")()

	for _, t := range tasks {
		q.lazydebug("adding task to queue", func() []any {
			return []any{
				slog.String("queue", q.Name),
				slog.String("task_id", t.GetId()),
				slog.String("task_type", string(t.GetType())),
				slog.String("task_description", t.GetDescription()),
				slog.Int("queue_length_before", q.storage.Length()),
			}
		})

		if q.storage.Get(t.GetId()) != nil {
			q.logger.Warn("task collision detected, unexpected behavior possible", slog.String("queue", q.Name), slog.String("task_id", t.GetId()))
		}

		q.storage.AddLast(t)
		q.queueTasksCounter.Add(t)

		// Update hook tasks counter
		hookName := q.getHookName(t)
		if hookName != "" {
			q.updateHookTasksCounter(hookName, 1)
		}

		// TODO: skip compactable check if is already compactable

		taskType := t.GetType()
		if _, ok := q.compactableTypes[taskType]; ok {
			q.lazydebug("task is mergeable, marking queue as dirty", func() []any {
				return []any{
					slog.String("queue", q.Name),
					slog.String("task_id", t.GetId()),
					slog.String("task_type", string(taskType)),
					slog.Int("queue_length", q.storage.Length()),
					slog.Bool("queue_is_dirty", q.queueTasksCounter.IsAnyCapReached()),
				}
			})

			// Only trigger compaction if queue is getting long and we have mergeable tasks
			if q.storage.Length() > compactionThreshold && q.queueTasksCounter.IsAnyCapReached() {
				q.lazydebug("triggering compaction due to queue length", func() []any {
					return []any{
						slog.String("queue", q.Name),
						slog.Int("queue_length", q.storage.Length()),
						slog.Int("compaction_threshold", compactionThreshold),
					}
				})

				currentQueue := q.storage.Length()
				q.compaction(q.queueTasksCounter.GetReachedCap())

				q.lazydebug("compaction finished", func() []any {
					return []any{
						slog.String("queue", q.Name),
						slog.Int("queue_length_before", currentQueue),
						slog.Int("queue_length_after", q.storage.Length()),
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
	if q.storage.Length() < 2 {
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
	q.storage.Iterate(func(e *list.Element[task.Task]) {
		t := e.Value
		taskType := t.GetType()

		if _, ok := q.compactableTypes[taskType]; !ok {
			return
		}

		if len(compactionIDs) > 0 {
			if _, ok := compactionIDs[t.GetCompactionID()]; !ok {
				return
			}
		}

		metadata := t.GetMetadata()
		if isNil(metadata) || t.IsProcessing() {
			return
		}

		hookNameAcessor, ok := metadata.(task_metadata.HookNameAccessor)
		if !ok {
			return
		}
		bindingContextAcessor, ok := metadata.(task_metadata.BindingContextAccessor)
		if !ok {
			return
		}
		monitorIDsAcessor, ok := metadata.(task_metadata.MonitorIDAccessor)
		if !ok {
			return
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
	})

	// Second pass: merge with pooled slices
	for hookName, group := range hookGroups {
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

			q.storage.RemoveElement(elementToMerge)
			q.queueTasksCounter.Remove(taskToMerge)

			// Update hook tasks counter for removed task
			mergeHookName := q.getHookName(taskToMerge)
			if mergeHookName != "" {
				q.updateHookTasksCounter(mergeHookName, -1)
			}
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

		// Record compaction operation metric
		if q.metricStorage != nil && len(group.elementsToMerge) > 0 {
			q.metricStorage.CounterAdd(metrics.TasksQueueCompactionOperationsTotal, 1, map[string]string{
				"queue_name": q.Name,
				"hook":       hookName,
			})
		}

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

	// is empty, nothing to do
	if q.Length() == 0 {
		return nil
	}

	t := q.storage.RemoveLast()
	if t != nil {
		q.queueTasksCounter.Remove(t)

		// Update hook tasks counter
		hookName := q.getHookName(t)
		if hookName != "" {
			q.updateHookTasksCounter(hookName, -1)
		}
	}

	return t
}

// GetLast returns a tail element.
func (q *TaskQueue) GetLast() task.Task {
	defer q.MeasureActionTime("GetLast")()
	return q.storage.GetLast()
}

// Get returns a task by id.
func (q *TaskQueue) Get(id string) task.Task {
	defer q.MeasureActionTime("Get")()
	return q.storage.Get(id)
}

// AddAfter inserts a task after the task with specified id.
func (q *TaskQueue) AddAfter(id string, tasks ...task.Task) {
	defer q.MeasureActionTime("AddAfter")()

	q.storage.AddAfter(id, tasks...)

	// Update queueTasksCounter for each added task
	for _, t := range tasks {
		q.queueTasksCounter.Add(t)

		// Update hook tasks counter
		hookName := q.getHookName(t)
		if hookName != "" {
			q.updateHookTasksCounter(hookName, 1)
		}
	}
}

// AddBefore inserts a task before the task with specified id.
func (q *TaskQueue) AddBefore(id string, newTask task.Task) {
	defer q.MeasureActionTime("AddBefore")()

	q.storage.AddBefore(id, newTask)
	q.queueTasksCounter.Add(newTask)

	// Update hook tasks counter
	hookName := q.getHookName(newTask)
	if hookName != "" {
		q.updateHookTasksCounter(hookName, 1)
	}
}

// Remove finds element by id and deletes it.
func (q *TaskQueue) Remove(id string) task.Task {
	defer q.MeasureActionTime("Remove")()

	t := q.storage.Remove(id)
	if t != nil {
		q.queueTasksCounter.Remove(t)

		// Update hook tasks counter
		hookName := q.getHookName(t)
		if hookName != "" {
			q.updateHookTasksCounter(hookName, -1)
		}
	}

	return t
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
	if q.started.Load() {
		return
	}

	if q.Handler == nil {
		log.Error("should set handler before start in queue", slog.String("name", q.Name))
		q.SetStatus(QueueStatusNoHandlerSet)
		return
	}

	go func() {
		q.SetStatus(QueueStatusIdle)
		var sleepDelay time.Duration
		for {
			q.logger.Debug("queue: wait for task", slog.String("queue", q.Name), slog.Duration("sleep_delay", sleepDelay))
			t := q.waitForTask(sleepDelay)
			if t == nil {
				q.SetStatus(QueueStatusStop)
				q.logger.Info("queue stopped", slog.String("name", q.Name))
				return
			}

			q.withLock(func() {
				if q.queueTasksCounter.IsAnyCapReached() {
					q.lazydebug("triggering compaction before task processing", func() []any {
						return []any{slog.String("queue", q.Name), slog.String("task_id", t.GetId()), slog.String("task_type", string(t.GetType())), slog.Int("queue_length", q.storage.Length())}
					})

					q.compaction(q.queueTasksCounter.GetReachedCap())

					q.lazydebug("compaction completed, queue no longer dirty", func() []any {
						return []any{slog.String("queue", q.Name), slog.Int("queue_length_after", q.storage.Length())}
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
			q.SetStatus(QueueStatusRunningTask)

			defer func() {
				if r := recover(); r != nil {
					q.logger.Warn("panic recovered in Start", slog.Any("error", r))
				}
			}()
			taskRes := q.Handler(ctx, t)

			// Check Done channel after long-running operation.
			select {
			case <-q.ctx.Done():
				q.logger.Info("queue stopped after task handling", slog.String("name", q.Name))
				q.SetStatus(QueueStatusStop)
				return
			default:
			}

			switch taskRes.Status {
			case Success, Keep:
				// Insert new tasks right after the current task in reverse order.
				q.withLock(func() {
					q.storage.ProcessResult(taskRes, t)

					t.SetProcessing(false) // release processing flag
				})

				q.SetStatus(QueueStatusIdle)
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
				q.SetStatusText(fmt.Sprintf("sleep after fail for %s", nextSleepDelay.String()))
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
				q.SetStatus(QueueStatusRepeatTask)
			}

			if taskRes.DelayBeforeNextTask != 0 {
				nextSleepDelay = taskRes.DelayBeforeNextTask
				q.SetStatusText(fmt.Sprintf("sleep for %s", nextSleepDelay.String()))
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

	q.started.Store(true)
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
	if q.Length() > 0 && sleepDelay == 0 {
		return q.GetFirst()
	}

	// Initialize wait settings.
	waitBegin := time.Now()
	waitUntil := q.DelayOnQueueIsEmpty
	if sleepDelay != 0 {
		waitUntil = sleepDelay
	}

	checkTicker := time.NewTicker(q.WaitLoopCheckInterval)
	q.waitInProgress.Store(true)
	q.cancelDelay.Store(false)

	// Snapshot original status
	origStatusType, origStatusText := q.status.Snapshot()

	defer func() {
		checkTicker.Stop()
		q.waitInProgress.Store(false)
		q.cancelDelay.Store(false)
		// Restore original status
		q.status.Restore(origStatusType, origStatusText)
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

			if q.cancelDelay.Load() {
				// Reset waitUntil to check task immediately.
				waitUntil = elapsed
			}

			// Wait loop is done or canceled: break select to check for the head task.
			if elapsed >= waitUntil {
				// Increase waitUntil to wait on the next iteration and go check for the head task.
				checkTask = true
			}
		}

		// Break the for-loop to see if the head task can be returned.
		if checkTask {
			// is empty
			if q.Length() == 0 {
				// No task to return: increase wait time.
				waitUntil += q.DelayOnQueueIsEmpty
			} else {
				return q.GetFirst()
			}
		}

		// Wait loop still in progress: update queue status.
		waitTime := time.Since(waitBegin).Truncate(time.Second)
		if sleepDelay == 0 {
			q.SetStatusText(fmt.Sprintf("waiting for task %s", waitTime.String()))
		} else {
			delay := sleepDelay.Truncate(time.Second)
			origStatusStr := origStatusText
			if origStatusStr == "" {
				origStatusStr = origStatusType.String()
			}
			q.SetStatusText(fmt.Sprintf("%s (%s left of %s delay)", origStatusStr, (delay - waitTime).String(), delay.String()))
		}
	}
}

// CancelTaskDelay breaks wait loop. Useful to break the possible long sleep delay.
func (q *TaskQueue) CancelTaskDelay() {
	if q.waitInProgress.Load() {
		q.cancelDelay.Store(true)
	}
}

// IterateSnapshot creates a snapshot of all tasks and iterates over the copy.
// This is safer than Iterate() when you need to call queue methods inside the callback,
// as no locks are held during callback execution.
//
// Note: The snapshot may become stale during iteration if tasks are added/removed
// by other goroutines or by the callback itself.
//
// Use this method when:
//   - You need to call queue methods inside the callback (Add, Length, Filter, etc.)
//   - You need to process tasks asynchronously
//   - Safety is more important than performance
//
// Memory overhead: O(n) where n is the number of tasks in the queue.
func (q *TaskQueue) IterateSnapshot(doFn func(task.Task)) {
	if doFn == nil {
		return
	}

	defer q.MeasureActionTime("IterateSnapshot")()

	// Create snapshot under lock
	snapshot := q.GetSnapshot()

	defer func() {
		if r := recover(); r != nil {
			q.logger.Warn("panic recovered in IterateSnapshot", slog.Any("error", r))
		}
	}()

	// Execute callbacks without holding any locks
	for _, t := range snapshot {
		doFn(t)
	}
}

// GetSnapshot returns a copy of all tasks in the queue.
// This is useful for external iteration or processing without holding locks.
//
// The returned slice is a snapshot at the time of the call and will not reflect
// subsequent changes to the queue.
func (q *TaskQueue) GetSnapshot() []task.Task {
	return q.storage.GetSnapshot()
}

// DeleteFunc runs fn on every task and removes each task for which fn returns false.
func (q *TaskQueue) DeleteFunc(fn func(task.Task) bool) {
	if fn == nil {
		return
	}

	defer q.MeasureActionTime("DeleteFunc")()

	q.storage.DeleteFunc(func(t task.Task) bool {
		if !fn(t) {
			q.queueTasksCounter.Remove(t)

			// Update hook tasks counter
			hookName := q.getHookName(t)
			if hookName != "" {
				q.updateHookTasksCounter(hookName, -1)
			}

			return false
		}

		return true
	})
}

// TODO define mapping method with QueueAction to insert, modify and delete tasks.

// Dump tasks in queue to one line
func (q *TaskQueue) String() string {
	var buf strings.Builder
	var index int

	qLen := q.Length()

	q.IterateSnapshot(func(t task.Task) {
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
	defer func() {
		q.m.Unlock()

		if r := recover(); r != nil {
			q.logger.Warn("panic recovered in withLock", slog.Any("error", r))
		}
	}()

	fn()
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
