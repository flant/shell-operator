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
	InformersStore map[string][]*KubeEventsInformer
}

func NewMainKubeEventsManager() *MainKubeEventsManager {
	em := &MainKubeEventsManager{
		InformersStore: map[string][]*KubeEventsInformer{},
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
func (mgr *MainKubeEventsManager) Run(monitorConfig *MonitorConfig) (string, error) {
	if monitorConfig.IsAnyNamespace() {
		informer := NewKubeEventsInformer(monitorConfig)
		informer.UpdateConfigId()
		err := informer.CreateSharedInformer()
		if err != nil {
			return "", err
		}

		mgr.AddInformer(informer)

		go informer.Run()
		return informer.ConfigId, nil
	}

	// TODO cleanup informers in case of error
	if monitorConfig.NamespaceSelector != nil {
		configId := NewKubeEventsInformer(monitorConfig).UpdateConfigId()
		for _, ns := range monitorConfig.NamespaceSelector.NameSelector.MatchNames {
			informer := NewKubeEventsInformer(monitorConfig)
			informer.ConfigId = configId
			informer.Namespace = ns
			err := informer.CreateSharedInformer()
			if err != nil {
				return "", err
			}

			mgr.AddInformer(informer)
		}

		for _, informer := range mgr.InformersStore[configId] {
			go informer.Run()
		}

		return configId, nil
	}

	return "", fmt.Errorf("unsupported namespace selector %+v", monitorConfig.NamespaceSelector)
}

func (mgr *MainKubeEventsManager) AddInformer(informer *KubeEventsInformer) {
	configId := informer.ConfigId

	if _, has := mgr.InformersStore[configId]; !has {
		mgr.InformersStore[configId] = make([]*KubeEventsInformer, 0)
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
