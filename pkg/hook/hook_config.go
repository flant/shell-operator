package hook

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/robfig/cron.v2"
	uuid "gopkg.in/satori/go.uuid.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	. "github.com/flant/shell-operator/pkg/schedule_manager/types"

	"github.com/flant/shell-operator/pkg/hook/config"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
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
	Schedule          []ScheduleConfigV1          `json:"schedule"`
	OnKubernetesEvent []OnKubernetesEventConfigV1 `json:"kubernetes"`
}

// Schedule configuration
type ScheduleConfigV0 struct {
	Name         string `json:"name"`
	Crontab      string `json:"crontab"`
	AllowFailure bool   `json:"allowFailure"`
}

// Schedule configuration
type ScheduleConfigV1 struct {
	Name                 string   `json:"name"`
	Crontab              string   `json:"crontab"`
	AllowFailure         bool     `json:"allowFailure"`
	IncludeSnapshotsFrom []string `json:"includeSnapshotsFrom"`
	Queue                string   `json:"queue"`
	Group                string   `json:"group,omitempty"`
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
	WatchEventTypes         []WatchEventType         `json:"watchEvent,omitempty"`
	ExecuteHookOnEvents     []WatchEventType         `json:"executeHookOnEvent,omitempty"`
	Mode                    KubeEventMode            `json:"mode,omitempty"`
	ApiVersion              string                   `json:"apiVersion,omitempty"`
	Kind                    string                   `json:"kind,omitempty"`
	NameSelector            *KubeNameSelectorV1      `json:"nameSelector,omitempty"`
	LabelSelector           *metav1.LabelSelector    `json:"labelSelector,omitempty"`
	FieldSelector           *KubeFieldSelectorV1     `json:"fieldSelector,omitempty"`
	Namespace               *KubeNamespaceSelectorV1 `json:"namespace,omitempty"`
	JqFilter                string                   `json:"jqFilter,omitempty"`
	AllowFailure            bool                     `json:"allowFailure,omitempty"`
	ResynchronizationPeriod string                   `json:"resynchronizationPeriod,omitempty"`
	IncludeSnapshotsFrom    []string                 `json:"includeSnapshotsFrom,omitempty"`
	Queue                   string                   `json:"queue,omitempty"`
	Group                   string                   `json:"group,omitempty"`
}

type KubeNameSelectorV1 NameSelector

type KubeFieldSelectorV1 FieldSelector

type KubeNamespaceSelectorV1 NamespaceSelector

// LoadAndValidate loads config from bytes and validate it. Returns multierror.
func (c *HookConfig) LoadAndValidate(data []byte) error {
	// - unmarshal json into map
	// - detect version
	// - validate with openapi schema
	// - load again as versioned struct
	// - convert
	// - make complex checks

	vu := config.NewDefaultVersionedUntyped()
	err := vu.Load(data)
	if err != nil {
		return err
	}

	err = config.ValidateConfig(vu.Obj, config.GetSchema(vu.Version), "")
	if err != nil {
		return err
	}

	c.Version = vu.Version

	err = c.ConvertAndCheck(data)
	if err != nil {
		return err
	}

	return nil
}

