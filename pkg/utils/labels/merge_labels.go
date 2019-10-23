package utils

import (
	log "github.com/sirupsen/logrus"
)

// MergeLabels merges several maps into one. Last map keys overrides keys from first maps.
//
// Can be used to copy a map if just one argument is used.
func MergeLabels(labelsMaps ...map[string]string) map[string]string {
	labels := make(map[string]string)
	for _, labelsMap := range labelsMaps {
		for k, v := range labelsMap {
			labels[k] = v
		}
	}
	return labels
}

func LabelsToLogFields(labelsMaps ...map[string]string) log.Fields {
	fields := log.Fields{}
	for _, labels := range labelsMaps {
		for k, v := range labels {
			fields[k] = v
		}
	}
	return fields
}
