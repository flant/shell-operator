package config

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

const VersionKey = "configVersion"

type VersionedUntyped struct {
	Obj     map[string]interface{}
	Version string

	VersionKey       string
	VersionValidator func(string, bool) (string, error)
}

// NewDefaultVersionedUntyped is a VersionedUntyper object with default version key
// and version validator against Schemas map
func NewDefaultVersionedUntyped() *VersionedUntyped {
	return &VersionedUntyped{
		VersionKey: VersionKey,
		VersionValidator: func(origVer string, found bool) (newVer string, err error) {
			newVer = origVer
			if !found {
				newVer = "v0"
			}

			_, hasSchema := Schemas[newVer]
			if hasSchema {
				return newVer, nil
			}

			return "", fmt.Errorf("'%s' value '%s' is unsupported", VersionKey, origVer)
		},
	}
}

func (u *VersionedUntyped) Load(data []byte) error {
	err := yaml.Unmarshal(data, &u.Obj)
	if err != nil {
		return fmt.Errorf("json unmarshal: %v", err)
	}

	// detect version
	u.Version, err = u.LoadConfigVersion()
	if err != nil {
		return fmt.Errorf("config version: %v", err)
	}

	return nil
}

// LoadConfigVersion
func (u *VersionedUntyped) LoadConfigVersion() (string, error) {
	value, found, err := u.GetString(u.VersionKey)
	if err != nil {
		return "", err
	}

	if u.VersionValidator != nil {
		newVer, err := u.VersionValidator(value, found)
		if err != nil {
			return value, err
		}
		return newVer, nil
	}

	return value, nil
}

// GetString returns string value by key
func (u *VersionedUntyped) GetString(key string) (value string, found bool, err error) {
	val, found := u.Obj[key]

	if !found {
		return "", false, nil
	}
	if val == nil {
		return "", true, fmt.Errorf("missing '%s' value", key)
	}
	value, ok := val.(string)
	if !ok {
		return "", true, fmt.Errorf("string value is expected for key '%s'", key)
	}
	return value, true, nil
}
