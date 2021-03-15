package kube_events_manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

func Test_MainKubeEventsManager_Run(t *testing.T) {
	// Init() replacement

	kubeClient := kube.NewFakeKubernetesClient()

	fakeDiscovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatalf("couldn't convert Discovery() to *FakeDiscovery")
	}

	fakeDiscovery.FakedServerVersion = &version.Info{
		GitCommit: "v1.0.0",
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

	mgr := NewKubeEventsManager()
	mgr.WithContext(context.Background())
	mgr.WithKubeClient(kubeClient)

	// monitor with 3 namespaces and 4 object names
	monitor := &MonitorConfig{
		ApiVersion: "v1",
		Kind:       "Pod",
		NamespaceSelector: &NamespaceSelector{
			NameSelector: &NameSelector{
				MatchNames: []string{"default", "prod", "stage"},
			},
		},
		NameSelector: &NameSelector{
			MatchNames: []string{"pod-1", "pod-2", "pod-3", "pod-4"},
		},
	}

	monitor.Metadata.MonitorId = "MonitorId"

	_, err := mgr.AddMonitor(monitor)

	if assert.NoError(t, err) {
		assert.Len(t, mgr.Monitors, 1)
	}
}

// FIXME: sometimes fails, skip for now.
//
// TODO: fake client is limited, move it to integration tests
//
// Test_MainKubeEventsManager_HandleEvents
// Scenario:
// - create new KubeEventManager, start informers
// - check if first event is Synchronization
// - add more objects
// - receive and check events with objects
func Test_MainKubeEventsManager_HandleEvents(t *testing.T) {
	t.SkipNow()
	timeout := time.Duration(3 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Add GVR
	kubeClient := kube.NewFakeKubernetesClient()
	fakeDiscovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
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

	dynClient := kubeClient.Dynamic()
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
	_, _ = dynClient.Resource(podGvr).Namespace("default").Create(context.TODO(), obj, metav1.CreateOptions{}, []string{}...)

	// Init() replacement
	mgr := NewKubeEventsManager()
	mgr.WithKubeClient(kubeClient)
	mgr.WithContext(ctx)
	mgr.KubeEventCh = make(chan KubeEvent, 10)

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
	monitor.Metadata.MonitorId = "MonitorId"

	_, err := mgr.AddMonitor(monitor)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	mgr.Start()

	fmt.Printf("mgr Started\n")

	// First event — Synchronization, second and third are Event
	eventCounter := 0
	done := false
	state := struct {
		podsCreated bool
		gotPod1     bool
		gotPod2     bool
	}{}
	for {
		fmt.Printf("Start select\n")
		select {
		case ev := <-mgr.Ch():
			eventCounter = eventCounter + 1
			t.Logf("Got event: %d %#v\n", eventCounter, ev)

			if !state.podsCreated {
				assert.Equal(t, "Synchronization", ev.Type)
				assert.Equal(t, "MonitorId", ev.MonitorId)
				assert.Len(t, ev.Objects, 1)

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
				_, _ = dynClient.Resource(podGvr).Namespace("default").Create(context.TODO(), obj1, metav1.CreateOptions{}, []string{}...)
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
				_, _ = dynClient.Resource(podGvr).Namespace("default").Create(context.TODO(), obj2, metav1.CreateOptions{}, []string{}...)
				t.Logf("DynamicClient Created pod\n")
				state.podsCreated = true
				break
			}

			assert.Equal(t, "Event", ev.Type)
			assert.Equal(t, "MonitorId", ev.MonitorId)
			assert.Equal(t, WatchEventAdded, ev.WatchEvents[0])
			assert.Len(t, ev.Objects, 1)

			name := ev.Objects[0].Object.GetName()
			if name == "pod-1" {
				state.gotPod1 = true
			}
			if name == "pod-2" {
				state.gotPod2 = true
			}
			if state.gotPod1 && state.gotPod2 {
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

// Test_FakeClient_CatchUpdates try to catch updates from fake client with additional WatchReactor
func Test_FakeClient_CatchUpdates(t *testing.T) {
	t.SkipNow()
	timeout := time.Duration(30 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Add GVR
	kubeClient := kube.NewFakeKubernetesClient()
	fakeDiscovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
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
					Verbs:   metav1.Verbs{"get", "list", "watch", "create", "patch", "update"},
					Group:   "",
					Version: "v1",
				},
			},
		},
	}

	dynClient := kubeClient.Dynamic()
	/*
		dc.PrependWatchReactor("*", func(action testing2.Action) (handled bool, ret watch.Interface, err error) {
			switch v := action.(type) {
			case testing2.CreateAction:
				gvr := action.GetResource()
				obj := v.GetObject()
				fmt.Printf("create object: %s %+v\n", gvr.String(), obj)
			case testing2.UpdateAction:
				gvr := action.GetResource()
				obj := v.GetObject()

				name, _, _ := metaFromEventObject(obj)

				oldObj, _ := dc.Resource(gvr).Namespace(action.GetNamespace()).Get(name, metav1.GetOptions{})

				fmt.Printf("updated object: %s\n>>> old: %+v\n>>> new: %+v\n", gvr.String(), oldObj.Object, obj)

			default:
				return false, nil, nil
			}
			return false, nil, nil
		})
	*/

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
	_, _ = dynClient.Resource(podGvr).Namespace("default").Create(context.TODO(), obj, metav1.CreateOptions{}, []string{}...)

	//// Init() replacement
	mgr := NewKubeEventsManager()
	mgr.WithContext(ctx)

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
	monitor.Metadata.MonitorId = "MonitorId"

	_, err := mgr.AddMonitor(monitor)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	mgr.Start()

	fmt.Printf("mgr Started\n")

	// First event — Synchronization, second and third are Event
	eventCounter := 0
	done := false
	state := struct {
		podsCreated bool
		gotPod1     bool
		gotPod2     bool
	}{}
	for {
		fmt.Printf("Start select\n")
		select {
		case ev := <-mgr.Ch():
			eventCounter = eventCounter + 1
			fmt.Printf("Got event: %d %#v\n", eventCounter, ev)
			//t.Logf("Got event: %d %#v\n", eventCounter, ev)

			if !state.podsCreated {
				assert.Equal(t, "Synchronization", ev.Type)
				assert.Equal(t, "MonitorId", ev.MonitorId)
				assert.Len(t, ev.Objects, 1)

				obj.Object["spec"] = "pod-0-new-spec"

				_, _ = dynClient.Resource(podGvr).Namespace("default").Update(context.TODO(), obj, metav1.UpdateOptions{}, []string{}...)

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
				_, _ = dynClient.Resource(podGvr).Namespace("default").Create(context.TODO(), obj1, metav1.CreateOptions{}, []string{}...)
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
				_, _ = dynClient.Resource(podGvr).Namespace("default").Create(context.TODO(), obj2, metav1.CreateOptions{}, []string{}...)
				t.Logf("DynamicClient Created pod\n")
				state.podsCreated = true
				break
			}

			assert.Equal(t, "Event", ev.Type)
			assert.Equal(t, "MonitorId", ev.MonitorId)
			assert.Equal(t, WatchEventAdded, ev.WatchEvents[0])
			assert.Len(t, ev.Objects, 1)

			name := ev.Objects[0].Object.GetName()
			if name == "pod-1" {
				state.gotPod1 = true
			}
			if name == "pod-2" {
				state.gotPod2 = true
			}
			if state.gotPod1 && state.gotPod2 {
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
