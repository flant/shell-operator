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
	. "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/metrics_storage"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

var (
	WorkingDir string
	TempDir    string

	TasksQueue *task.TasksQueue

	HookManager hook.HookManager

	ScheduleManager   schedule_manager.ScheduleManager
	KubeEventsManager kube_events_manager.KubeEventsManager

	MetricsStorage *metrics_storage.MetricStorage

	// StopHandleEventsFromManagersCh channel is used to stop loop of the HandleEventsFromManagers.
	StopHandleEventsFromManagersCh = make(chan struct{}, 1)
)

var (
	QueueIsEmptyDelay = 3 * time.Second
	FailedHookDelay   = 5 * time.Second
)

// Init does some basic checks and instantiate managers
//
// - check settings: directories, kube config
// - initialize managers: hook manager, kube events manager, schedule manager
// - create an empty task queue
func Init() (err error) {
	log.Debug("MAIN Init")

	WorkingDir, err := filepath.Abs(app.WorkingDir)
	if err != nil {
		log.Errorf("MAIN Fatal: Cannot determine a current dir: %s", err)
		return err
	}
	if exists, _ := utils_file.DirExists(WorkingDir); !exists {
		log.Errorf("MAIN Fatal: working dir '%s' is not exists", WorkingDir)
		return fmt.Errorf("no working dir")
	}
	log.Infof("Working dir: %s", WorkingDir)

	TempDir = app.TempDir
	if exists, _ := utils_file.DirExists(TempDir); !exists {
		err = os.Mkdir(TempDir, os.FileMode(0777))
		if err != nil {
			log.Errorf("MAIN Fatal: Cannot create a temporary dir: %s", err)
			return err
		}
	}
	log.Infof("Use temporary dir: %s", TempDir)

	// Initializing the empty task queue.
	TasksQueue = task.NewTasksQueue()

	// Initializing schedule manager.
	ScheduleManager = schedule_manager.NewScheduleManager()

	// Initialize kube client for kube events hooks.
	err = kube.Init(kube.InitOptions{KubeContext: app.KubeContext, KubeConfig: app.KubeConfig})
	if err != nil {
		log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}

	KubeEventsManager = kube_events_manager.NewKubeEventsManager()
	KubeEventsManager.WithContext(context.Background())

	// Initializing hook manager (load hooks from WorkingDir)
	HookManager = hook.NewHookManager()
	HookManager.WithDirectories(WorkingDir, TempDir)
	HookManager.WithKubeEventManager(KubeEventsManager)
	HookManager.WithScheduleManager(ScheduleManager)
	err = HookManager.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize hook manager: %s\n", err)
		return err
	}

	// Initialiaze prometheus client
	MetricsStorage = metrics_storage.Init()

	return nil
}

func Run() {
	logEntry := log.WithField("operator.component", "mainRun")
	logEntry.Info("start Run")

	// Metric storage and live metrics
	go MetricsStorage.Run()
	RunMetrics()

	// Prepopulate queue with onStartup tasks and enable kubernetes bindings tasks
	TasksQueue.ChangesDisable()
	CreateOnStartupTasks()

	kubeHooks, _ := HookManager.GetHooksInOrder(OnKubernetesEvent)
	enableBindingsTasks := []task.Task{}
	for _, hookName := range kubeHooks {
		newTask := task.NewTask(task.EnableKubernetesBindings, hookName)
		enableBindingsTasks = append(enableBindingsTasks, newTask)
	}
	for _, resTask := range enableBindingsTasks {
		TasksQueue.Add(resTask)
		logEntry.Infof("queue task %s@%s %s", resTask.GetType(), resTask.GetBinding(), resTask.GetName())
	}
	TasksQueue.ChangesEnable(true)

	// Managers are generating events. This go-routine handles all events and converts them into queued tasks.
	// Start it before start all informers to catch all kubernetes events (#42)
	go HandleEventsFromManagers()

	// add schedules to schedule manager
	HookManager.EnableScheduleBindings()
	go ScheduleManager.Run()

	// TasksRunner runs tasks from the queue.
	go TasksRunner()
}

