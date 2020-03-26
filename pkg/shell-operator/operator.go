package shell_operator

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	uuid "gopkg.in/satori/go.uuid.v1"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/debug"
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

type ShellOperator struct {
	ctx    context.Context
	cancel context.CancelFunc

	HooksDir string
	TempDir  string

	MetricStorage *metrics_storage.MetricStorage
	KubeClient    kube.KubernetesClient

	ScheduleManager   schedule_manager.ScheduleManager
	KubeEventsManager kube_events_manager.KubeEventsManager

	TaskQueues *queue.TaskQueueSet

	ManagerEventsHandler *ManagerEventsHandler

	HookManager hook.HookManager

	DebugServer *debug.Server
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

func (op *ShellOperator) WithContext(ctx context.Context) *ShellOperator {
	op.ctx, op.cancel = context.WithCancel(ctx)
	return op
}

func (op *ShellOperator) Stop() {
	if op.cancel != nil {
		op.cancel()
	}
}

func (op *ShellOperator) WithKubernetesClient(klient kube.KubernetesClient) {
	op.KubeClient = klient
}

func (op *ShellOperator) WithMetricStorage(metricStorage *metrics_storage.MetricStorage) {
	op.MetricStorage = metricStorage
}

// Init does some basic checks and instantiate dependencies
//
// - check directories
// - start debug server
// - initialize dependencies:
//   - metric storage
//   - kubernetes client config
//   - empty set of task queues
//   - hook manager
//   - kubernetes events manager
//   - schedule manager
func (op *ShellOperator) Init() (err error) {
	log.Debug("MAIN Init")

	if op.HooksDir == "" {
		op.HooksDir, err = filepath.Abs(app.HooksDir)
		if err != nil {
			log.Errorf("MAIN Fatal: Cannot determine a current dir: %s", err)
			return err
		}
		if exists, _ := utils_file.DirExists(op.HooksDir); !exists {
			log.Errorf("MAIN Fatal: working dir '%s' is not exists", op.HooksDir)
			return fmt.Errorf("no working dir")
		}
	}
	log.Infof("Hooks dir: %s", op.HooksDir)

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

	op.DebugServer = debug.NewServer()
	op.DebugServer.WithPrefix("/debug")
	op.DebugServer.WithSocketPath(app.DebugUnixSocket)
	err = op.DebugServer.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: Cannot create Debug server: %s", err)
		return err
	}

	// init and start metrics gathering loop
	if op.MetricStorage == nil {
		op.MetricStorage = metrics_storage.NewMetricStorage()
		op.MetricStorage.WithContext(op.ctx)
		op.MetricStorage.WithPrefix(app.PrometheusMetricsPrefix)
		// Metric storage and live metrics
		op.MetricStorage.Start()
	}

	if op.KubeClient == nil {
		op.KubeClient = kube.NewKubernetesClient()
		op.KubeClient.WithContextName(app.KubeContext)
		op.KubeClient.WithConfigPath(app.KubeConfig)
		op.KubeClient.WithRateLimiterSettings(app.KubeClientQps, app.KubeClientBurst)
		// Initialize kube client for kube events hooks.
		err = op.KubeClient.Init()
		if err != nil {
			log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
			return err
		}
	}

	// Initialize the task queues set with the "main" queue.
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(op.ctx)

	// Initialize schedule manager.
	op.ScheduleManager = schedule_manager.NewScheduleManager()
	op.ScheduleManager.WithContext(op.ctx)

	// Initialize kubernetes events manager.
	op.KubeEventsManager = kube_events_manager.NewKubeEventsManager()
	op.KubeEventsManager.WithKubeClient(op.KubeClient)
	op.KubeEventsManager.WithContext(op.ctx)

	// Initialize events handler that emit tasks to run hooks
	op.ManagerEventsHandler = NewManagerEventsHandler()
	op.ManagerEventsHandler.WithContext(op.ctx)
	op.ManagerEventsHandler.WithTaskQueueSet(op.TaskQueues)
	op.ManagerEventsHandler.WithScheduleManager(op.ScheduleManager)
	op.ManagerEventsHandler.WithKubeEventsManager(op.KubeEventsManager)

	return nil
}

