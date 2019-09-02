package hook

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubemgr "github.com/flant/shell-operator/pkg/kube_events_manager"
)

// HookConfig is a structure with versioned hook configuration
type HookConfig struct {
	// effective version of config
	Version string

	// versioned raw config values
	V0 *HookConfigV0
	V1 *HookConfigV1

	// effective config values
	OnStartup          *OnStartupConfig
	Schedules          []ScheduleConfig
	OnKubernetesEvents []OnKubernetesEventConfig
}

type HookConfigV0 struct {
	OnStartup         interface{}                 `json:"onStartup"`
	Schedule          []ScheduleConfigV0          `json:"schedule"`
	OnKubernetesEvent []OnKubernetesEventConfigV0 `json:"onKubernetesEvent"`
}

type HookConfigV1 struct {
	ConfigVersion     string                      `json:"configVersion"`
	OnStartup         interface{}                 `json:"onStartup"`
	Schedule          []ScheduleConfigV0          `json:"schedule"`
	OnKubernetesEvent []OnKubernetesEventConfigV1 `json:"onKubernetesEvent"`
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
	EventTypes        []kubemgr.KubeEventType  `json:"event,omitempty"`
	ApiVersion        string                   `json:"apiVersion"`
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

// version 1 of kubernetes event configuration
type OnKubernetesEventConfigV1 struct {
	Name          string                   `json:"name,omitempty"`
	EventTypes    []kubemgr.KubeEventType  `json:"event,omitempty"`
	ApiVersion    string                   `json:"apiVersion,omitempty"`
	Kind          string                   `json:"kind,omitempty"`
	NameSelector  *KubeNameSelectorV1      `json:"nameSelector,omitempty"`
	LabelSelector *metav1.LabelSelector    `json:"labelSelector,omitempty"`
	FieldSelector *KubeFieldSelectorV1     `json:"fieldSelector,omitempty"`
	Namespace     *KubeNamespaceSelectorV1 `json:"namespace,omitempty"`
	JqFilter      string                   `json:"jqFilter,omitempty"`
	AllowFailure  bool                     `json:"allowFailure,omitempty"`
}

type KubeNameSelectorV1 kubemgr.NameSelector

type KubeFieldSelectorV1 kubemgr.FieldSelector

type KubeNamespaceSelectorV1 kubemgr.NamespaceSelector

type BindingType string

const (
	Schedule          BindingType = "SCHEDULE"
	OnStartup         BindingType = "ON_STARTUP"
	OnKubernetesEvent BindingType = "ON_KUBERNETES_EVENT"
)

var ContextBindingType = map[BindingType]string{
	Schedule:          "schedule",
	OnStartup:         "onStartup",
	OnKubernetesEvent: "onKubernetesEvent",
}

// Types for effective binding configs
type CommonBindingConfig struct {
	ConfigName   string
	AllowFailure bool
}

type OnStartupConfig struct {
	CommonBindingConfig
	Order float64
}

type ScheduleConfig struct {
	CommonBindingConfig
	Crontab string
}

type OnKubernetesEventConfig struct {
	CommonBindingConfig
	Monitor *kubemgr.MonitorConfig
}

func (c *HookConfig) UnmarshalJSON(data []byte) error {
	versionDetector := &struct {
		Version *string `json:"configVersion"`
	}{}

	err := json.Unmarshal(data, versionDetector)
	if err != nil {
		return fmt.Errorf("detect HookConfig version: %s", err)
	}

	// v0 if configVersion is not specified
	if versionDetector.Version == nil {
		configV0 := &HookConfigV0{}
		err := json.Unmarshal(data, configV0)
		if err != nil {
			return fmt.Errorf("unmarshal HookConfigV0: %s", err)
		}
		c.V0 = configV0
		c.Version = "v0"
		return nil
	}

	c.Version = *versionDetector.Version
	switch c.Version {
	case "v1":
		configV1 := &HookConfigV1{}
		err := json.Unmarshal(data, configV1)
		if err != nil {
			return fmt.Errorf("unmarshal HookConfigV1: %s", err)
		}
		c.V1 = configV1
	default:
		return fmt.Errorf("HookConfig version '%s' is unsupported", c.Version)
	}

	return nil
}

// Convert creates a MonitorConfig array from versioned OnKubernetesEvent configurations.
func (c *HookConfig) Convert() (err error) {
	if c == nil {
		return fmt.Errorf("HookConfig is nil")
	}

	switch c.Version {
	case "v0":
		err = c.ConvertV0()
	case "v1":
		err = c.ConvertV1()
	default:
		err = fmt.Errorf("HookConfig version '%s' is unsupported", c.Version)
	}

	return
}

func (c *HookConfig) ConvertV0() (err error) {
	c.OnStartup, err = c.ValidateOnStartup(c.V0.OnStartup)
	if err != nil {
		return err
	}

	c.Schedules = []ScheduleConfig{}
	for _, rawSchedule := range c.V0.Schedule {
		schedule, err := c.ValidateScheduleV0(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for _, config := range c.V0.OnKubernetesEvent {
		monitor := &kubemgr.MonitorConfig{}
		monitor.ConfigIdPrefix = config.Name
		monitor.WithEventTypes(config.EventTypes)
		monitor.ApiVersion = config.ApiVersion
		monitor.Kind = config.Kind
		if config.ObjectName != "" {
			monitor.AddFieldSelectorRequirement("metadata.name", "=", config.ObjectName)
		}
		if config.NamespaceSelector != nil && !config.NamespaceSelector.Any {
			monitor.WithNamespaceSelector(&kubemgr.NamespaceSelector{
				NameSelector: &kubemgr.NameSelector{
					MatchNames: config.NamespaceSelector.MatchNames,
				},
			})
		}
		monitor.WithLabelSelector(config.Selector)
		monitor.JqFilter = config.JqFilter

		kubeConfig := OnKubernetesEventConfig{}
		kubeConfig.Monitor = monitor
		kubeConfig.AllowFailure = config.AllowFailure
		if config.Name == "" {
			kubeConfig.ConfigName = ContextBindingType[OnKubernetesEvent]
		} else {
			kubeConfig.ConfigName = config.Name
		}

		c.OnKubernetesEvents = append(c.OnKubernetesEvents, kubeConfig)
	}

	return nil
}

func (c *HookConfig) ConvertV1() (err error) {
	c.OnStartup, err = c.ValidateOnStartup(c.V1.OnStartup)
	if err != nil {
		return err
	}

	c.Schedules = []ScheduleConfig{}
	for _, rawSchedule := range c.V1.Schedule {
		schedule, err := c.ValidateScheduleV0(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for _, config := range c.V1.OnKubernetesEvent {
		monitor := &kubemgr.MonitorConfig{}
		monitor.ConfigIdPrefix = config.Name
		monitor.WithEventTypes(config.EventTypes)
		monitor.ApiVersion = config.ApiVersion
		monitor.Kind = config.Kind
		monitor.WithFieldSelector((*kubemgr.FieldSelector)(config.FieldSelector))
		monitor.WithNamespaceSelector((*kubemgr.NamespaceSelector)(config.Namespace))
		monitor.WithLabelSelector(config.LabelSelector)
		monitor.JqFilter = config.JqFilter

		kubeConfig := OnKubernetesEventConfig{}
		kubeConfig.Monitor = monitor
		kubeConfig.AllowFailure = config.AllowFailure
		if config.Name == "" {
			kubeConfig.ConfigName = ContextBindingType[OnKubernetesEvent]
		} else {
			kubeConfig.ConfigName = config.Name
		}

		c.OnKubernetesEvents = append(c.OnKubernetesEvents, kubeConfig)
	}

	return nil
}

func (c *HookConfig) Bindings() []BindingType {
	res := []BindingType{}

	for binding := range ContextBindingType {
		if c.HasBinding(binding) {
			res = append(res, binding)
		}
	}

	return res
}

func (c *HookConfig) HasBinding(binding BindingType) bool {
	switch binding {
	case OnStartup:
		return c.OnStartup != nil
	case Schedule:
		return len(c.Schedules) > 0
	case OnKubernetesEvent:
		return len(c.OnKubernetesEvents) > 0
	}
	return false
}

func (c *HookConfig) ValidateOnStartup(value interface{}) (*OnStartupConfig, error) {
	if value == nil {
		return nil, nil
	}
	if floatValue, ok := value.(float64); ok {
		res := &OnStartupConfig{}
		res.AllowFailure = false
		res.ConfigName = ContextBindingType[OnStartup]
		res.Order = floatValue
		return res, nil
	}
	return nil, fmt.Errorf("'%v' for OnStartup binding is unsupported", value)
}

func (c *HookConfig) ValidateScheduleV0(schedule ScheduleConfigV0) (ScheduleConfig, error) {
	res := ScheduleConfig{}

	if schedule.Name != "" {
		res.ConfigName = schedule.Name
	} else {
		res.ConfigName = ContextBindingType[Schedule]
	}

	res.AllowFailure = schedule.AllowFailure
	res.Crontab = schedule.Crontab

	return res, nil
}
