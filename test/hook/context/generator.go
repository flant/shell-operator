package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/flant/shell-operator/pkg/hook"
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/kube"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
)

var KubeClient kube.KubernetesClient

// convertBindingContexts render json with array of binding contexts
func convertBindingContexts(bindingContexts []BindingContext) (string, error) {
	// Only v1 binding contexts supported by now
	bcList := ConvertBindingContextList("v1", bindingContexts)
	data, err := bcList.Json()
	if err != nil {
		return "", fmt.Errorf("marshaling binding context error: %v", err)
	}
	return string(data), nil
}

type BindingContextController struct {
	HookCtrl          controller.HookController
	HookMap           map[string]string
	HookConfig        string
	InitialState      string
	Controller        StateController
	KubeEventsManager kubeeventsmanager.KubeEventsManager
	ScheduleManager   schedulemanager.ScheduleManager
	Context           context.Context
	Cancel            context.CancelFunc
}

func NewBindingContextController(config, initialState string) (BindingContextController, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	c := BindingContextController{
		HookMap:      make(map[string]string),
		HookConfig:   config,
		InitialState: initialState,
		Context:      ctx,
		Cancel:       cancel,
	}
	return c, nil
}

// RegisterCRD registers custom resources for the cluster
func (b *BindingContextController) RegisterCRD(group, version, kind string, namespaced bool) {
	scheme.Scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: group, Version: version, Kind: kind}, &unstructured.Unstructured{})
	newResource := metav1.APIResource{
		Kind:       kind,
		Name:       strings.ToLower(kind) + "s",
		Verbs:      metav1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
		Group:      group,
		Version:    version,
		Namespaced: namespaced,
	}
	for _, resource := range ClusterResources {
		if resource.GroupVersion == group+"/"+version {
			resource.APIResources = append(resource.APIResources, newResource)
			return
		}
	}
	ClusterResources = append(ClusterResources, &metav1.APIResourceList{
		GroupVersion: group + "/" + version,
		APIResources: []metav1.APIResource{newResource},
	})
}

// BindingContextsGenerator generates binding contexts for hook tests
func (b *BindingContextController) Run() (string, error) {
	// Create fake clients: typed and dynamic
	KubeClient = kube.NewFakeKubernetesClient()

	fakeDiscovery, ok := KubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		return "", fmt.Errorf("couldn't convert Discovery() to *FakeDiscovery")
	}
	fakeDiscovery.FakedServerVersion = &version.Info{GitCommit: "v1.0.0"}
	fakeDiscovery.Resources = ClusterResources

	b.KubeEventsManager = kubeeventsmanager.NewKubeEventsManager()
	b.KubeEventsManager.WithContext(b.Context)
	b.KubeEventsManager.WithKubeClient(KubeClient)

	b.ScheduleManager = schedulemanager.NewScheduleManager()
	b.ScheduleManager.WithContext(b.Context)
	// Use StateController to apply changes
	stateController, err := NewStateController(b.InitialState)
	if err != nil {
		return "", err
	}
	b.Controller = stateController

	testHook := hook.NewHook("test", "test")
	testHook, err = testHook.WithConfig([]byte(b.HookConfig))
	if err != nil {
		return "", fmt.Errorf("couldn't load or validate hook configuration: %v", err)
	}

	b.HookCtrl = controller.NewHookController()
	b.HookCtrl.InitKubernetesBindings(testHook.GetConfig().OnKubernetesEvents, b.KubeEventsManager)
	b.HookCtrl.InitScheduleBindings(testHook.GetConfig().Schedules, b.ScheduleManager)
	b.HookCtrl.EnableScheduleBindings()

	testHook.WithHookController(b.HookCtrl)

	bindingContexts := make([]BindingContext, 0)
	err = b.HookCtrl.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, b.HookCtrl.UpdateSnapshots(info.BindingContext)...)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't enable kubernetes bindings: %v", err)
	}
	b.HookCtrl.StartMonitors()

	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) ChangeState(newState ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	bindingContexts := make([]BindingContext, 0)

	done := false
	go func() {
		time.Sleep(100 * time.Millisecond)
		for _, state := range newState {
			err := b.Controller.ChangeState(state)
			if err != nil {
				break
			}
		}
		done = true
	}()

	for {
		select {
		case ev := <-b.KubeEventsManager.Ch():
			b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
				bindingContexts = append(bindingContexts, info.BindingContext...)
			})
			continue
		case <-ctx.Done():
			break
		}
		time.Sleep(10 * time.Millisecond)
		if done {
			break
		}
	}
	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) RunSchedule(crontab string) (string, error) {
	bindingContexts := make([]BindingContext, 0)

	b.HookCtrl.HandleScheduleEvent(crontab, func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, b.HookCtrl.UpdateSnapshots(info.BindingContext)...)
	})
	return convertBindingContexts(bindingContexts)
}
