package kube_event

import (
	"context"
	"testing"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/stretchr/testify/assert"
)

type MockKubeEventsManager struct {
}

var _ kube_events_manager.KubeEventsManager = &MockKubeEventsManager{}

func (m *MockKubeEventsManager) WithContext(ctx context.Context) {
	return
}

func (m *MockKubeEventsManager) AddMonitor(name string, monitorConfig *kube_events_manager.MonitorConfig) error {
	return nil
}

func (m *MockKubeEventsManager) HasMonitor(configId string) bool {
	return false
}

func (m *MockKubeEventsManager) Start() {
	return
}

func (m *MockKubeEventsManager) StopMonitor(configId string) error {
	return nil
}

func (m *MockKubeEventsManager) Ch() chan kube_events_manager.KubeEvent {
	return nil
}

type MockHookManager struct {
}

func (hm *MockHookManager) Init() error {
	panic("implement me")
}

func (hm *MockHookManager) WithDirectories(workingDir string, tempDir string) {
	panic("implement me")
}

func (hm *MockHookManager) WorkingDir() string {
	panic("implement me")
}

func (hm *MockHookManager) TempDir() string {
	panic("implement me")
}

func (hm *MockHookManager) Run() {
	panic("implement me")
}

func (hm *MockHookManager) GetHook(name string) (*hook.Hook, error) {
	switch name {
	case "hook-1":
		monitor := &kube_events_manager.MonitorConfig{
			Kind:       "ConfigMap",
			EventTypes: []kube_events_manager.WatchEventType{kube_events_manager.WatchEventModified},
		}
		monitor.Metadata.ConfigId = "monitor-configmaps"
		monitor.Metadata.DebugName = "monitor-configmaps"
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
						Monitor: monitor,
					},
				},
			},
		}, nil
	case "second":
		monitor := &kube_events_manager.MonitorConfig{
			Kind:       "pod",
			EventTypes: []kube_events_manager.WatchEventType{kube_events_manager.WatchEventAdded},
		}
		monitor.Metadata.ConfigId = "monitor-pods"
		monitor.Metadata.DebugName = "monitor-pods"
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
						Monitor: monitor,
					},
				},
			},
		}, nil
	}
	return nil, nil
}

func (hm *MockHookManager) GetHooksInOrder(bindingType hook.BindingType) []string {
	return []string{
		"hook-1",
		"second",
	}
}

func (hm *MockHookManager) RunHook(hookName string, binding hook.BindingType, bindingContext []hook.BindingContext) error {
	return nil
}

func Test_KubernetesHooksController_EnableHooks(t *testing.T) {
	ctrl := NewKubernetesHooksController()
	ctrl.WithHookManager(&MockHookManager{})
	ctrl.WithKubeEventsManager(&MockKubeEventsManager{})

	err := ctrl.EnableHooks()

	if assert.NoError(t, err) {
		assert.Len(t, ctrl.KubeHooks, 2)
	}
}
