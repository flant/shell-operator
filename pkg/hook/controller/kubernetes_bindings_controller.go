package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

// KubernetesBindingToMonitorLink is a link between a binding config and a Monitor.
type KubernetesBindingToMonitorLink struct {
	MonitorId     string
	BindingConfig OnKubernetesEventConfig
}

// KubernetesBindingsController handles kubernetes bindings for one hook.
type KubernetesBindingsController interface {
	WithKubernetesBindings([]OnKubernetesEventConfig)
	WithKubeEventsManager(kube_events_manager.KubeEventsManager)
	EnableKubernetesBindings() ([]BindingExecutionInfo, error)
	UpdateMonitor(monitorId string, kind, apiVersion string) error
	UnlockEvents()
	UnlockEventsFor(monitorID string)
	StopMonitors()
	CanHandleEvent(kubeEvent KubeEvent) bool
	HandleEvent(kubeEvent KubeEvent) BindingExecutionInfo
	BindingNames() []string

	SnapshotsFrom(bindingNames ...string) map[string][]ObjectAndFilterResult
	SnapshotsFor(bindingName string) []ObjectAndFilterResult
	Snapshots() map[string][]ObjectAndFilterResult
	SnapshotsInfo() []string
	SnapshotsDump() map[string]interface{}
}

// kubernetesHooksController is a main implementation of KubernetesHooksController
type kubernetesBindingsController struct {
	// All hooks with 'kubernetes' bindings
	BindingMonitorLinks map[string]*KubernetesBindingToMonitorLink

	// bindings configurations
	KubernetesBindings []OnKubernetesEventConfig

	// dependencies
	kubeEventsManager kube_events_manager.KubeEventsManager
}

// kubernetesHooksController should implement the KubernetesHooksController
var _ KubernetesBindingsController = &kubernetesBindingsController{}

// NewKubernetesHooksController returns an implementation of KubernetesHooksController
var NewKubernetesBindingsController = func() *kubernetesBindingsController {
	return &kubernetesBindingsController{
		BindingMonitorLinks: make(map[string]*KubernetesBindingToMonitorLink),
	}
}

func (c *kubernetesBindingsController) WithKubernetesBindings(bindings []OnKubernetesEventConfig) {
	c.KubernetesBindings = bindings
}

func (c *kubernetesBindingsController) WithKubeEventsManager(kubeEventsManager kube_events_manager.KubeEventsManager) {
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
		c.BindingMonitorLinks[config.Monitor.Metadata.MonitorId] = &KubernetesBindingToMonitorLink{
			MonitorId:     config.Monitor.Metadata.MonitorId,
			BindingConfig: config,
		}
		// Start monitor's informers to fill the cache.
		c.kubeEventsManager.StartMonitor(config.Monitor.Metadata.MonitorId)

		synchronizationInfo := c.HandleEvent(KubeEvent{
			MonitorId: config.Monitor.Metadata.MonitorId,
			Type:      TypeSynchronization,
		})
		res = append(res, synchronizationInfo)
	}

	return res, nil
}

