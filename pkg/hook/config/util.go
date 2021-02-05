package config

import (
	"fmt"

	uuid "gopkg.in/satori/go.uuid.v1"
)

func ConvertFloatForBinding(value interface{}, bindingName string) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if floatValue, ok := value.(float64); ok {
		return &floatValue, nil
	}
	return nil, fmt.Errorf("binding %s has unsupported value '%v'", bindingName, value)
}

// MergeArrays returns merged array with unique elements. Preserve elements order.
func MergeArrays(a1 []string, a2 []string) []string {
	union := make(map[string]bool)
	for _, a := range a2 {
		union[a] = true
	}
	res := make([]string, 0)
	for _, a := range a1 {
		res = append(res, a)
		union[a] = false
	}
	for _, a := range a2 {
		if union[a] {
			res = append(res, a)
			union[a] = false
		}
	}
	return res
}

func MonitorDebugName(configName string, configIndex int) string {
	if configName == "" {
		return fmt.Sprintf("kubernetes[%d]", configIndex)
	} else {
		return fmt.Sprintf("kubernetes[%d]{%s}", configIndex, configName)
	}
}

// TODO uuid is not a good choice here. Make it more readable.
func MonitorConfigID() string {
	return uuid.NewV4().String()
	//ei.DebugName = uuid.NewV4().String()
	//if ei.Monitor.ConfigIdPrefix != "" {
	//	ei.DebugName = ei.Monitor.ConfigIdPrefix + "-" + ei.DebugName[len(ei.Monitor.ConfigIdPrefix)+1:]
	//}
	//return ei.DebugName
}

// TODO uuid is not a good choice here. Make it more readable.
func ScheduleID() string {
	return uuid.NewV4().String()
}
