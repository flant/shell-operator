package utils

import (
	"log/slog"
	"sort"

	"github.com/deckhouse/deckhouse/pkg/log"
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

func EnrichLoggerWithLabels(logger *log.Logger, labelsMaps ...map[string]string) *log.Logger {
	loggerEntry := logger

	for _, labels := range labelsMaps {
		for k, v := range labels {
			loggerEntry.With(slog.String(k, v))
		}
	}

	return loggerEntry
}

// LabelNames returns sorted label keys
func LabelNames(labels map[string]string) []string {
	names := make([]string, 0)
	for labelName := range labels {
		names = append(names, labelName)
	}
	sort.Strings(names)
	return names
}

func LabelValues(labels map[string]string, labelNames []string) []string {
	values := make([]string, 0)
	for _, name := range labelNames {
		values = append(values, labels[name])
	}
	return values
}

func DefaultIfEmpty(m map[string]string, def map[string]string) map[string]string {
	if len(m) == 0 {
		return def
	}
	return m
}

// IsSubset checks if a set contains b subset
func IsSubset(a, b []string) bool {
	aMap := make(map[string]struct{}, len(a))
	for _, v := range a {
		aMap[v] = struct{}{}
	}

	for _, v := range b {
		if _, found := aMap[v]; !found {
			return false
		}
	}
	return true
}
