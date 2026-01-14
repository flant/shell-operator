package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

// KubernetesBindingToMonitorLink is a link between a binding config and a Monitor.
type KubernetesBindingToMonitorLink struct {
	MonitorId     string
	BindingConfig htypes.OnKubernetesEventConfig
}

// KubernetesBindingsController handles kubernetes bindings for one hook.
type KubernetesBindingsController interface {
	WithKubernetesBindings([]htypes.OnKubernetesEventConfig)
	WithKubeEventsManager(kubeeventsmanager.KubeEventsManager)
	EnableKubernetesBindings() ([]BindingExecutionInfo, error)
	UpdateMonitor(monitorId string, kind, apiVersion string) error
	UnlockEvents()
	UnlockEventsFor(monitorID string)
	StopMonitors()
	CanHandleEvent(kubeEvent kemtypes.KubeEvent) bool
	HandleEvent(ctx context.Context, kubeEvent kemtypes.KubeEvent) BindingExecutionInfo
	BindingNames() []string

	SnapshotsFrom(bindingNames ...string) map[string][]kemtypes.ObjectAndFilterResult
	SnapshotsFor(bindingName string) []kemtypes.ObjectAndFilterResult
	Snapshots() map[string][]kemtypes.ObjectAndFilterResult
	SnapshotsInfo() []string
	SnapshotsDump() map[string]interface{}
}

// kubernetesHooksController is a main implementation of KubernetesHooksController
type kubernetesBindingsController struct {
	// bindings configurations
	KubernetesBindings []htypes.OnKubernetesEventConfig

	// dependencies
	kubeEventsManager kubeeventsmanager.KubeEventsManager

	logger *log.Logger

	l sync.RWMutex
	// All hooks with 'kubernetes' bindings
	BindingMonitorLinks map[string]*KubernetesBindingToMonitorLink
}

// kubernetesHooksController should implement the KubernetesHooksController
var _ KubernetesBindingsController = (*kubernetesBindingsController)(nil)

// NewKubernetesBindingsController returns an implementation of KubernetesBindingsController
var NewKubernetesBindingsController = func(logger *log.Logger) *kubernetesBindingsController {
	return &kubernetesBindingsController{
		BindingMonitorLinks: make(map[string]*KubernetesBindingToMonitorLink),
		logger:              logger,
	}
}

func (c *kubernetesBindingsController) WithKubernetesBindings(bindings []htypes.OnKubernetesEventConfig) {
	c.KubernetesBindings = bindings
}

func (c *kubernetesBindingsController) WithKubeEventsManager(kubeEventsManager kubeeventsmanager.KubeEventsManager) {
	c.kubeEventsManager = kubeEventsManager
}

// EnableKubernetesBindings adds a monitor for each 'kubernetes' binding. This method
// returns an array of BindingExecutionInfo to help construct initial tasks to run hooks.
// Informers in each monitor are started immediately to keep up the "fresh" state of object caches.
func (c *kubernetesBindingsController) EnableKubernetesBindings() ([]BindingExecutionInfo, error) {
	res := make([]BindingExecutionInfo, 0)

	for _, config := range c.KubernetesBindings {
		err := c.kubeEventsManager.AddMonitor(config.Monitor)
		if err != nil {
			return nil, fmt.Errorf("run monitor: %s", err)
		}
		c.setBindingMonitorLinks(config.Monitor.Metadata.MonitorId, &KubernetesBindingToMonitorLink{
			MonitorId:     config.Monitor.Metadata.MonitorId,
			BindingConfig: config,
		})
		// Start monitor's informers to fill the cache.
		c.kubeEventsManager.StartMonitor(config.Monitor.Metadata.MonitorId)

		synchronizationInfo := c.HandleEvent(context.TODO(), kemtypes.KubeEvent{
			MonitorId: config.Monitor.Metadata.MonitorId,
			Type:      kemtypes.TypeSynchronization,
		})
		res = append(res, synchronizationInfo)
	}

	return res, nil
}

