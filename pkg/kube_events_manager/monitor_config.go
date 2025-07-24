package kubeeventsmanager

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/itchyny/gojq"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
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
	EventTypes              []kemtypes.WatchEventType
	ApiVersion              string
	Kind                    string
	NameSelector            *kemtypes.NameSelector
	NamespaceSelector       *kemtypes.NamespaceSelector
	LabelSelector           *metav1.LabelSelector
	FieldSelector           *kemtypes.FieldSelector
	JqFilter                *gojq.Code
	Logger                  *log.Logger
	Mode                    kemtypes.KubeEventMode
	KeepFullObjectsInMemory bool
	FilterFunc              func(*unstructured.Unstructured) (interface{}, error)
}

func (c *MonitorConfig) WithEventTypes(types []kemtypes.WatchEventType) *MonitorConfig {
	if types == nil {
		c.EventTypes = []kemtypes.WatchEventType{
			kemtypes.WatchEventAdded,
			kemtypes.WatchEventModified,
			kemtypes.WatchEventDeleted,
		}
	} else {
		c.EventTypes = []kemtypes.WatchEventType{}
		c.EventTypes = append(c.EventTypes, types...)
	}
	return c
}

// WithNameSelector copies input NameSelector into monitor.NameSelector
func (c *MonitorConfig) WithNameSelector(nSel *kemtypes.NameSelector) {
	if nSel != nil {
		c.NameSelector = &kemtypes.NameSelector{
			MatchNames: nSel.MatchNames,
		}
	}
}

// WithNamespaceSelector copies input NamespaceSelector into monitor.NamespaceSelector
func (c *MonitorConfig) WithNamespaceSelector(nsSel *kemtypes.NamespaceSelector) {
	if nsSel != nil {
		c.NamespaceSelector = &kemtypes.NamespaceSelector{}
		if nsSel.NameSelector != nil {
			c.NamespaceSelector.NameSelector = &kemtypes.NameSelector{
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
func (c *MonitorConfig) WithFieldSelector(fieldSel *kemtypes.FieldSelector) {
	if fieldSel != nil {
		c.FieldSelector = &kemtypes.FieldSelector{
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
func (c *MonitorConfig) namespaces() []string {
	if c.NamespaceSelector == nil {
		return []string{""}
	}

	if c.NamespaceSelector.LabelSelector != nil {
		return nil
	}

	if c.NamespaceSelector.NameSelector == nil {
		return []string{""}
	}

	if len(c.NamespaceSelector.NameSelector.MatchNames) == 0 {
		return []string{""}
	}

	return c.NamespaceSelector.NameSelector.MatchNames
}

func (c *MonitorConfig) WithMode(mode kemtypes.KubeEventMode) {
	if mode == "" {
		c.Mode = kemtypes.ModeIncremental
	}
	c.Mode = mode
}
