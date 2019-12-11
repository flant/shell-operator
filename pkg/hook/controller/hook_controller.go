package controller

import (
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
)

type BindingExecutionInfo struct {
	BindingContext      []BindingContext
	IncludeSnapshots    []string
	IncludeAllSnapshots bool
	AllowFailure        bool
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

	CanHandleKubeEvent(kubeEvent KubeEvent) bool
	CanHandleScheduleEvent(crontab string) bool

	// These method should call underlying BindingController to get binding context
	// and then add Snapshots to binding context
	HandleEnableKubernetesBindings(createTasksFn func(BindingExecutionInfo)) error
	HandleKubeEvent(event KubeEvent, createTasksFn func(BindingExecutionInfo))
	HandleScheduleEvent(crontab string, createTasksFn func(BindingExecutionInfo))

	//WithKubernetesBindingsControllers([]*KubernetesBindingsController)
	//WithScheduleBindingsControllers([]*ScheduleBindingsController)

	StartMonitors()
	StopMonitors()

	EnableScheduleBindings()
	DisableScheduleBindings()

	KubernetesSnapshots() map[string][]ObjectAndFilterResult
}

var _ HookController = &hookController{}

func NewHookController() HookController {
	return &hookController{}
}

type hookController struct {
	KubernetesController KubernetesBindingsController
	ScheduleController   ScheduleBindingsController
}

func (hc *hookController) InitKubernetesBindings(bindings []OnKubernetesEventConfig, kubeEventMgr kube_events_manager.KubeEventsManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewKubernetesBindingsController()
	bindingCtrl.WithKubeEventsManager(kubeEventMgr)
	bindingCtrl.WithKubernetesBindings(bindings)
	hc.KubernetesController = bindingCtrl
}

func (hc *hookController) InitScheduleBindings(bindings []ScheduleConfig, scheduleMgr schedule_manager.ScheduleManager) {
	if len(bindings) == 0 {
		return
	}

	bindingCtrl := NewScheduleBindingsController()
	bindingCtrl.WithScheduleManager(scheduleMgr)
	bindingCtrl.WithScheduleBindings(bindings)
	hc.ScheduleController = bindingCtrl
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
			// Inject IncludeSnapshots to BindingContext
			if len(execInfo.BindingContext) > 0 && len(execInfo.IncludeSnapshots) > 0 {
				execInfo.BindingContext[0].Snapshots = hc.KubernetesController.Snapshot(execInfo.IncludeSnapshots...)
			}
			createTasksFn(execInfo)
		}
	}
	return
}

func (hc *hookController) HandleScheduleEvent(crontab string, createTasksFn func(BindingExecutionInfo)) {
	if hc.ScheduleController != nil {
		infos := hc.ScheduleController.HandleEvent(crontab)
		for _, info := range infos {
			if createTasksFn != nil {
				// Inject IncludeKubernetesSnapshots to BindingContext
				if hc.KubernetesController != nil && len(info.BindingContext) > 0 && len(info.IncludeSnapshots) > 0 {
					info.BindingContext[0].KubernetesSnapshots = hc.KubernetesController.Snapshot(info.IncludeSnapshots...)
				}
				createTasksFn(info)
			}
		}
	}
	return
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
	return
}

func (hc *hookController) DisableScheduleBindings() {
	if hc.ScheduleController != nil {
		hc.ScheduleController.DisableScheduleBindings()
	}
	return
}

// KubernetesSnapshots returns all exited objects for all registered kubernetes bindings.
func (hc *hookController) KubernetesSnapshots() map[string][]ObjectAndFilterResult {
	if hc.KubernetesController != nil {
		return hc.KubernetesController.Snapshots()
	}
	return map[string][]ObjectAndFilterResult{}
}