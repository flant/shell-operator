package task_metadata

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/task"
)

const (
	// a task to run a particular hook
	HookRun                  task.TaskType = "HookRun"
	EnableKubernetesBindings task.TaskType = "EnableKubernetesBindings"
	EnableScheduleBindings   task.TaskType = "EnableScheduleBindings"
)

type HookNameAccessor interface {
	GetHookName() string
}

type BindingContextAccessor interface {
	GetBindingContext() []BindingContext
}

type MonitorIDAccessor interface {
	GetMonitorIDs() []string
}

type HookMetadata struct {
	HookName       string // hook name
	Binding        string // binding name
	Group          string
	BindingType    BindingType
	BindingContext []BindingContext
	AllowFailure   bool     // Task considered as 'ok' if hook failed. False by default. Can be true for some schedule hooks.
	MonitorIDs     []string // monitor ids for Synchronization tasks

	ExecuteOnSynchronization bool // A flag to skip hook execution in Synchronization tasks.
}

var _ HookNameAccessor = HookMetadata{}
var _ BindingContextAccessor = HookMetadata{}
var _ MonitorIDAccessor = HookMetadata{}
var _ task.MetadataDescriptable = HookMetadata{}

func HookMetadataAccessor(t task.Task) (hookMeta HookMetadata) {
	meta := t.GetMetadata()
	if meta == nil {
		log.Errorf("Possible Bug! task metadata is nil")
		return
	}
	hookMeta, ok := meta.(HookMetadata)
	if !ok {
		log.Errorf("Possible Bug! task metadata is not of type HookMetadata: got %T", meta)
		return
	}
	return
}

func (m HookMetadata) GetHookName() string {
	return m.HookName
}

func (m HookMetadata) GetBinding() BindingType {
	return m.BindingType
}

func (m HookMetadata) GetBindingContext() []BindingContext {
	return m.BindingContext
}

func (m HookMetadata) GetAllowFailure() bool {
	return m.AllowFailure
}

func (m HookMetadata) GetMonitorIDs() []string {
	return m.MonitorIDs
}

func (m *HookMetadata) WithHookName(name string) *HookMetadata {
	m.HookName = name
	return m
}

func (m *HookMetadata) WithBinding(binding BindingType) *HookMetadata {
	m.BindingType = binding
	return m
}

func (m *HookMetadata) WithBindingContext(context []BindingContext) *HookMetadata {
	m.BindingContext = context
	return m
}

func (m *HookMetadata) AppendBindingContext(context BindingContext) *HookMetadata {
	m.BindingContext = append(m.BindingContext, context)
	return m
}

func (m *HookMetadata) WithAllowFailure(allowFailure bool) *HookMetadata {
	m.AllowFailure = allowFailure
	return m
}

func (m HookMetadata) GetDescription() string {
	additional := ""
	if m.Group != "" {
		additional += ":group=" + m.Group
	}
	if m.Binding != "" {
		additional += ":" + m.Binding
	}
	return fmt.Sprintf("%s:%s%s", string(m.BindingType), m.HookName, additional)
}

func (m HookMetadata) IsSynchronization() bool {
	// Synchronization binding contexts are not combined with others, so check the first item is enough.
	return len(m.BindingContext) > 0 && m.BindingContext[0].IsSynchronization()
}