func (c *kubernetesBindingsController) UpdateMonitor(monitorId string, kind, apiVersion string) error {
	// Find binding for monitorId
	link, ok := c.getBindingMonitorLinksById(monitorId)
	if !ok {
		return nil
	}

	bindingName := link.BindingConfig.BindingName
	// Stop and remove previous monitor instance.
	err := c.kubeEventsManager.StopMonitor(monitorId)
	if err != nil {
		return fmt.Errorf("stop monitor for binding '%s': %v", bindingName, err)
	}

	// Update monitor config if kind or apiVersion are changed.
	if link.BindingConfig.Monitor.Kind != kind || link.BindingConfig.Monitor.ApiVersion != apiVersion {
		link.BindingConfig.Monitor.Kind = kind
		link.BindingConfig.Monitor.ApiVersion = apiVersion
		link.BindingConfig.Monitor.Metadata.MetricLabels["kind"] = kind
	}

	// Recreate monitor with new kind.
	err = c.kubeEventsManager.AddMonitor(link.BindingConfig.Monitor)
	if err != nil {
		return fmt.Errorf("recreate monitor for binding '%s': %v", bindingName, err)
	}

	utils.EnrichLoggerWithLabels(c.logger, link.BindingConfig.Monitor.Metadata.LogLabels).
		Info("Monitor is recreated",
			slog.String("bindingName", link.BindingConfig.BindingName),
			slog.String("kind", link.BindingConfig.Monitor.Kind),
			slog.String("apiVersion", link.BindingConfig.Monitor.ApiVersion))

	// Synchronization has no meaning for UpdateMonitor. Just emit Added event to handle objects of
	// a new kind.
	kubeEvent := kemtypes.KubeEvent{
		MonitorId:   monitorId,
		Type:        kemtypes.TypeEvent,
		WatchEvents: []kemtypes.WatchEventType{kemtypes.WatchEventAdded},
		Objects:     nil,
	}
	c.kubeEventsManager.Ch() <- kubeEvent

	// Start monitor and allow emitting kubernetes events immediately.
	c.kubeEventsManager.StartMonitor(monitorId)
	c.UnlockEventsFor(monitorId)

	return nil
}

// UnlockEvents turns on eventCb for all monitors to emit events after Synchronization.
func (c *kubernetesBindingsController) UnlockEvents() {
	c.iterateBindingMonitorLinks(func(monitorID string) bool {
		m := c.kubeEventsManager.GetMonitor(monitorID)
		m.EnableKubeEventCb()
		return false
	})
}

// UnlockEventsFor turns on eventCb for matched monitor to emit events after Synchronization.
func (c *kubernetesBindingsController) UnlockEventsFor(monitorID string) {
	m := c.kubeEventsManager.GetMonitor(monitorID)
	if m == nil {
		log.Warn("monitor was not found", slog.String("monitorID", monitorID))
		return
	}
	m.EnableKubeEventCb()
}

// StopMonitors stops all monitors for the hook.
// TODO handle error!
func (c *kubernetesBindingsController) StopMonitors() {
	c.iterateBindingMonitorLinks(func(monitorID string) bool {
		_ = c.kubeEventsManager.StopMonitor(monitorID)
		return false
	})
}

func (c *kubernetesBindingsController) CanHandleEvent(kubeEvent kemtypes.KubeEvent) bool {
	var canHandleEvent bool

	c.iterateBindingMonitorLinks(func(monitorID string) bool {
		if monitorID == kubeEvent.MonitorId {
			canHandleEvent = true
			return true
		}
		return false
	})

	return canHandleEvent
}

func (c *kubernetesBindingsController) iterateBindingMonitorLinks(doFn func(monitorID string) bool) {
	c.l.RLock()
	for monitorID := range c.BindingMonitorLinks {
		if exit := doFn(monitorID); exit {
			break
		}
	}
	c.l.RUnlock()
}

func (c *kubernetesBindingsController) getBindingMonitorLinksById(monitorId string) (*KubernetesBindingToMonitorLink, bool) {
	c.l.RLock()
	link, found := c.BindingMonitorLinks[monitorId]
	c.l.RUnlock()
	return link, found
}

func (c *kubernetesBindingsController) setBindingMonitorLinks(monitorId string, link *KubernetesBindingToMonitorLink) {
	c.l.Lock()
	c.BindingMonitorLinks[monitorId] = link
	c.l.Unlock()
}

// HandleEvent receives event from KubeEventManager and returns a BindingExecutionInfo
// to help create a new task to run a hook.
func (c *kubernetesBindingsController) HandleEvent(_ context.Context, kubeEvent kemtypes.KubeEvent) BindingExecutionInfo {
	link, hasKey := c.getBindingMonitorLinksById(kubeEvent.MonitorId)
	if !hasKey {
		log.Error("Possible bug!!! Unknown kube event: no such monitor id registered", slog.String("monitorID", kubeEvent.MonitorId))
		return BindingExecutionInfo{
			BindingContext: []bctx.BindingContext{},
			AllowFailure:   false,
		}
	}

	bindingContext := ConvertKubeEventToBindingContext(kubeEvent, link)

	bInfo := BindingExecutionInfo{
		BindingContext:    bindingContext,
		IncludeSnapshots:  link.BindingConfig.IncludeSnapshotsFrom,
		AllowFailure:      link.BindingConfig.AllowFailure,
		QueueName:         link.BindingConfig.Queue,
		Binding:           link.BindingConfig.BindingName,
		Group:             link.BindingConfig.Group,
		KubernetesBinding: link.BindingConfig,
	}
	return bInfo
}

