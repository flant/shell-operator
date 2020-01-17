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
)

type HookMetadata struct {
	HookName       string // hook name
	BindingType    BindingType
	BindingContext []BindingContext
	AllowFailure   bool //Task considered as 'ok' if hook failed. False by default. Can be true for some schedule hooks.
}

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

func (m *HookMetadata) GetHookName() string {
	return m.HookName
}

func (m *HookMetadata) GetBinding() BindingType {
	return m.BindingType
}

func (m *HookMetadata) GetBindingContext() []BindingContext {
	return m.BindingContext
}

func (m *HookMetadata) GetAllowFailure() bool {
	return m.AllowFailure
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

func (m *HookMetadata) GetDescription() string {
	return fmt.Sprintf("%s:%s", string(m.BindingType), m.HookName)
}
