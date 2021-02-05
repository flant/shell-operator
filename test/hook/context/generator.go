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

type GeneratedBindingContexts struct {
	Rendered        string
	BindingContexts []BindingContext
}

// convertBindingContexts render json with array of binding contexts
func convertBindingContexts(bindingContexts []BindingContext) (GeneratedBindingContexts, error) {
	// Compact groups
	lastGroup := ""
	lastGroupIndex := 0
	compactedBindingContexts := make([]BindingContext, 0, len(bindingContexts))

	for _, bindingContext := range bindingContexts {
		if bindingContext.Metadata.Group == "" {
			lastGroup = ""
			compactedBindingContexts = append(compactedBindingContexts, bindingContext)
			continue
		}

		if lastGroup != bindingContext.Metadata.Group {
			compactedBindingContexts = append(compactedBindingContexts, bindingContext)
			lastGroup = bindingContext.Metadata.Group
			lastGroupIndex = len(compactedBindingContexts) - 1
			continue
		}

		compactedBindingContexts[lastGroupIndex] = bindingContext
	}

	res := GeneratedBindingContexts{}

	// Support only v1 binding contexts.
	bcList := ConvertBindingContextList("v1", compactedBindingContexts)
	data, err := bcList.Json()
	if err != nil {
		return res, fmt.Errorf("marshaling binding context error: %v", err)
	}

	res.BindingContexts = bindingContexts
	res.Rendered = string(data)
	return res, nil
}

type BindingContextController struct {
	Hook       *hook.Hook
	HookCtrl   controller.HookController
	HookMap    map[string]string
	HookConfig string

	Controller        *StateController
	KubeEventsManager kubeeventsmanager.KubeEventsManager
	ScheduleManager   schedulemanager.ScheduleManager

	UpdateTimeout time.Duration
}

func NewBindingContextController(config string) (*BindingContextController, error) {
	FakeCluster = fake.NewFakeCluster()

	return &BindingContextController{
		HookMap:    make(map[string]string),
		HookConfig: config,

		Controller:    NewStateController(),
		UpdateTimeout: 1500 * time.Millisecond,
	}, nil
}

func (b *BindingContextController) WithHook(h *hook.Hook) {
	b.Hook = h
	b.HookConfig = ""
}

// RegisterCRD registers custom resources for the cluster
func (b *BindingContextController) RegisterCRD(group, version, kind string, namespaced bool) {
	FakeCluster.RegisterCRD(group, version, kind, namespaced)
}

// BindingContextsGenerator generates binding contexts for hook tests
func (b *BindingContextController) Run(initialState string) (GeneratedBindingContexts, error) {
	ctx := context.Background()

	b.KubeEventsManager = kubeeventsmanager.NewKubeEventsManager()
	b.KubeEventsManager.WithContext(ctx)
	b.KubeEventsManager.WithKubeClient(FakeCluster.KubeClient)

	b.ScheduleManager = schedulemanager.NewScheduleManager()
	b.ScheduleManager.WithContext(ctx)

	// Use StateController to apply changes
	err := b.Controller.SetInitialState(initialState)
	if err != nil {
		return GeneratedBindingContexts{}, err
	}

	if b.Hook == nil {
		testHook := hook.NewHook("test", "test")
		testHook, err = testHook.LoadConfig([]byte(b.HookConfig))
		if err != nil {
			return GeneratedBindingContexts{}, fmt.Errorf("couldn't load or validate hook configuration: %v", err)
		}
		b.Hook = testHook
	}

	b.HookCtrl = controller.NewHookController()
	b.HookCtrl.InitKubernetesBindings(b.Hook.GetConfig().OnKubernetesEvents, b.KubeEventsManager)
	b.HookCtrl.InitScheduleBindings(b.Hook.GetConfig().Schedules, b.ScheduleManager)
	b.HookCtrl.EnableScheduleBindings()

	b.Hook.WithHookController(b.HookCtrl)

	bindingContexts := make([]BindingContext, 0)
	err = b.HookCtrl.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, info.BindingContext...)
	})
	if err != nil {
		return GeneratedBindingContexts{}, fmt.Errorf("couldn't enable kubernetes bindings: %v", err)
	}

	b.HookCtrl.StartMonitors()

	<-time.After(time.Millisecond) // tick to trigger informers
	bindingContexts = b.HookCtrl.UpdateSnapshots(bindingContexts)
	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) ChangeState(newState ...string) (GeneratedBindingContexts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.UpdateTimeout)
	defer cancel()

	bindingContexts := make([]BindingContext, 0)

	for _, state := range newState {
		generatedEvents, err := b.Controller.ChangeState(state)
		if err != nil {
			return GeneratedBindingContexts{}, fmt.Errorf("error while changing BindingContextGenerator state: %v", err)
		}

		for receivedEvents := 0; receivedEvents < generatedEvents; receivedEvents++ {
			select {
			case ev := <-b.KubeEventsManager.Ch():
				b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
					bindingContexts = append(bindingContexts, info.BindingContext...)
				})
				continue
			case <-ctx.Done():
				break
			}
		}
	}
	bindingContexts = b.HookCtrl.UpdateSnapshots(bindingContexts)
	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) ChangeStateAndWaitForBindingContexts(desiredQuantity int, newState string) (GeneratedBindingContexts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bindingContexts := make([]BindingContext, 0)

	_, err := b.Controller.ChangeState(newState)
	if err != nil {
		return GeneratedBindingContexts{}, fmt.Errorf("error while changing BindingContextGenerator state: %v", err)
	}

	for len(bindingContexts) != desiredQuantity {
		select {
		case ev := <-b.KubeEventsManager.Ch():
			b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
				bindingContexts = append(bindingContexts, info.BindingContext...)
			})
			continue
		case <-ctx.Done():
			return GeneratedBindingContexts{}, fmt.Errorf("timeout occurred while waiting for binding contexts")
		}
	}

	bindingContexts = b.HookCtrl.UpdateSnapshots(bindingContexts)
	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) RunSchedule(crontab string) (GeneratedBindingContexts, error) {
	bindingContexts := make([]BindingContext, 0)

	b.HookCtrl.HandleScheduleEvent(crontab, func(info controller.BindingExecutionInfo) {
		bindingContexts = append(bindingContexts, b.HookCtrl.UpdateSnapshots(info.BindingContext)...)
	})
	return convertBindingContexts(bindingContexts)
}

func (b *BindingContextController) Stop() {
	if b.HookCtrl != nil {
		b.HookCtrl.StopMonitors()
	}
}
