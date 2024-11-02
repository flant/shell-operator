package kube_events_manager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// MonitorConfig is a config that suits the latest
// version of OnKubernetesEventConfig.
type MonitorConfig struct {
	Metadata struct {
		MonitorId    string
		DebugName    string
		LogLabels    map[string]string
		MetricLabels map[string]string
	}
	EventTypes              []WatchEventType
	ApiVersion              string
	Kind                    string
	NameSelector            *NameSelector
	NamespaceSelector       *NamespaceSelector
	LabelSelector           *metav1.LabelSelector
	FieldSelector           *FieldSelector
	JqFilter                string
	LogEntry                *log.Logger
	Mode                    KubeEventMode
	KeepFullObjectsInMemory bool
	FilterFunc              func(*unstructured.Unstructured) (interface{}, error)
}

func (c *MonitorConfig) WithEventTypes(types []WatchEventType) *MonitorConfig {
	if types == nil {
		c.EventTypes = []WatchEventType{
			WatchEventAdded,
			WatchEventModified,
			WatchEventDeleted,
		}
	} else {
		c.EventTypes = []WatchEventType{}
		c.EventTypes = append(c.EventTypes, types...)
	}
	return c
}

// WithNameSelector copies input NameSelector into monitor.NameSelector
func (c *MonitorConfig) WithNameSelector(nSel *NameSelector) {
	if nSel != nil {
		c.NameSelector = &NameSelector{
			MatchNames: nSel.MatchNames,
		}
	}
}

// WithNamespaceSelector copies input NamespaceSelector into monitor.NamespaceSelector
func (c *MonitorConfig) WithNamespaceSelector(nsSel *NamespaceSelector) {
	if nsSel != nil {
		c.NamespaceSelector = &NamespaceSelector{}
		if nsSel.NameSelector != nil {
			c.NamespaceSelector.NameSelector = &NameSelector{
				MatchNames: nsSel.NameSelector.MatchNames,
			}
		}
		if nsSel.LabelSelector != nil {
			c.NamespaceSelector.LabelSelector = &metav1.LabelSelector{
				MatchLabels:      nsSel.LabelSelector.MatchLabels,
				MatchExpressions: nsSel.LabelSelector.MatchExpressions,
			}
		}
	}
}

// WithFieldSelector copies input FieldSelector into monitor.FieldSelector
func (c *MonitorConfig) WithFieldSelector(fieldSel *FieldSelector) {
	if fieldSel != nil {
		c.FieldSelector = &FieldSelector{
			MatchExpressions: fieldSel.MatchExpressions,
		}
	}
}

// WithLabelSelector copies input LabelSelector into monitor.LabelSelector
func (c *MonitorConfig) WithLabelSelector(labelSel *metav1.LabelSelector) {
	if labelSel != nil {
		c.LabelSelector = &metav1.LabelSelector{
			MatchLabels:      labelSel.MatchLabels,
			MatchExpressions: labelSel.MatchExpressions,
		}
	}
}

// names returns names of monitored objects if nameSelector.matchNames is defined in config.
func (c *MonitorConfig) names() []string {
	res := make([]string, 0)

	if c.NameSelector != nil {
		res = c.NameSelector.MatchNames
	}

	return res
}

// Namespaces returns names of namespaces if namescpace.nameSelector
// is defined in config.
//
// If no namespace specified or no namespace.nameSelector or
// length of namespace.nameSeletor.matchNames is 0
// then empty string is returned to monitor all namespaces.
//
// If namespace.labelSelector is specified, then return empty array.
func (c *MonitorConfig) namespaces() (nsNames []string) {
	if c.NamespaceSelector == nil {
		return []string{""}
	}

	if c.NamespaceSelector.LabelSelector != nil {
		return []string{}
	}

	if c.NamespaceSelector.NameSelector == nil {
		return []string{""}
	}

	if len(c.NamespaceSelector.NameSelector.MatchNames) == 0 {
		return []string{""}
	}

	nsNames = c.NamespaceSelector.NameSelector.MatchNames
	return nsNames
}

func (c *MonitorConfig) WithMode(mode KubeEventMode) {
	if mode == "" {
		c.Mode = ModeIncremental
	}
	c.Mode = mode
}
