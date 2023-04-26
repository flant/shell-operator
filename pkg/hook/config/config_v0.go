package config

import (
	"fmt"

	"gopkg.in/robfig/cron.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	. "github.com/flant/shell-operator/pkg/schedule_manager/types"
)

type HookConfigV0 struct {
	OnStartup         interface{}                 `json:"onStartup"`
	Schedule          []ScheduleConfigV0          `json:"schedule"`
	OnKubernetesEvent []OnKubernetesEventConfigV0 `json:"onKubernetesEvent"`
}

// Schedule configuration
type ScheduleConfigV0 struct {
	Name         string `json:"name"`
	Crontab      string `json:"crontab"`
	AllowFailure bool   `json:"allowFailure"`
}

// Legacy version of kubernetes event configuration
type OnKubernetesEventConfigV0 struct {
	Name              string                   `json:"name,omitempty"`
	EventTypes        []string                 `json:"event,omitempty"`
	Kind              string                   `json:"kind,omitempty"`
	Selector          *metav1.LabelSelector    `json:"selector,omitempty"`
	ObjectName        string                   `json:"objectName,omitempty"`
	NamespaceSelector *KubeNamespaceSelectorV0 `json:"namespaceSelector,omitempty"`
	JqFilter          string                   `json:"jqFilter,omitempty"`
	AllowFailure      bool                     `json:"allowFailure,omitempty"`
}

type KubeNamespaceSelectorV0 struct {
	MatchNames []string `json:"matchNames"`
	Any        bool     `json:"any"`
}

// ConvertAndCheckV0 fills non-versioned structures and run inter-field checks not covered by OpenAPI schemas.
func (cv0 *HookConfigV0) ConvertAndCheck(c *HookConfig) (err error) {
	c.OnStartup, err = c.ConvertOnStartup(cv0.OnStartup)
	if err != nil {
		return err
	}

	c.Schedules = []ScheduleConfig{}
	for i, rawSchedule := range cv0.Schedule {
		err := cv0.CheckSchedule(rawSchedule)
		if err != nil {
			return fmt.Errorf("invalid schedule config [%d]: %v", i, err)
		}
		schedule, err := cv0.ConvertSchedule(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for i, kubeCfg := range cv0.OnKubernetesEvent {
		err := cv0.CheckOnKubernetesEvent(kubeCfg, fmt.Sprintf("onKubernetesEvent[%d]", i))
		if err != nil {
			return fmt.Errorf("invalid onKubernetesEvent config [%d]: %v", i, err)
		}

		monitor := &kube_events_manager.MonitorConfig{}
		monitor.Metadata.DebugName = MonitorDebugName(kubeCfg.Name, i)
		monitor.Metadata.MonitorId = MonitorConfigID()
		monitor.Metadata.LogLabels = map[string]string{}
		monitor.Metadata.MetricLabels = map[string]string{}
		monitor.WithMode(ModeV0)

		// convert event names from legacy config.
		eventTypes := []WatchEventType{}
		for _, eventName := range kubeCfg.EventTypes {
			switch eventName {
			case "add":
				eventTypes = append(eventTypes, WatchEventAdded)
			case "update":
				eventTypes = append(eventTypes, WatchEventModified)
			case "delete":
				eventTypes = append(eventTypes, WatchEventDeleted)
			default:
				return fmt.Errorf("event '%s' is unsupported", eventName)
			}
		}
		monitor.WithEventTypes(eventTypes)

		monitor.Kind = kubeCfg.Kind
		if kubeCfg.ObjectName != "" {
			monitor.WithNameSelector(&NameSelector{
				MatchNames: []string{kubeCfg.ObjectName},
			})
		}
		if kubeCfg.NamespaceSelector != nil && !kubeCfg.NamespaceSelector.Any {
			monitor.WithNamespaceSelector(&NamespaceSelector{
				NameSelector: &NameSelector{
					MatchNames: kubeCfg.NamespaceSelector.MatchNames,
				},
			})
		}
		monitor.WithLabelSelector(kubeCfg.Selector)
		monitor.JqFilter = kubeCfg.JqFilter

		kubeConfig := OnKubernetesEventConfig{}
		kubeConfig.Monitor = monitor
		kubeConfig.AllowFailure = kubeCfg.AllowFailure
		if kubeCfg.Name == "" {
			kubeConfig.BindingName = "onKubernetesEvent"
		} else {
			kubeConfig.BindingName = kubeCfg.Name
		}
		kubeConfig.Queue = "main"

		c.OnKubernetesEvents = append(c.OnKubernetesEvents, kubeConfig)
	}

	return nil
}

func (cv0 *HookConfigV0) CheckSchedule(schV0 ScheduleConfigV0) error {
	_, err := cron.Parse(schV0.Crontab)
	if err != nil {
		return fmt.Errorf("crontab is invalid: %v", err)
	}
	return nil
}

func (cv0 *HookConfigV0) ConvertSchedule(schV0 ScheduleConfigV0) (ScheduleConfig, error) {
	res := ScheduleConfig{}

	if schV0.Name != "" {
		res.BindingName = schV0.Name
	} else {
		res.BindingName = string(Schedule)
	}

	res.AllowFailure = schV0.AllowFailure
	res.ScheduleEntry = ScheduleEntry{
		Crontab: schV0.Crontab,
		Id:      ScheduleID(),
	}
	res.Queue = "main"

	return res, nil
}

func (cv0 *HookConfigV0) CheckOnKubernetesEvent(kubeCfg OnKubernetesEventConfigV0, rootPath string) error {
	return nil
}
