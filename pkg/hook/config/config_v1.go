package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/robfig/cron.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	. "github.com/flant/shell-operator/pkg/schedule_manager/types"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/validating"
	"github.com/flant/shell-operator/pkg/webhook/validating/validation"
)

type HookConfigV1 struct {
	ConfigVersion        string                         `json:"configVersion"`
	OnStartup            interface{}                    `json:"onStartup"`
	Schedule             []ScheduleConfigV1             `json:"schedule"`
	OnKubernetesEvent    []OnKubernetesEventConfigV1    `json:"kubernetes"`
	KubernetesValidating []KubernetesValidatingConfigV1 `json:"kubernetesValidating"`
	KubernetesConversion []KubernetesConversionConfigV1 `json:"kubernetesCustomResourceConversion"`
	Settings             *SettingsV1                    `json:"settings"`
}

// Schedule configuration
type ScheduleConfigV1 struct {
	Name                 string        `json:"name"`
	Crontab              string        `json:"crontab"`
	FirstRunDelay        time.Duration `json:"firstRunDelay,omitempty"`
	AllowFailure         bool          `json:"allowFailure"`
	IncludeSnapshotsFrom []string      `json:"includeSnapshotsFrom"`
	Queue                string        `json:"queue"`
	Group                string        `json:"group,omitempty"`
}

// version 1 of kubernetes event configuration
type OnKubernetesEventConfigV1 struct {
	Name                         string                   `json:"name,omitempty"`
	WatchEventTypes              []WatchEventType         `json:"watchEvent,omitempty"`
	ExecuteHookOnEvents          []WatchEventType         `json:"executeHookOnEvent,omitempty"`
	ExecuteHookOnSynchronization string                   `json:"executeHookOnSynchronization,omitempty"`
	WaitForSynchronization       string                   `json:"waitForSynchronization,omitempty"`
	KeepFullObjectsInMemory      string                   `json:"keepFullObjectsInMemory,omitempty"`
	Mode                         KubeEventMode            `json:"mode,omitempty"`
	ApiVersion                   string                   `json:"apiVersion,omitempty"`
	Kind                         string                   `json:"kind,omitempty"`
	NameSelector                 *KubeNameSelectorV1      `json:"nameSelector,omitempty"`
	LabelSelector                *metav1.LabelSelector    `json:"labelSelector,omitempty"`
	FieldSelector                *KubeFieldSelectorV1     `json:"fieldSelector,omitempty"`
	Namespace                    *KubeNamespaceSelectorV1 `json:"namespace,omitempty"`
	JqFilter                     string                   `json:"jqFilter,omitempty"`
	AllowFailure                 bool                     `json:"allowFailure,omitempty"`
	ResynchronizationPeriod      string                   `json:"resynchronizationPeriod,omitempty"`
	IncludeSnapshotsFrom         []string                 `json:"includeSnapshotsFrom,omitempty"`
	Queue                        string                   `json:"queue,omitempty"`
	Group                        string                   `json:"group,omitempty"`
}

type KubeNameSelectorV1 NameSelector

type KubeFieldSelectorV1 FieldSelector

type KubeNamespaceSelectorV1 NamespaceSelector

// version 1 of kubernetes event configuration
type KubernetesValidatingConfigV1 struct {
	Name                 string                   `json:"name,omitempty"`
	IncludeSnapshotsFrom []string                 `json:"includeSnapshotsFrom,omitempty"`
	Group                string                   `json:"group,omitempty"`
	Rules                []v1.RuleWithOperations  `json:"rules,omitempty"`
	FailurePolicy        *v1.FailurePolicyType    `json:"failurePolicy"`
	LabelSelector        *metav1.LabelSelector    `json:"labelSelector,omitempty"`
	Namespace            *KubeNamespaceSelectorV1 `json:"namespace,omitempty"`
	SideEffects          *v1.SideEffectClass      `json:"sideEffects"`
	TimeoutSeconds       *int32                   `json:"timeoutSeconds,omitempty"`
}

// version 1 of kubernetes conversion configuration
type KubernetesConversionConfigV1 struct {
	Name                 string            `json:"name,omitempty"`
	IncludeSnapshotsFrom []string          `json:"includeSnapshotsFrom,omitempty"`
	Group                string            `json:"group,omitempty"`
	CrdName              string            `json:"crdName,omitempty"`
	Conversions          []conversion.Rule `json:"conversions,omitempty"`
}

