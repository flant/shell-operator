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