func (c *kubernetesBindingsController) BindingNames() []string {
	names := make([]string, 0, len(c.KubernetesBindings))
	for _, binding := range c.KubernetesBindings {
		names = append(names, binding.BindingName)
	}
	return names
}

// SnapshotsFor returns snapshot for single onKubernetes binding.
// It finds a monitorId for a binding name and returns an array of objects.
func (c *kubernetesBindingsController) SnapshotsFor(bindingName string) []kemtypes.ObjectAndFilterResult {
	for _, binding := range c.KubernetesBindings {
		if bindingName == binding.BindingName {
			monitorID := binding.Monitor.Metadata.MonitorId
			if c.kubeEventsManager.HasMonitor(monitorID) {
				return c.kubeEventsManager.GetMonitor(monitorID).Snapshot()
			}
		}
	}

	return nil
}

// SnapshotsFrom returns snapshot for several binding names.
// It finds a monitorId for each binding name and get its Snapshot,
// then returns a map of object arrays for each binding name.
func (c *kubernetesBindingsController) SnapshotsFrom(bindingNames ...string) map[string][]kemtypes.ObjectAndFilterResult {
	res := map[string][]kemtypes.ObjectAndFilterResult{}

	for _, bindingName := range bindingNames {
		// Initialize all keys with empty arrays.
		res[bindingName] = make([]kemtypes.ObjectAndFilterResult, 0)

		snapshot := c.SnapshotsFor(bindingName)
		if snapshot != nil {
			res[bindingName] = snapshot
		}
	}

	return res
}

func (c *kubernetesBindingsController) Snapshots() map[string][]kemtypes.ObjectAndFilterResult {
	return c.SnapshotsFrom(c.BindingNames()...)
}

func (c *kubernetesBindingsController) SnapshotsInfo() []string {
	infos := make([]string, 0)
	for _, binding := range c.KubernetesBindings {
		monitorID := binding.Monitor.Metadata.MonitorId
		if c.kubeEventsManager.HasMonitor(monitorID) {
			total, last := c.kubeEventsManager.GetMonitor(monitorID).SnapshotOperations()

			info := fmt.Sprintf("%s: size=%d, operations since last execution: add=%d, mod=%d, del=%d, clear=%d, operations since start: add=%d, mod=%d, del=%d",
				binding.BindingName,
				total.Count,
				last.Added,
				last.Modified,
				last.Deleted,
				last.Cleaned,
				total.Added,
				total.Modified,
				total.Deleted,
			)
			infos = append(infos, info)
		}
	}

	return infos
}

func (c *kubernetesBindingsController) SnapshotsDump() map[string]interface{} {
	dumps := make(map[string]interface{})
	for _, binding := range c.KubernetesBindings {
		monitorID := binding.Monitor.Metadata.MonitorId
		if c.kubeEventsManager.HasMonitor(monitorID) {
			total, last := c.kubeEventsManager.GetMonitor(monitorID).SnapshotOperations()
			dumps[binding.BindingName] = map[string]interface{}{
				"snapshot": c.kubeEventsManager.GetMonitor(monitorID).Snapshot(),
				"operations": map[string]interface{}{
					"sinceStart":         total,
					"sinceLastExecution": last,
				},
			}
		}
	}

	return dumps
}

func ConvertKubeEventToBindingContext(kubeEvent kemtypes.KubeEvent, link *KubernetesBindingToMonitorLink) []bctx.BindingContext {
	bindingContexts := make([]bctx.BindingContext, 0)

	switch kubeEvent.Type {
	case kemtypes.TypeSynchronization:
		bc := bctx.BindingContext{
			Binding: link.BindingConfig.BindingName,
			Type:    kubeEvent.Type,
			Objects: kubeEvent.Objects,
		}
		bc.Metadata.JqFilter = link.BindingConfig.Monitor.JqFilter
		bc.Metadata.BindingType = htypes.OnKubernetesEvent
		bc.Metadata.IncludeSnapshots = link.BindingConfig.IncludeSnapshotsFrom
		bc.Metadata.Group = link.BindingConfig.Group

		bindingContexts = append(bindingContexts, bc)

	case kemtypes.TypeEvent:
		for _, kEvent := range kubeEvent.WatchEvents {
			bc := bctx.BindingContext{
				Binding:    link.BindingConfig.BindingName,
				Type:       kubeEvent.Type,
				WatchEvent: kEvent,
				Objects:    kubeEvent.Objects,
			}
			bc.Metadata.JqFilter = link.BindingConfig.Monitor.JqFilter
			bc.Metadata.BindingType = htypes.OnKubernetesEvent
			bc.Metadata.IncludeSnapshots = link.BindingConfig.IncludeSnapshotsFrom
			bc.Metadata.Group = link.BindingConfig.Group

			bindingContexts = append(bindingContexts, bc)
		}
	}

	return bindingContexts
}
