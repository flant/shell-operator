package controller

import (
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	. "github.com/flant/shell-operator/pkg/webhook/validating/types"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/validating"
)

type BindingExecutionInfo struct {
	BindingContext         []BindingContext
	IncludeSnapshots       []string
	IncludeAllSnapshots    bool
	AllowFailure           bool
	QueueName              string
	Binding                string
	Group                  string
	WaitForSynchronization bool
}

// В каждый хук надо будет положить этот объект.
// Предварительно позвав With*Bindings и InitBindingsControllers

// Для kube надо будет сделать HandleEnableKubernetesBindings, чтобы получить списки существующих обхектов
// и потом StartMonitors

// Всё Handle* методы принимают callback, чтобы создавать задания независимо.

// методом KubernetesSnapshots можно достать все кубовые объекты, чтобы добавить
// их в какой-то свой binding context

type HookController interface {
	InitKubernetesBindings([]OnKubernetesEventConfig, kube_events_manager.KubeEventsManager)
	InitScheduleBindings([]ScheduleConfig, schedule_manager.ScheduleManager)
	InitValidatingBindings([]ValidatingConfig, *validating.WebhookManager)
	InitConversionBindings([]ConversionConfig, *conversion.WebhookManager)

	CanHandleKubeEvent(kubeEvent KubeEvent) bool
	CanHandleScheduleEvent(crontab string) bool
	CanHandleValidatingEvent(event ValidatingEvent) bool
	CanHandleConversionEvent(event conversion.Event, rule conversion.Rule) bool

	// These method should call underlying BindingController to get binding context
	// and then add Snapshots to binding context
	HandleEnableKubernetesBindings(createTasksFn func(BindingExecutionInfo)) error
	HandleKubeEvent(event KubeEvent, createTasksFn func(BindingExecutionInfo))
	HandleScheduleEvent(crontab string, createTasksFn func(BindingExecutionInfo))
	HandleValidatingEvent(event ValidatingEvent, createTasksFn func(BindingExecutionInfo))
	HandleConversionEvent(event conversion.Event, rule conversion.Rule, createTasksFn func(BindingExecutionInfo))

	StartMonitors()
	StopMonitors()

	EnableScheduleBindings()
	DisableScheduleBindings()

	EnableValidatingBindings()

	EnableConversionBindings()

	KubernetesSnapshots() map[string][]ObjectAndFilterResult
	UpdateSnapshots([]BindingContext) []BindingContext
}

var _ HookController = &hookController{}

func NewHookController() HookController {
	return &hookController{}
}

type hookController struct {
	KubernetesController KubernetesBindingsController
	ScheduleController   ScheduleBindingsController
	ValidatingController ValidatingBindingsController
	ConversionController ConversionBindingsController
	kubernetesBindings   []OnKubernetesEventConfig
	scheduleBindings     []ScheduleConfig
	validatingBindings   []ValidatingConfig
	conversionBindings   []ConversionConfig
}

func (hc *hookController) InitKubernetesBindings(bindings []OnKubernetesEventConfig, kubeEventMgr kube_events_manager.KubeEventsManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewKubernetesBindingsController()
	bindingCtrl.WithKubeEventsManager(kubeEventMgr)
	bindingCtrl.WithKubernetesBindings(bindings)
	hc.KubernetesController = bindingCtrl
	hc.kubernetesBindings = bindings
}

func (hc *hookController) InitScheduleBindings(bindings []ScheduleConfig, scheduleMgr schedule_manager.ScheduleManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewScheduleBindingsController()
	bindingCtrl.WithScheduleManager(scheduleMgr)
	bindingCtrl.WithScheduleBindings(bindings)
	hc.ScheduleController = bindingCtrl
	hc.scheduleBindings = bindings
}

func (hc *hookController) InitValidatingBindings(bindings []ValidatingConfig, webhookMgr *validating.WebhookManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewValidatingBindingsController()
	bindingCtrl.WithWebhookManager(webhookMgr)
	bindingCtrl.WithValidatingBindings(bindings)
	hc.ValidatingController = bindingCtrl
	hc.validatingBindings = bindings
}

func (hc *hookController) InitConversionBindings(bindings []ConversionConfig, webhookMgr *conversion.WebhookManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewConversionBindingsController()
	bindingCtrl.WithWebhookManager(webhookMgr)
	bindingCtrl.WithBindings(bindings)
	hc.ConversionController = bindingCtrl
	hc.conversionBindings = bindings
}

func (hc *hookController) CanHandleKubeEvent(kubeEvent KubeEvent) bool {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.CanHandleEvent(kubeEvent)
	}
	return false
}

func (hc *hookController) CanHandleScheduleEvent(crontab string) bool {
	if hc.ScheduleController != nil {
		return hc.ScheduleController.CanHandleEvent(crontab)
	}
	return false
}

