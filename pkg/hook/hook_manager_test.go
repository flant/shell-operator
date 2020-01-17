package hook

//import (
//	"testing"
//
//	"github.com/flant/shell-operator/pkg/kube_events_manager"
//	"github.com/stretchr/testify/assert"
//)
//
//func (hm *MockHookManager) GetHook(name string) (*hook.Hook, error) {
//	switch name {
//	case "hook-1":
//		monitor := &kube_events_manager.MonitorConfig{
//			Kind:       "ConfigMap",
//			EventTypes: []WatchEventType{WatchEventModified},
//		}
//		monitor.Metadata.MonitorId = "monitor-configmaps"
//		monitor.Metadata.DebugName = "monitor-configmaps"
//		return &hook.Hook{
//			Name: "hook-1",
//			Path: "/hooks/hook-1",
//			Config: &hook.HookConfig{
//				Version: "v1",
//				OnKubernetesEvents: []hook.OnKubernetesEventConfig{
//					{
//						CommonBindingConfig: hook.CommonBindingConfig{
//							Name:         "monitor configmaps",
//							AllowFailure: false,
//						},
//						Monitor: monitor,
//					},
//				},
//			},
//		}, nil
//	case "second":
//		monitor := &kube_events_manager.MonitorConfig{
//			Kind:       "pod",
//			EventTypes: []WatchEventType{WatchEventAdded},
//		}
//		monitor.Metadata.MonitorId = "monitor-pods"
//		monitor.Metadata.DebugName = "monitor-pods"
//		return &hook.Hook{
//			Name: "second",
//			Path: "/hooks/second",
//			Config: &hook.HookConfig{
//				Version: "v1",
//				OnKubernetesEvents: []hook.OnKubernetesEventConfig{
//					{
//						CommonBindingConfig: hook.CommonBindingConfig{
//							Name:         "monitor pods",
//							AllowFailure: false,
//						},
//						Monitor: monitor,
//					},
//				},
//			},
//		}, nil
//	}
//	return nil, nil
//}
//
//func (hm *MockHookManager) GetHooksInOrder(bindingType hook.BindingType) ([]string, error) {
//	return []string{
//		"hook-1",
//		"second",
//	}, nil
//}
//
//func (hm *MockHookManager) RunHook(hookName string, binding hook.BindingType, bindingContext []hook.BindingContext, logLabels map[string]string) error {
//	return nil
//}
//
//func Test_KubernetesHooksController_EnableHooks(t *testing.T) {
//	ctrl := NewKubernetesHooksController()
//	ctrl.WithHookManager(&MockHookManager{})
//	ctrl.WithKubeEventsManager(&MockKubeEventsManager{})
//
//	tasks, err := ctrl.EnableHooks()
//
//	if assert.NoError(t, err) {
//		assert.Len(t, tasks, 2)
//	}
//}