// Stop closes event handle loop and pushes Stop task to wait until current task is done.
func Stop() {
	StopHandleEventsFromManagersCh <- struct{}{}
	// FIXME: Double push to prevent Pop in task handlers.
	TasksQueue.Push(task.NewTask(task.Stop, "Stop"))
	TasksQueue.Push(task.NewTask(task.Stop, "Stop"))
}

// HandleEventsFromManagers read schedule and kubernetes events and emit tasks to the working queue.
func HandleEventsFromManagers() {
	for {
		select {
		case crontab := <-ScheduleManager.Ch():
			logEntry := log.
				WithField("operator.component", "handleEvents").
				WithField("binding", ContextBindingType[Schedule])
			logEntry.Infof("Schedule event '%s'", crontab)

			tasks := []task.Task{}
			err := HookManager.HandleScheduleEvent(crontab, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
				newTask := task.NewTask(task.HookRun, hook.Name).
					WithBinding(Schedule).
					WithBindingContext(info.BindingContext).
					WithAllowFailure(info.AllowFailure)
				tasks = append(tasks, newTask)
			})

			if err != nil {
				logEntry.Errorf("handle '%s': %s", crontab, err)
				break
			}

			for _, resTask := range tasks {
				TasksQueue.Add(resTask)
				logEntry.Infof("queue task %s@%s %s", resTask.GetType(), resTask.GetBinding(), resTask.GetName())
			}

		case kubeEvent := <-KubeEventsManager.Ch():
			logEntry := log.
				WithField("operator.component", "handleEvents").
				WithField("binding", ContextBindingType[OnKubernetesEvent])
			logEntry.Infof("Kubernetes event %s", kubeEvent.String())

			tasks := []task.Task{}

			HookManager.HandleKubeEvent(kubeEvent, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
				newTask := task.NewTask(task.HookRun, hook.Name).
					WithBinding(OnKubernetesEvent).
					WithBindingContext(info.BindingContext).
					WithAllowFailure(info.AllowFailure)
				tasks = append(tasks, newTask)
			})

			for _, resTask := range tasks {
				TasksQueue.Add(resTask)
				logEntry.Infof("queue task %s@%s %s", resTask.GetType(), resTask.GetBinding(), resTask.GetName())
			}

		case <-StopHandleEventsFromManagersCh:
			logEntry := log.
				WithField("operator.component", "handleEvents").
				WithField("binding", "stop")
			logEntry.Infof("trigger Stop HandleEventsFromManagers Loop")
			return
		}
	}
}

