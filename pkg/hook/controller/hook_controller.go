package controller

import (
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	. "github.com/flant/shell-operator/pkg/webhook/admission/types"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

type BindingExecutionInfo struct {
	BindingContext      []BindingContext
	IncludeSnapshots    []string
	IncludeAllSnapshots bool
	AllowFailure        bool
	QueueName           string
	Binding             string
	Group               string
	KubernetesBinding   OnKubernetesEventConfig
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
	InitAdmissionBindings([]ValidatingConfig, []MutatingConfig, *admission.WebhookManager)
	InitConversionBindings([]ConversionConfig, *conversion.WebhookManager)

	CanHandleKubeEvent(kubeEvent KubeEvent) bool
	CanHandleScheduleEvent(crontab string) bool
	CanHandleAdmissionEvent(event AdmissionEvent) bool
	CanHandleConversionEvent(event conversion.Event, rule conversion.Rule) bool

	// These method should call an underlying *Binding*Controller to get binding context
	// and then add Snapshots to binding context
	HandleEnableKubernetesBindings(createTasksFn func(BindingExecutionInfo)) error
	HandleKubeEvent(event KubeEvent, createTasksFn func(BindingExecutionInfo))
	HandleScheduleEvent(crontab string, createTasksFn func(BindingExecutionInfo))
	HandleAdmissionEvent(event AdmissionEvent, createTasksFn func(BindingExecutionInfo))
	HandleConversionEvent(event conversion.Event, rule conversion.Rule, createTasksFn func(BindingExecutionInfo))

	UnlockKubernetesEvents()
	UnlockKubernetesEventsFor(monitorID string)
	StopMonitors()
	UpdateMonitor(monitorId string, kind, apiVersion string) error

	EnableScheduleBindings()
	DisableScheduleBindings()

	EnableAdmissionBindings()

	EnableConversionBindings()

	KubernetesSnapshots() map[string][]ObjectAndFilterResult
	UpdateSnapshots([]BindingContext) []BindingContext
	SnapshotsInfo() []string
	SnapshotsDump() map[string]interface{}
}

var _ HookController = &hookController{}

func NewHookController() HookController {
	return &hookController{}
}

type hookController struct {
	KubernetesController KubernetesBindingsController
	ScheduleController   ScheduleBindingsController
	AdmissionController  AdmissionBindingsController
	ConversionController ConversionBindingsController
	kubernetesBindings   []OnKubernetesEventConfig
	scheduleBindings     []ScheduleConfig
	validatingBindings   []ValidatingConfig
	mutatingBindings     []MutatingConfig
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

func (hc *hookController) InitAdmissionBindings(vbindings []ValidatingConfig, mbindings []MutatingConfig, webhookMgr *admission.WebhookManager) {
	bindingCtrl := NewValidatingBindingsController()
	bindingCtrl.WithWebhookManager(webhookMgr)
	hc.AdmissionController = bindingCtrl

	hc.initValidatingBindings(vbindings)
	hc.initMutatingBindings(mbindings)
}

func (hc *hookController) initValidatingBindings(bindings []ValidatingConfig) {
	if len(bindings) == 0 {
		return
	}

	hc.AdmissionController.WithValidatingBindings(bindings)
	hc.validatingBindings = bindings
}

func (hc *hookController) initMutatingBindings(bindings []MutatingConfig) {
	if len(bindings) == 0 {
		return
	}

	hc.AdmissionController.WithMutatingBindings(bindings)
	hc.mutatingBindings = bindings
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

func (hc *hookController) CanHandleAdmissionEvent(event AdmissionEvent) bool {
	if hc.AdmissionController != nil {
		return hc.AdmissionController.CanHandleEvent(event)
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

func (hc *hookController) HandleAdmissionEvent(event AdmissionEvent, createTasksFn func(BindingExecutionInfo)) {
	if hc.AdmissionController == nil {
		return
	}
	execInfo := hc.AdmissionController.HandleEvent(event)
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

func (hc *hookController) UnlockKubernetesEvents() {
	if hc.KubernetesController != nil {
		hc.KubernetesController.UnlockEvents()
	}
}

func (hc *hookController) UnlockKubernetesEventsFor(monitorID string) {
	if hc.KubernetesController != nil {
		hc.KubernetesController.UnlockEventsFor(monitorID)
	}
}

func (hc *hookController) StopMonitors() {
	if hc.KubernetesController != nil {
		hc.KubernetesController.StopMonitors()
	}
}

func (hc *hookController) UpdateMonitor(monitorId string, kind, apiVersion string) error {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.UpdateMonitor(monitorId, kind, apiVersion)
	}
	return nil
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

func (hc *hookController) EnableAdmissionBindings() {
	if hc.AdmissionController != nil {
		hc.AdmissionController.EnableValidatingBindings()
		hc.AdmissionController.EnableMutatingBindings()
	}
}

func (hc *hookController) EnableConversionBindings() {
	if hc.ConversionController != nil {
		hc.ConversionController.EnableConversionBindings()
	}
}

// KubernetesSnapshots returns a 'full snapshot': all snapshots for all registered kubernetes bindings.
// Note: no caching as in UpdateSnapshots because KubernetesSnapshots used for non-combined binding contexts.
func (hc *hookController) KubernetesSnapshots() map[string][]ObjectAndFilterResult {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.Snapshots()
	}
	return map[string][]ObjectAndFilterResult{}
}

// getIncludeSnapshotsFrom returns binding names from 'includeSnapshotsFrom' field.
func (hc *hookController) getIncludeSnapshotsFrom(bindingType BindingType, bindingName string) []string {
	includeSnapshotsFrom := []string{}

	switch bindingType {
	case OnKubernetesEvent:
		for _, binding := range hc.kubernetesBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case Schedule:
		for _, binding := range hc.scheduleBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case KubernetesValidating:
		for _, binding := range hc.validatingBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case KubernetesConversion:
		for _, binding := range hc.conversionBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	}

	return includeSnapshotsFrom
}

// UpdateSnapshots ensures fresh consistent snapshots for combined binding contexts.
//
// It uses caching to retrieve snapshots for a particular binding name only once.
// This caching is important for Synchronization and self-includes:
// Combined "Synchronization" binging contexts or "Synchronization"
// with self-inclusion may require several calls to Snapshot*() methods, but objects
// may change between these calls.
func (hc *hookController) UpdateSnapshots(context []BindingContext) []BindingContext {
	if hc.KubernetesController == nil {
		return context
	}

	// Cache retrieved snapshots to make them consistent.
	cache := make(map[string][]ObjectAndFilterResult)

	newContext := []BindingContext{}
	for _, bc := range context {
		newBc := bc

		// Update 'snapshots' field to fresh snapshot based on 'includeSnapshotsFrom' field.
		// Note: it is a cache-enabled version of KubernetesController.SnapshotsFrom.
		newBc.Snapshots = make(map[string][]ObjectAndFilterResult)
		includeSnapshotsFrom := hc.getIncludeSnapshotsFrom(bc.Metadata.BindingType, bc.Binding)
		for _, bindingName := range includeSnapshotsFrom {
			// Initialize all keys with empty arrays.
			newBc.Snapshots[bindingName] = make([]ObjectAndFilterResult, 0)
			if _, has := cache[bindingName]; !has {
				cache[bindingName] = hc.KubernetesController.SnapshotsFor(bindingName)
			}
			if cache[bindingName] != nil {
				newBc.Snapshots[bindingName] = cache[bindingName]
			}
		}

		// Also refresh 'objects' field for Kubernetes.Synchronization event.
		if newBc.Metadata.BindingType == OnKubernetesEvent && newBc.Type == TypeSynchronization {
			if _, has := cache[bc.Binding]; !has {
				cache[bc.Binding] = hc.KubernetesController.SnapshotsFor(bc.Binding)
			}
			newBc.Objects = cache[bc.Binding]
		}

		newContext = append(newContext, newBc)
	}

	return newContext
}

func (hc *hookController) SnapshotsInfo() []string {
	if hc.KubernetesController == nil {
		return []string{"no kubernetes bindings for hook"}
	}

	return hc.KubernetesController.SnapshotsInfo()
}

func (hc *hookController) SnapshotsDump() map[string]interface{} {
	if hc.KubernetesController == nil {
		return nil
	}

	return hc.KubernetesController.SnapshotsDump()
}
