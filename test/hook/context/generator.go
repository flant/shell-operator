package context

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube"
	manager "github.com/flant/shell-operator/pkg/kube_events_manager"
)

// kubeEventToBindingContext returns binding context in json format to use in hook tests
func kubeEventToBindingContext(kubeEvent manager.KubeEvent, configName string) (string, error) {
	bindingContext := make([]hook.BindingContext, 0)

	// todo: get the code from shell operator instead of copy-pasting
	switch kubeEvent.Type {
	case "Synchronization":
		// Send all objects
		objList := make([]interface{}, 0)
		for _, obj := range kubeEvent.Objects {
			objList = append(objList, interface{}(obj))
		}
		bindingContext = append(bindingContext, hook.BindingContext{
			Binding: configName,
			Type:    kubeEvent.Type,
			Objects: objList,
		})
	default:
		for _, kEvent := range kubeEvent.WatchEvents {
			bindingContext = append(bindingContext, hook.BindingContext{
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
	}
	// Only v1 binding contexts supported by now
	data, err := json.Marshal(hook.ConvertBindingContextListV1(bindingContext))
	if err != nil {
		return "", fmt.Errorf("marshaling binding context error: %v", err)
	}
	return string(data), nil
}

type BindingContextController struct {
	HookConfig string
	Controller StateController
	Manager    manager.KubeEventsManager
	Context    context.Context
	Cancel     context.CancelFunc
}

func NewBindingContextController(config, initialState string) (BindingContextController, error) {
	kube.Kubernetes = fake.NewSimpleClientset()
	fakeDiscovery, ok := kube.Kubernetes.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		return BindingContextController{}, fmt.Errorf("couldn't convert Discovery() to *FakeDiscovery")
	}
	fakeDiscovery.FakedServerVersion = &version.Info{GitCommit: "v1.0.0"}
	fakeDiscovery.Resources = ClusterResources

	// Configure dynamic client
	runtimeScheme := runtime.NewScheme()
	kube.DynamicClient = fakedynamic.NewSimpleDynamicClient(runtimeScheme, []runtime.Object{}...)

	// Use StateController to apply changes
	stateController, err := NewStateController(initialState)
	if err != nil {
		return BindingContextController{}, err
	}
	mgr := manager.NewKubeEventsManager()

	// todo: think about channel size
	manager.KubeEventCh = make(chan manager.KubeEvent)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	return BindingContextController{
		HookConfig: config,
		Controller: stateController,
		Manager:    mgr,
		Context:    ctx,
		Cancel:     cancel,
	}, nil
}

// BindingContextsGenerator generates binding contexts for hook tests
func (b *BindingContextController) Run() (*[]string, error) {
	hc := &hook.HookConfig{}
	err := hc.LoadAndValidate([]byte(b.HookConfig))
	if err != nil {
		return nil, fmt.Errorf("couldn't load or validate hook configuration: %v", err)
	}

	// Add onKubernetes hook monitors to manager
	for _, binding := range hc.OnKubernetesEvents {
		err = b.Manager.AddMonitor(binding.ConfigName, binding.Monitor, log.WithField("test", "yes"))
		if err != nil {
			return nil, fmt.Errorf("monitor %q adding failed: %v", binding.ConfigName, err)
		}
	}
	b.Manager.WithContext(b.Context)
	b.Manager.Start()

	var bindingContexts []string
	ev := <-manager.KubeEventCh
	data, _ := kubeEventToBindingContext(ev, ev.ConfigId)
	bindingContexts = append(bindingContexts, data)
	return &bindingContexts, nil
}

func (b *BindingContextController) ChangeState(newState ...string) (*[]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var bindingContexts []string

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
			data, _ := kubeEventToBindingContext(ev, ev.ConfigId)
			bindingContexts = append(bindingContexts, data)
			continue
		case <-ctx.Done():
			break
		}
		time.Sleep(100 * time.Millisecond)
		if done {
			break
		}
	}
	return &bindingContexts, nil
}
