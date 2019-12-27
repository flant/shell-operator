package shell_operator

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/metrics_storage"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/dump"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

//var (
//	WorkingDir string
//	TempDir    string
//
//	TaskQueues *queue.TaskQueueSet
//
//	HookManager hook.HookManager
//
//	ScheduleManager   schedule_manager.ScheduleManager
//	KubeEventsManager kube_events_manager.KubeEventsManager
//
//	MetricsStorage *metrics_storage.MetricStorage
//
//	ManagersEventsHandler *ManagerEventsHandler
//
//	// StopHandleEventsFromManagersCh channel is used to stop loop of the HandleEventsFromManagers.
//	StopHandleEventsFromManagersCh = make(chan struct{}, 1)
//
//	CancelOperator context.CancelFunc
//)

type ShellOperator struct {
	ctx    context.Context
	cancel context.CancelFunc

	HooksDir string
	TempDir  string

	TaskQueues *queue.TaskQueueSet

	HookManager hook.HookManager

	ScheduleManager   schedule_manager.ScheduleManager
	KubeEventsManager kube_events_manager.KubeEventsManager

	MetricsStorage *metrics_storage.MetricStorage

	ManagerEventsHandler *ManagerEventsHandler
}

func NewShellOperator() *ShellOperator {
	return &ShellOperator{}
}

func (op *ShellOperator) WithHooksDir(dir string) {
	op.HooksDir = dir
}

func (op *ShellOperator) WithTempDir(dir string) {
	op.TempDir = dir
}

func (op *ShellOperator) WithContext(ctx context.Context) {
	op.ctx, op.cancel = context.WithCancel(ctx)
}

func (op *ShellOperator) Stop() {
	if op.cancel != nil {
		op.cancel()
	}
}

// Init does some basic checks and instantiate managers
//
// - check settings: directories, kube config
// - initialize managers: hook manager, kube events manager, schedule manager
// - create an empty task queue
func (op *ShellOperator) Init() (err error) {
	log.Debug("MAIN Init")

	if op.HooksDir == "" {
		op.HooksDir, err = filepath.Abs(app.WorkingDir)
		if err != nil {
			log.Errorf("MAIN Fatal: Cannot determine a current dir: %s", err)
			return err
		}
		if exists, _ := utils_file.DirExists(op.HooksDir); !exists {
			log.Errorf("MAIN Fatal: working dir '%s' is not exists", op.HooksDir)
			return fmt.Errorf("no working dir")
		}
	}
	log.Infof("Working dir: %s", op.HooksDir)

	if op.TempDir == "" {
		op.TempDir = app.TempDir
		if exists, _ := utils_file.DirExists(op.TempDir); !exists {
			err = os.Mkdir(op.TempDir, os.FileMode(0777))
			if err != nil {
				log.Errorf("MAIN Fatal: Cannot create a temporary dir: %s", err)
				return err
			}
		}
	}
	log.Infof("Use temporary dir: %s", op.TempDir)

	// Initializing the task queues set with the "main" queue.
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(op.ctx)

	// Initializing schedule manager.
	op.ScheduleManager = schedule_manager.NewScheduleManager()
	op.ScheduleManager.WithContext(op.ctx)

	// Initialize kube client for kube events hooks.
	err = kube.Init(kube.InitOptions{KubeContext: app.KubeContext, KubeConfig: app.KubeConfig})
	if err != nil {
		log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}

	op.KubeEventsManager = kube_events_manager.NewKubeEventsManager()
	op.KubeEventsManager.WithContext(op.ctx)

	// Initializing hook manager (load hooks from WorkingDir)
	op.HookManager = hook.NewHookManager()
	op.HookManager.WithDirectories(op.HooksDir, op.TempDir)
	op.HookManager.WithKubeEventManager(op.KubeEventsManager)
	op.HookManager.WithScheduleManager(op.ScheduleManager)
	err = op.HookManager.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize hook manager: %s\n", err)
		return err
	}

	op.ManagerEventsHandler = NewManagerEventsHandler()
	op.ManagerEventsHandler.WithContext(op.ctx)
	op.ManagerEventsHandler.WithTaskQueueSet(op.TaskQueues)
	op.ManagerEventsHandler.WithScheduleManager(op.ScheduleManager)
	op.ManagerEventsHandler.WithKubeEventsManager(op.KubeEventsManager)
	op.ManagerEventsHandler.WithKubeEventHandler(func(kubeEvent KubeEvent) []task.Task {
		//logEntry := log.WithField("binding", ContextBindingType[OnKubernetesEvent])
		//logEntry.Infof("Kubernetes event %s", kubeEvent.String())

		tasks := []task.Task{}

		op.HookManager.HandleKubeEvent(kubeEvent, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       hook.Name,
					BindingType:    OnKubernetesEvent,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
				})
			tasks = append(tasks, newTask)
		})

		return tasks
	})
	op.ManagerEventsHandler.WithScheduleEventHandler(func(crontab string) []task.Task {
		var tasks []task.Task
		err := op.HookManager.HandleScheduleEvent(crontab, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       hook.Name,
					BindingType:    Schedule,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
				})
			tasks = append(tasks, newTask)
		})

		if err != nil {
			log.Errorf("handle event from ScheduleManager '%s': %s", crontab, err)
			return []task.Task{}
		}

		return tasks
	})

	// Initialiaze prometheus client
	op.MetricsStorage = metrics_storage.Init()

	return nil
}

