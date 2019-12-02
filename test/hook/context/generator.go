package context

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube"
	manager "github.com/flant/shell-operator/pkg/kube_events_manager"
)

// convertBindingContexts render json with array of binding contexts
func convertBindingContexts(bindingContexts []hook.BindingContext) (string, error) {
	// Only v1 binding contexts supported by now
	data, err := json.Marshal(hook.ConvertBindingContextListV1(bindingContexts))
	if err != nil {
		return "", fmt.Errorf("marshaling binding context error: %v", err)
	}
	return string(data), nil
}

// kubeEventToBindingContext returns binding context in json format to use in hook tests
func kubeEventToBindingContext(kubeEvent manager.KubeEvent, configName string) []hook.BindingContext {
	bindingContexts := make([]hook.BindingContext, 0)

	// todo: get the code from shell operator instead of copy-pasting
	for _, kEvent := range kubeEvent.WatchEvents {
		bindingContexts = append(bindingContexts, hook.BindingContext{
			// Remove this
			Binding:    configName,
			Type:       "Event",
			WatchEvent: kEvent,

			Namespace: kubeEvent.Namespace,
			Kind:      kubeEvent.Kind,
			Name:      kubeEvent.Name,

			Object:       kubeEvent.Object,
			FilterResult: kubeEvent.FilterResult,
		})
	}
	return bindingContexts
}

type BindingContextController struct {
	HookMap      map[string]string
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
		HookMap:      make(map[string]string),
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
	manager.KubeEventCh = make(chan manager.KubeEvent)

	// Use StateController to apply changes
	controller, err := NewStateController(b.InitialState)
	if err != nil {
		return "", err
	}
	b.Controller = controller

	hc := &hook.HookConfig{}
	err = hc.LoadAndValidate([]byte(b.HookConfig))
	if err != nil {
		return "", fmt.Errorf("couldn't load or validate hook configuration: %v", err)
	}

	bindingContexts := make([]hook.BindingContext, 0)

	// Add onKubernetes hook monitors to manager
	for _, binding := range hc.OnKubernetesEvents {
		existedObjects, err := b.Manager.AddMonitor(binding.CommonBindingConfig.ConfigName, binding.Monitor, log.WithField("test", "yes"))
		if err != nil {
			return "", fmt.Errorf("monitor %q adding failed: %v", binding.ConfigName, err)
		}
		b.HookMap[binding.Monitor.Metadata.ConfigId] = binding.CommonBindingConfig.ConfigName

		// Get existed objects and create HookRun task with Synchronization type
		objList := make([]interface{}, 0)
		for _, obj := range existedObjects {
			objList = append(objList, interface{}(obj))
		}
		bindingContexts = append(bindingContexts, hook.BindingContext{
			Binding: binding.CommonBindingConfig.ConfigName,
			Type:    "Synchronization",
			Objects: objList,
		})
	}
	b.Manager.WithContext(b.Context)
	b.Manager.Start()

	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) ChangeState(newState ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	bindingContexts := make([]hook.BindingContext, 0)

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
		case ev := <-manager.KubeEventCh:
			data := kubeEventToBindingContext(ev, b.HookMap[ev.ConfigId])
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
