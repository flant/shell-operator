package controller

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

type BindingExecutionInfo struct {
	BindingContext      []bctx.BindingContext
	IncludeSnapshots    []string
	IncludeAllSnapshots bool
	AllowFailure        bool
	QueueName           string
	Binding             string
	Group               string
	KubernetesBinding   htypes.OnKubernetesEventConfig
}

// В каждый хук надо будет положить этот объект.
// Предварительно позвав With*Bindings и InitBindingsControllers

// Для kube надо будет сделать HandleEnableKubernetesBindings, чтобы получить списки существующих обхектов
// и потом StartMonitors

// Всё Handle* методы принимают callback, чтобы создавать задания независимо.

// методом KubernetesSnapshots можно достать все кубовые объекты, чтобы добавить
// их в какой-то свой binding context

func NewHookController() *HookController {
	return &HookController{}
}

type HookController struct {
	KubernetesController *KubernetesBindingsController
	ScheduleController   *ScheduleBindingsController
	AdmissionController  *AdmissionBindingsController
	ConversionController *ConversionBindingsController
	kubernetesBindings   []htypes.OnKubernetesEventConfig
	scheduleBindings     []htypes.ScheduleConfig
	validatingBindings   []htypes.ValidatingConfig
	mutatingBindings     []htypes.MutatingConfig
	conversionBindings   []htypes.ConversionConfig

	logger *log.Logger
}

func (hc *HookController) InitKubernetesBindings(bindings []htypes.OnKubernetesEventConfig, kubeEventMgr *kubeeventsmanager.KubeEventsManager, logger *log.Logger) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewKubernetesBindingsController(logger)
	bindingCtrl.WithKubeEventsManager(kubeEventMgr)
	bindingCtrl.WithKubernetesBindings(bindings)
	hc.KubernetesController = bindingCtrl
	hc.kubernetesBindings = bindings
	hc.logger = logger
}

func (hc *HookController) InitScheduleBindings(bindings []htypes.ScheduleConfig, scheduleMgr *schedulemanager.ScheduleManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewScheduleBindingsController()
	bindingCtrl.WithScheduleManager(scheduleMgr)
	bindingCtrl.WithScheduleBindings(bindings)
	hc.ScheduleController = bindingCtrl
	hc.scheduleBindings = bindings
}

func (hc *HookController) InitAdmissionBindings(vbindings []htypes.ValidatingConfig, mbindings []htypes.MutatingConfig, webhookMgr *admission.WebhookManager) {
	bindingCtrl := NewValidatingBindingsController()
	bindingCtrl.WithWebhookManager(webhookMgr)
	hc.AdmissionController = bindingCtrl

	hc.initValidatingBindings(vbindings)
	hc.initMutatingBindings(mbindings)
}

func (hc *HookController) initValidatingBindings(bindings []htypes.ValidatingConfig) {
	if len(bindings) == 0 {
		return
	}

	hc.AdmissionController.WithValidatingBindings(bindings)
	hc.validatingBindings = bindings
}

func (hc *HookController) initMutatingBindings(bindings []htypes.MutatingConfig) {
	if len(bindings) == 0 {
		return
	}

	hc.AdmissionController.WithMutatingBindings(bindings)
	hc.mutatingBindings = bindings
}

func (hc *HookController) InitConversionBindings(bindings []htypes.ConversionConfig, webhookMgr *conversion.WebhookManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewConversionBindingsController()
	bindingCtrl.WithWebhookManager(webhookMgr)
	bindingCtrl.WithBindings(bindings)
	hc.ConversionController = bindingCtrl
	hc.conversionBindings = bindings
}

func (hc *HookController) CanHandleKubeEvent(kubeEvent kemtypes.KubeEvent) bool {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.CanHandleEvent(kubeEvent)
	}
	return false
}

func (hc *HookController) CanHandleScheduleEvent(crontab string) bool {
	if hc.ScheduleController != nil {
		return hc.ScheduleController.CanHandleEvent(crontab)
	}
	return false
}

func (hc *HookController) CanHandleAdmissionEvent(event admission.Event) bool {
	if hc.AdmissionController != nil {
		return hc.AdmissionController.CanHandleEvent(event)
	}
	return false
}

func (hc *HookController) CanHandleConversionEvent(crdName string, event *v1.ConversionRequest, rule conversion.Rule) bool {
	if hc.ConversionController != nil {
		return hc.ConversionController.CanHandleEvent(crdName, event, rule)
	}
	return false
}

