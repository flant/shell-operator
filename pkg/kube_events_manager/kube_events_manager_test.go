package kube_events_manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/stretchr/testify/assert"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

type MockResourceInformer struct {
}

func (*MockResourceInformer) WithDebugName(string) {
	return
}

func (*MockResourceInformer) WithNamespace(string) {
	return
}

func (*MockResourceInformer) CreateSharedInformer() error {
	return nil
}

func (*MockResourceInformer) Run(stopCh <-chan struct{}) {
	return
}

func (*MockResourceInformer) Stop() {
	return
}

func Test_MainKubeEventsManager_Run(t *testing.T) {
	// Init() replacement
	mgr := NewKubeEventsManager()

	// Mock KubeEventInformer constructor method
	oldResInf := NewResourceInformer
	NewResourceInformer = func(monitor *MonitorConfig) ResourceInformer {
		return &MockResourceInformer{}
	}
	defer func() {
		NewResourceInformer = oldResInf
	}()

	// monitor with 3 namespaces and 4 object names
	monitor := &MonitorConfig{
		Kind: "Pod",
		NamespaceSelector: &NamespaceSelector{
			NameSelector: &NameSelector{
				MatchNames: []string{"default", "prod", "stage"},
			},
		},
		NameSelector: &NameSelector{
			MatchNames: []string{"pod-1", "pod-2", "pod-3", "pod-4"},
		},
	}

	monitor.Metadata.ConfigId = "ConfigId"
	err := mgr.AddMonitor("test", monitor)
	if assert.NoError(t, err) {
		assert.Len(t, mgr.Monitors, 1)
	}
}

// Test_MainKubeEventsManager_HandleEvents
// Scenario:
// - create new KubeEventManager, start informers
// - check if first event is Synchronization
// - add more objects
// - receive and check events with objects
func Test_MainKubeEventsManager_HandleEvents(t *testing.T) {
	timeout := time.Duration(3 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Add GVR
	kube.Kubernetes = fake.NewSimpleClientset()
	fakeDiscovery, ok := kube.Kubernetes.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatalf("couldn't convert Discovery() to *FakeDiscovery")
	}

	fakeDiscovery.FakedServerVersion = &version.Info{
		GitCommit: "v1.0.0",
	}

	podGvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	fakeDiscovery.Resources = []*metav1.APIResourceList{
		&metav1.APIResourceList{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Kind:    "Pod",
					Name:    "pods",
					Verbs:   metav1.Verbs{"get", "list", "watch"},
					Group:   "",
					Version: "v1",
				},
			},
		},
	}

	// Configure dynamic client
	scheme := runtime.NewScheme()
	objs := []runtime.Object{}

	kube.DynamicClient = fakedynamic.NewSimpleDynamicClient(scheme, objs...)
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "pod-0",
			},
			"spec": "pod-0",
		},
	}
	_, _ = kube.DynamicClient.Resource(podGvr).Namespace("default").Create(obj, metav1.CreateOptions{}, []string{}...)

	// Init() replacement
	mgr := NewKubeEventsManager()
	mgr.WithContext(ctx)
	KubeEventCh = make(chan KubeEvent, 10)

	// monitor with 3 namespaces and 4 object names and all event types
	monitor := &MonitorConfig{
		ApiVersion: "v1",
		Kind:       "Pod",
		EventTypes: []WatchEventType{WatchEventAdded, WatchEventModified, WatchEventDeleted},
		NamespaceSelector: &NamespaceSelector{
			NameSelector: &NameSelector{
				MatchNames: []string{"default"},
			},
		},
	}
	monitor.Metadata.ConfigId = "ConfigId"

	err := mgr.AddMonitor("test", monitor)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	mgr.Start()

	fmt.Printf("mgr Started\n")

	// First event — Synchronization, second is Event
	eventCounter := 1
	done := false
	for {
		fmt.Printf("Start select\n")
		select {
		case ev := <-KubeEventCh:
			fmt.Printf("Got event: %d %#v\n", eventCounter, ev)
			switch eventCounter {
			case 1:
				assert.Equal(t, "Synchronization", ev.Type)
				assert.Equal(t, "ConfigId", ev.ConfigId)
				assert.Len(t, ev.Objects, 1)
				eventCounter = eventCounter + 1

				// Inject an event into the fake client.
				obj1 := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"namespace": "default",
							"name":      "pod-1",
						},
						"spec": "pod-1",
					},
				}
				_, _ = kube.DynamicClient.Resource(podGvr).Namespace("default").Create(obj1, metav1.CreateOptions{}, []string{}...)
				// Inject second event into the fake client.
				obj2 := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"namespace": "default",
							"name":      "pod-2",
						},
						"spec": "pod-2",
					},
				}
				_, _ = kube.DynamicClient.Resource(podGvr).Namespace("default").Create(obj2, metav1.CreateOptions{}, []string{}...)
				fmt.Printf("DynamicClient Created pod\n")
			case 2:
				assert.Equal(t, "Event", ev.Type)
				assert.Equal(t, "ConfigId", ev.ConfigId)
				assert.Equal(t, WatchEventAdded, ev.WatchEvents[0])
				metadata := ev.Object["metadata"].(map[string]interface{})
				assert.Equal(t, "pod-1", metadata["name"])
				eventCounter = eventCounter + 1
				//done = true
			case 3:
				assert.Equal(t, "Event", ev.Type)
				assert.Equal(t, "ConfigId", ev.ConfigId)
				assert.Equal(t, WatchEventAdded, ev.WatchEvents[0])
				metadata := ev.Object["metadata"].(map[string]interface{})
				assert.Equal(t, "pod-2", metadata["name"])
				eventCounter = eventCounter + 1
				done = true
			}
		case <-ctx.Done():
			t.Error("Kube events manager did not get the added pod")
			done = true
		}
		if done {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

}