// InitHookManager load hooks from HooksDir and defines event handlers that emit tasks.
func (op *ShellOperator) InitHookManager() (err error) {
	// Initialize hook manager (load hooks from HooksDir)
	op.HookManager = hook.NewHookManager()
	op.HookManager.WithDirectories(op.HooksDir, op.TempDir)
	op.HookManager.WithKubeEventManager(op.KubeEventsManager)
	op.HookManager.WithScheduleManager(op.ScheduleManager)
	err = op.HookManager.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize hook manager: %s\n", err)
		return err
	}

	// Define event handlers for schedule event and kubernetes event.
	op.ManagerEventsHandler.WithKubeEventHandler(func(kubeEvent KubeEvent) []task.Task {
		logLabels := map[string]string{
			"event.id": uuid.NewV4().String(),
			"binding":  ContextBindingType[Schedule],
		}
		logEntry := log.WithFields(utils.LabelsToLogFields(logLabels))
		logEntry.Debugf("Create tasks for 'kubernetes' event '%s'", kubeEvent.String())

		var tasks []task.Task
		op.HookManager.HandleKubeEvent(kubeEvent, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       hook.Name,
					BindingType:    OnKubernetesEvent,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(logLabels).
				WithQueueName(info.QueueName)
			tasks = append(tasks, newTask)
		})

		return tasks
	})
	op.ManagerEventsHandler.WithScheduleEventHandler(func(crontab string) []task.Task {
		logLabels := map[string]string{
			"event.id": uuid.NewV4().String(),
			"binding":  ContextBindingType[Schedule],
		}
		logEntry := log.WithFields(utils.LabelsToLogFields(logLabels))
		logEntry.Debugf("Create tasks for 'schedule' event '%s'", crontab)

		var tasks []task.Task
		op.HookManager.HandleScheduleEvent(crontab, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       hook.Name,
					BindingType:    Schedule,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(logLabels).
				WithQueueName(info.QueueName)
			tasks = append(tasks, newTask)
		})

		return tasks
	})
	return nil
}

func (op *ShellOperator) Start() {
	log.Info("start shell-operator")

	// Start emit "live" metrics
	op.RunMetrics()

	// Prepopulate main queue with onStartup tasks and enable kubernetes bindings tasks.
	op.PrepopulateMainQueue(op.TaskQueues)
	// Start main task queue handler
	op.TaskQueues.StartMain()
	op.InitAndStartHookQueues()

	// Managers are generating events. This go-routine handles all events and converts them into queued tasks.
	// Start it before start all informers to catch all kubernetes events (#42)
	op.ManagerEventsHandler.Start()

	// Unlike KubeEventsManager, ScheduleManager has one go-routine.
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
		hookLogLabels["queue"] = t.GetQueueName()
		taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))
		taskLogEntry.Info("Execute hook")

		taskHook := op.HookManager.GetHook(hookMeta.HookName)
		if taskHook.Config.Version == "v1" {
			bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName(t.GetQueueName()), t)
			if bcs != nil {
				hookMeta.BindingContext = bcs
				t.UpdateMetadata(hookMeta)
			}
		}

		err := taskHook.Run(hookMeta.BindingType, hookMeta.BindingContext, hookLogLabels)
		if err != nil {
			hookLabel := taskHook.SafeName()

			if hookMeta.AllowFailure {
				taskLogEntry.Infof("Hook failed, but allowed to fail: %v", err)
				op.MetricStorage.SendCounter("shell_operator_hook_allowed_errors", 1.0, map[string]string{"hook": hookLabel})
				res.Status = "Success"
			} else {
				op.MetricStorage.SendCounter("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
				t.UpdateFailureMessage(err.Error())
				taskLogEntry.Errorf("Hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount()+1, err)
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
		hookLogLabels["queue"] = "main"

		taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))

		taskLogEntry.Info("Enable kubernetes binding for hook")

		taskHook := op.HookManager.GetHook(hookMeta.HookName)

		hookRunTasks := []task.Task{}

		// Run hook with Synchronization binding context. Ignore queue name here, execute in main queue.
		err := taskHook.HookController.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       taskHook.Name,
					BindingType:    OnKubernetesEvent,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(hookLogLabels).
				WithQueueName("main")
			hookRunTasks = append(hookRunTasks, newTask)
		})

		if err != nil {
			hookLabel := taskHook.SafeName()
			op.MetricStorage.SendCounter("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
			t.UpdateFailureMessage(err.Error())
			taskLogEntry.Errorf("Enable Kubernetes binding for hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount()+1, err)
			res.Status = "Fail"
		} else {
			// return Synchronization tasks to add to the queue head.
			// Informers can be started now — their events will
			// be added to the tail of the main queue or to the other queues.
			op.HookManager.GetHook(hookMeta.HookName).HookController.StartMonitors()
			taskLogEntry.Infof("Kubernetes binding for hook enabled successfully")
			res.Status = "Success"
			res.HeadTasks = hookRunTasks
		}
	case EnableScheduleBindings:
		hookLogLabels := map[string]string{}
		hookLogLabels["hook"] = hookMeta.HookName
		hookLogLabels["binding"] = string(Schedule)
		hookLogLabels["task"] = "EnableScheduleBindings"
		hookLogLabels["queue"] = "main"

		taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))

		taskHook := op.HookManager.GetHook(hookMeta.HookName)
		taskHook.HookController.EnableScheduleBindings()
		taskLogEntry.Infof("Schedule binding for hook enabled successfully")
		res.Status = "Success"
	}

	return res
}

