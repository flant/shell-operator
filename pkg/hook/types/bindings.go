package types

import (
	"fmt"
	"time"

	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	smtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

var _ fmt.Stringer = (*BindingType)(nil)

type BindingType string

func (bt BindingType) String() string {
	return string(bt)
}

const (
	Schedule             BindingType = "schedule"
	OnStartup            BindingType = "onStartup"
	OnKubernetesEvent    BindingType = "kubernetes"
	KubernetesConversion BindingType = "kubernetesCustomResourceConversion"
	KubernetesValidating BindingType = "kubernetesValidating"
	KubernetesMutating   BindingType = "kubernetesMutating"
)

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
	ScheduleEntry        smtypes.ScheduleEntry
	IncludeSnapshotsFrom []string
	Queue                string
	Group                string
}

type OnKubernetesEventConfig struct {
	CommonBindingConfig
	Monitor                      *kubeeventsmanager.MonitorConfig
	IncludeSnapshotsFrom         []string
	Queue                        string
	Group                        string
	ExecuteHookOnSynchronization bool
	WaitForSynchronization       bool
	KeepFullObjectsInMemory      bool
}

type ConversionConfig struct {
	CommonBindingConfig
	IncludeSnapshotsFrom []string
	Group                string
	Webhook              *conversion.WebhookConfig
}

type ValidatingConfig struct {
	CommonBindingConfig
	IncludeSnapshotsFrom []string
	Group                string
	Webhook              *admission.ValidatingWebhookConfig
}

type MutatingConfig struct {
	CommonBindingConfig
	IncludeSnapshotsFrom []string
	Group                string
	Webhook              *admission.MutatingWebhookConfig
}

type Settings struct {
	ExecutionMinInterval time.Duration
	ExecutionBurst       int
}