func (op *ShellOperator) Run() {
	logEntry := log.WithField("operator.component", "mainRun")
	logEntry.Info("start Run")

	// Metric storage and live metrics
	go op.MetricsStorage.Run()
	op.RunMetrics()

	// Prepopulate main queue with onStartup tasks and enable kubernetes bindings tasks.
	op.PrepopulateMainQueue(op.TaskQueues)
	// Start main task queue handler
	op.TaskQueues.StartMain()

	// Managers are generating events. This go-routine handles all events and converts them into queued tasks.
	// Start it before start all informers to catch all kubernetes events (#42)
	op.ManagerEventsHandler.Start()
	//go HandleEventsFromManagers(op.TaskQueues)

	// add schedules to schedule manager
	op.HookManager.EnableScheduleBindings()
	op.ScheduleManager.Start()

}

// TaskHandler
func (op *ShellOperator) TaskHandler(t task.Task) queue.TaskResult {
	var logEntry = log.WithField("operator.component", "taskRunner")
	var hookMeta = HookMetadataAccessor(t)
	var res queue.TaskResult

	switch t.GetType() {
	case HookRun:
		hookLogLabels := map[string]string{}
		hookLogLabels["hook"] = hookMeta.HookName
		hookLogLabels["binding"] = string(hookMeta.BindingType)
		hookLogLabels["task"] = "HookRun"
		taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))
		taskLogEntry.Info("Execute hook")

		taskHook := op.HookManager.GetHook(hookMeta.HookName)

		err := taskHook.Run(hookMeta.BindingType, hookMeta.BindingContext, hookLogLabels)
		if err != nil {
			hookLabel := taskHook.SafeName()

			if hookMeta.AllowFailure {
				taskLogEntry.Infof("Hook failed, but allowed to fail: %v", err)
				op.MetricsStorage.SendCounterMetric("shell_operator_hook_allowed_errors", 1.0, map[string]string{"hook": hookLabel})
				res.Status = "Success"
			} else {
				op.MetricsStorage.SendCounterMetric("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
				t.IncrementFailureCount()
				taskLogEntry.Errorf("Hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount(), err)
				res.Status = "Fail"
			}
		} else {
			taskLogEntry.Infof("Hook executed successfully")
			res.Status = "Success"
		}

	case EnableKubernetesBindings:
		hookLogLabels := map[string]string{}
		hookLogLabels["hook"] = hookMeta.HookName
		hookLogLabels["binding"] = string(OnKubernetesEvent)
		hookLogLabels["task"] = "EnableKubernetesBindings"

		taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))

		taskLogEntry.Info("Enable kubernetes binding for hook")

		taskHook := op.HookManager.GetHook(hookMeta.HookName)

		hookRunTasks := []task.Task{}

		err := taskHook.HookController.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       taskHook.Name,
					BindingType:    OnKubernetesEvent,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
				})
			hookRunTasks = append(hookRunTasks, newTask)
		})

		if err != nil {
			hookLabel := taskHook.SafeName()
			op.MetricsStorage.SendCounterMetric("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
			t.IncrementFailureCount()
			taskLogEntry.Errorf("Enable Kubernetes binding for hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount(), err)
			res.Status = "Fail"
		} else {
			// return Synchronization tasks to add to the queue head.
			// Informers can be started now — their events will
			// be added to the tail of the queue.
			op.HookManager.GetHook(hookMeta.HookName).HookController.StartMonitors()
			taskLogEntry.Infof("Kubernetes binding for hook enabled successfully")
			res.Status = "Success"
			res.HeadTasks = hookRunTasks
		}
	}

	return res
}

