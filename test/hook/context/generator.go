package context

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/kube"
	manager "github.com/flant/shell-operator/pkg/kube_events_manager"
)

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

// kubeEventToBindingContext returns binding context in json format to use in hook tests
func kubeEventToBindingContext(kubeEvent KubeEvent, configName string) []BindingContext {
	bindingContexts := make([]BindingContext, 0)

	// todo: get the code from shell operator instead of copy-pasting
	for _, kEvent := range kubeEvent.WatchEvents {
		bindingContexts = append(bindingContexts, BindingContext{
			// Remove this
			Binding:    configName,
			Type:       "Event",
			WatchEvent: kEvent,

			Object:       kubeEvent.Object,
			FilterResult: kubeEvent.FilterResult,
		})
	}
	return bindingContexts
}

type BindingContextController struct {
	//HookMap      map[string]string
	HookConfig   string
	InitialState string
	Controller   StateController
	Manager      manager.KubeEventsManager
	Context      context.Context
	Cancel       context.CancelFunc
}

func NewBindingContextController(config, initialState string) (BindingContextController, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	controller := BindingContextController{
		//HookMap:      make(map[string]string),
		HookConfig:   config,
		InitialState: initialState,
		Context:      ctx,
		Cancel:       cancel,
	}
	return controller, nil
}

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
	kube.Kubernetes = fake.NewSimpleClientset()
	fakeDiscovery, ok := kube.Kubernetes.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		return "", fmt.Errorf("couldn't convert Discovery() to *FakeDiscovery")
	}
	fakeDiscovery.FakedServerVersion = &version.Info{GitCommit: "v1.0.0"}
	fakeDiscovery.Resources = ClusterResources

	// Configure dynamic client
	kube.DynamicClient = fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())

	b.Manager = manager.NewKubeEventsManager()

	// Use StateController to apply changes
	stateController, err := NewStateController(b.InitialState)
	if err != nil {
		return "", err
	}
	b.Controller = stateController

	hook := hook.NewHook("test", "test")

	_, err := hook.WithConfig(b.HookConfig)
	if err != nil {
		return "", fmt.Errorf("couldn't load or validate hook configuration: %v", err)
	}

	hookCtrl := controller.NewHookController()
	hookCtrl.InitKubernetesBindings(hook.GetConfig().OnKubernetesEvents, hm.kubeEventsManager)

	hook.WithHookController(hookCtrl)

	bindingContexts := make([]BindingContext, 0)

	hookCtrl.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, info.BindingContext...)
	})

	b.Manager.WithContext(b.Context)
	hookCtrl.StartMonitors()

	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) ChangeState(newState ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	bindingContexts := make([]BindingContext, 0)

	done := false
	go func() {
		time.Sleep(time.Second * 1)
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
		case ev := <-b.Manager.Ch():
			// operator.go <- KubeEventManager.Ch()

			data := kubeEventToBindingContext(ev, b.HookMap[ev.MonitorId])
			bindingContexts = append(bindingContexts, data...)
			continue
		case <-ctx.Done():
			break
		}
		time.Sleep(100 * time.Millisecond)
		if done {
			break
		}
	}

	return convertBindingContexts(bindingContexts)
}
