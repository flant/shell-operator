package kube_event

import (
	//"github.com/flant/shell-operator/pkg/kube_events_manager"
	"fmt"

	"github.com/romana/rlog"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/task"
)

type KubeEventsHooksController interface {
	//	EnableGlobalHooks(moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	//	EnableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	//	DisableModuleHooks(moduleName string, moduleManager module_manager.ModuleManager, eventsManager kube_events_manager.KubeEventsManager) error
	EnableHooks(hookManager hook.HookManager, eventsManager kube_events_manager.KubeEventsManager) error
	HandleEvent(kubeEvent kube_events_manager.KubeEvent) (*struct{ Tasks []task.Task }, error)
}

type MainKubeEventsHooksController struct {
	KubeHooks map[string]*KubeEventHook
}

func NewMainKubeEventsHooksController() *MainKubeEventsHooksController {
	return &MainKubeEventsHooksController{
		KubeHooks: make(map[string]*KubeEventHook),
	}
}

func (obj *MainKubeEventsHooksController) EnableHooks(hookManager hook.HookManager, eventsManager kube_events_manager.KubeEventsManager) error {
	hooks := hookManager.GetHooksInOrder(hook.KubeEvents)

	for _, hookName := range hooks {
		hmHook, _ := hookManager.GetHook(hookName)

		for _, desc := range MakeKubeEventHookDescriptors(hmHook) {
			configId, err := eventsManager.Run(desc.EventTypes, desc.Kind, desc.Namespace, desc.Selector, desc.JqFilter, desc.Debug)
			if err != nil {
				return err
			}
			obj.KubeHooks[configId] = desc

			rlog.Debugf("MAIN: run informer %s for hook %s on kind=%s, events=%+v", configId, desc.Kind, desc.EventTypes, hmHook.Name)
		}
	}

	return nil
}

func (obj *MainKubeEventsHooksController) HandleEvent(kubeEvent kube_events_manager.KubeEvent) (*struct{ Tasks []task.Task }, error) {
	res := &struct{ Tasks []task.Task }{Tasks: make([]task.Task, 0)}
	var desc *KubeEventHook
	var taskType task.TaskType

	if moduleDesc, hasKey := obj.KubeHooks[kubeEvent.ConfigId]; hasKey {
		desc = moduleDesc
		taskType = task.HookRun
	}

	if desc != nil && taskType != "" {
		bindingName := desc.Name
		if desc.Name == "" {
			bindingName = hook.ContextBindingType[hook.KubeEvents]
		}

		bindingContext := make([]hook.BindingContext, 0)
		for _, kEvent := range kubeEvent.Events {
			bindingContext = append(bindingContext, hook.BindingContext{
				Binding:           bindingName,
				ResourceEvent:     kEvent,
				ResourceNamespace: kubeEvent.Namespace,
				ResourceKind:      kubeEvent.Kind,
				ResourceName:      kubeEvent.Name,
			})
		}

		newTask := task.NewTask(taskType, desc.HookName).
			WithBinding(hook.KubeEvents).
			WithBindingContext(bindingContext).
			WithAllowFailure(desc.Config.AllowFailure)

		res.Tasks = append(res.Tasks, newTask)
	} else {
		return nil, fmt.Errorf("Unknown kube event: no such config id '%s' registered", kubeEvent.ConfigId)
	}

	return res, nil
}

func MakeKubeEventHookDescriptors(hook *hook.Hook) []*KubeEventHook {
	res := make([]*KubeEventHook, 0)

	for _, config := range hook.Config.OnKubernetesEvent {
		if config.NamespaceSelector.Any {
			res = append(res, ConvertOnKubernetesEventToKubeEventHook(hook, config, ""))
		} else {
			for _, namespace := range config.NamespaceSelector.MatchNames {
				res = append(res, ConvertOnKubernetesEventToKubeEventHook(hook, config, namespace))
			}
		}
	}

	return res
}

func ConvertOnKubernetesEventToKubeEventHook(hook *hook.Hook, config kube_events_manager.OnKubernetesEventConfig, namespace string) *KubeEventHook {
	return &KubeEventHook{
		HookName:     hook.Name,
		Name:         config.Name,
		EventTypes:   config.EventTypes,
		Kind:         config.Kind,
		Namespace:    namespace,
		Selector:     config.Selector,
		JqFilter:     config.JqFilter,
		AllowFailure: config.AllowFailure,
		Debug:        !config.DisableDebug,
	}
}
