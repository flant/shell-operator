package config

import (
	"fmt"

	"sigs.k8s.io/yaml"

	. "github.com/flant/shell-operator/pkg/hook/types"
)

var validBindingTypes = []BindingType{OnStartup, Schedule, OnKubernetesEvent, KubernetesValidating, KubernetesMutating, KubernetesConversion}

// HookConfig is a structure with versioned hook configuration
type HookConfig struct {
	// effective version of config
	Version string

	// versioned raw config values
	V0 *HookConfigV0
	V1 *HookConfigV1

	// effective config values
	OnStartup            *OnStartupConfig
	Schedules            []ScheduleConfig
	OnKubernetesEvents   []OnKubernetesEventConfig
	KubernetesValidating []ValidatingConfig
	KubernetesConversion []ConversionConfig
	Settings             *Settings
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
func (c *HookConfig) Bindings() []BindingType {
	res := []BindingType{}

	for _, binding := range validBindingTypes {
		if c.HasBinding(binding) {
			res = append(res, binding)
		}
	}

	return res
}

// HasBinding returns true if a hook configuration has binding type.
func (c *HookConfig) HasBinding(binding BindingType) bool {
	switch binding {
	case OnStartup:
		return c.OnStartup != nil
	case Schedule:
		return len(c.Schedules) > 0
	case OnKubernetesEvent:
		return len(c.OnKubernetesEvents) > 0
	case KubernetesValidating:
		return len(c.KubernetesValidating) > 0
	case KubernetesConversion:
		return len(c.KubernetesConversion) > 0
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

// CheckIncludeSnapshots check if all includes has corresponding kubernetes
// binding. Rules:
//
// - binding name should exists,
//
// - binding name should not be repeated.
func CheckIncludeSnapshots(kubeConfigs []OnKubernetesEventConfig, includes ...string) error {
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
