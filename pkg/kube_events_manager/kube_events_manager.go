package kube_events_manager

import (
	"context"
	"runtime/trace"
	"sync"

	"github.com/deckhouse/deckhouse/go_lib/log"

	klient "github.com/flant/kube-client/client"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric_storage"
)

type KubeEventsManager interface {
	WithMetricStorage(mstor *metric_storage.MetricStorage)
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
	// channel to emit KubeEvent objects
	KubeEventCh chan KubeEvent

	KubeClient *klient.Client

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage *metric_storage.MetricStorage

	m        sync.RWMutex
	Monitors map[string]Monitor

	logger *log.Logger
}

// kubeEventsManager should implement KubeEventsManager.
var _ KubeEventsManager = &kubeEventsManager{}

// NewKubeEventsManager returns an implementation of KubeEventsManager.
func NewKubeEventsManager(ctx context.Context, client *klient.Client, logger *log.Logger) *kubeEventsManager {
	cctx, cancel := context.WithCancel(ctx)
	em := &kubeEventsManager{
		ctx:         cctx,
		cancel:      cancel,
		KubeClient:  client,
		m:           sync.RWMutex{},
		Monitors:    make(map[string]Monitor),
		KubeEventCh: make(chan KubeEvent, 1),
		logger:      logger,
	}
	return em
}

func (mgr *kubeEventsManager) WithMetricStorage(mstor *metric_storage.MetricStorage) {
	mgr.metricStorage = mstor
}

// AddMonitor creates a monitor with informers and return a KubeEvent with existing objects.
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *kubeEventsManager) AddMonitor(monitorConfig *MonitorConfig) error {
	log.Debugf("Add MONITOR %+v", monitorConfig)
	monitor := NewMonitor(
		mgr.ctx,
		mgr.KubeClient,
		mgr.metricStorage,
		monitorConfig,
		func(ev KubeEvent) {
			defer trace.StartRegion(context.Background(), "EmitKubeEvent").End()
			mgr.KubeEventCh <- ev
		},
		mgr.logger.Named("monitor"),
	)

	err := monitor.CreateInformers()
	if err != nil {
		return err
	}

	mgr.m.Lock()
	mgr.Monitors[monitorConfig.Metadata.MonitorId] = monitor
	mgr.m.Unlock()

	return nil
}

// HasMonitor returns true if there is a monitor with monitorID.
func (mgr *kubeEventsManager) HasMonitor(monitorID string) bool {
	mgr.m.RLock()
	_, has := mgr.Monitors[monitorID]
	mgr.m.RUnlock()
	return has
}

// GetMonitor returns monitor by its ID.
func (mgr *kubeEventsManager) GetMonitor(monitorID string) Monitor {
	mgr.m.RLock()
	defer mgr.m.RUnlock()
	return mgr.Monitors[monitorID]
}

// StartMonitor starts all informers for the monitor.
func (mgr *kubeEventsManager) StartMonitor(monitorID string) {
	mgr.m.RLock()
	monitor := mgr.Monitors[monitorID]
	mgr.m.RUnlock()
	monitor.Start(mgr.ctx)
}

// StopMonitor stops monitor and removes it from the index.
func (mgr *kubeEventsManager) StopMonitor(monitorID string) error {
	mgr.m.RLock()
	monitor, ok := mgr.Monitors[monitorID]
	mgr.m.RUnlock()
	if ok {
		monitor.Stop()
		mgr.m.Lock()
		delete(mgr.Monitors, monitorID)
		mgr.m.Unlock()
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
	mgr.m.RLock()
	defer mgr.m.RUnlock()
	for _, monitor := range mgr.Monitors {
		monitor.PauseHandleEvents()
	}
}