// version 1 of hook settings
type SettingsV1 struct {
	ExecutionMinInterval string `json:"executionMinInterval,omitempty"`
	ExecutionBurst       string `json:"executionBurst,omitempty"`
}

// ConvertAndCheck fills non-versioned structures and run inter-field checks not covered by OpenAPI schemas.
func (cv1 *HookConfigV1) ConvertAndCheck(c *HookConfig) (err error) {
	c.Settings, err = cv1.CheckAndConvertSettings(cv1.Settings)
	if err != nil {
		return err
	}

	c.OnStartup, err = c.ConvertOnStartup(cv1.OnStartup)
	if err != nil {
		return err
	}

	c.OnKubernetesEvents = []OnKubernetesEventConfig{}
	for i, kubeCfg := range cv1.OnKubernetesEvent {
		err := cv1.CheckOnKubernetesEvent(kubeCfg, fmt.Sprintf("kubernetes[%d]", i))
		if err != nil {
			return fmt.Errorf("invalid kubernetes config [%d]: %v", i, err)
		}

		monitor := &kube_events_manager.MonitorConfig{}
		monitor.Metadata.DebugName = MonitorDebugName(kubeCfg.Name, i)
		monitor.Metadata.MonitorId = MonitorConfigID()
		monitor.Metadata.LogLabels = map[string]string{}
		monitor.Metadata.MetricLabels = map[string]string{}
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

		// ExecuteHookOnSynchronization is enabled by default.
		kubeConfig.ExecuteHookOnSynchronization = true
		if kubeCfg.ExecuteHookOnSynchronization == "false" {
			kubeConfig.ExecuteHookOnSynchronization = false
		}

		// WaitForSynchronization is enabled by default. It can be disabled only for named queues.
		kubeConfig.WaitForSynchronization = true
		if kubeCfg.WaitForSynchronization == "false" && kubeCfg.Queue != "" {
			kubeConfig.WaitForSynchronization = false
		}

		// KeepFullObjectsInMemory is enabled by default.
		kubeConfig.KeepFullObjectsInMemory = true
		if kubeCfg.KeepFullObjectsInMemory == "false" {
			kubeConfig.KeepFullObjectsInMemory = false
		}
		kubeConfig.Monitor.KeepFullObjectsInMemory = kubeConfig.KeepFullObjectsInMemory

		c.OnKubernetesEvents = append(c.OnKubernetesEvents, kubeConfig)
	}

	// Chsck snapshots in result config.
	for i, kubeCfg := range c.OnKubernetesEvents {
		if len(kubeCfg.IncludeSnapshotsFrom) > 0 {
			err := CheckIncludeSnapshots(c.OnKubernetesEvents, kubeCfg.IncludeSnapshotsFrom...)
			if err != nil {
				return fmt.Errorf("invalid kubernetes config [%d]: includeSnapshots %v", i, err)
			}
		}
	}

	// schedule bindings with includeSnapshotsFrom
	// are depend on kubernetes bindings.
	c.Schedules = []ScheduleConfig{}
	for i, rawSchedule := range cv1.Schedule {
		err := cv1.CheckSchedule(c.OnKubernetesEvents, rawSchedule)
		if err != nil {
			return fmt.Errorf("invalid schedule config [%d]: %v", i, err)
		}
		schedule, err := cv1.ConvertSchedule(rawSchedule)
		if err != nil {
			return err
		}
		c.Schedules = append(c.Schedules, schedule)
	}

	// Validating webhooks
	c.KubernetesValidating = []ValidatingConfig{}
	for i, rawValidating := range c.V1.KubernetesValidating {
		err := cv1.CheckValidating(c.OnKubernetesEvents, rawValidating)
		if err != nil {
			return fmt.Errorf("invalid kubernetesValidating config [%d]: %v", i, err)
		}
		validating, err := cv1.ConvertValidating(rawValidating)
		if err != nil {
			return err
		}
		c.KubernetesValidating = append(c.KubernetesValidating, validating)
	}
	// Validate webhooks
	webhooks := []v1.ValidatingWebhook{}
	for _, cfg := range c.KubernetesValidating {
		webhooks = append(webhooks, *cfg.Webhook.ValidatingWebhook)
	}
	err = validation.ValidateValidatingWebhooks(&v1.ValidatingWebhookConfiguration{
		Webhooks: webhooks,
	})
	if err != nil {
		return err
	}

	// Conversion webhooks.
	c.KubernetesConversion = []ConversionConfig{}
	for i, rawConversion := range c.V1.KubernetesConversion {
		err := cv1.CheckConversion(c.OnKubernetesEvents, rawConversion)
		if err != nil {
			return fmt.Errorf("invalid kubernetesCustomResourceConversion config [%d]: %v", i, err)
		}
		conversionConfig, err := cv1.ConvertConversion(rawConversion)
		if err != nil {
			return err
		}
		c.KubernetesConversion = append(c.KubernetesConversion, conversionConfig)
	}

	// Update IncludeSnapshotsFrom for every binding with a group.
	// Merge binding's IncludeSnapshotsFrom with snapshots list calculated for group.
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
			cfg.IncludeSnapshotsFrom = MergeArrays(cfg.IncludeSnapshotsFrom, snapshots)
		}
		newKubeEvents = append(newKubeEvents, cfg)
	}
	c.OnKubernetesEvents = newKubeEvents

	newSchedules := make([]ScheduleConfig, 0)
	for _, cfg := range c.Schedules {
		if snapshots, ok := groupSnapshots[cfg.Group]; ok {
			cfg.IncludeSnapshotsFrom = MergeArrays(cfg.IncludeSnapshotsFrom, snapshots)
		}
		newSchedules = append(newSchedules, cfg)
	}
	c.Schedules = newSchedules

	newValidating := make([]ValidatingConfig, 0)
	for _, cfg := range c.KubernetesValidating {
		if snapshots, ok := groupSnapshots[cfg.Group]; ok {
			cfg.IncludeSnapshotsFrom = MergeArrays(cfg.IncludeSnapshotsFrom, snapshots)
		}
		newValidating = append(newValidating, cfg)
	}
	c.KubernetesValidating = newValidating

	newConversion := make([]ConversionConfig, 0)
	for _, cfg := range c.KubernetesConversion {
		if snapshots, ok := groupSnapshots[cfg.Group]; ok {
			cfg.IncludeSnapshotsFrom = MergeArrays(cfg.IncludeSnapshotsFrom, snapshots)
		}
		newConversion = append(newConversion, cfg)
	}
	c.KubernetesConversion = newConversion

	return nil
}

