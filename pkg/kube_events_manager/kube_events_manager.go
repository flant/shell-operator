package kube_events_manager

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type KubeEventsManager interface {
	WithContext(ctx context.Context)
	AddMonitor(name string, monitorConfig *MonitorConfig, logEntry *log.Entry) ([]ObjectAndFilterResult, error)
	HasMonitor(configId string) bool
	StartMonitor(configId string)
	Start()

	StopMonitor(configId string) error
	Ch() chan KubeEvent
}

// kubeEventsManager is a main implementation of KubeEventsManager.
type kubeEventsManager struct {
	// Array of monitors
	Monitors map[string]Monitor
	// channel to emit KubeEvent objects
	//KubeEventCh chan KubeEvent

	ctx    context.Context
	cancel context.CancelFunc
}

// kubeEventsManager should implement KubeEventsManager.
var _ KubeEventsManager = &kubeEventsManager{}

// NewKubeEventsManager returns an implementation of KubeEventsManager.
var NewKubeEventsManager = func() *kubeEventsManager {
	em := &kubeEventsManager{
		Monitors: make(map[string]Monitor, 0),
		//KubeEventCh: make(chan KubeEvent, 1),
	}

	KubeEventCh = make(chan KubeEvent, 1)

	return em
}

func (mgr *kubeEventsManager) WithContext(ctx context.Context) {
	mgr.ctx, mgr.cancel = context.WithCancel(ctx)
}

// AddMonitor creates a monitor with informers
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *kubeEventsManager) AddMonitor(name string, monitorConfig *MonitorConfig, logEntry *log.Entry) ([]ObjectAndFilterResult, error) {
	log.Debugf("Add MONITOR %+v", monitorConfig)
	monitor := NewMonitor()
	monitor.WithName(name)
	monitor.WithConfig(monitorConfig)

	err := monitor.CreateInformers(logEntry)
	if err != nil {
		return nil, err
	}

	mgr.Monitors[monitorConfig.Metadata.ConfigId] = monitor

	return monitor.GetExistedObjects(), nil
}

// HasMonitor returns true if there is a monitor with configId
func (mgr *kubeEventsManager) HasMonitor(configId string) bool {
	_, has := mgr.Monitors[configId]
	return has
}

// StartMonitor starts all informers for monitor
func (mgr *kubeEventsManager) StartMonitor(configId string) {
	monitor := mgr.Monitors[configId]
	monitor.Start(mgr.ctx)
}

// Start starts all informers, created by monitors
func (mgr *kubeEventsManager) Start() {
	for _, monitor := range mgr.Monitors {
		monitor.Start(mgr.ctx)
	}
}

// StopMonitor stops monitor and removes it from Monitors
func (mgr *kubeEventsManager) StopMonitor(configId string) error {
	monitor, ok := mgr.Monitors[configId]
	if ok {
		monitor.Stop()
		delete(mgr.Monitors, configId)
	}
	return nil
}

func (mgr *kubeEventsManager) Ch() chan KubeEvent {
	return nil
}

func (mgr *kubeEventsManager) StopAll() {
	mgr.cancel()
	// wait?
	//
	//informers, ok := mgr.InformersStore[configId]
	//if ok {
	//	for _, informer := range informers {
	//		informer.Stop()
	//	}
	//	delete(mgr.InformersStore, configId)
	//} else {
	//	log.Errorf("configId '%s' has no informers to stop", configId)
	//}
	//return nil
}
