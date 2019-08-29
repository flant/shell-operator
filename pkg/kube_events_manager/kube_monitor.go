package kube_events_manager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubeEventMonitorConfig is a config that suits the latest
// version of OnKuberneteEventConfig.
type MonitorConfig struct {
	ConfigIdPrefix    string
	EventTypes        []KubeEventType
	ApiVersion        string
	Kind              string
	NameSelector      *NameSelector
	NamespaceSelector *NamespaceSelector
	LabelSelector     *metav1.LabelSelector
	FieldSelector     *FieldSelector
	JqFilter          string
}

func (c *MonitorConfig) WithEventTypes(types []KubeEventType) *MonitorConfig {
	if types == nil {
		c.EventTypes = []KubeEventType{
			KubeEventAdd,
			KubeEventUpdate,
			KubeEventDelete,
		}
	} else {
		c.EventTypes = []KubeEventType{}
		for _, eventType := range types {
			c.EventTypes = append(c.EventTypes, eventType)
		}
	}
	return c
}

// WithNamespaceSelector copies input NamespaceSelector into monitor.NamespaceSelector
func (c *MonitorConfig) WithNameSelector(nSel *NameSelector) {
	if nSel != nil {
		c.NameSelector = &NameSelector{
			nSel.MatchNames,
		}
	}
}

// WithNamespaceSelector copies input NamespaceSelector into monitor.NamespaceSelector
func (c *MonitorConfig) WithNamespaceSelector(nsSel *NamespaceSelector) {
	if nsSel != nil {
		c.NamespaceSelector = &NamespaceSelector{}
		if nsSel.NameSelector != nil {
			c.NamespaceSelector.NameSelector = &NameSelector{
				nsSel.NameSelector.MatchNames,
			}
		}
		if nsSel.LabelSelector != nil {
			c.NamespaceSelector.LabelSelector = &metav1.LabelSelector{
				nsSel.LabelSelector.MatchLabels,
				nsSel.LabelSelector.MatchExpressions,
			}
		}
	}
}

// WithFieldSelector copies input FieldSelector into monitor.FieldSelector
func (c *MonitorConfig) WithFieldSelector(fieldSel *FieldSelector) {
	if fieldSel != nil {
		c.FieldSelector = &FieldSelector{
			fieldSel.MatchExpressions,
		}
	}
}

func (c *MonitorConfig) AddFieldSelectorRequirement(field string, op string, value string) {
	if c.FieldSelector == nil {
		c.FieldSelector = &FieldSelector{
			[]FieldSelectorRequirement{},
		}
	}

	req := FieldSelectorRequirement{
		field,
		op,
		value,
	}

	c.FieldSelector.MatchExpressions = append(c.FieldSelector.MatchExpressions, req)
}

// WithLabelSelector copies input LabelSelector into monitor.LabelSelector
func (c *MonitorConfig) WithLabelSelector(labelSel *metav1.LabelSelector) {
	if labelSel != nil {
		c.LabelSelector = &metav1.LabelSelector{
			labelSel.MatchLabels,
			labelSel.MatchExpressions,
		}
	}
}

func (c *MonitorConfig) IsAnyNamespace() bool {
	return c.NamespaceSelector == nil ||
		(c.NamespaceSelector.NameSelector == nil && c.NamespaceSelector.LabelSelector == nil) ||
		(c.NamespaceSelector.NameSelector != nil && len(c.NamespaceSelector.NameSelector.MatchNames) == 0)
}

// Names returns names of monitored objects if nameSelector.matchNames is defined in config.
func (c *MonitorConfig) Names() []string {
	res := []string{}

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
func (c *MonitorConfig) Namespaces() (nsNames []string) {
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
