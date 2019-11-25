package task

import (
	"bytes"
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/hook"
)

type TaskType string

const (
	// a task to run a particular hook
	HookRun                  TaskType = "TASK_HOOK_RUN"
	EnableKubernetesBindings TaskType = "TASK_ENABLE_KUBERNETES_BINDINGS"

	// queue control tasks
	Delay TaskType = "TASK_DELAY"
	Stop  TaskType = "TASK_STOP"

	// Exit a program
	Exit TaskType = "TASK_EXIT"
)

type Task interface {
	GetName() string
	GetType() TaskType
	GetBinding() hook.BindingType
	GetBindingContext() []hook.BindingContext
	GetFailureCount() int
	IncrementFailureCount()
	GetDelay() time.Duration
	GetAllowFailure() bool
}

type BaseTask struct {
	FailureCount   int    // Failed executions count
	Name           string // hook name
	Type           TaskType
	Binding        hook.BindingType
	BindingContext []hook.BindingContext
	Delay          time.Duration
	AllowFailure   bool // Task considered as 'ok' if hook failed. False by default. Can be true for some schedule hooks.
}

func NewTask(taskType TaskType, name string) *BaseTask {
	return &BaseTask{
		FailureCount:   0,
		Name:           name,
		Type:           taskType,
		AllowFailure:   false,
		BindingContext: make([]hook.BindingContext, 0),
	}
}

func (t *BaseTask) GetName() string {
	return t.Name
}

func (t *BaseTask) GetType() TaskType {
	return t.Type
}

func (t *BaseTask) GetBinding() hook.BindingType {
	return t.Binding
}

func (t *BaseTask) GetBindingContext() []hook.BindingContext {
	return t.BindingContext
}

func (t *BaseTask) GetDelay() time.Duration {
	return t.Delay
}

func (t *BaseTask) GetAllowFailure() bool {
	return t.AllowFailure
}

func (t *BaseTask) WithBinding(binding hook.BindingType) *BaseTask {
	t.Binding = binding
	return t
}

func (t *BaseTask) WithBindingContext(context []hook.BindingContext) *BaseTask {
	t.BindingContext = context
	return t
}

func (t *BaseTask) AppendBindingContext(context hook.BindingContext) *BaseTask {
	t.BindingContext = append(t.BindingContext, context)
	return t
}

func (t *BaseTask) WithAllowFailure(allowFailure bool) *BaseTask {
	t.AllowFailure = allowFailure
	return t
}

func (t *BaseTask) DumpAsText() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s '%s'", t.Type, t.Name))
	if t.FailureCount > 0 {
		buf.WriteString(fmt.Sprintf(" failed %d times. ", t.FailureCount))
	}
	return buf.String()
}

func (t *BaseTask) GetFailureCount() int {
	return t.FailureCount
}

func (t *BaseTask) IncrementFailureCount() {
	t.FailureCount++
}

func NewTaskDelay(delay time.Duration) *BaseTask {
	return &BaseTask{
		Type:  Delay,
		Delay: delay,
	}
}