// CombineBindingContextForHook combines binding contexts from a sequence of task with similar
// hook name and task type into array of binding context and delete excess tasks from queue.
// Also, compacts sequences of binding contexts with similar group.
// If input task has no metadata, result will be nil.
// Metadata should implement HookNameAccessor and BindingContextAccessor interfaces.
func (op *ShellOperator) CombineBindingContextForHook(q *queue.TaskQueue, t task.Task) []BindingContext {
	if q == nil {
		return nil
	}
	var taskMeta = t.GetMetadata()
	if taskMeta == nil {
		// Ignore task without metadata
		return nil
	}
	var hookName = taskMeta.(HookNameAccessor).GetHookName()

	var otherTasks = make([]task.Task, 0)
	var stopIterate = false
	q.Iterate(func(tsk task.Task) {
		if stopIterate {
			return
		}
		// ignore current task
		if tsk.GetId() == t.GetId() {
			return
		}
		hm := tsk.GetMetadata()
		if hm == nil {
			stopIterate = true
			return
		}
		nextHookName := hm.(HookNameAccessor).GetHookName()
		if nextHookName != hookName || t.GetType() != tsk.GetType() {
			stopIterate = true
			return
		}
		otherTasks = append(otherTasks, tsk)
	})

	// no tasks found to combine
	if len(otherTasks) == 0 {
		return nil
	}

	// Combine binding context and make a map to delete excess tasks
	var combinedContext = make([]BindingContext, 0)
	var tasksFilter = make(map[string]bool)
	// current task always remain in queue
	combinedContext = append(combinedContext, taskMeta.(BindingContextAccessor).GetBindingContext()...)
	tasksFilter[t.GetId()] = true
	for _, tsk := range otherTasks {
		combinedContext = append(combinedContext, tsk.GetMetadata().(BindingContextAccessor).GetBindingContext()...)
		tasksFilter[tsk.GetId()] = false
	}
	log.Infof("Will delete %d tasks from queue '%s'", len(tasksFilter)-1, t.GetQueueName())

	// Delete tasks with false in tasksFilter map
	op.TaskQueues.GetByName(t.GetQueueName()).Filter(func(tsk task.Task) bool {
		if v, ok := tasksFilter[tsk.GetId()]; ok {
			return v
		}
		return true
	})

	// group is used to compact binding contexts when only snapshots are needed
	var compactedContext = make([]BindingContext, 0)
	var isGroup = false
	var groupHasSynchronization = false
	for i := 0; i < len(combinedContext); i++ {
		var shouldSkip = false
		var groupName = combinedContext[i].Metadata.Group

		// binding context is ignore for similar group
		if groupName != "" && (i+1 <= len(combinedContext)-1) && combinedContext[i+1].Metadata.Group == groupName {
			shouldSkip = true
			isGroup = true
		}
		if isGroup && combinedContext[i].Type == TypeSynchronization {
			groupHasSynchronization = true
		}

		if shouldSkip {
			continue
		} else {
			if isGroup {
				if groupHasSynchronization {
					combinedContext[i].Metadata.GroupType = string(TypeSynchronization)
				} else {
					combinedContext[i].Metadata.GroupType = "Group"
				}
			}
			isGroup = false
			groupHasSynchronization = false
			compactedContext = append(compactedContext, combinedContext[i])
		}
	}

	return compactedContext
}

// PrepopulateMainQueue adds tasks to run hooks with OnStartup bindings
// and tasks to enable kubernetes bindings.
func (op *ShellOperator) PrepopulateMainQueue(tqs *queue.TaskQueueSet) {
	logEntry := log.WithField("operator.component", "initMainQueue")

	// Prepopulate main queue with 'onStartup' tasks and 'enable kubernetes bindings' tasks.
	tqs.WithMainName("main")
	tqs.NewNamedQueue("main", op.TaskHandler)

	mainQueue := tqs.GetMain()

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

	// Add tasks to enable kubernetes monitors and schedules for each hook
	for _, hookName := range op.HookManager.GetHookNames() {
		h := op.HookManager.GetHook(hookName)

		if h.GetConfig().HasBinding(OnKubernetesEvent) {
			newTask := task.NewTask(EnableKubernetesBindings).
				WithMetadata(HookMetadata{
					HookName: hookName,
					Binding:  string(EnableKubernetesBindings),
				})
			mainQueue.AddLast(newTask)
			logEntry.Infof("queue task %s with hook %s", newTask.GetDescription(), hookName)
		}

		if h.GetConfig().HasBinding(Schedule) {
			newTask := task.NewTask(EnableScheduleBindings).
				WithMetadata(HookMetadata{
					HookName: hookName,
					Binding:  string(EnableScheduleBindings),
				})
			mainQueue.AddLast(newTask)
			logEntry.Infof("queue task %s with hook %s", newTask.GetDescription(), hookName)
		}
	}
}