// TasksRunner is a tasks queue handler
func TasksRunner() {
	logEntry := log.WithField("operator.component", "taskRunner")
	for {
		if TasksQueue.IsEmpty() {
			time.Sleep(QueueIsEmptyDelay)
		}
		for {
			t, _ := TasksQueue.Peek()
			if t == nil {
				break
			}

			switch t.GetType() {
			case task.HookRun:
				hookLogLabels := map[string]string{}
				hookLogLabels["hook"] = t.GetName()
				hookLogLabels["binding"] = ContextBindingType[t.GetBinding()]
				hookLogLabels["task"] = "HookRun"
				taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))
				taskLogEntry.Info("Execute hook")

				taskHook := HookManager.GetHook(t.GetName())

				// get actual snapshots
				bindingContext := taskHook.HookController.UpdateSnapshots(t.GetBindingContext())

				err := taskHook.Run(t.GetBinding(), bindingContext, hookLogLabels)
				if err != nil {
					hookLabel := taskHook.SafeName()

					if t.GetAllowFailure() {
						taskLogEntry.Infof("Hook failed, but allowed to fail: %v", err)
						MetricsStorage.SendCounterMetric("shell_operator_hook_allowed_errors", 1.0, map[string]string{"hook": hookLabel})
						TasksQueue.Pop()
					} else {
						MetricsStorage.SendCounterMetric("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
						t.IncrementFailureCount()
						taskLogEntry.Errorf("Hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount(), err)
						delayTask := task.NewTaskDelay(FailedHookDelay)
						delayTask.Name = t.GetName()
						delayTask.Binding = t.GetBinding()
						TasksQueue.Push(delayTask)
					}
				} else {
					taskLogEntry.Infof("Hook executed successfully")
					TasksQueue.Pop()
				}

			case task.EnableKubernetesBindings:
				hookLogLabels := map[string]string{}
				hookLogLabels["hook"] = t.GetName()
				hookLogLabels["binding"] = ContextBindingType[OnKubernetesEvent]
				hookLogLabels["task"] = "EnableKubernetesBindings"

				taskLogEntry := logEntry.WithFields(utils.LabelsToLogFields(hookLogLabels))

				taskLogEntry.Info("Enable kubernetes binding for hook")

				taskHook := HookManager.GetHook(t.GetName())

				hookRunTasks := []task.Task{}

				err := taskHook.HookController.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
					newTask := task.NewTask(task.HookRun, taskHook.Name).
						WithBinding(OnKubernetesEvent).
						WithBindingContext(info.BindingContext).
						WithAllowFailure(info.AllowFailure)
					hookRunTasks = append(hookRunTasks, newTask)
				})

				if err != nil {
					hookLabel := taskHook.SafeName()
					MetricsStorage.SendCounterMetric("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
					t.IncrementFailureCount()
					taskLogEntry.Errorf("Enable Kubernetes binding for hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount(), err)
					delayTask := task.NewTaskDelay(FailedHookDelay)
					delayTask.Name = t.GetName()
					delayTask.Binding = t.GetBinding()
					TasksQueue.Push(delayTask)
				} else {
					// Push Synchronization tasks to queue head. Informers can be started now — their events will
					// be added to the queue tail.
					taskLogEntry.Infof("Kubernetes binding for hook enabled successfully")
					TasksQueue.Pop()
					for _, hookRunTask := range hookRunTasks {
						TasksQueue.Push(hookRunTask)
					}
					HookManager.GetHook(t.GetName()).HookController.StartMonitors()
				}

			case task.Delay:
				logEntry := log.
					WithField("operator.component", "taskRunner").
					WithField("task", "Delay").
					WithField("hook", t.GetName()).
					WithField("binding", ContextBindingType[t.GetBinding()])

				logEntry.Infof("Delay for %s", t.GetDelay().String())
				TasksQueue.Pop()
				time.Sleep(t.GetDelay())
			case task.Stop:
				log.WithField("operator.component", "taskRunner").
					WithField("task", "Stop").
					Infof("Stop TaskRunner loop.")
				TasksQueue.Pop()
				return
			case task.Exit:
				log.WithField("operator.component", "taskRunner").
					WithField("task", "Exit").
					Infof("Program will exit now.")
				TasksQueue.Pop()
				os.Exit(1)
			}

			// Breaking, if the task queue is empty to prevent the infinite loop.
			if TasksQueue.IsEmpty() {
				log.WithField("operator.component", "taskRunner").
					Debug("Task queue is empty. Will sleep now.")
				break
			}
		}
	}
}

func CreateOnStartupTasks() {
	logEntry := log.
		WithField("operator.component", "createOnStartupTasks").
		WithField("binding", ContextBindingType[OnStartup])

	onStartupHooks, err := HookManager.GetHooksInOrder(OnStartup)
	if err != nil {
		logEntry.Errorf("%v", err)
		return
	}

	logEntry.Infof("add HookRun@OnStartup tasks for %d hooks", len(onStartupHooks))

	for _, hookName := range onStartupHooks {
		newTask := task.NewTask(task.HookRun, hookName).
			WithBinding(OnStartup).
			AppendBindingContext(BindingContext{Binding: ContextBindingType[OnStartup]})
		TasksQueue.Add(newTask)
		logEntry.Debugf("new task HookRun@OnStartup '%s'", hookName)
	}

	return
}

func RunMetrics() {
	// live ticks.
	go func() {
		for {
			MetricsStorage.SendCounterMetric("shell_operator_live_ticks", 1.0, map[string]string{})
			time.Sleep(10 * time.Second)
		}
	}()

	// task queue length
	go func() {
		for {
			queueLen := float64(TasksQueue.Length())
			MetricsStorage.SendGaugeMetric("shell_operator_tasks_queue_length", queueLen, map[string]string{})
			time.Sleep(5 * time.Second)
		}
	}()
}

func InitHttpServer(listenAddr *net.TCPAddr) {
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
		_, _ = io.Copy(writer, TasksQueue.DumpReader())
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
