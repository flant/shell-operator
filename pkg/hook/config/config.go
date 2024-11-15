package config

import (
	"fmt"

	"sigs.k8s.io/yaml"

	htypes "github.com/flant/shell-operator/pkg/hook/types"
)

var validBindingTypes = []htypes.BindingType{htypes.OnStartup, htypes.Schedule, htypes.OnKubernetesEvent, htypes.KubernetesValidating, htypes.KubernetesMutating, htypes.KubernetesConversion}

// HookConfig is a structure with versioned hook configuration
type HookConfig struct {
	// effective version of config
	Version string

	// versioned raw config values
	V0 *HookConfigV0
	V1 *HookConfigV1

	// effective config values
	OnStartup            *htypes.OnStartupConfig
	Schedules            []htypes.ScheduleConfig
	OnKubernetesEvents   []htypes.OnKubernetesEventConfig
	KubernetesValidating []htypes.ValidatingConfig
	KubernetesMutating   []htypes.MutatingConfig
	KubernetesConversion []htypes.ConversionConfig
	Settings             *htypes.Settings
}

// LoadAndValidate loads config from bytes and validate it. Returns multierror.
func (c *HookConfig) LoadAndValidate(data []byte) error {
	// - unmarshal json into map
	// - detect version
	// - validate with openapi schema
	// - load again as versioned struct
	// - convert
	// - make complex checks

	vu := NewDefaultVersionedUntyped()
	err := vu.Load(data)
	if err != nil {
		return err
	}

	err = ValidateConfig(vu.Obj, GetSchema(vu.Version), "")
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

// ConvertAndCheck transforms a versioned configuration to latest internal structures.
func (c *HookConfig) ConvertAndCheck(data []byte) error {
	switch c.Version {
	case "v0":
		configV0 := &HookConfigV0{}
		err := yaml.Unmarshal(data, configV0)
		if err != nil {
			return fmt.Errorf("unmarshal HookConfig version 0: %s", err)
		}
		c.V0 = configV0
		err = configV0.ConvertAndCheck(c)
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
		err = configV1.ConvertAndCheck(c)
		if err != nil {
			return err
		}
	default:
		// NOTE: this should not happen
		return fmt.Errorf("version '%s' is unsupported", c.Version)
	}
	return nil
}

// Bindings returns a list of binding types in hook configuration.
func (c *HookConfig) Bindings() []htypes.BindingType {
	res := []htypes.BindingType{}

	for _, binding := range validBindingTypes {
		if c.HasBinding(binding) {
			res = append(res, binding)
		}
	}

	return res
}

// HasBinding returns true if a hook configuration has binding type.
func (c *HookConfig) HasBinding(binding htypes.BindingType) bool {
	switch binding {
	case htypes.OnStartup:
		return c.OnStartup != nil
	case htypes.Schedule:
		return len(c.Schedules) > 0
	case htypes.OnKubernetesEvent:
		return len(c.OnKubernetesEvents) > 0
	case htypes.KubernetesValidating:
		return len(c.KubernetesValidating) > 0
	case htypes.KubernetesMutating:
		return len(c.KubernetesMutating) > 0
	case htypes.KubernetesConversion:
		return len(c.KubernetesConversion) > 0
	}
	return false
}

func (c *HookConfig) ConvertOnStartup(value interface{}) (*htypes.OnStartupConfig, error) {
	floatValue, err := ConvertFloatForBinding(value, "onStartup")
	if err != nil || floatValue == nil {
		return nil, err
	}

	res := &htypes.OnStartupConfig{}
	res.AllowFailure = false
	res.BindingName = string(htypes.OnStartup)
	res.Order = *floatValue
	return res, nil
}

// CheckIncludeSnapshots check if all includes has corresponding kubernetes
// binding. Rules:
//
// - binding name should exists,
//
// - binding name should not be repeated.
func CheckIncludeSnapshots(kubeConfigs []htypes.OnKubernetesEventConfig, includes ...string) error {
	for _, include := range includes {
		bindings := 0
		for _, kubeCfg := range kubeConfigs {
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
