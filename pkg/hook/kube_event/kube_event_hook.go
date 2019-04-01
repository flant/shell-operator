package kube_event

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

// a link between a hook and a kube event
type KubeEventHook struct {
	HookName string
	Name     string

	EventTypes   []kube_events_manager.OnKubernetesEventType
	Kind         string
	Namespace    string
	Selector     *metav1.LabelSelector
	JqFilter     string
	AllowFailure bool
	Debug        bool

	Config kube_events_manager.OnKubernetesEventConfig
}
