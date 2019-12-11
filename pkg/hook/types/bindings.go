package types

import (
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/schedule_manager/types"
)

type BindingType string

const (
	Schedule          BindingType = "SCHEDULE"
	OnStartup         BindingType = "ON_STARTUP"
	OnKubernetesEvent BindingType = "ON_KUBERNETES_EVENT"
)

var ContextBindingType = map[BindingType]string{
	Schedule:          "schedule",
	OnStartup:         "onStartup",
	OnKubernetesEvent: "kubernetes",
}

// Types for effective binding configs
type CommonBindingConfig struct {
	BindingName  string
	AllowFailure bool
}

type OnStartupConfig struct {
	CommonBindingConfig
	Order float64
}

type ScheduleConfig struct {
	CommonBindingConfig
	ScheduleEntry                  ScheduleEntry
	IncludeKubernetesSnapshotsFrom []string
}

type OnKubernetesEventConfig struct {
	CommonBindingConfig
	Monitor              *kube_events_manager.MonitorConfig
	IncludeSnapshotsFrom []string
}
