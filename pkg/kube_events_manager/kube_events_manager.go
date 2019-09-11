package kube_events_manager

import (
	"context"

	"github.com/romana/rlog"
)

type KubeEventsManager interface {
	WithContext(ctx context.Context)
	AddMonitor(name string, monitorConfig *MonitorConfig) error
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
func (mgr *kubeEventsManager) AddMonitor(name string, monitorConfig *MonitorConfig) error {
	rlog.Debugf("Add MOINITOR %+v", monitorConfig)
	monitor := NewMonitor()
	monitor.WithName(name)
	monitor.WithConfig(monitorConfig)

	err := monitor.CreateInformers()
	if err != nil {
		return err
	}

	mgr.Monitors[monitorConfig.Metadata.ConfigId] = monitor

	return nil
}

// Start starts all informers, created by monitors
func (mgr *kubeEventsManager) Start() {
	rlog.Infof("Start monitors: %d", len(mgr.Monitors))
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
	//	rlog.Errorf("configId '%s' has no informers to stop", configId)
	//}
	//return nil
}
