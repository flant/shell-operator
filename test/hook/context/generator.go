package context

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/hook"
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/kube/fake"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
)

// FakeCluster is global for now. It can be encapsulated in BindingContextController lately.
var FakeCluster *fake.FakeCluster

// convertBindingContexts render json with array of binding contexts
func convertBindingContexts(bindingContexts []BindingContext) (string, error) {
	// Only v1 binding contexts supported by now
	bcList := ConvertBindingContextList("v1", bindingContexts)
	data, err := bcList.Json()
	if err != nil {
		return "", fmt.Errorf("marshaling binding context error: %v", err)
	}
	return string(data), nil
}

type BindingContextController struct {
	HookCtrl          controller.HookController
	HookMap           map[string]string
	HookConfig        string
	InitialState      string
	Controller        *StateController
	KubeEventsManager kubeeventsmanager.KubeEventsManager
	ScheduleManager   schedulemanager.ScheduleManager
	Context           context.Context
	Cancel            context.CancelFunc
	Timeout           time.Duration
}

func NewBindingContextController(config, initialState string) (*BindingContextController, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	FakeCluster = fake.NewFakeCluster()

	return &BindingContextController{
		HookMap:      make(map[string]string),
		HookConfig:   config,
		InitialState: initialState,
		Controller:   NewStateController(),
		Context:      ctx,
		Cancel:       cancel,
		Timeout:      1500 * time.Millisecond,
	}, nil
}

// RegisterCRD registers custom resources for the cluster
func (b *BindingContextController) RegisterCRD(group, version, kind string, namespaced bool) {
	FakeCluster.RegisterCRD(group, version, kind, namespaced)
}

// BindingContextsGenerator generates binding contexts for hook tests
func (b *BindingContextController) Run() (string, error) {
	b.KubeEventsManager = kubeeventsmanager.NewKubeEventsManager()
	b.KubeEventsManager.WithContext(b.Context)
	b.KubeEventsManager.WithKubeClient(FakeCluster.KubeClient)

	b.ScheduleManager = schedulemanager.NewScheduleManager()
	b.ScheduleManager.WithContext(b.Context)

	// Use StateController to apply changes
	err := b.Controller.SetInitialState(b.InitialState)
	if err != nil {
		return "", err
	}

	testHook := hook.NewHook("test", "test")
	testHook, err = testHook.WithConfig([]byte(b.HookConfig))
	if err != nil {
		return "", fmt.Errorf("couldn't load or validate hook configuration: %v", err)
	}

	b.HookCtrl = controller.NewHookController()
	b.HookCtrl.InitKubernetesBindings(testHook.GetConfig().OnKubernetesEvents, b.KubeEventsManager)
	b.HookCtrl.InitScheduleBindings(testHook.GetConfig().Schedules, b.ScheduleManager)
	b.HookCtrl.EnableScheduleBindings()

	testHook.WithHookController(b.HookCtrl)

	bindingContexts := make([]BindingContext, 0)
	err = b.HookCtrl.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, b.HookCtrl.UpdateSnapshots(info.BindingContext)...)
	})
	if err != nil {
		return "", fmt.Errorf("couldn't enable kubernetes bindings: %v", err)
	}
	b.HookCtrl.StartMonitors()

	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) ChangeState(newState ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.Timeout)
	defer cancel()

	bindingContexts := make([]BindingContext, 0)

	for _, state := range newState {
		generatedEvents, err := b.Controller.ChangeState(state)
		if err != nil {
			return "", fmt.Errorf("error while changing BindingContextGenerator state: %v", err)
		}

		for receivedEvents := 0; receivedEvents < generatedEvents; receivedEvents++ {
			select {
			case ev := <-b.KubeEventsManager.Ch():
				b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
					bindingContexts = append(bindingContexts, info.BindingContext...)
				})
				continue
			case <-ctx.Done():
				return convertBindingContexts(bindingContexts)
			}
		}
	}
	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) RunSchedule(crontab string) (string, error) {
	bindingContexts := make([]BindingContext, 0)

	b.HookCtrl.HandleScheduleEvent(crontab, func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, b.HookCtrl.UpdateSnapshots(info.BindingContext)...)
	})
	return convertBindingContexts(bindingContexts)
}
