package context

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/log"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/hook/types"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	shell_operator "github.com/flant/shell-operator/pkg/shell-operator"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

// TestTaskType is a type for tasks created by ContextCombiner.
const TestTaskType task.TaskType = "TestTask"

// TestQueueName is a name of the queue created by ContextCombiner.
const TestQueueName string = "test-queue"

// ContextCombiner is used to bring logic behind the combineBindingContextForHook method
// into tests. This method requires a ShellOperator instance with at least one queue
// with tasks.
type ContextCombiner struct {
	op *shell_operator.ShellOperator
	q  *queue.TaskQueue
}

func NewContextCombiner() *ContextCombiner {
	op := &shell_operator.ShellOperator{}
	op.MetricStorage = metricstorage.NewMetricStorage(context.Background(), "test-prefix", false, log.NewNop())
	op.TaskQueues = queue.NewTaskQueueSet().WithMetricStorage(op.MetricStorage)
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue(TestQueueName, nil)

	return &ContextCombiner{
		op: op,
		q:  op.TaskQueues.GetByName(TestQueueName),
	}
}

func (c *ContextCombiner) AddBindingContext(bindingType types.BindingType, info controller.BindingExecutionInfo) {
	t := task.NewTask(TestTaskType)
	t.WithMetadata(task_metadata.HookMetadata{
		Binding:        info.Binding,
		Group:          info.Group,
		BindingType:    bindingType,
		BindingContext: info.BindingContext,
		AllowFailure:   info.AllowFailure,
	})
	t.WithQueueName(TestQueueName)
	c.q.AddLast(t)
}

// CombinedContext returns a combined context or a binding context
// from the first task.
func (c *ContextCombiner) Combined() []bindingcontext.BindingContext {
	firstTask := c.q.GetFirst()
	if firstTask == nil {
		return []bindingcontext.BindingContext{}
	}
	taskMeta := firstTask.GetMetadata()

	bc := taskMeta.(task_metadata.BindingContextAccessor).GetBindingContext()

	combineResult := c.op.CombineBindingContextForHook(c.q, c.q.GetFirst(), nil)
	if combineResult != nil {
		bc = combineResult.BindingContexts
	}

	return bc
}

func (c *ContextCombiner) CombinedAndUpdated(hookCtrl *controller.HookController) (GeneratedBindingContexts, error) {
	bc := c.Combined()
	bc = hookCtrl.UpdateSnapshots(bc)
	return ConvertToGeneratedBindingContexts(bc)
}

func (c *ContextCombiner) QueueLen() int {
	return c.q.Length()
}

func ConvertToGeneratedBindingContexts(bindingContexts []bindingcontext.BindingContext) (GeneratedBindingContexts, error) {
	res := GeneratedBindingContexts{}

	// Support only v1 binding contexts.
	bcList := bindingcontext.ConvertBindingContextList("v1", bindingContexts)
	data, err := bcList.Json()
	if err != nil {
		return res, fmt.Errorf("marshaling binding context error: %v", err)
	}

	res.BindingContexts = bindingContexts
	res.Rendered = string(data)
	return res, nil
}
