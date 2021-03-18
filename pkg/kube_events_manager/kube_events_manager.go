package kube_events_manager

import (
	"context"
	"runtime/trace"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/kube"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric_storage"
)

type KubeEventsManager interface {
	WithContext(ctx context.Context)
	WithMetricStorage(mstor *metric_storage.MetricStorage)
	WithKubeClient(client kube.KubernetesClient)

	AddMonitor(monitorConfig *MonitorConfig) error
	HasMonitor(monitorID string) bool
	GetMonitor(monitorID string) Monitor
	StartMonitor(monitorID string)
	StopMonitor(monitorID string) error

	Ch() chan KubeEvent
	PauseHandleEvents()
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
	metricStorage *metric_storage.MetricStorage
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

func (mgr *kubeEventsManager) WithMetricStorage(mstor *metric_storage.MetricStorage) {
	mgr.metricStorage = mstor
}

func (mgr *kubeEventsManager) WithKubeClient(client kube.KubernetesClient) {
	mgr.KubeClient = client
}

// AddMonitor creates a monitor with informers and return a KubeEvent with existing objects.
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *kubeEventsManager) AddMonitor(monitorConfig *MonitorConfig) error {
	log.Debugf("Add MONITOR %+v", monitorConfig)
	monitor := NewMonitor()
	monitor.WithContext(mgr.ctx)
	monitor.WithKubeClient(mgr.KubeClient)
	monitor.WithMetricStorage(mgr.metricStorage)
	monitor.WithConfig(monitorConfig)
	monitor.WithKubeEventCb(func(ev KubeEvent) {
		defer trace.StartRegion(context.Background(), "EmitKubeEvent").End()
		mgr.KubeEventCh <- ev
	})

	err := monitor.CreateInformers()
	if err != nil {
		return err
	}

	mgr.Monitors[monitorConfig.Metadata.MonitorId] = monitor

	return nil
}

// HasMonitor returns true if there is a monitor with monitorID.
func (mgr *kubeEventsManager) HasMonitor(monitorID string) bool {
	_, has := mgr.Monitors[monitorID]
	return has
}

// GetMonitor returns monitor by its ID.
func (mgr *kubeEventsManager) GetMonitor(monitorID string) Monitor {
	return mgr.Monitors[monitorID]
}

// StartMonitor starts all informers for the monitor.
func (mgr *kubeEventsManager) StartMonitor(monitorID string) {
	monitor := mgr.Monitors[monitorID]
	monitor.Start(mgr.ctx)
}

// StopMonitor stops monitor and removes it from the index.
func (mgr *kubeEventsManager) StopMonitor(monitorID string) error {
	monitor, ok := mgr.Monitors[monitorID]
	if ok {
		monitor.Stop()
		delete(mgr.Monitors, monitorID)
	}
	return nil
}

// Ch returns a channel to receive KubeEvent objects.
func (mgr *kubeEventsManager) Ch() chan KubeEvent {
	return mgr.KubeEventCh
}

// PauseHandleEvents set flags for all informers to ignore incoming events.
// Useful for shutdown without panicking.
// Calling cancel() leads to a race and panicking, see https://github.com/kubernetes/kubernetes/issues/59822
func (mgr *kubeEventsManager) PauseHandleEvents() {
	for _, monitor := range mgr.Monitors {
		monitor.PauseHandleEvents()
	}
}
