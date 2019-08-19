package kube_event

import (
	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

// a link between a hook and a kube event informer
type KubeEventHook struct {
	// hook name
	HookName string
	// one OnKubernetesEvent configuration
	Config kube_events_manager.OnKubernetesEventConfig
	// ConfigId from kube_events_manager
	ConfigId string
}