func (hc *HookController) HandleEnableKubernetesBindings(createTasksFn func(BindingExecutionInfo)) error {
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

func (hc *HookController) HandleKubeEvent(event kemtypes.KubeEvent, createTasksFn func(BindingExecutionInfo)) {
	if hc.KubernetesController != nil {
		execInfo := hc.KubernetesController.HandleEvent(event)
		if createTasksFn != nil {
			createTasksFn(execInfo)
		}
	}
}

func (hc *HookController) HandleAdmissionEvent(event admission.Event, createTasksFn func(BindingExecutionInfo)) {
	if hc.AdmissionController == nil {
		return
	}
	execInfo := hc.AdmissionController.HandleEvent(event)
	if createTasksFn != nil {
		createTasksFn(execInfo)
	}
}

func (hc *HookController) HandleConversionEvent(crdName string, request *v1.ConversionRequest, rule conversion.Rule, createTasksFn func(BindingExecutionInfo)) {
	if hc.ConversionController == nil {
		return
	}
	execInfo := hc.ConversionController.HandleEvent(crdName, request, rule)
	if createTasksFn != nil {
		createTasksFn(execInfo)
	}
}

func (hc *HookController) HandleScheduleEvent(crontab string, createTasksFn func(BindingExecutionInfo)) {
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

func (hc *HookController) UnlockKubernetesEvents() {
	if hc.KubernetesController != nil {
		hc.KubernetesController.UnlockEvents()
	}
}

func (hc *HookController) UnlockKubernetesEventsFor(monitorID string) {
	if hc.KubernetesController != nil {
		hc.KubernetesController.UnlockEventsFor(monitorID)
	}
}

func (hc *HookController) StopMonitors() {
	if hc.KubernetesController != nil {
		hc.KubernetesController.StopMonitors()
	}
}

func (hc *HookController) UpdateMonitor(monitorId string, kind, apiVersion string) error {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.UpdateMonitor(monitorId, kind, apiVersion)
	}
	return nil
}

func (hc *HookController) EnableScheduleBindings() {
	if hc.ScheduleController != nil {
		hc.ScheduleController.EnableScheduleBindings()
	}
}

func (hc *HookController) DisableScheduleBindings() {
	if hc.ScheduleController != nil {
		hc.ScheduleController.DisableScheduleBindings()
	}
}

func (hc *HookController) EnableAdmissionBindings() {
	if hc.AdmissionController != nil {
		hc.AdmissionController.EnableValidatingBindings()
		hc.AdmissionController.EnableMutatingBindings()
	}
}

func (hc *HookController) EnableConversionBindings() {
	if hc.ConversionController != nil {
		hc.ConversionController.EnableConversionBindings()
	}
}

// KubernetesSnapshots returns a 'full snapshot': all snapshots for all registered kubernetes bindings.
// Note: no caching as in UpdateSnapshots because KubernetesSnapshots used for non-combined binding contexts.
func (hc *HookController) KubernetesSnapshots() map[string][]kemtypes.ObjectAndFilterResult {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.Snapshots()
	}
	return map[string][]kemtypes.ObjectAndFilterResult{}
}

// getIncludeSnapshotsFrom returns binding names from 'includeSnapshotsFrom' field.
func (hc *HookController) getIncludeSnapshotsFrom(bindingType htypes.BindingType, bindingName string) []string {
	includeSnapshotsFrom := make([]string, 0)

	switch bindingType {
	case htypes.OnKubernetesEvent:
		for _, binding := range hc.kubernetesBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case htypes.Schedule:
		for _, binding := range hc.scheduleBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case htypes.KubernetesValidating:
		for _, binding := range hc.validatingBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case htypes.KubernetesMutating:
		for _, binding := range hc.mutatingBindings {
			if bindingName == binding.BindingName {
				includeSnapshotsFrom = binding.IncludeSnapshotsFrom
				break
			}
		}
	case htypes.KubernetesConversion:
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
func (hc *HookController) UpdateSnapshots(context []bctx.BindingContext) []bctx.BindingContext {
	if hc.KubernetesController == nil {
		return context
	}

	// Cache retrieved snapshots to make them consistent.
	cache := make(map[string][]kemtypes.ObjectAndFilterResult)

	newContext := make([]bctx.BindingContext, 0)
	for _, bc := range context {
		newBc := bc

		// Update 'snapshots' field to fresh snapshot based on 'includeSnapshotsFrom' field.
		// Note: it is a cache-enabled version of KubernetesController.SnapshotsFrom.
		newBc.Snapshots = make(map[string][]kemtypes.ObjectAndFilterResult)
		includeSnapshotsFrom := hc.getIncludeSnapshotsFrom(bc.Metadata.BindingType, bc.Binding)
		for _, bindingName := range includeSnapshotsFrom {
			// Initialize all keys with empty arrays.
			newBc.Snapshots[bindingName] = make([]kemtypes.ObjectAndFilterResult, 0)
			if _, has := cache[bindingName]; !has {
				cache[bindingName] = hc.KubernetesController.SnapshotsFor(bindingName)
			}
			if cache[bindingName] != nil {
				newBc.Snapshots[bindingName] = cache[bindingName]
			}
		}

		// Also refresh 'objects' field for Kubernetes.Synchronization event.
		if newBc.Metadata.BindingType == htypes.OnKubernetesEvent && newBc.Type == kemtypes.TypeSynchronization {
			if _, has := cache[bc.Binding]; !has {
				cache[bc.Binding] = hc.KubernetesController.SnapshotsFor(bc.Binding)
			}
			newBc.Objects = cache[bc.Binding]
		}

		newContext = append(newContext, newBc)
	}

	return newContext
}

func (hc *HookController) SnapshotsInfo() []string {
	if hc.KubernetesController == nil {
		return []string{"no kubernetes bindings for hook"}
	}

	return hc.KubernetesController.SnapshotsInfo()
}

func (hc *HookController) SnapshotsDump() map[string]interface{} {
	if hc.KubernetesController == nil {
		return nil
	}

	return hc.KubernetesController.SnapshotsDump()
}
