package controller

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

// A link between a hook and a kube monitor
type KubernetesBindingToMonitorLink struct {
	BindingName string
	MonitorId   string
	// Useful fields to create a BindingContext
	IncludeSnapshots []string
	AllowFailure     bool
	JqFilter         string
	QueueName        string
	SkipKey          string
}

// KubernetesBindingsController handles kubernetes bindings for one hook.
type KubernetesBindingsController interface {
	WithKubernetesBindings([]OnKubernetesEventConfig)
	WithKubeEventsManager(kube_events_manager.KubeEventsManager)
	EnableKubernetesBindings() ([]BindingExecutionInfo, error)
	StartMonitors()
	StopMonitors()
	CanHandleEvent(kubeEvent KubeEvent) bool
	HandleEvent(kubeEvent KubeEvent) BindingExecutionInfo
	BindingNames() []string
	SnapshotsFrom(bindingNames ...string) map[string][]ObjectAndFilterResult
	Snapshots() map[string][]ObjectAndFilterResult
}

// kubernetesHooksController is a main implementation of KubernetesHooksController
type kubernetesBindingsController struct {
	// All hooks with 'kubernetes' bindings
	BindingMonitorLinks map[string]*KubernetesBindingToMonitorLink

	// bindings configurations
	KubernetesBindings []OnKubernetesEventConfig

	// dependencies
	kubeEventsManager kube_events_manager.KubeEventsManager
	logEntry          *log.Entry
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

// EnableKubernetesBindings adds monitor for each 'kubernetes' binding. This method
// returns an array of BindingExecutionInfo to help construct initial tasks to run hooks.
func (c *kubernetesBindingsController) EnableKubernetesBindings() ([]BindingExecutionInfo, error) {
	res := make([]BindingExecutionInfo, 0)

	for _, config := range c.KubernetesBindings {
		firstKubeEvent, err := c.kubeEventsManager.AddMonitor(config.Monitor)
		if err != nil {
			return nil, fmt.Errorf("run monitor: %s", err)
		}
		c.BindingMonitorLinks[config.Monitor.Metadata.MonitorId] = &KubernetesBindingToMonitorLink{
			MonitorId:        config.Monitor.Metadata.MonitorId,
			BindingName:      config.BindingName,
			IncludeSnapshots: config.IncludeSnapshotsFrom,
			AllowFailure:     config.AllowFailure,
			JqFilter:         config.Monitor.JqFilter,
			QueueName:        config.Queue,
			SkipKey:          config.SkipKey,
		}

		// There is no Synchronization event for 'v0' binding configuration.
		if firstKubeEvent == nil {
			continue
		}

		info := c.HandleEvent(*firstKubeEvent)
		res = append(res, info)
	}

	return res, nil
}

// StartMonitors starts kubernetes informers to actually get events from cluster
func (c *kubernetesBindingsController) StartMonitors() {
	for monitorId := range c.BindingMonitorLinks {
		c.kubeEventsManager.StartMonitor(monitorId)
	}
}

// StartMonitors starts kubernetes informers to actually get events from cluster
// TODO handle error!
func (c *kubernetesBindingsController) StopMonitors() {
	for monitorId := range c.BindingMonitorLinks {
		_ = c.kubeEventsManager.StopMonitor(monitorId)
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

	return BindingExecutionInfo{
		BindingContext:   bindingContext,
		IncludeSnapshots: link.IncludeSnapshots,
		AllowFailure:     link.AllowFailure,
		QueueName:        link.QueueName,
	}
}

func (c *kubernetesBindingsController) BindingNames() []string {
	names := []string{}
	for _, binding := range c.KubernetesBindings {
		names = append(names, binding.BindingName)
	}
	return names
}

func (c *kubernetesBindingsController) SnapshotsFrom(bindingNames ...string) map[string][]ObjectAndFilterResult {
	res := map[string][]ObjectAndFilterResult{}

	for _, bindingName := range bindingNames {
		// initialize all keys with empty arrays.
		res[bindingName] = make([]ObjectAndFilterResult, 0)
		for _, binding := range c.KubernetesBindings {
			if bindingName == binding.BindingName {
				monitorId := binding.Monitor.Metadata.MonitorId
				if c.kubeEventsManager.HasMonitor(monitorId) {
					res[bindingName] = c.kubeEventsManager.GetMonitor(monitorId).GetExistedObjects()
				}
			}
		}
	}

	return res
}

func (c *kubernetesBindingsController) Snapshots() map[string][]ObjectAndFilterResult {
	return c.SnapshotsFrom(c.BindingNames()...)
}

func ConvertKubeEventToBindingContext(kubeEvent KubeEvent, link *KubernetesBindingToMonitorLink) []BindingContext {
	bindingContexts := make([]BindingContext, 0)

	switch kubeEvent.Type {
	case TypeSynchronization:
		bc := BindingContext{
			Binding: link.BindingName,
			Type:    kubeEvent.Type,
			Objects: kubeEvent.Objects,
		}
		bc.Metadata.JqFilter = link.JqFilter
		bc.Metadata.BindingType = OnKubernetesEvent
		bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
		bc.Metadata.SkipKey = link.SkipKey

		bindingContexts = append(bindingContexts, bc)

	case TypeEvent:
		for _, kEvent := range kubeEvent.WatchEvents {
			bc := BindingContext{
				Binding:    link.BindingName,
				Type:       kubeEvent.Type,
				WatchEvent: kEvent,
				Objects:    kubeEvent.Objects,
			}
			bc.Metadata.JqFilter = link.JqFilter
			bc.Metadata.BindingType = OnKubernetesEvent
			bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
			bc.Metadata.SkipKey = link.SkipKey

			bindingContexts = append(bindingContexts, bc)
		}
	}

	return bindingContexts
}