func (c *kubernetesBindingsController) UpdateMonitor(monitorId string, kind, apiVersion string) error {
	// Find binding for monitorId
	link, ok := c.BindingMonitorLinks[monitorId]
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

	log.WithFields(utils.LabelsToLogFields(link.BindingConfig.Monitor.Metadata.LogLabels)).
		Infof("Monitor for '%s' is recreated with new kind=%s and apiVersion=%s",
			link.BindingConfig.BindingName, link.BindingConfig.Monitor.Kind, link.BindingConfig.Monitor.ApiVersion)

	// Synchronization has no meaning for UpdateMonitor. Just emit Added event to handle objects of
	// a new kind.
	kubeEvent := KubeEvent{
		MonitorId:   monitorId,
		Type:        TypeEvent,
		WatchEvents: []WatchEventType{WatchEventAdded},
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
	for monitorID := range c.BindingMonitorLinks {
		m := c.kubeEventsManager.GetMonitor(monitorID)
		m.EnableKubeEventCb()
	}
}

// UnlockEventsFor turns on eventCb for matched monitor to emit events after Synchronization.
func (c *kubernetesBindingsController) UnlockEventsFor(monitorID string) {
	m := c.kubeEventsManager.GetMonitor(monitorID)
	m.EnableKubeEventCb()
}

// StopMonitors stops all monitors for the hook.
// TODO handle error!
func (c *kubernetesBindingsController) StopMonitors() {
	for monitorID := range c.BindingMonitorLinks {
		_ = c.kubeEventsManager.StopMonitor(monitorID)
	}
}

func (c *kubernetesBindingsController) CanHandleEvent(kubeEvent KubeEvent) bool {
	for key := range c.BindingMonitorLinks {
		if key == kubeEvent.MonitorId {
			return true
		}
	}
	return false
}

// HandleEvent receives event from KubeEventManager and returns a BindingExecutionInfo
// to help create a new task to run a hook.
func (c *kubernetesBindingsController) HandleEvent(kubeEvent KubeEvent) BindingExecutionInfo {
	link, hasKey := c.BindingMonitorLinks[kubeEvent.MonitorId]
	if !hasKey {
		log.Errorf("Possible bug!!! Unknown kube event: no such monitor id '%s' registered", kubeEvent.MonitorId)
		return BindingExecutionInfo{
			BindingContext: []BindingContext{},
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
	names := []string{}
	for _, binding := range c.KubernetesBindings {
		names = append(names, binding.BindingName)
	}
	return names
}

// SnapshotsFor returns snapshot for single onKubernetes binding.
// It finds a monitorId for a binding name and returns an array of objects.
func (c *kubernetesBindingsController) SnapshotsFor(bindingName string) []ObjectAndFilterResult {
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
func (c *kubernetesBindingsController) SnapshotsFrom(bindingNames ...string) map[string][]ObjectAndFilterResult {
	res := map[string][]ObjectAndFilterResult{}

	for _, bindingName := range bindingNames {
		// Initialize all keys with empty arrays.
		res[bindingName] = make([]ObjectAndFilterResult, 0)

		snapshot := c.SnapshotsFor(bindingName)
		if snapshot != nil {
			res[bindingName] = snapshot
		}
	}

	return res
}

func (c *kubernetesBindingsController) Snapshots() map[string][]ObjectAndFilterResult {
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

func ConvertKubeEventToBindingContext(kubeEvent KubeEvent, link *KubernetesBindingToMonitorLink) []BindingContext {
	bindingContexts := make([]BindingContext, 0)

	switch kubeEvent.Type {
	case TypeSynchronization:
		bc := BindingContext{
			Binding: link.BindingConfig.BindingName,
			Type:    kubeEvent.Type,
			Objects: kubeEvent.Objects,
		}
		bc.Metadata.JqFilter = link.BindingConfig.Monitor.JqFilter
		bc.Metadata.BindingType = OnKubernetesEvent
		bc.Metadata.IncludeSnapshots = link.BindingConfig.IncludeSnapshotsFrom
		bc.Metadata.Group = link.BindingConfig.Group

		bindingContexts = append(bindingContexts, bc)

	case TypeEvent:
		for _, kEvent := range kubeEvent.WatchEvents {
			bc := BindingContext{
				Binding:    link.BindingConfig.BindingName,
				Type:       kubeEvent.Type,
				WatchEvent: kEvent,
				Objects:    kubeEvent.Objects,
			}
			bc.Metadata.JqFilter = link.BindingConfig.Monitor.JqFilter
			bc.Metadata.BindingType = OnKubernetesEvent
			bc.Metadata.IncludeSnapshots = link.BindingConfig.IncludeSnapshotsFrom
			bc.Metadata.Group = link.BindingConfig.Group

			bindingContexts = append(bindingContexts, bc)
		}
	}

	return bindingContexts
}
