package context

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/hook"
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube/fake"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
)

type GeneratedBindingContexts struct {
	Rendered        string
	BindingContexts []BindingContext
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

	fakeCluster *fake.FakeCluster
}

func NewBindingContextController(config string) (*BindingContextController, error) {
	return &BindingContextController{
		HookMap:    make(map[string]string),
		HookConfig: config,

		Controller:    NewStateController(),
		UpdateTimeout: 1500 * time.Millisecond,

		fakeCluster: fake.NewFakeCluster(fake.ClusterVersionV119),
	}, nil
}

func (b *BindingContextController) WithHook(h *hook.Hook) {
	b.Hook = h
	b.HookConfig = ""
}

// RegisterCRD registers custom resources for the cluster
func (b *BindingContextController) RegisterCRD(group, version, kind string, namespaced bool) {
	b.fakeCluster.RegisterCRD(group, version, kind, namespaced)
}

// BindingContextsGenerator generates binding contexts for hook tests
func (b *BindingContextController) Run(initialState string) (GeneratedBindingContexts, error) {
	ctx := context.Background()

	b.KubeEventsManager = kubeeventsmanager.NewKubeEventsManager()
	b.KubeEventsManager.WithContext(ctx)
	b.KubeEventsManager.WithKubeClient(b.fakeCluster.KubeClient)

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

	cc := NewContextCombiner()
	err = b.HookCtrl.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		if info.KubernetesBinding.ExecuteHookOnSynchronization {
			cc.AddBindingContext(types.OnKubernetesEvent, info)
		}
	})
	if err != nil {
		return GeneratedBindingContexts{}, fmt.Errorf("couldn't enable kubernetes bindings: %v", err)
	}

	b.HookCtrl.UnlockKubernetesEvents()

	<-time.After(time.Millisecond) // tick to trigger informers
	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) ChangeState(newState ...string) (GeneratedBindingContexts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.UpdateTimeout)
	defer cancel()

	cc := NewContextCombiner()

	for _, state := range newState {
		generatedEvents, err := b.Controller.ChangeState(state)
		if err != nil {
			return GeneratedBindingContexts{}, fmt.Errorf("error while changing BindingContextGenerator state: %v", err)
		}

		for receivedEvents := 0; receivedEvents < generatedEvents; receivedEvents++ {
			select {
			case ev := <-b.KubeEventsManager.Ch():
				b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
					cc.AddBindingContext(types.OnKubernetesEvent, info)
				})
				continue
			case <-ctx.Done():
				break
			}
		}
	}
	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) ChangeStateAndWaitForBindingContexts(desiredQuantity int, newState string) (GeneratedBindingContexts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cc := NewContextCombiner()

	_, err := b.Controller.ChangeState(newState)
	if err != nil {
		return GeneratedBindingContexts{}, fmt.Errorf("error while changing BindingContextGenerator state: %v", err)
	}

	for cc.QueueLen() != desiredQuantity {
		select {
		case ev := <-b.KubeEventsManager.Ch():
			b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
				cc.AddBindingContext(types.OnKubernetesEvent, info)
			})
			continue
		case <-ctx.Done():
			return GeneratedBindingContexts{}, fmt.Errorf("timeout occurred while waiting for binding contexts")
		}
	}

	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) RunSchedule(crontab string) (GeneratedBindingContexts, error) {
	cc := NewContextCombiner()
	b.HookCtrl.HandleScheduleEvent(crontab, func(info controller.BindingExecutionInfo) {
		cc.AddBindingContext(types.Schedule, info)
	})
	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) RunBindingWithAllSnapshots(binding types.BindingType) (GeneratedBindingContexts, error) {
	bc := BindingContext{
		Binding:   string(binding),
		Snapshots: b.HookCtrl.KubernetesSnapshots(),
	}
	bc.Metadata.BindingType = binding
	bc.Metadata.IncludeAllSnapshots = true

	return ConvertToGeneratedBindingContexts([]BindingContext{bc})
}

func (b *BindingContextController) Stop() {
	if b.HookCtrl != nil {
		b.HookCtrl.StopMonitors()
	}
}
