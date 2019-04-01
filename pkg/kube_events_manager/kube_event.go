package kube_events_manager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OnKubernetesEventType string

const (
	KubernetesEventOnAdd    OnKubernetesEventType = "add"
	KubernetesEventOnUpdate OnKubernetesEventType = "update"
	KubernetesEventOnDelete OnKubernetesEventType = "delete"
)

type OnKubernetesEventConfig struct {
	Name              string                  `json:"name"`
	EventTypes        []OnKubernetesEventType `json:"event"`
	Kind              string                  `json:"kind"`
	Selector          *metav1.LabelSelector   `json:"selector"`
	NamespaceSelector *KubeNamespaceSelector  `json:"namespaceSelector"`
	JqFilter          string                  `json:"jqFilter"`
	AllowFailure      bool                    `json:"allowFailure"`
	DisableDebug      bool                    `json:"disableDebug"`
}

type KubeNamespaceSelector struct {
	MatchNames []string `json:"matchNames"`
	Any        bool     `json:"any"`
}

var (
	KubeEventCh chan KubeEvent
)

// KubeEvent contains event type and k8s object identification
type KubeEvent struct {
	ConfigId  string
	Events    []string
	Namespace string
	Kind      string
	Name      string
}
