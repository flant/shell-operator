package shell_operator

import (
	"fmt"
	"log/slog"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

type CombineResult struct {
	BindingContexts []bctx.BindingContext
	MonitorIDs      []string
}

// combineBindingContextForHook combines binding contexts from a sequence of task with similar
// hook name and task type into array of binding context and delete excess tasks from queue.
//
// Also, sequences of binding contexts with similar group are compacted in one binding context.
//
// If input task has no metadata, result will be nil.
// Metadata should implement HookNameAccessor, BindingContextAccessor and MonitorIDAccessor interfaces.
// DEV WARNING! Do not use HookMetadataAccessor here. Use only *Accessor interfaces because this method is used from addon-operator.
func (op *ShellOperator) combineBindingContextForHook(tqs *queue.TaskQueueSet, q *queue.TaskQueue, t task.Task) *CombineResult {
	if q == nil {
		return nil
	}
	taskMeta := t.GetMetadata()
	if taskMeta == nil {
		// Ignore task without metadata
		return nil
	}
	hookName := taskMeta.(HookNameAccessor).GetHookName()

	fmt.Printf("[TRACE] combineBindingContextForHook called: hook=%s, task=%s\n", hookName, t.GetId())

	res := new(CombineResult)

	otherTasks := make([]task.Task, 0)
	stopIterate := false
	q.Iterate(func(tsk task.Task) {
		if stopIterate {
			return
		}

		fmt.Printf("[TRACE] Iterating task in queue: taskId=%s, hookName=%s\n", tsk.GetId(), hookName)

		// ignore current task
		if tsk.GetId() == t.GetId() {
			return
		}

		hm := tsk.GetMetadata()

		// Stop on task without metadata
		if hm == nil {
			stopIterate = true
			return
		}

		nextHookName := hm.(HookNameAccessor).GetHookName()

		// Only tasks for the same hook and of the same type can be combined (HookRun cannot be combined with OnStartup).
		// Using stopCombineFn function more stricter combine rules can be defined.
		if nextHookName != hookName || t.GetType() != tsk.GetType() {
			stopIterate = true
		}

		if !stopIterate {
			otherTasks = append(otherTasks, tsk)
		}
	})

	// no tasks found to combine
	if len(otherTasks) == 0 {
		return nil
	}

	fmt.Printf("[TRACE] Found tasks to combine: count=%d\n", len(otherTasks))

	// Combine binding context and make a map to delete excess tasks
	combinedContext := make([]bctx.BindingContext, 0)
	monitorIDs := taskMeta.(MonitorIDAccessor).GetMonitorIDs()
	tasksFilter := make(map[string]bool)
	// current task always remain in queue
	combinedContext = append(combinedContext, taskMeta.(BindingContextAccessor).GetBindingContext()...)
	tasksFilter[t.GetId()] = true
	for _, tsk := range otherTasks {
		combinedContext = append(combinedContext, tsk.GetMetadata().(BindingContextAccessor).GetBindingContext()...)
		tskMonitorIDs := tsk.GetMetadata().(MonitorIDAccessor).GetMonitorIDs()
		if len(tskMonitorIDs) > 0 {
			monitorIDs = append(monitorIDs, tskMonitorIDs...)
		}
		tasksFilter[tsk.GetId()] = false
	}

	fmt.Printf("[TRACE] Combined binding contexts before compaction: contexts=%+v\n", combinedContext)

	// Delete tasks with false in tasksFilter map
	tqs.GetByName(t.GetQueueName()).Filter(func(tsk task.Task) bool {
		if v, ok := tasksFilter[tsk.GetId()]; ok {
			return v
		}
		return true
	})

	// group is used to compact binding contexts when only snapshots are needed
	compactedContext := make([]bctx.BindingContext, 0)
	for i := 0; i < len(combinedContext); i++ {
		keep := true

		// Binding context is ignored if next binding context has the similar group.
		groupName := combinedContext[i].Metadata.Group
		if groupName != "" && (i+1 <= len(combinedContext)-1) && combinedContext[i+1].Metadata.Group == groupName {
			keep = false
		}
		fmt.Printf("[TRACE] Compacting binding context: group=%s, keep=%t\n", groupName, keep)

		if keep {
			compactedContext = append(compactedContext, combinedContext[i])
		}
	}

	// Describe what was done.
	compactMsg := ""
	if len(compactedContext) < len(combinedContext) {
		compactMsg = fmt.Sprintf("are combined and compacted to %d contexts", len(compactedContext))
	} else {
		compactMsg = fmt.Sprintf("are combined to %d contexts", len(combinedContext))
	}
	op.logger.Info("Binding contexts from are dropped from queue",
		slog.Int("count", len(otherTasks)+1),
		slog.String("message", compactMsg),
		slog.Int("dropped", len(tasksFilter)-1),
		slog.String("queue", t.GetQueueName()))

	res.BindingContexts = compactedContext
	res.MonitorIDs = monitorIDs
	return res
}
