package kube_event

import (
	"fmt"

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

		for _, config := range hmHook.Config.OnKubernetesEvent {
			if config.NamespaceSelector.Any {
				informerConfig := kube_events_manager.KubeEventInformerConfig{
					Name:          fmt.Sprintf("%s-%s", hmHook.Name, config.Name),
					Kind:          config.Kind,
					Namespace:     "",
					ObjectName:    config.ObjectName,
					EventTypes:    config.EventTypes,
					JqFilter:      config.JqFilter,
					LabelSelector: config.Selector,
				}
				configId, err := eventsManager.Run(informerConfig)
				if err != nil {
					return err
				}
				obj.KubeHooks[configId] = &KubeEventHook{
					HookName: hmHook.Name,
					Config:   config,
					ConfigId: configId,
				}
			} else if len(config.NamespaceSelector.MatchNames) > 0 {
				for _, ns := range config.NamespaceSelector.MatchNames {
					informerConfig := kube_events_manager.KubeEventInformerConfig{
						Name:          fmt.Sprintf("%s-%s", hmHook.Name, config.Name),
						Kind:          config.Kind,
						Namespace:     ns,
						ObjectName:    config.ObjectName,
						EventTypes:    config.EventTypes,
						JqFilter:      config.JqFilter,
						LabelSelector: config.Selector,
					}
					configId, err := eventsManager.Run(informerConfig)
					if err != nil {
						return err
					}
					obj.KubeHooks[configId] = &KubeEventHook{
						HookName: hmHook.Name,
						Config:   config,
						ConfigId: configId,
					}
				}
			}
		}
	}

	return nil
}

func (obj *MainKubeEventsHooksController) DisableHook(name string, configId string, hookManager hook.HookManager, eventsManager kube_events_manager.KubeEventsManager) error {
	//hook, err := hookManager.GetHook(name)
	//if err != nil {
	//	return err
	//}

	for configId, desc := range obj.KubeHooks {
		if desc.HookName == name {
			err := eventsManager.Stop(configId)
			if err != nil {
				return err
			}
			delete(obj.KubeHooks, configId)
		}
	}

	return nil
}

// HandleEvent
func (obj *MainKubeEventsHooksController) HandleEvent(kubeEvent kube_events_manager.KubeEvent) ([]task.Task, error) {
	res := make([]task.Task, 0)

	desc, hasKey := obj.KubeHooks[kubeEvent.ConfigId]
	if !hasKey {
		return nil, fmt.Errorf("Unknown kube event: no such config id '%s' registered", kubeEvent.ConfigId)
	}

	bindingName := desc.Config.Name
	if desc.Config.Name == "" {
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

	newTask := task.NewTask(task.HookRun, desc.HookName).
		WithBinding(hook.KubeEvents).
		WithBindingContext(bindingContext).
		WithAllowFailure(desc.Config.AllowFailure)

	res = append(res, newTask)

	return res, nil
}
