package shell_operator

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"

	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

type managerEventsHandlerConfig struct {
	tqs  *queue.TaskQueueSet
	mgr  kubeeventsmanager.KubeEventsManager
	smgr schedulemanager.ScheduleManager

	logger *log.Logger
}

type ManagerEventsHandler struct {
	ctx    context.Context
	cancel context.CancelFunc

	kubeEventsManager kubeeventsmanager.KubeEventsManager
	scheduleManager   schedulemanager.ScheduleManager

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
			logEntry := m.logger.With("operator.component", "handleEvents")

			ctx := context.Background()

			select {
			case crontab := <-m.scheduleManager.Ch():
				if m.scheduleCb != nil {
					tailTasks = m.scheduleCb(ctx, crontab)
				}

			case kubeEvent := <-m.kubeEventsManager.Ch():
				if m.kubeEventCb != nil {
					tailTasks = m.kubeEventCb(ctx, kubeEvent)
				}

			case <-m.ctx.Done():
				logEntry.Info("Stop")
				return
			}

			m.taskQueues.DoWithLock(func(tqs *queue.TaskQueueSet) {
				for _, resTask := range tailTasks {
					if q, ok := tqs.Queues.Get(resTask.GetQueueName()); !ok {
						log.Error("Possible bug!!! Got task for queue but queue is not created yet.",
							slog.String("queueName", resTask.GetQueueName()),
							slog.String("description", resTask.GetDescription()))
					} else {
						q.AddLast(resTask)
					}
				}
			})
		}
	}()
}
