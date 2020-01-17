package shell_operator

import (
	"context"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

type ManagerEventsHandler struct {
	ctx    context.Context
	cancel context.CancelFunc

	kubeEventsManager kube_events_manager.KubeEventsManager
	scheduleManager   schedule_manager.ScheduleManager

	kubeEventCb func(kubeEvent KubeEvent) []task.Task
	scheduleCb  func(crontab string) []task.Task

	taskQueues *queue.TaskQueueSet
}

func NewManagerEventsHandler() *ManagerEventsHandler {
	return &ManagerEventsHandler{}
}

func (m *ManagerEventsHandler) WithTaskQueueSet(tqs *queue.TaskQueueSet) {
	m.taskQueues = tqs
}

func (m *ManagerEventsHandler) WithKubeEventsManager(mgr kube_events_manager.KubeEventsManager) {
	m.kubeEventsManager = mgr
}

func (m *ManagerEventsHandler) WithKubeEventHandler(fn func(kubeEvent KubeEvent) []task.Task) {
	m.kubeEventCb = fn
}

func (m *ManagerEventsHandler) WithScheduleManager(mgr schedule_manager.ScheduleManager) {
	m.scheduleManager = mgr
}

func (m *ManagerEventsHandler) WithScheduleEventHandler(fn func(crontab string) []task.Task) {
	m.scheduleCb = fn
}

func (m *ManagerEventsHandler) WithContext(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
}

func (m *ManagerEventsHandler) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *ManagerEventsHandler) Start() {
	go func() {
		for {
			var tailTasks []task.Task
			var logEntry = log.WithField("operator.component", "handleEvents")
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
						logEntry.WithField("queue", q.Name).Infof("queue task %s", resTask.GetDescription())
					}
				}
			})
		}
	}()
}
