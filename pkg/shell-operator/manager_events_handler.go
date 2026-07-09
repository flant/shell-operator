package shell_operator

import (
	"context"
	"log/slog"
	"runtime/debug"

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

// Start runs events handler. This function is used in addon-operator
func (m *ManagerEventsHandler) Start() {
	go func() {
		for {
			var tailTasks []task.Task
			logEntry := m.logger.With(pkg.LogKeyOperatorComponent, "handleEvents")

			ctx := context.Background()

			select {
			case crontab := <-m.scheduleManager.Ch():
				if m.scheduleCb != nil {
					tailTasks = runEventCb(logEntry, "schedule", func() []task.Task {
						return m.scheduleCb(ctx, crontab)
					})
				}

			case kubeEvent := <-m.kubeEventsManager.Ch():
				if m.kubeEventCb != nil {
					tailTasks = runEventCb(logEntry, "kubernetes", func() []task.Task {
						return m.kubeEventCb(ctx, kubeEvent)
					})
				}

			case <-m.ctx.Done():
				logEntry.Info("Stop")
				return
			}

			m.taskQueues.AddTailTasks(tailTasks...)
		}
	}()
}

// runEventCb invokes an event callback with panic isolation, mirroring the
// recover in TaskQueue.processOne: a panicking hook dispatch must not kill
// the events goroutine and with it the whole process. On panic the event's
// tail tasks are dropped and nil is returned.
func runEventCb(logEntry *log.Logger, binding string, cb func() []task.Task) []task.Task {
	defer func() {
		if r := recover(); r != nil {
			logEntry.Warn(
				"panic recovered in ManagerEventsHandler",
				slog.String(pkg.LogKeyBinding, binding),
				slog.Any(pkg.LogKeyError, r),
				slog.String(pkg.LogKeyStack, string(debug.Stack())),
			)
		}
	}()

	return cb()
}
