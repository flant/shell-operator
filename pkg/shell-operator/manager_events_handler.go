package shell_operator

import (
	"context"

	"github.com/deckhouse/deckhouse/go_lib/log"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

type managerEventsHandlerConfig struct {
	tqs  *queue.TaskQueueSet
	mgr  kube_events_manager.KubeEventsManager
	smgr schedule_manager.ScheduleManager

	logger *log.Logger
}

type ManagerEventsHandler struct {
	ctx    context.Context
	cancel context.CancelFunc

	kubeEventsManager kube_events_manager.KubeEventsManager
	scheduleManager   schedule_manager.ScheduleManager

	kubeEventCb func(kubeEvent KubeEvent) []task.Task
	scheduleCb  func(crontab string) []task.Task

	taskQueues *queue.TaskQueueSet

	logger *log.Logger
}

func newManagerEventsHandler(ctx context.Context, cfg *managerEventsHandlerConfig) *ManagerEventsHandler {
	if ctx == nil {
		ctx = context.Background()
	}
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
func (m *ManagerEventsHandler) WithKubeEventHandler(fn func(kubeEvent KubeEvent) []task.Task) {
	m.kubeEventCb = fn
}

// WithScheduleEventHandler sets custom scheduler function.
// This function is used inside addon-operator.
func (m *ManagerEventsHandler) WithScheduleEventHandler(fn func(crontab string) []task.Task) {
	m.scheduleCb = fn
}

// Start runs events handler. This function is used in addon-operator
func (m *ManagerEventsHandler) Start() {
	go func() {
		for {
			var tailTasks []task.Task
			logEntry := m.logger.With("operator.component", "handleEvents")
			select {
			case crontab := <-m.scheduleManager.Ch():
				if m.scheduleCb != nil {
					tailTasks = m.scheduleCb(crontab)
				}

			case kubeEvent := <-m.kubeEventsManager.Ch():
				if m.kubeEventCb != nil {
					tailTasks = m.kubeEventCb(kubeEvent)
				}

			case <-m.ctx.Done():
				logEntry.Infof("Stop")
				return
			}

			m.taskQueues.DoWithLock(func(tqs *queue.TaskQueueSet) {
				for _, resTask := range tailTasks {
					q := tqs.GetByName(resTask.GetQueueName())
					if q == nil {
						log.Errorf("Possible bug!!! Got task for queue '%s' but queue is not created yet. task: %s", resTask.GetQueueName(), resTask.GetDescription())
					} else {
						q.AddLast(resTask)
					}
				}
			})
		}
	}()
}
