package kube_event

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/task"
)

type KubernetesHooksController interface {
	WithHookManager(hook.HookManager)
	WithKubeEventsManager(kube_events_manager.KubeEventsManager)
	EnableHooks() error
	HandleEvent(kubeEvent kube_events_manager.KubeEvent) ([]task.Task, error)
}

// kubernetesHooksController is a main implementation of KubernetesHooksController
type kubernetesHooksController struct {
	// All hooks with OnKubernetesEvent bindings
	KubeHooks map[string]*KubeEventHook

	// dependencies
	hookManager       hook.HookManager
	kubeEventsManager kube_events_manager.KubeEventsManager
}

// kubernetesHooksController should implement the KubernetesHooksController
var _ KubernetesHooksController = &kubernetesHooksController{}

// NewKubernetesHooksController returns an implementation of KubernetesHooksController
var NewKubernetesHooksController = func() *kubernetesHooksController {
	return &kubernetesHooksController{
		KubeHooks: make(map[string]*KubeEventHook),
	}
}

func (c *kubernetesHooksController) WithHookManager(hookManager hook.HookManager) {
	c.hookManager = hookManager
}

func (c *kubernetesHooksController) WithKubeEventsManager(kubeEventsManager kube_events_manager.KubeEventsManager) {
	c.kubeEventsManager = kubeEventsManager
}

func (c *kubernetesHooksController) EnableHooks() error {
	hooks := c.hookManager.GetHooksInOrder(hook.OnKubernetesEvent)

	for _, hookName := range hooks {
		hmHook, _ := c.hookManager.GetHook(hookName)

		for _, config := range hmHook.Config.OnKubernetesEvents {
			err := c.kubeEventsManager.AddMonitor("", config.Monitor)
			if err != nil {
				return fmt.Errorf("run kube monitor for hook %s: %s", hmHook.Name, err)
			}
			c.KubeHooks[config.Monitor.Metadata.ConfigId] = &KubeEventHook{
				HookName:     hmHook.Name,
				ConfigName:   config.ConfigName,
				AllowFailure: config.AllowFailure,
			}
		}

	}

	return nil
}

// HandleEvent receives event from kube_event_manager and generate a new task to run a hook.
func (c *kubernetesHooksController) HandleEvent(kubeEvent kube_events_manager.KubeEvent) ([]task.Task, error) {
	res := make([]task.Task, 0)

	kubeHook, hasKey := c.KubeHooks[kubeEvent.ConfigId]
	if !hasKey {
		return nil, fmt.Errorf("Unknown kube event: no such config id '%s' registered", kubeEvent.ConfigId)
	}

	switch kubeEvent.Type {
	case "Synchronization":
		// Send all objects
		objList := make([]interface{}, 0)
		for _, obj := range kubeEvent.Objects {
			objList = append(objList, interface{}(obj))
		}
		bindingContext := make([]hook.BindingContext, 0)
		bindingContext = append(bindingContext, hook.BindingContext{
			Binding: kubeHook.ConfigName,
			Type:    kubeEvent.Type,
			Objects: objList,
		})

		newTask := task.NewTask(task.HookRun, kubeHook.HookName).
			WithBinding(hook.OnKubernetesEvent).
			WithBindingContext(bindingContext).
			WithAllowFailure(kubeHook.AllowFailure)

		res = append(res, newTask)
	default:
		bindingContext := make([]hook.BindingContext, 0)
		for _, kEvent := range kubeEvent.WatchEvents {
			bindingContext = append(bindingContext, hook.BindingContext{
				Binding:    kubeHook.ConfigName,
				Type:       "Event",
				WatchEvent: kEvent,

				Namespace: kubeEvent.Namespace,
				Kind:      kubeEvent.Kind,
				Name:      kubeEvent.Name,

				Object:       kubeEvent.Object,
				FilterResult: kubeEvent.FilterResult,
			})
		}

		newTask := task.NewTask(task.HookRun, kubeHook.HookName).
			WithBinding(hook.OnKubernetesEvent).
			WithBindingContext(bindingContext).
			WithAllowFailure(kubeHook.AllowFailure)

		res = append(res, newTask)
	}

	return res, nil
}