func (cv1 *HookConfigV1) ConvertSchedule(schV1 ScheduleConfigV1) (ScheduleConfig, error) {
	res := ScheduleConfig{}

	if schV1.Name != "" {
		res.BindingName = schV1.Name
	} else {
		res.BindingName = string(Schedule)
	}

	res.AllowFailure = schV1.AllowFailure
	res.ScheduleEntry = ScheduleEntry{
		Crontab:       schV1.Crontab,
		Id:            ScheduleID(),
		FirstRunDelay: schV1.FirstRunDelay,
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

func (cv1 *HookConfigV1) CheckSchedule(kubeConfigs []OnKubernetesEventConfig, schV1 ScheduleConfigV1) (allErr error) {
	var err error
	_, err = cron.Parse(schV1.Crontab)
	if err != nil {
		allErr = multierror.Append(allErr, fmt.Errorf("crontab is invalid: %v", err))
	}

	if len(schV1.IncludeSnapshotsFrom) > 0 {
		err = CheckIncludeSnapshots(kubeConfigs, schV1.IncludeSnapshotsFrom...)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("includeSnapshotsFrom is invalid: %v", err))
		}
	}

	return allErr
}

func (сv1 *HookConfigV1) CheckOnKubernetesEvent(kubeCfg OnKubernetesEventConfigV1, rootPath string) (allErr error) {
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

	return allErr
}

func (cv1 *HookConfigV1) CheckValidating(kubeConfigs []OnKubernetesEventConfig, cfgV1 KubernetesValidatingConfigV1) (allErr error) {
	var err error

	if len(cfgV1.IncludeSnapshotsFrom) > 0 {
		err = CheckIncludeSnapshots(kubeConfigs, cfgV1.IncludeSnapshotsFrom...)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("includeSnapshotsFrom is invalid: %v", err))
		}
	}

	if cfgV1.LabelSelector != nil {
		_, err := kube_events_manager.FormatLabelSelector(cfgV1.LabelSelector)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("labelSelector is invalid: %v", err))
		}
	}

	if cfgV1.Namespace != nil && cfgV1.Namespace.LabelSelector != nil {
		_, err := kube_events_manager.FormatLabelSelector(cfgV1.Namespace.LabelSelector)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("namespace.labelSelector is invalid: %v", err))
		}
	}

	return allErr
}

