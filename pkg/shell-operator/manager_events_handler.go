package shell_operator

import (
	"context"

	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	. "github.com/flant/shell-operator/pkg/hook/types"
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
				logEntry = log.WithField("binding", string(Schedule))
				logEntry.Infof("Schedule event '%s'", crontab)

				if m.scheduleCb != nil {
					tailTasks = m.scheduleCb(crontab)
				}
				//
				//err := HookManager.HandleScheduleEvent(crontab, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
				//	newTask := task.NewTask(HookRun).
				//		WithMetadata(HookMetadata{
				//			HookName:       hook.Name,
				//			BindingType:    Schedule,
				//			BindingContext: info.BindingContext,
				//			AllowFailure:   info.AllowFailure,
				//		})
				//	tailTasks = append(tailTasks, newTask)
				//})

				//if err != nil {
				//	logEntry.Errorf("handle '%s': %s", crontab, err)
				//	break
				//}

			case kubeEvent := <-m.kubeEventsManager.Ch():
				logEntry = log.WithField("binding", string(OnKubernetesEvent))
				logEntry.Infof("Kubernetes event %s", kubeEvent.String())

				if m.kubeEventCb != nil {
					tailTasks = m.kubeEventCb(kubeEvent)
				}
				//
				//tasks := []task.Task{}
				//
				//HookManager.HandleKubeEvent(kubeEvent, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
				//	newTask := task.NewTask(HookRun).
				//		WithMetadata(HookMetadata{
				//			HookName:       hook.Name,
				//			BindingType:    OnKubernetesEvent,
				//			BindingContext: info.BindingContext,
				//			AllowFailure:   info.AllowFailure,
				//		})
				//	tasks = append(tasks, newTask)
				//})

			case <-m.ctx.Done():
				logEntry.Infof("Stop")
				return
			}

			m.taskQueues.DoWithLock(func(tqs *queue.TaskQueueSet) {
				for _, resTask := range tailTasks {
					tqs.GetMain().AddLast(resTask)
					hm := HookMetadataAccessor(resTask)
					logEntry.Infof("queue task %s@%s %s", resTask.GetType(), hm.BindingType, hm.HookName)
				}
			})

		}
	}()
}
