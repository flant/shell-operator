package kubeeventsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/app"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
)

// --- Debouncer structs ---
type pendingEvent struct {
	Object    kemtypes.ObjectAndFilterResult
	EventType kemtypes.WatchEventType
}
type pendingState map[string]pendingEvent

type debouncer struct {
	outputChan    chan<- kemtypes.KubeEvent
	debounce      time.Duration
	pendingStates map[string]pendingState // monitorId -> state
	timers        map[string]*time.Timer  // monitorId -> timer
	mu            sync.Mutex
	logger        *log.Logger
}

func newDebouncer(outputChan chan<- kemtypes.KubeEvent, debounce time.Duration, logger *log.Logger) *debouncer {
	return &debouncer{
		outputChan:    outputChan,
		debounce:      debounce,
		pendingStates: make(map[string]pendingState),
		timers:        make(map[string]*time.Timer),
		logger:        logger.Named("debouncer"),
	}
}

func (d *debouncer) handleEvent(event kemtypes.KubeEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	monitorID := event.MonitorId
	fmt.Printf("[TRACE] Debouncer: received event for monitor '%s' with %d objects.\n", monitorID, len(event.Objects))

	if _, ok := d.pendingStates[monitorID]; !ok {
		d.pendingStates[monitorID] = make(pendingState)
	}
	state := d.pendingStates[monitorID]

	for i := range event.Objects {
		eventType := event.WatchEvents[i]
		resourceID := event.Objects[i].Metadata.ResourceId

		d.logger.Debug("debouncing event", "monitor.id", monitorID, "resource.id", resourceID, "event.type", eventType)
		state[resourceID] = pendingEvent{
			Object:    event.Objects[i],
			EventType: eventType,
		}

	}

	if timer, ok := d.timers[monitorID]; ok {
		fmt.Printf("[TRACE] Debouncer: resetting timer for monitor '%s'.\n", monitorID)
		timer.Stop()
	}

	d.timers[monitorID] = time.AfterFunc(d.debounce, func() {
		fmt.Printf("[TRACE] Debouncer: timer fired for monitor '%s'. Firing aggregated event.\n", monitorID)
		d.fire(monitorID)
	})
}

func (d *debouncer) fire(monitorID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, ok := d.pendingStates[monitorID]
	if !ok || len(state) == 0 {
		return
	}

	d.logger.Info("debounce timer fired", "monitor.id", monitorID, "events.count", len(state))
	fmt.Printf("[TRACE] Debouncer: sending %d aggregated events for monitor '%s'.\n", len(state), monitorID)

	// Создаем один агрегированный KubeEvent со всеми объектами
	finalEvent := kemtypes.KubeEvent{
		MonitorId:   monitorID,
		Type:        kemtypes.TypeEvent,
		WatchEvents: make([]kemtypes.WatchEventType, 0, len(state)),
		Objects:     make([]kemtypes.ObjectAndFilterResult, 0, len(state)),
	}

	for _, pEvent := range state {
		event := pEvent

		finalEvent.Objects = append(finalEvent.Objects, event.Object)
		finalEvent.WatchEvents = append(finalEvent.WatchEvents, event.EventType)
	}

	fmt.Printf("[TRACE] Debouncer: sending aggregated event with %d objects.\n", len(finalEvent.Objects))
	d.outputChan <- finalEvent

	delete(d.pendingStates, monitorID)
	delete(d.timers, monitorID)
}

// --- end Debouncer structs ---

type KubeEventsManager interface {
	WithMetricStorage(mstor metric.Storage)
	MetricStorage() metric.Storage
	AddMonitor(monitorConfig *MonitorConfig) error
	HasMonitor(monitorID string) bool
	GetMonitor(monitorID string) Monitor
	StartMonitor(monitorID string)
	StopMonitor(monitorID string) error

	Ch() chan kemtypes.KubeEvent
	PauseHandleEvents()
}

// kubeEventsManager is a main implementation of KubeEventsManager.
type kubeEventsManager struct {
	// channel to emit KubeEvent objects
	KubeEventCh chan kemtypes.KubeEvent

	KubeClient *klient.Client

	ctx           context.Context
	cancel        context.CancelFunc
	metricStorage metric.Storage
	debouncer     *debouncer

	m        sync.RWMutex
	Monitors map[string]Monitor

	logger *log.Logger
}

// kubeEventsManager should implement KubeEventsManager.
var _ KubeEventsManager = (*kubeEventsManager)(nil)

// NewKubeEventsManager returns an implementation of KubeEventsManager.
func NewKubeEventsManager(ctx context.Context, kubeClient *klient.Client, logger *log.Logger) KubeEventsManager {
	cctx, cancel := context.WithCancel(ctx)

	mgr := &kubeEventsManager{
		KubeEventCh: make(chan kemtypes.KubeEvent, 1000),
		KubeClient:  kubeClient,
		ctx:         cctx,
		cancel:      cancel,
		Monitors:    make(map[string]Monitor),
		logger:      logger.Named("kube-events-manager"),
	}

	// Initialize debouncer only if feature flag is enabled
	if app.IsEventDebouncerEnabled() {
		mgr.debouncer = newDebouncer(mgr.KubeEventCh, 2*time.Second, logger)
		logger.Info("Event debouncer enabled with 2-second delay")
	} else {
		logger.Info("Event debouncer disabled")
	}

	return mgr
}

func (mgr *kubeEventsManager) WithMetricStorage(mstor metric.Storage) {
	mgr.metricStorage = mstor
}

// AddMonitor creates a monitor with informers and return a KubeEvent with existing objects.
// TODO cleanup informers in case of error
// TODO use Context to stop informers
func (mgr *kubeEventsManager) AddMonitor(monitorConfig *MonitorConfig) error {
	log.Debug("Add MONITOR",
		slog.String("config", fmt.Sprintf("%+v", monitorConfig)))

	// Create event callback based on whether debouncer is enabled
	var eventCb func(ev kemtypes.KubeEvent)
	if app.IsEventDebouncerEnabled() && mgr.debouncer != nil {
		eventCb = func(ev kemtypes.KubeEvent) {
			mgr.debouncer.handleEvent(ev)
		}
	} else {
		eventCb = func(ev kemtypes.KubeEvent) {
			mgr.KubeEventCh <- ev
		}
	}

	monitor := NewMonitor(
		mgr.ctx,
		mgr.KubeClient,
		mgr.metricStorage,
		monitorConfig,
		eventCb,
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

func (mgr *kubeEventsManager) MetricStorage() metric.Storage {
	return mgr.metricStorage
}