func (c *HookConfig) ConvertAndCheck(data []byte) error {
	switch c.Version {
	case "v0":
		configV0 := &HookConfigV0{}
		err := yaml.Unmarshal(data, configV0)
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
		err := yaml.Unmarshal(data, configV1)
		if err != nil {
			return fmt.Errorf("unmarshal HookConfig v1: %s", err)
		}
		c.V1 = configV1
		err = c.ConvertAndCheckV1()
		if err != nil {
			return err
		}
	default:
		// NOTE: this should not happen
		return fmt.Errorf("version '%s' is unsupported", c.Version)
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
	for i, rawSchedule := range c.V0.Schedule {
		err := c.CheckScheduleV0(rawSchedule)
		if err != nil {
			return fmt.Errorf("invalid schedule config [%d]: %v", i, err)
		}
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

		monitor := &kube_events_manager.MonitorConfig{}
		monitor.Metadata.DebugName = c.MonitorDebugName(kubeCfg.Name, i)
		monitor.Metadata.MonitorId = c.MonitorConfigId()
		monitor.Metadata.LogLabels = map[string]string{}
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

// ConvertAndCheckV0 fills non-versioned structures and run inter-field checks not covered by OpenAPI schemas.
func (c *HookConfig) ConvertAndCheckV1() (err error) {
	c.OnStartup, err = c.ConvertOnStartup(c.V1.OnStartup)
	if err != nil {
		return err
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for i, kubeCfg := range c.V1.OnKubernetesEvent {
		err := c.CheckOnKubernetesEventV1(kubeCfg, fmt.Sprintf("kubernetes[%d]", i))
		if err != nil {
			return fmt.Errorf("invalid kubernetes config [%d]: %v", i, err)
		}

		monitor := &kube_events_manager.MonitorConfig{}
		monitor.Metadata.DebugName = c.MonitorDebugName(kubeCfg.Name, i)
		monitor.Metadata.MonitorId = c.MonitorConfigId()
		monitor.Metadata.LogLabels = map[string]string{}
		monitor.WithMode(kubeCfg.Mode)
		monitor.ApiVersion = kubeCfg.ApiVersion
		monitor.Kind = kubeCfg.Kind
		monitor.WithNameSelector((*NameSelector)(kubeCfg.NameSelector))
		monitor.WithFieldSelector((*FieldSelector)(kubeCfg.FieldSelector))
		monitor.WithNamespaceSelector((*NamespaceSelector)(kubeCfg.Namespace))
		monitor.WithLabelSelector(kubeCfg.LabelSelector)
		monitor.JqFilter = kubeCfg.JqFilter
		// executeHookOnEvent is a priority
		if kubeCfg.ExecuteHookOnEvents != nil {
			monitor.WithEventTypes(kubeCfg.ExecuteHookOnEvents)
		} else {
			if kubeCfg.WatchEventTypes != nil {
				monitor.WithEventTypes(kubeCfg.WatchEventTypes)
			} else {
				monitor.WithEventTypes(nil)
			}
		}

		kubeConfig := OnKubernetesEventConfig{}
		kubeConfig.Monitor = monitor
		kubeConfig.AllowFailure = kubeCfg.AllowFailure
		if kubeCfg.Name == "" {
			kubeConfig.BindingName = string(OnKubernetesEvent)
		} else {
			kubeConfig.BindingName = kubeCfg.Name
		}
		kubeConfig.IncludeSnapshotsFrom = kubeCfg.IncludeSnapshotsFrom
		if kubeCfg.Queue == "" {
			kubeConfig.Queue = "main"
		} else {
			kubeConfig.Queue = kubeCfg.Queue
		}
		kubeConfig.Group = kubeCfg.Group

		c.OnKubernetesEvents = append(c.OnKubernetesEvents, kubeConfig)
	}

	for i, kubeCfg := range c.V1.OnKubernetesEvent {
		if len(kubeCfg.IncludeSnapshotsFrom) > 0 {
			err := c.CheckIncludeSnapshots(kubeCfg.IncludeSnapshotsFrom...)
			if err != nil {
				return fmt.Errorf("invalid kubernetes config [%d]: includeSnapshots %v", i, err)
			}
		}
	}

	// schedule bindings with includeSnapshotsFrom
	// are depend on kubernetes bindings.
	c.Schedules = []ScheduleConfig{}
	for i, rawSchedule := range c.V1.Schedule {
		err := c.CheckScheduleV1(rawSchedule)
		if err != nil {
			return fmt.Errorf("invalid schedule config [%d]: %v", i, err)
		}
		schedule, err := c.ConvertScheduleV1(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	// Update IncludeSnapshotsFrom for groups
	var groupSnapshots = make(map[string][]string)
	for _, kubeCfg := range c.OnKubernetesEvents {
		if kubeCfg.Group == "" {
			continue
		}
		if _, ok := groupSnapshots[kubeCfg.Group]; !ok {
			groupSnapshots[kubeCfg.Group] = make([]string, 0)
		}
		groupSnapshots[kubeCfg.Group] = append(groupSnapshots[kubeCfg.Group], kubeCfg.BindingName)
	}
	newKubeEvents := make([]OnKubernetesEventConfig, 0)
	for _, cfg := range c.OnKubernetesEvents {
		if snapshots, ok := groupSnapshots[cfg.Group]; ok {
			cfg.IncludeSnapshotsFrom = snapshots
		}
		newKubeEvents = append(newKubeEvents, cfg)
	}
	c.OnKubernetesEvents = newKubeEvents
	newSchedules := make([]ScheduleConfig, 0)
	for _, cfg := range c.Schedules {
		if snapshots, ok := groupSnapshots[cfg.Group]; ok {
			cfg.IncludeSnapshotsFrom = snapshots
		}
		newSchedules = append(newSchedules, cfg)
	}
	c.Schedules = newSchedules

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
	floatValue, err := ConvertFloatForBinding(value, "onStartup")
	if err != nil || floatValue == nil {
		return nil, err
	}

	res := &OnStartupConfig{}
	res.AllowFailure = false
	res.BindingName = string(OnStartup)
	res.Order = *floatValue
	return res, nil
}

func (c *HookConfig) ConvertScheduleV0(schV0 ScheduleConfigV0) (ScheduleConfig, error) {
	res := ScheduleConfig{}

	if schV0.Name != "" {
		res.BindingName = schV0.Name
	} else {
		res.BindingName = string(Schedule)
	}

	res.AllowFailure = schV0.AllowFailure
	res.ScheduleEntry = ScheduleEntry{
		Crontab: schV0.Crontab,
		Id:      c.ScheduleId(),
	}
	res.Queue = "main"

	return res, nil
}

func (c *HookConfig) ConvertScheduleV1(schV1 ScheduleConfigV1) (ScheduleConfig, error) {
	res := ScheduleConfig{}

	if schV1.Name != "" {
		res.BindingName = schV1.Name
	} else {
		res.BindingName = string(Schedule)
	}

	res.AllowFailure = schV1.AllowFailure
	res.ScheduleEntry = ScheduleEntry{
		Crontab: schV1.Crontab,
		Id:      c.ScheduleId(),
	}
	res.IncludeSnapshotsFrom = schV1.IncludeSnapshotsFrom

	if schV1.Queue == "" {
		res.Queue = "main"
	} else {
		res.Queue = schV1.Queue
	}
	res.Group = schV1.Group

	return res, nil
}

func (c *HookConfig) CheckScheduleV0(schV0 ScheduleConfigV0) error {
	_, err := cron.Parse(schV0.Crontab)
	if err != nil {
		return fmt.Errorf("crontab is invalid: %v", err)
	}
	return nil
}

func (c *HookConfig) CheckScheduleV1(schV1 ScheduleConfigV1) (allErr error) {
	var err error
	_, err = cron.Parse(schV1.Crontab)
	if err != nil {
		allErr = multierror.Append(allErr, fmt.Errorf("crontab is invalid: %v", err))
	}

	if len(schV1.IncludeSnapshotsFrom) > 0 {
		err = c.CheckIncludeSnapshots(schV1.IncludeSnapshotsFrom...)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("includeSnapshotsFrom is invalid: %v", err))
		}
	}

	return allErr
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
		_, err := kube_events_manager.FormatLabelSelector(kubeCfg.LabelSelector)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("labelSelector is invalid: %v", err))
		}
	}

	if kubeCfg.FieldSelector != nil {
		_, err := kube_events_manager.FormatFieldSelector((*FieldSelector)(kubeCfg.FieldSelector))
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

	if kubeCfg.Group != "" && len(kubeCfg.IncludeSnapshotsFrom) > 0 {
		allErr = multierror.Append(allErr, fmt.Errorf("group and includeSnapshotsFrom are mutually exclusive"))
	}

	return allErr
}

// CheckIncludeSnapshots check if all includes has corresponding kubernetes
// binding. Rules:
//
// - binding name should exists,
//
// - binding name should not be repeated.
func (c *HookConfig) CheckIncludeSnapshots(includes ...string) error {
	for _, include := range includes {
		bindings := 0
		for _, kubeCfg := range c.OnKubernetesEvents {
			if kubeCfg.BindingName == include {
				bindings++
			}
		}
		if bindings == 0 {
			return fmt.Errorf("'%s' binding name not found", include)
		}
		if bindings > 1 {
			return fmt.Errorf("there are %d '%s' binding names", bindings, include)
		}
	}
	return nil
}

func (c *HookConfig) MonitorDebugName(configName string, configIndex int) string {
	if configName == "" {
		return fmt.Sprintf("kubernetes[%d]", configIndex)
	} else {
		return fmt.Sprintf("kubernetes[%d]{%s}", configIndex, configName)
	}
}

// TODO uuid is not a good choice here. Make it more readable.
func (c *HookConfig) MonitorConfigId() string {
	return uuid.NewV4().String()
	//ei.DebugName = uuid.NewV4().String()
	//if ei.Monitor.ConfigIdPrefix != "" {
	//	ei.DebugName = ei.Monitor.ConfigIdPrefix + "-" + ei.DebugName[len(ei.Monitor.ConfigIdPrefix)+1:]
	//}
	//return ei.DebugName
}

// TODO uuid is not a good choice here. Make it more readable.
func (c *HookConfig) ScheduleId() string {
	return uuid.NewV4().String()
}

func ConvertFloatForBinding(value interface{}, bindingName string) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if floatValue, ok := value.(float64); ok {
		return &floatValue, nil
	}
	return nil, fmt.Errorf("binding %s has unsupported value '%v'", bindingName, value)
}
