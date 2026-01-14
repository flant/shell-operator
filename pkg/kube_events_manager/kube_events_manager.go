package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/trace"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	klient "github.com/flant/kube-client/client"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

type KubeEventsManager interface {
	WithMetricStorage(mstor metricsstorage.Storage)
	MetricStorage() metricsstorage.Storage
	AddMonitor(monitorConfig *MonitorConfig) error
	HasMonitor(monitorID string) bool
	GetMonitor(monitorID string) Monitor
	StartMonitor(monitorID string)
	StopMonitor(monitorID string) error

	Ch() chan kemtypes.KubeEvent
	Stop()
	Wait()
}

// kubeEventsManager is a main implementation of KubeEventsManager.
type kubeEventsManager struct {
	// channel to emit KubeEvent objects
	KubeEventCh chan kemtypes.KubeEvent

	KubeClient *klient.Client

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage metricsstorage.Storage

	m        sync.RWMutex
	Monitors map[string]Monitor

	logger *log.Logger
}

// kubeEventsManager should implement KubeEventsManager.
var _ KubeEventsManager = (*kubeEventsManager)(nil)

// NewKubeEventsManager returns an implementation of KubeEventsManager.
func NewKubeEventsManager(ctx context.Context, client *klient.Client, logger *log.Logger) *kubeEventsManager {
	cctx, cancel := context.WithCancel(ctx)
	em := &kubeEventsManager{
		ctx:         cctx,
		cancel:      cancel,
		KubeClient:  client,
		m:           sync.RWMutex{},
		Monitors:    make(map[string]Monitor),
		KubeEventCh: make(chan kemtypes.KubeEvent, 1),
		logger:      logger,
	}
	return em
}

func (mgr *kubeEventsManager) WithMetricStorage(mstor metricsstorage.Storage) {
	mgr.metricStorage = mstor
}

// AddMonitor creates a monitor with informers and return a KubeEvent with existing objects.
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *kubeEventsManager) AddMonitor(monitorConfig *MonitorConfig) error {
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
func (mgr *kubeEventsManager) Ch() chan kemtypes.KubeEvent {
	return mgr.KubeEventCh
}

// Stop the kube events manager and all the informers inside monitors.
func (mgr *kubeEventsManager) Stop() {
	mgr.cancel()
}

func (mgr *kubeEventsManager) Wait() {
	mgr.m.RLock()
	monitors := make([]Monitor, 0, len(mgr.Monitors))
	for _, mon := range mgr.Monitors {
		monitors = append(monitors, mon)
	}
	mgr.m.RUnlock()
	for _, mon := range monitors {
		mon.Wait()
	}
}

func (mgr *kubeEventsManager) MetricStorage() metricsstorage.Storage {
	return mgr.metricStorage
}
