package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/trace"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	klient "github.com/flant/kube-client/client"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
)

// KubeEventsManager is a main implementation of KubeEventsManager.
type KubeEventsManager struct {
	// channel to emit KubeEvent objects
	KubeEventCh chan kemtypes.KubeEvent

	KubeClient *klient.Client

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage metric.Storage

	m        sync.RWMutex
	Monitors map[string]*Monitor

	logger *log.Logger
}

// NewKubeEventsManager returns an implementation of KubeEventsManager.
func NewKubeEventsManager(ctx context.Context, client *klient.Client, logger *log.Logger) *KubeEventsManager {
	cctx, cancel := context.WithCancel(ctx)
	em := &KubeEventsManager{
		ctx:         cctx,
		cancel:      cancel,
		KubeClient:  client,
		m:           sync.RWMutex{},
		Monitors:    make(map[string]*Monitor),
		KubeEventCh: make(chan kemtypes.KubeEvent, 1),
		logger:      logger,
	}
	return em
}

func (mgr *KubeEventsManager) WithMetricStorage(mstor metric.Storage) {
	mgr.metricStorage = mstor
}

// AddMonitor creates a monitor with informers and return a KubeEvent with existing objects.
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *KubeEventsManager) AddMonitor(monitorConfig *MonitorConfig) error {
	log.Debug("Add MONITOR",
		slog.String("config", fmt.Sprintf("%+v", monitorConfig)))
	monitor := NewMonitor(
		mgr.ctx,
		mgr.KubeClient,
		mgr.metricStorage,
		monitorConfig,
		func(ev kemtypes.KubeEvent) {
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
func (mgr *KubeEventsManager) HasMonitor(monitorID string) bool {
	mgr.m.RLock()
	_, has := mgr.Monitors[monitorID]
	mgr.m.RUnlock()
	return has
}

// GetMonitor returns monitor by its ID.
func (mgr *KubeEventsManager) GetMonitor(monitorID string) *Monitor {
	mgr.m.RLock()
	defer mgr.m.RUnlock()
	return mgr.Monitors[monitorID]
}

// StartMonitor starts all informers for the monitor.
func (mgr *KubeEventsManager) StartMonitor(monitorID string) {
	mgr.m.RLock()
	monitor, ok := mgr.Monitors[monitorID]
	if !ok {
		mgr.m.RUnlock()
		mgr.logger.Error("Monitor not found", slog.String("monitorID", monitorID))
		return
	}
	mgr.m.RUnlock()
	monitor.Start(mgr.ctx)
}

// StopMonitor stops monitor and removes it from the index.
func (mgr *KubeEventsManager) StopMonitor(monitorID string) error {
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
func (mgr *KubeEventsManager) Ch() chan kemtypes.KubeEvent {
	return mgr.KubeEventCh
}

// PauseHandleEvents set flags for all informers to ignore incoming events.
// Useful for shutdown without panicking.
// Calling cancel() leads to a race and panicking, see https://github.com/kubernetes/kubernetes/issues/59822
func (mgr *KubeEventsManager) PauseHandleEvents() {
	mgr.m.RLock()
	defer mgr.m.RUnlock()
	for _, monitor := range mgr.Monitors {
		monitor.PauseHandleEvents()
	}
}
