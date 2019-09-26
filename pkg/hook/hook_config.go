package hook

import (
	"encoding/json"
	"fmt"

	"github.com/flant/shell-operator/pkg/hook/config"
	"github.com/hashicorp/go-multierror"
	"github.com/romana/rlog"
	uuid "gopkg.in/satori/go.uuid.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
	OnKubernetesEvent []OnKubernetesEventConfigV1 `json:"kubernetes"`
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

// version 1 of kubernetes event configuration
type OnKubernetesEventConfigV1 struct {
	Name                    string                   `json:"name,omitempty"`
	WatchEventTypes         []kubemgr.WatchEventType `json:"watchEvent,omitempty"`
	ApiVersion              string                   `json:"apiVersion,omitempty"`
	Kind                    string                   `json:"kind,omitempty"`
	NameSelector            *KubeNameSelectorV1      `json:"nameSelector,omitempty"`
	LabelSelector           *metav1.LabelSelector    `json:"labelSelector,omitempty"`
	FieldSelector           *KubeFieldSelectorV1     `json:"fieldSelector,omitempty"`
	Namespace               *KubeNamespaceSelectorV1 `json:"namespace,omitempty"`
	JqFilter                string                   `json:"JqFilter,omitempty"`
	AllowFailure            bool                     `json:"allowFailure,omitempty"`
	EventType               []string                 `json:"event,omitempty"`
	ResynchronizationPeriod string                   `json:"resynchronizationPeriod,omitempty"`
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

// LoadAndValidate loads config from bytes and validate it. Returns multierror.
func (c *HookConfig) LoadAndValidate(data []byte) error {
	// - unmarshal json into map
	// - detect version
	// - validate with openapi schema
	// - load again as versioned struct
	// - convert
	// - complex, inter fields checks

	var blank map[string]interface{}
	err := json.Unmarshal(data, &blank)
	if err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// detect version
	version, err := getConfigVersion(blank)
	if err != nil {
		return fmt.Errorf("config version: %v", err)
	}
	c.Version = version

	isValid, multiErr := config.ValidateConfig(blank, version, "")
	if multiErr != nil {
		return multiErr
	}
	if !isValid {
		return fmt.Errorf("configuration is not valid")
	}

	switch version {
	case "v0":
		configV0 := &HookConfigV0{}
		err := json.Unmarshal(data, configV0)
		if err != nil {
			return fmt.Errorf("unmarshal HookConfig version 0: %s", err)
		}
		c.V0 = configV0
		err = c.ConvertAndCheckV0()
		if err != nil {
			return err
		}
	case "v1":
		configV1 := &HookConfigV1{}
		err := json.Unmarshal(data, configV1)
		if err != nil {
			return fmt.Errorf("unmarshal HookConfig v1: %s", err)
		}
		c.V1 = configV1
		err = c.ConvertAndCheckV1()
		if err != nil {
			return err
		}
	default:
		// NOTE this should not happen because getConfigVersion should return supported version
		return fmt.Errorf("version '%s' is unsupported", version)
	}

	return nil
}

// ConvertAndCheckV0 fills non-versioned structures and run inter-field checks not covered by OpenAPI schemas.
func (c *HookConfig) ConvertAndCheckV0() (err error) {

	c.OnStartup, err = c.ConvertOnStartup(c.V0.OnStartup)
	if err != nil {
		return err
	}

	c.Schedules = []ScheduleConfig{}
	for _, rawSchedule := range c.V0.Schedule {
		schedule, err := c.ConvertScheduleV0(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for i, kubeCfg := range c.V0.OnKubernetesEvent {
		err := c.CheckOnKubernetesEventV0(kubeCfg, fmt.Sprintf("onKubernetesEvent[%d]", i))
		if err != nil {
			return fmt.Errorf("invalid onKubernetesEvent config [%d]: %v", i, err)
		}

		monitor := &kubemgr.MonitorConfig{}
		monitor.Metadata.DebugName = c.MonitorDebugName(kubeCfg.Name, i)
		monitor.Metadata.ConfigId = c.MonitorConfigId()

		// a quick fix for legacy version.
		eventTypes := []kubemgr.WatchEventType{}
		for _, eventName := range kubeCfg.EventTypes {
			switch eventName {
			case "add":
				eventTypes = append(eventTypes, kubemgr.WatchEventAdded)
			case "update":
				eventTypes = append(eventTypes, kubemgr.WatchEventModified)
			case "delete":
				eventTypes = append(eventTypes, kubemgr.WatchEventDeleted)
			default:
				return fmt.Errorf("event '%s' is unsupported", eventName)
			}
		}
		monitor.WithEventTypes(eventTypes)

		monitor.Kind = kubeCfg.Kind
		if kubeCfg.ObjectName != "" {
			monitor.WithNameSelector(&kubemgr.NameSelector{
				MatchNames: []string{kubeCfg.ObjectName},
			})
		}
		if kubeCfg.NamespaceSelector != nil && !kubeCfg.NamespaceSelector.Any {
			monitor.WithNamespaceSelector(&kubemgr.NamespaceSelector{
				NameSelector: &kubemgr.NameSelector{
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
			kubeConfig.ConfigName = "onKubernetesEvent"
		} else {
			kubeConfig.ConfigName = kubeCfg.Name
		}

		c.OnKubernetesEvents = append(c.OnKubernetesEvents, kubeConfig)
	}

	return nil
}

// ConvertAndCheckV0 fills non-versioned structures and run inter-field checks not covered by OpenAPI schemas.
func (c *HookConfig) ConvertAndCheckV1() (err error) {
	c.OnStartup, err = c.ConvertOnStartup(c.V1.OnStartup)
	if err != nil {
		return err
	}

	c.Schedules = []ScheduleConfig{}
	for _, rawSchedule := range c.V1.Schedule {
		schedule, err := c.ConvertScheduleV0(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for i, kubeCfg := range c.V1.OnKubernetesEvent {
		err := c.CheckOnKubernetesEventV1(kubeCfg, fmt.Sprintf("kubernetes[%d]", i))
		if err != nil {
			return fmt.Errorf("invalid kubernetes config [%d]: %v", i, err)
		}

		monitor := &kubemgr.MonitorConfig{}
		monitor.Metadata.DebugName = c.MonitorDebugName(kubeCfg.Name, i)
		monitor.Metadata.ConfigId = c.MonitorConfigId()
		monitor.WithEventTypes(kubeCfg.WatchEventTypes)
		monitor.ApiVersion = kubeCfg.ApiVersion
		monitor.Kind = kubeCfg.Kind
		monitor.WithNameSelector((*kubemgr.NameSelector)(kubeCfg.NameSelector))
		monitor.WithFieldSelector((*kubemgr.FieldSelector)(kubeCfg.FieldSelector))
		monitor.WithNamespaceSelector((*kubemgr.NamespaceSelector)(kubeCfg.Namespace))
		monitor.WithLabelSelector(kubeCfg.LabelSelector)
		monitor.JqFilter = kubeCfg.JqFilter

		kubeConfig := OnKubernetesEventConfig{}
		kubeConfig.Monitor = monitor
		kubeConfig.AllowFailure = kubeCfg.AllowFailure
		if kubeCfg.Name == "" {
			kubeConfig.ConfigName = ContextBindingType[OnKubernetesEvent]
		} else {
			kubeConfig.ConfigName = kubeCfg.Name
		}

		rlog.Debugf("kubernetes[%d]: %+v", i, kubeConfig)

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

func (c *HookConfig) ConvertOnStartup(value interface{}) (*OnStartupConfig, error) {
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

func (c *HookConfig) ConvertScheduleV0(schedule ScheduleConfigV0) (ScheduleConfig, error) {
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

func (c *HookConfig) CheckOnKubernetesEventV0(kubeCfg OnKubernetesEventConfigV0, rootPath string) error {
	return nil
}

func (c *HookConfig) CheckOnKubernetesEventV1(kubeCfg OnKubernetesEventConfigV1, rootPath string) (allErr error) {
	if kubeCfg.ApiVersion != "" {
		_, err := schema.ParseGroupVersion(kubeCfg.ApiVersion)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("apiVersion is invalid"))
		}
	}

	if kubeCfg.LabelSelector != nil {
		_, err := kubemgr.FormatLabelSelector(kubeCfg.LabelSelector)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("labelSelector is invalid: %v", err))
		}
	}

	if kubeCfg.FieldSelector != nil {
		_, err := kubemgr.FormatFieldSelector((*kubemgr.FieldSelector)(kubeCfg.FieldSelector))
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("fieldSelector is invalid: %v", err))
		}
	}

	if kubeCfg.NameSelector != nil && len(kubeCfg.NameSelector.MatchNames) > 0 {
		if kubeCfg.FieldSelector != nil && len(kubeCfg.FieldSelector.MatchExpressions) > 0 {
			for _, expr := range kubeCfg.FieldSelector.MatchExpressions {
				if expr.Field == "metadata.name" {
					allErr = multierror.Append(allErr, fmt.Errorf("fieldSelector 'metadata.name' and nameSelector.matchNames are mutually exclusive"))
				}
			}
		}
	}

	return allErr
}

func (c *HookConfig) MonitorDebugName(configName string, configIndex int) string {
	if configName == "" {
		return fmt.Sprintf("kubernetes[%d]", configIndex)
	} else {
		return fmt.Sprintf("kubernetes[%d]{%s}", configIndex, configName)
	}
}

func (c *HookConfig) MonitorConfigId() string {
	return uuid.NewV4().String()
	//ei.DebugName = uuid.NewV4().String()
	//if ei.Monitor.ConfigIdPrefix != "" {
	//	ei.DebugName = ei.Monitor.ConfigIdPrefix + "-" + ei.DebugName[len(ei.Monitor.ConfigIdPrefix)+1:]
	//}
	//return ei.DebugName
}

func getConfigVersion(obj map[string]interface{}) (string, error) {
	key := "configVersion"
	value, found := obj[key]
	if !found {
		return "v0", nil
		//return "", fmt.Errorf("missing '%s' key", key)
	}
	if value == nil {
		return "", fmt.Errorf("missing '%s' value", key)
	}
	typedValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("string value is expected for key '%s'", key)
	}
	// FIXME add version validator
	if typedValue != "v1" {
		return "", fmt.Errorf("'%s' value '%s' is unsupported", key, typedValue)
	}
	return typedValue, nil
}