// CreateQueues create all queues defined in hooks
func (op *ShellOperator) InitAndStartHookQueues() {
	schHooks, _ := op.HookManager.GetHooksInOrder(Schedule)
	for _, hookName := range schHooks {
		h := op.HookManager.GetHook(hookName)
		for _, hookBinding := range h.Config.Schedules {
			if op.TaskQueues.GetByName(hookBinding.Queue) == nil {
				op.TaskQueues.NewNamedQueue(hookBinding.Queue, op.TaskHandler)
				op.TaskQueues.GetByName(hookBinding.Queue).Start()
			}
		}
	}

	kubeHooks, _ := op.HookManager.GetHooksInOrder(OnKubernetesEvent)
	for _, hookName := range kubeHooks {
		h := op.HookManager.GetHook(hookName)
		for _, hookBinding := range h.Config.OnKubernetesEvents {
			if op.TaskQueues.GetByName(hookBinding.Queue) == nil {
				op.TaskQueues.NewNamedQueue(hookBinding.Queue, op.TaskHandler)
				op.TaskQueues.GetByName(hookBinding.Queue).Start()
			}
		}
	}
}

func (op *ShellOperator) RunMetrics() {
	// live ticks.
	go func() {
		for {
			op.MetricStorage.SendCounter("shell_operator_live_ticks", 1.0, map[string]string{})
			time.Sleep(10 * time.Second)
		}
	}()

	// task queue length
	go func() {
		for {
			op.TaskQueues.Iterate(func(queue *queue.TaskQueue) {
				queueLen := float64(queue.Length())
				op.MetricStorage.SendGauge("shell_operator_tasks_queue_length", queueLen, map[string]string{"queue": queue.Name})
			})
			time.Sleep(5 * time.Second)
		}
	}()
}

func (op *ShellOperator) SetupDebugServerHandles() {
	op.DebugServer.Router.Get("/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprintf(writer, "%s control endpoint is alive", app.AppName)
	})

	op.DebugServer.Router.Get("/queue/list.{format:(json|yaml|text)}", func(writer http.ResponseWriter, request *http.Request) {
		format := chi.URLParam(request, "format")
		debug.GetLogEntry(request).Debugf("queue list using format %s", format)
		_, _ = writer.Write([]byte(dump.TaskQueueSetToText(op.TaskQueues)))
	})
}

func (op *ShellOperator) SetupHttpServerHandles() {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprintf(writer, `<html>
    <head><title>Shell operator</title></head>
    <body>
    <h1>Shell operator</h1>
    <pre>go tool pprof goprofex http://&lt;SHELL_OPERATOR_IP&gt;:%s/debug/pprof/profile</pre>
    </body>
    </html>`, app.ListenPort)
	})

	http.Handle("/metrics", promhttp.Handler())
}

func (op *ShellOperator) StartHttpServer(ip string, port string) error {
	address := fmt.Sprintf("%s:%s", ip, port)

	// Check if port is available
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Errorf("Fail to listen on '%s': %v", address, err)
		return err
	}

	log.Infof("Listen on %s", address)

	go func() {
		if err := http.Serve(listener, nil); err != nil {
			log.Errorf("Error starting HTTP server: %s", err)
			os.Exit(1)
		}
	}()

	return nil
}

func DefaultOperator() *ShellOperator {
	operator := NewShellOperator()
	operator.WithContext(context.Background())
	return operator
}

func InitAndStart(operator *ShellOperator) error {
	operator.SetupHttpServerHandles()

	err := operator.StartHttpServer(app.ListenAddress, app.ListenPort)
	if err != nil {
		log.Errorf("HTTP SERVER start failed: %v", err)
		return err
	}

	err = operator.Init()
	if err != nil {
		log.Errorf("INIT failed: %s", err)
		return err
	}

	operator.SetupDebugServerHandles()

	err = operator.InitHookManager()
	if err != nil {
		log.Errorf("INIT HookManager failed: %s", err)
		return err
	}

	operator.Start()

	return nil
}