func (cv1 *HookConfigV1) ConvertValidating(cfgV1 KubernetesValidatingConfigV1) (ValidatingConfig, error) {
	cfg := ValidatingConfig{}

	cfg.Group = cfgV1.Group
	cfg.IncludeSnapshotsFrom = cfgV1.IncludeSnapshotsFrom
	cfg.BindingName = cfgV1.Name

	DefaultFailurePolicy := v1.Fail
	DefaultSideEffects := v1.SideEffectClassNone
	DefaultTimeoutSeconds := int32(10)

	webhook := &v1.ValidatingWebhook{
		Name:  cfgV1.Name,
		Rules: cfgV1.Rules,
	}
	if cfgV1.Namespace != nil {
		webhook.NamespaceSelector = cfgV1.Namespace.LabelSelector
	}
	if cfgV1.LabelSelector != nil {
		webhook.ObjectSelector = cfgV1.LabelSelector
	}
	if cfgV1.FailurePolicy != nil {
		webhook.FailurePolicy = cfgV1.FailurePolicy
	} else {
		webhook.FailurePolicy = &DefaultFailurePolicy
	}
	if cfgV1.SideEffects != nil {
		webhook.SideEffects = cfgV1.SideEffects
	} else {
		webhook.SideEffects = &DefaultSideEffects
	}
	if cfgV1.TimeoutSeconds != nil {
		webhook.TimeoutSeconds = cfgV1.TimeoutSeconds
	} else {
		webhook.TimeoutSeconds = &DefaultTimeoutSeconds
	}

	cfg.Webhook = &validating.ValidatingWebhookConfig{
		ValidatingWebhook: webhook,
	}
	cfg.Webhook.Metadata.LogLabels = map[string]string{}
	cfg.Webhook.Metadata.MetricLabels = map[string]string{}

	return cfg, nil
}

func (cv1 *HookConfigV1) CheckConversion(kubeConfigs []OnKubernetesEventConfig, cfgV1 KubernetesConversionConfigV1) (allErr error) {
	var err error

	if len(cfgV1.IncludeSnapshotsFrom) > 0 {
		err = CheckIncludeSnapshots(kubeConfigs, cfgV1.IncludeSnapshotsFrom...)
		if err != nil {
			allErr = multierror.Append(allErr, fmt.Errorf("includeSnapshotsFrom is invalid: %v", err))
		}
	}

	return allErr
}

func (cv1 *HookConfigV1) ConvertConversion(cfgV1 KubernetesConversionConfigV1) (ConversionConfig, error) {
	cfg := ConversionConfig{}

	cfg.Group = cfgV1.Group
	cfg.IncludeSnapshotsFrom = cfgV1.IncludeSnapshotsFrom
	cfg.BindingName = cfgV1.Name

	cfg.Webhook = &conversion.WebhookConfig{
		Rules:   cfgV1.Conversions,
		CrdName: cfgV1.CrdName,
	}

	cfg.Webhook.Metadata.LogLabels = map[string]string{}
	cfg.Webhook.Metadata.MetricLabels = map[string]string{}

	return cfg, nil
}

// CheckAndConvertSettings validates a duration and returns a Settings struct.
func (cv1 *HookConfigV1) CheckAndConvertSettings(settings *SettingsV1) (out *Settings, allErr error) {
	if settings == nil {
		return nil, nil
	}

	interval, err := time.ParseDuration(settings.ExecutionMinInterval)
	if err != nil {
		allErr = multierror.Append(allErr, fmt.Errorf("executionMinInterval is invalid: %v", err))
	}

	burst, err := strconv.ParseInt(settings.ExecutionBurst, 10, 32)
	if err != nil {
		allErr = multierror.Append(allErr, fmt.Errorf("executionMinInterval is invalid: %v", err))
	}
	if allErr != nil {
		return nil, allErr
	}

	return &Settings{
		ExecutionMinInterval: interval,
		ExecutionBurst:       int(burst),
	}, nil
}
