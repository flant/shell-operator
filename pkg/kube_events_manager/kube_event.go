package kube_events_manager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WatchEventType string

const (
	WatchEventAdded    WatchEventType = "Added"
	WatchEventModified WatchEventType = "Modified"
	WatchEventDeleted  WatchEventType = "Deleted"
	Synchronization    WatchEventType = "Synchronization"
)

var (
	KubeEventCh chan KubeEvent
)

type ObjectAndFilterResult struct {
	Object       map[string]interface{} `json:"object,omitempty"`
	FilterResult string                 `json:"filterResult,omitempty"`
}

// KubeEvent contains Config id returned from Run method, event types
// and k8s object identification
type KubeEvent struct {
	ConfigId     string
	Type         string // Event or Synchronization
	WatchEvents  []WatchEventType
	Event        string
	Namespace    string
	Kind         string
	Name         string
	Object       map[string]interface{}
	FilterResult string
	Objects      []ObjectAndFilterResult
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
