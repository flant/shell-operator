package kube_events_manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	NewResourceInformer = func(monitor *MonitorConfig) ResourceInformer {
		return &MockResourceInformer{}
	}

	// TODO make more test cases

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
		assert.Len(t, mgr.Monitors["ConfigId"], 12)
	}
}

//func Test_MainKubeEventsManager_Life(t *testing.T) {
//	client := fake.NewSimpleClientset()
//
//}
