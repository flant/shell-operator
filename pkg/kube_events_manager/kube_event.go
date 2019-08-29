package kube_events_manager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeEventType string

const (
	KubeEventAdd    KubeEventType = "add"
	KubeEventUpdate KubeEventType = "update"
	KubeEventDelete KubeEventType = "delete"
)

var (
	KubeEventCh chan KubeEvent
)

// KubeEvent contains Config id returned from Run method, event types
// and k8s object identification
type KubeEvent struct {
	ConfigId  string
	Events    []KubeEventType
	Namespace string
	Kind      string
	Name      string
}

type NameSelector struct {
	MatchNames []string `json:"matchNames"`
}

type FieldSelectorRequirement struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}

type FieldSelector struct {
	MatchExpressions []FieldSelectorRequirement `json:"matchExpressions"`
}

type NamespaceSelector struct {
	NameSelector  *NameSelector         `json:"nameSelector,omitempty"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}
