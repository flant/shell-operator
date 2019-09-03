package kube_events_manager

import (
	"fmt"

	"github.com/romana/rlog"
)

type KubeEventsManager interface {
	Run(monitorConfig *MonitorConfig) (string, error)
	Stop(configId string) error
}

type MainKubeEventsManager struct {
	// all created kube informers. Informers are addressed by config id — uuid
	InformersStore map[string][]KubeEventsInformer
}

var NewMainKubeEventsManager = func() *MainKubeEventsManager {
	em := &MainKubeEventsManager{
		InformersStore: map[string][]KubeEventsInformer{},
	}
	return em
}

func Init() (KubeEventsManager, error) {
	em := NewMainKubeEventsManager()
	KubeEventCh = make(chan KubeEvent, 1)
	return em, nil
}

// Run launches informer for each namespace in MonitorConfig.NamespaceSelector.MatchNames or
// for one informer for any namespace if NamespaceSelector is nil
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *MainKubeEventsManager) Run(monitorConfig *MonitorConfig) (string, error) {
	nsNames := monitorConfig.Namespaces()
	if len(nsNames) == 0 {
		return "", fmt.Errorf("unsupported namespace selector %+v", monitorConfig.NamespaceSelector)
	}

	// Generate new config id
	configId := NewKubeEventsInformer(monitorConfig).UpdateConfigId()
	// create informers for each specified object name in each specified namespace
	for _, nsName := range nsNames {
		objNames := monitorConfig.Names()
		if len(objNames) == 0 {
			objNames = []string{""}
		}

		for _, objName := range objNames {
			if objName != "" {
				monitorConfig.AddFieldSelectorRequirement("metadata.name", "=", objName)
			}
			informer := NewKubeEventsInformer(monitorConfig)
			informer.WithConfigId(configId)
			informer.WithNamespace(nsName)
			err := informer.CreateSharedInformer()
			if err != nil {
				return "", err
			}
			mgr.AddInformer(informer, configId)
		}
	}

	for _, informer := range mgr.InformersStore[configId] {
		go informer.Run()
	}
	return configId, nil
}

func (mgr *MainKubeEventsManager) AddInformer(informer KubeEventsInformer, configId string) {
	if _, has := mgr.InformersStore[configId]; !has {
		mgr.InformersStore[configId] = make([]KubeEventsInformer, 0)
	}

	mgr.InformersStore[configId] = append(mgr.InformersStore[configId], informer)
}

func (mgr *MainKubeEventsManager) Stop(configId string) error {
	informers, ok := mgr.InformersStore[configId]
	if ok {
		for _, informer := range informers {
			informer.Stop()
		}
		delete(mgr.InformersStore, configId)
	} else {
		rlog.Errorf("configId '%s' has no informers to stop", configId)
	}
	return nil
}
