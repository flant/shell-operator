package kube_event

import (
	"testing"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/stretchr/testify/assert"
)

type MockKubeEventsManager struct {
}

func (MockKubeEventsManager) Run(monitorConfig *kube_events_manager.MonitorConfig) (string, error) {
	return monitorConfig.Kind, nil
}

func (MockKubeEventsManager) Stop(configId string) error {
	return nil
}

type MockHookManager struct {
}

func (*MockHookManager) Run() {
	panic("implement me")
}

func (*MockHookManager) GetHook(name string) (*hook.Hook, error) {
	switch name {
	case "hook-1":
		return &hook.Hook{
			Name: "hook-1",
			Path: "/hooks/hook-1",
			Config: &hook.HookConfig{
				Version: "v1",
				OnKubernetesEvents: []hook.OnKubernetesEventConfig{
					{
						CommonBindingConfig: hook.CommonBindingConfig{
							ConfigName:   "monitor configmaps",
							AllowFailure: false,
						},
						Monitor: &kube_events_manager.MonitorConfig{
							ConfigIdPrefix: "monitor-configmaps",
							Kind:           "ConfigMap",
							EventTypes:     []kube_events_manager.WatchEventType{kube_events_manager.WatchEventModified},
						},
					},
				},
			},
		}, nil
	case "second":
		return &hook.Hook{
			Name: "second",
			Path: "/hooks/second",
			Config: &hook.HookConfig{
				Version: "v1",
				OnKubernetesEvents: []hook.OnKubernetesEventConfig{
					{
						CommonBindingConfig: hook.CommonBindingConfig{
							ConfigName:   "monitor pods",
							AllowFailure: false,
						},
						Monitor: &kube_events_manager.MonitorConfig{
							ConfigIdPrefix: "monitor-pods",
							Kind:           "pod",
							EventTypes:     []kube_events_manager.WatchEventType{kube_events_manager.WatchEventAdded},
						},
					},
				},
			},
		}, nil
	}
	return nil, nil
}

func (*MockHookManager) GetHooksInOrder(bindingType hook.BindingType) []string {
	return []string{
		"hook-1",
		"second",
	}
}

func (*MockHookManager) RunHook(hookName string, binding hook.BindingType, bindingContext []hook.BindingContext) error {
	panic("implement me")
}

func Test_KubeHooksController_EnableHooks(t *testing.T) {
	ctrl := NewMainKubeEventsHooksController()

	err := ctrl.EnableHooks(&MockHookManager{}, MockKubeEventsManager{})

	if assert.NoError(t, err) {
		assert.Len(t, ctrl.KubeHooks, 2)
	}
}