// PrepopulateMainQueue adds tasks to run hooks with OnStartup bindings
// and tasks to enable kubernetes bindings.
func (op *ShellOperator) PrepopulateMainQueue(tqs *queue.TaskQueueSet) {
	logEntry := log.WithField("operator.component", "initMainQueue")

	// Prepopulate main queue with 'onStartup' tasks and 'enable kubernetes bindings' tasks.
	tqs.WithMainName("main")
	tqs.NewNamedQueue("main", op.TaskHandler)

	mainQueue := tqs.GetMain()
	mainQueue.ChangesDisable()

	// Add tasks to run OnStartup bindings

	onStartupHooks, err := op.HookManager.GetHooksInOrder(OnStartup)
	if err != nil {
		logEntry.Errorf("%v", err)
		return
	}

	logEntry.Infof("add HookRun@OnStartup tasks for %d hooks", len(onStartupHooks))

	for _, hookName := range onStartupHooks {
		bc := BindingContext{
			Binding: string(OnStartup),
		}
		bc.Metadata.BindingType = OnStartup

		newTask := task.NewTask(HookRun).
			WithMetadata(HookMetadata{
				HookName:       hookName,
				BindingType:    OnStartup,
				BindingContext: []BindingContext{bc},
			})
		mainQueue.AddLast(newTask)
		logEntry.Debugf("new task HookRun@OnStartup '%s'", hookName)
	}

	// Add tasks to enable kubernetes monitors

	kubeHooks, _ := op.HookManager.GetHooksInOrder(OnKubernetesEvent)
	for _, hookName := range kubeHooks {
		newTask := task.NewTask(EnableKubernetesBindings).
			WithMetadata(HookMetadata{
				HookName: hookName,
			})
		mainQueue.AddLast(newTask)
		logEntry.Infof("queue task %s@%s %s", newTask.GetType(), hookName)

	}

	mainQueue.ChangesEnable(true)
}

func (op *ShellOperator) RunMetrics() {
	// live ticks.
	go func() {
		for {
			op.MetricsStorage.SendCounterMetric("shell_operator_live_ticks", 1.0, map[string]string{})
			time.Sleep(10 * time.Second)
		}
	}()

	// task queue length
	go func() {
		for {
			queueLen := float64(op.TaskQueues.GetMain().Length())
			op.MetricsStorage.SendGaugeMetric("shell_operator_tasks_queue_length", queueLen, map[string]string{})
			time.Sleep(5 * time.Second)
		}
	}()
}

func (op *ShellOperator) InitHttpServer(listenAddr *net.TCPAddr) {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(fmt.Sprintf(`<html>
    <head><title>Shell operator</title></head>
    <body>
    <h1>Shell operator</h1>
    <pre>go tool pprof goprofex http://&lt;SHELL_OPERATOR_IP&gt;:%d/debug/pprof/profile</pre>
    </body>
    </html>`, app.ListenAddress.Port)))
	})

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/queue", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(writer, dump.TaskQueueToReader(op.TaskQueues.GetMain()))
	})

	go func() {
		logEntry := log.
			WithField("operator.component", "httpServer")
		logEntry.Infof("Listen on %s", listenAddr.String())
		if err := http.ListenAndServe(listenAddr.String(), nil); err != nil {
			logEntry.Errorf("Starting HTTP server: %s", err)
		}
	}()
}
