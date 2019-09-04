package kube_event

import (
	"fmt"
	"strings"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/task"
)

type KubeEventsHooksController interface {
	EnableHooks(hookManager hook.HookManager, eventsManager kube_events_manager.KubeEventsManager) error
	HandleEvent(kubeEvent kube_events_manager.KubeEvent) ([]task.Task, error)
}

type MainKubeEventsHooksController struct {
	// All hooks with OnKubernetesEvent bindings
	KubeHooks map[string]*KubeEventHook
}

func NewMainKubeEventsHooksController() *MainKubeEventsHooksController {
	return &MainKubeEventsHooksController{
		KubeHooks: make(map[string]*KubeEventHook),
	}
}

func (ctrl *MainKubeEventsHooksController) EnableHooks(hookManager hook.HookManager, kubeEventsMgr kube_events_manager.KubeEventsManager) error {
	hooks := hookManager.GetHooksInOrder(hook.OnKubernetesEvent)

	for _, hookName := range hooks {
		hmHook, _ := hookManager.GetHook(hookName)

		for _, config := range hmHook.Config.OnKubernetesEvents {
			configId, err := kubeEventsMgr.Run(config.Monitor)
			if err != nil {
				return fmt.Errorf("run kube monitor for hook %s: %s", hmHook.Name, err)
			}
			ctrl.KubeHooks[configId] = &KubeEventHook{
				HookName:     hmHook.Name,
				ConfigName:   config.ConfigName,
				AllowFailure: config.AllowFailure,
			}
		}

	}

	return nil
}

// HandleEvent receives event from kube_event_manager and generate a new task to run a hook.
func (obj *MainKubeEventsHooksController) HandleEvent(kubeEvent kube_events_manager.KubeEvent) ([]task.Task, error) {
	res := make([]task.Task, 0)

	kubeHook, hasKey := obj.KubeHooks[kubeEvent.ConfigId]
	if !hasKey {
		return nil, fmt.Errorf("Unknown kube event: no such config id '%s' registered", kubeEvent.ConfigId)
	}

	bindingContext := make([]hook.BindingContext, 0)
	for _, kEvent := range kubeEvent.Events {
		bindingContext = append(bindingContext, hook.BindingContext{
			Binding:           kubeHook.ConfigName,
			WatchEvent:        string(kEvent),
			ResourceEvent:     strings.ToLower(string(kEvent)),
			ResourceNamespace: kubeEvent.Namespace,
			ResourceKind:      kubeEvent.Kind,
			ResourceName:      kubeEvent.Name,
		})
	}

	newTask := task.NewTask(task.HookRun, kubeHook.HookName).
		WithBinding(hook.OnKubernetesEvent).
		WithBindingContext(bindingContext).
		WithAllowFailure(kubeHook.AllowFailure)

	res = append(res, newTask)

	return res, nil
}
