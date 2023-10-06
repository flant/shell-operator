package shell_operator

import (
	"context"

	log "github.com/sirupsen/logrus"

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
}

type ManagerEventsHandler struct {
	ctx    context.Context
	cancel context.CancelFunc

	kubeEventsManager kube_events_manager.KubeEventsManager
	scheduleManager   schedule_manager.ScheduleManager

	kubeEventCb func(kubeEvent KubeEvent) []task.Task
	scheduleCb  func(crontab string) []task.Task

	taskQueues *queue.TaskQueueSet
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
	}
}

func (m *ManagerEventsHandler) withKubeEventHandler(fn func(kubeEvent KubeEvent) []task.Task) {
	m.kubeEventCb = fn
}

func (m *ManagerEventsHandler) withScheduleEventHandler(fn func(crontab string) []task.Task) {
	m.scheduleCb = fn
}

func (m *ManagerEventsHandler) start() {
	go func() {
		for {
			var tailTasks []task.Task
			logEntry := log.WithField("operator.component", "handleEvents")
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
