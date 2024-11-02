package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/kube-client/fake"
	"github.com/flant/shell-operator/pkg/hook"
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
)

func init() {
	kubeeventsmanager.DefaultSyncTime = time.Microsecond
}

type GeneratedBindingContexts struct {
	Rendered        string
	BindingContexts []BindingContext
}

type BindingContextController struct {
	Hook       *hook.Hook
	HookCtrl   *controller.HookController
	HookMap    map[string]string
	HookConfig string

	Controller        *StateController
	KubeEventsManager kubeeventsmanager.KubeEventsManager
	ScheduleManager   schedulemanager.ScheduleManager

	fakeCluster *fake.Cluster

	mu      sync.Mutex
	started bool

	logger *log.Logger
}

func NewBindingContextController(config string, logger *log.Logger, version ...fake.ClusterVersion) *BindingContextController {
	log.SetDefaultLevel(log.LevelError)

	k8sVersion := fake.ClusterVersionV119
	if len(version) > 0 {
		k8sVersion = version[0]
	}

	fc := fake.NewFakeCluster(k8sVersion)
	ctx := context.Background()

	b := &BindingContextController{
		HookMap:     make(map[string]string),
		HookConfig:  config,
		fakeCluster: fc,
		logger:      logger,
	}

	b.KubeEventsManager = kubeeventsmanager.NewKubeEventsManager(ctx, b.fakeCluster.Client, b.logger.Named("kube-events-manager"))
	// Re-create factory to drop informers created using different b.fakeCluster.Client.
	kubeeventsmanager.DefaultFactoryStore = kubeeventsmanager.NewFactoryStore()

	b.ScheduleManager = schedulemanager.NewScheduleManager(ctx, b.logger.Named("schedule-manager"))

	b.Controller = NewStateController(fc, b.KubeEventsManager)

	return b
}

func (b *BindingContextController) WithHook(h *hook.Hook) {
	b.Hook = h
	b.HookConfig = ""
}

func (b *BindingContextController) FakeCluster() *fake.Cluster {
	return b.fakeCluster
}

// RegisterCRD registers custom resources for the cluster
func (b *BindingContextController) RegisterCRD(group, version, kind string, namespaced bool) {
	b.fakeCluster.RegisterCRD(group, version, kind, namespaced)
}

// Run generates binding contexts for hook tests
func (b *BindingContextController) Run(initialState string) (GeneratedBindingContexts, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return GeneratedBindingContexts{}, fmt.Errorf("attempt to runner started runner, it cannot be started twice")
	}

	err := b.Controller.SetInitialState(initialState)
	if err != nil {
		return GeneratedBindingContexts{}, err
	}

	if b.Hook == nil {
		testHook := hook.NewHook("test", "test", b.logger.Named("hook"))
		testHook, err = testHook.LoadConfig([]byte(b.HookConfig))
		if err != nil {
			return GeneratedBindingContexts{}, fmt.Errorf("couldn't load or validate hook configuration: %v", err)
		}
		b.Hook = testHook
	}

	b.HookCtrl = controller.NewHookController()
	b.HookCtrl.InitKubernetesBindings(b.Hook.GetConfig().OnKubernetesEvents, b.KubeEventsManager, b.logger.Named("kubernetes-bindings"))
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
	b.started = true

	time.Sleep(50 * time.Millisecond)
	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) ChangeState(newState string) (GeneratedBindingContexts, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	cc := NewContextCombiner()

	if err := b.Controller.ChangeState(newState); err != nil {
		return GeneratedBindingContexts{}, fmt.Errorf("error while changing BindingContextGenerator state: %v", err)
	}

outer:
	for {
		select {
		case ev := <-b.KubeEventsManager.Ch():
			switch ev.MonitorId {
			case "STOP_EVENTS":
				break outer
			default:
				b.HookCtrl.HandleKubeEvent(ev, func(info controller.BindingExecutionInfo) {
					cc.AddBindingContext(types.OnKubernetesEvent, info)
				})
			}
		case <-ctx.Done():
			return GeneratedBindingContexts{}, fmt.Errorf("timeout occurred while waiting for binding contexts")
		}
	}
	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) RunSchedule(crontab string) (GeneratedBindingContexts, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cc := NewContextCombiner()
	b.HookCtrl.HandleScheduleEvent(crontab, func(info controller.BindingExecutionInfo) {
		cc.AddBindingContext(types.Schedule, info)
	})
	return cc.CombinedAndUpdated(b.HookCtrl)
}

func (b *BindingContextController) RunBindingWithAllSnapshots(binding types.BindingType) (GeneratedBindingContexts, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

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
