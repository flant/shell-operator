package kube_events_manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockKubeEventsInformer struct {
}

func (i *MockKubeEventsInformer) UpdateConfigId() string {
	return "ConfigId"
}

func (i *MockKubeEventsInformer) WithConfigId(string) {
	return
}

func (i *MockKubeEventsInformer) WithNamespace(string) {
	return
}

func (i *MockKubeEventsInformer) CreateSharedInformer() error {
	return nil
}

func (i *MockKubeEventsInformer) Run() {
	return
}

func (i *MockKubeEventsInformer) Stop() {
	return
}

func Test_MainKubeEventsManager_Run(t *testing.T) {
	// Init() replacement
	mgr := NewMainKubeEventsManager()

	// Mock KubeEventInformer constructor method
	NewKubeEventsInformer = func(monitor *MonitorConfig) KubeEventsInformer {
		return &MockKubeEventsInformer{}
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

	cfgId, err := mgr.Run(monitor)
	if assert.NoError(t, err) {
		assert.Equal(t, "ConfigId", cfgId)
		assert.Len(t, mgr.InformersStore[cfgId], 12)
	}
}
