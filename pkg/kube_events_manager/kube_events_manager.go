package kube_events_manager

import (
	"context"
	"runtime/trace"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metrics_storage"
)

type KubeEventsManager interface {
	WithContext(ctx context.Context)
	WithMetricStorage(mstor *metrics_storage.MetricStorage)
	WithKubeClient(client kube.KubernetesClient)
	AddMonitor(monitorConfig *MonitorConfig) (*KubeEvent, error)
	HasMonitor(monitorId string) bool
	GetMonitor(monitorId string) Monitor
	StartMonitor(monitorId string)
	Start()

	StopMonitor(configId string) error
	Ch() chan KubeEvent
}

// kubeEventsManager is a main implementation of KubeEventsManager.
type kubeEventsManager struct {
	// Array of monitors
	Monitors map[string]Monitor
	// channel to emit KubeEvent objects
	KubeEventCh chan KubeEvent

	KubeClient kube.KubernetesClient

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage *metrics_storage.MetricStorage
}

// kubeEventsManager should implement KubeEventsManager.
var _ KubeEventsManager = &kubeEventsManager{}

// NewKubeEventsManager returns an implementation of KubeEventsManager.
var NewKubeEventsManager = func() *kubeEventsManager {
	em := &kubeEventsManager{
		Monitors:    make(map[string]Monitor),
		KubeEventCh: make(chan KubeEvent, 1),
	}
	return em
}

func (mgr *kubeEventsManager) WithContext(ctx context.Context) {
	mgr.ctx, mgr.cancel = context.WithCancel(ctx)
}

func (mgr *kubeEventsManager) WithMetricStorage(mstor *metrics_storage.MetricStorage) {
	mgr.metricStorage = mstor
}

func (mgr *kubeEventsManager) WithKubeClient(client kube.KubernetesClient) {
	mgr.KubeClient = client
}

// AddMonitor creates a monitor with informers and return a KubeEvent with existing objects.
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *kubeEventsManager) AddMonitor(monitorConfig *MonitorConfig) (*KubeEvent, error) {
	log.Debugf("Add MONITOR %+v", monitorConfig)
	monitor := NewMonitor()
	monitor.WithKubeClient(mgr.KubeClient)
	monitor.WithMetricStorage(mgr.metricStorage)
	monitor.WithConfig(monitorConfig)
	monitor.WithKubeEventCb(func(ev KubeEvent) {
		defer trace.StartRegion(context.Background(), "EmitKubeEvent").End()
		outEvent := mgr.MakeKubeEvent(monitor, ev)
		if outEvent != nil {
			mgr.KubeEventCh <- *outEvent
		}
	})

	err := monitor.CreateInformers()
	if err != nil {
		return nil, err
	}

	mgr.Monitors[monitorConfig.Metadata.MonitorId] = monitor

	return mgr.MakeKubeEvent(monitor), nil
}

func (mgr *kubeEventsManager) MakeKubeEvent(monitor Monitor, ev ...KubeEvent) *KubeEvent {
	if len(ev) == 0 {
		// Ignore first Synchronization for v0 config
		if monitor.GetConfig().Mode == ModeV0 {
			return nil
		}
		return &KubeEvent{
			MonitorId: monitor.GetConfig().Metadata.MonitorId,
			Type:      TypeSynchronization,
			Objects:   monitor.GetExistedObjects(),
		}
	}

	return &KubeEvent{
		MonitorId:   ev[0].MonitorId,
		Type:        TypeEvent,
		WatchEvents: ev[0].WatchEvents,
		Objects:     ev[0].Objects,
		//Object:       ev[0].Object,
		//FilterResult: ev[0].FilterResult,
	}
}

// HasMonitor returns true if there is a monitor with configId
func (mgr *kubeEventsManager) HasMonitor(monitorId string) bool {
	_, has := mgr.Monitors[monitorId]
	return has
}

func (mgr *kubeEventsManager) GetMonitor(monitorId string) Monitor {
	return mgr.Monitors[monitorId]
}

// StartMonitor starts all informers for monitor
func (mgr *kubeEventsManager) StartMonitor(monitorId string) {
	monitor := mgr.Monitors[monitorId]
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
	return mgr.KubeEventCh
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