func (hc *hookController) CanHandleValidatingEvent(event ValidatingEvent) bool {
	if hc.ValidatingController != nil {
		return hc.ValidatingController.CanHandleEvent(event)
	}
	return false
}

func (hc *hookController) CanHandleConversionEvent(event conversion.Event, rule conversion.Rule) bool {
	if hc.ConversionController != nil {
		return hc.ConversionController.CanHandleEvent(event, rule)
	}
	return false
}

func (hc *hookController) HandleEnableKubernetesBindings(createTasksFn func(BindingExecutionInfo)) error {
	if hc.KubernetesController != nil {

		execInfos, err := hc.KubernetesController.EnableKubernetesBindings()
		if err != nil {
			return err
		}

		if createTasksFn != nil {
			for _, execInfo := range execInfos {
				createTasksFn(execInfo)
			}
		}
	}
	return nil
}

func (hc *hookController) HandleKubeEvent(event KubeEvent, createTasksFn func(BindingExecutionInfo)) {
	if hc.KubernetesController != nil {
		execInfo := hc.KubernetesController.HandleEvent(event)
		if createTasksFn != nil {
			createTasksFn(execInfo)
		}
	}
}

func (hc *hookController) HandleValidatingEvent(event ValidatingEvent, createTasksFn func(BindingExecutionInfo)) {
	if hc.ValidatingController == nil {
		return
	}
	execInfo := hc.ValidatingController.HandleEvent(event)
	if createTasksFn != nil {
		createTasksFn(execInfo)
	}
}

func (hc *hookController) HandleConversionEvent(event conversion.Event, rule conversion.Rule, createTasksFn func(BindingExecutionInfo)) {
	if hc.ConversionController == nil {
		return
	}
	execInfo := hc.ConversionController.HandleEvent(event, rule)
	if createTasksFn != nil {
		createTasksFn(execInfo)
	}
}

func (hc *hookController) HandleScheduleEvent(crontab string, createTasksFn func(BindingExecutionInfo)) {
	if hc.ScheduleController == nil {
		return
	}
	infos := hc.ScheduleController.HandleEvent(crontab)
	if createTasksFn == nil {
		return
	}
	for _, info := range infos {
		createTasksFn(info)
	}
}

func (hc *hookController) StartMonitors() {
	if hc.KubernetesController != nil {
		hc.KubernetesController.StartMonitors()
	}
}

func (hc *hookController) StopMonitors() {
	if hc.KubernetesController != nil {
		hc.KubernetesController.StopMonitors()
	}
}

func (hc *hookController) EnableScheduleBindings() {
	if hc.ScheduleController != nil {
		hc.ScheduleController.EnableScheduleBindings()
	}
}

func (hc *hookController) DisableScheduleBindings() {
	if hc.ScheduleController != nil {
		hc.ScheduleController.DisableScheduleBindings()
	}
}

func (hc *hookController) EnableValidatingBindings() {
	if hc.ValidatingController != nil {
		hc.ValidatingController.EnableValidatingBindings()
	}
}

func (hc *hookController) EnableConversionBindings() {
	if hc.ConversionController != nil {
		hc.ConversionController.EnableConversionBindings()
	}
}

// KubernetesSnapshots returns all exited objects for all registered kubernetes bindings.
func (hc *hookController) KubernetesSnapshots() map[string][]ObjectAndFilterResult {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.Snapshots()
	}
	return map[string][]ObjectAndFilterResult{}
}

// KubernetesSnapshotsFor returns snapshots for schedule or kubernetes binding
func (hc *hookController) KubernetesSnapshotsFor(bindingType BindingType, bindingName string) map[string][]ObjectAndFilterResult {
	includeSnapshots := []string{}

	switch bindingType {
	case OnKubernetesEvent:
		for _, binding := range hc.kubernetesBindings {
			if bindingName == binding.BindingName {
				includeSnapshots = binding.IncludeSnapshotsFrom
				break
			}
		}
	case Schedule:
		for _, binding := range hc.scheduleBindings {
			if bindingName == binding.BindingName {
				includeSnapshots = binding.IncludeSnapshotsFrom
				break
			}
		}
	case KubernetesValidating:
		for _, binding := range hc.validatingBindings {
			if bindingName == binding.BindingName {
				includeSnapshots = binding.IncludeSnapshotsFrom
				break
			}
		}
	case KubernetesConversion:
		for _, binding := range hc.conversionBindings {
			if bindingName == binding.BindingName {
				includeSnapshots = binding.IncludeSnapshotsFrom
				break
			}
		}
	}

	return hc.KubernetesController.SnapshotsFrom(includeSnapshots...)
}

func (hc *hookController) UpdateSnapshots(context []BindingContext) []BindingContext {
	if hc.KubernetesController == nil {
		return context
	}

	newContext := []BindingContext{}
	for _, bc := range context {
		newBc := bc
		newBc.Snapshots = hc.KubernetesSnapshotsFor(bc.Metadata.BindingType, bc.Binding)
		newContext = append(newContext, newBc)
	}

	return newContext
}
