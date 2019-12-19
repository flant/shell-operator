package task

import (
	"bytes"
	"fmt"
	"time"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
	uuid "gopkg.in/satori/go.uuid.v1"
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
	GetBinding() BindingType
	GetBindingContext() []BindingContext
	GetFailureCount() int
	IncrementFailureCount()
	GetDelay() time.Duration
	GetAllowFailure() bool
	GetLogLabels() map[string]string
}

type BaseTask struct {
	FailureCount   int    // Failed executions count
	Name           string // hook name
	Type           TaskType
	Binding        BindingType
	BindingContext []BindingContext
	Delay          time.Duration
	AllowFailure   bool //Task considered as 'ok' if hook failed. False by default. Can be true for some schedule hooks.
	LogLabels      map[string]string
}

func NewTask(taskType TaskType, name string) *BaseTask {
	return &BaseTask{
		FailureCount:   0,
		Name:           name,
		Type:           taskType,
		AllowFailure:   false,
		BindingContext: make([]BindingContext, 0),
		LogLabels:      map[string]string{"task.id": uuid.NewV4().String()},
	}
}

func (t *BaseTask) GetName() string {
	return t.Name
}

func (t *BaseTask) GetType() TaskType {
	return t.Type
}

func (t *BaseTask) GetBinding() BindingType {
	return t.Binding
}

func (t *BaseTask) GetBindingContext() []BindingContext {
	return t.BindingContext
}

func (t *BaseTask) GetDelay() time.Duration {
	return t.Delay
}

func (t *BaseTask) GetAllowFailure() bool {
	return t.AllowFailure
}

func (t *BaseTask) GetLogLabels() map[string]string {
	return t.LogLabels
}

func (t *BaseTask) WithBinding(binding BindingType) *BaseTask {
	t.Binding = binding
	return t
}

func (t *BaseTask) WithBindingContext(context []BindingContext) *BaseTask {
	t.BindingContext = context
	return t
}

func (t *BaseTask) AppendBindingContext(context BindingContext) *BaseTask {
	t.BindingContext = append(t.BindingContext, context)
	return t
}

func (t *BaseTask) WithAllowFailure(allowFailure bool) *BaseTask {
	t.AllowFailure = allowFailure
	return t
}

func (t *BaseTask) WithLogLabels(labels map[string]string) *BaseTask {
	t.LogLabels = utils.MergeLabels(t.LogLabels, labels)
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
