package shell_operator

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/pkg/log"

	pkg "github.com/flant/shell-operator/pkg"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

type managerEventsHandlerConfig struct {
	tqs  *queue.TaskQueueSet
	mgr  kubeeventsmanager.KubeEventEmitter
	smgr schedulemanager.ScheduleEmitter

	logger *log.Logger
}

type ManagerEventsHandler struct {
	ctx    context.Context
	cancel context.CancelFunc

	kubeEventsManager kubeeventsmanager.KubeEventEmitter
	scheduleManager   schedulemanager.ScheduleEmitter

	kubeEventCb func(ctx context.Context, kubeEvent kemtypes.KubeEvent) []task.Task
	scheduleCb  func(ctx context.Context, crontab string) []task.Task

	taskQueues *queue.TaskQueueSet

	startOnce sync.Once
	started   atomic.Bool
	doneCh    chan struct{}

	logger *log.Logger
}

func newManagerEventsHandler(ctx context.Context, cfg *managerEventsHandlerConfig) *ManagerEventsHandler {
	cctx, cancel := context.WithCancel(ctx)

	return &ManagerEventsHandler{
		ctx:               cctx,
		cancel:            cancel,
		scheduleManager:   cfg.smgr,
		kubeEventsManager: cfg.mgr,
		taskQueues:        cfg.tqs,
		doneCh:            make(chan struct{}),
		logger:            cfg.logger,
	}
}

// WithKubeEventHandler sets custom function for event handling.
// This function is used inside addon-operator.
func (m *ManagerEventsHandler) WithKubeEventHandler(fn func(ctx context.Context, kubeEvent kemtypes.KubeEvent) []task.Task) {
	m.kubeEventCb = fn
}

// WithScheduleEventHandler sets custom scheduler function.
// This function is used inside addon-operator.
func (m *ManagerEventsHandler) WithScheduleEventHandler(fn func(ctx context.Context, crontab string) []task.Task) {
	m.scheduleCb = fn
}

// Start runs the events handler. Calling Start a second time is a no-op;
// the original loop keeps running until Stop is called.
func (m *ManagerEventsHandler) Start() {
	m.startOnce.Do(func() {
		m.started.Store(true)
		go func() {
			defer close(m.doneCh)
			logEntry := m.logger.With(pkg.LogKeyOperatorComponent, "handleEvents")

			for {
				var tailTasks []task.Task

				select {
				case crontab := <-m.scheduleManager.Ch():
					if m.scheduleCb != nil {
						tailTasks = m.scheduleCb(m.ctx, crontab)
					}

				case kubeEvent := <-m.kubeEventsManager.Ch():
					if m.kubeEventCb != nil {
						tailTasks = m.kubeEventCb(m.ctx, kubeEvent)
					}

				case <-m.ctx.Done():
					logEntry.Info("Stop")
					return
				}

				m.taskQueues.AddTailTasks(tailTasks...)
			}
		}()
	})
}

// Stop signals the events handler loop to exit. Stop is non-blocking; pair it
// with Wait when you need a synchronous teardown.
func (m *ManagerEventsHandler) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Wait blocks until the events handler loop has exited or ctx is canceled.
// When Start was never called, Wait returns immediately.
func (m *ManagerEventsHandler) Wait(ctx context.Context) error {
	if !m.started.Load() {
		return nil
	}
	select {
	case <-m.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
