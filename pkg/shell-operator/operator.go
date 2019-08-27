package shell_operator

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/romana/rlog"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/hook"
	kube_event_hook "github.com/flant/shell-operator/pkg/hook/kube_event"
	schedule_hook "github.com/flant/shell-operator/pkg/hook/schedule"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/metrics_storage"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

var (
	WorkingDir string
	TempDir    string

	TasksQueue *task.TasksQueue

	HookManager hook.HookManager

	ScheduleManager schedule_manager.ScheduleManager
	ScheduledHooks  schedule_hook.ScheduledHooksStorage

	KubeEventsManager kube_events_manager.KubeEventsManager
	KubeEventsHooks   kube_event_hook.KubeEventsHooksController

	MetricsStorage *metrics_storage.MetricStorage

	// StopHandleEventsFromManagersCh channel is used to stop loop of the HandleEventsFromManagers.
	StopHandleEventsFromManagersCh chan struct{}
)

var (
	QueueIsEmptyDelay = 3 * time.Second
	FailedHookDelay   = 5 * time.Second
)

// Start is an implementation of a start command
func Init() (err error) {
	// Init phase
	// Collecting the settings: directories, kube config
	// Initializing all necessary objects: hook manager,kube events manager, schedule manager
	// Creating an empty queue with onStartup tasks.
	rlog.Debug("MAIN Init")

	if app.WorkingDir != "" {
		WorkingDir = app.WorkingDir
	} else {
		WorkingDir, err = os.Getwd()
		if err != nil {
			rlog.Errorf("MAIN Fatal: Cannot determine a working dir: %s", err)
			return err
		}
		rlog.Infof("Working dir: %s", WorkingDir)
	}

	TempDir = app.TempDir
	if exists, _ := utils_file.DirExists(TempDir); !exists {
		err = os.Mkdir(TempDir, os.FileMode(0777))
		if err != nil {
			rlog.Errorf("MAIN Fatal: Cannot create a temporary dir: %s", err)
			os.Exit(1)
		}
	}
	rlog.Infof("Use temporary dir: %s", TempDir)

	// Initializing hook manager (load hooks from WorkingDir)
	HookManager, err = hook.Init(WorkingDir, TempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialize hook manager: %s\n", err)
		return err
	}

	// Initializing the empty task queue.
	TasksQueue = task.NewTasksQueue()

	// Initializing the hooks schedule.
	ScheduleManager, err = schedule_manager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize schedule manager: %s", err)
		os.Exit(1)
	}

	// Initialize kube client and kube events informers for kube events hooks.

	err = kube.Init(kube.InitOptions{KubeContext: app.KubeContext, KubeConfig: app.KubeConfig})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialize kube client: %s\n", err)
		return err
	}

	KubeEventsManager, err = kube_events_manager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize kube events manager: %s", err)
		os.Exit(1)
	}
	KubeEventsHooks = kube_event_hook.NewMainKubeEventsHooksController()

	// Initialiaze prometheus client
	MetricsStorage = metrics_storage.Init()

	return nil
}

func Run() {
	rlog.Info("MAIN: run main loop")

	rlog.Info("MAIN: add onStartup tasks")

	//go HookManager.Run()
	go ScheduleManager.Run()

	// Metric storage
	go MetricsStorage.Run()

	// Managers are generating event. This go-routine handles all events and converts them into queued tasks.
	go HandleEventsFromManagers()

	// TasksRunner runs tasks from the queue.
	go TasksRunner()

	RunMetrics()
}

func HandleEventsFromManagers() {
	for {
		select {
		case hooksEvent := <-hook.EventCh:
			if hooksEvent.Type == hook.HooksLoaded {
				// Load queue with onStartup tasks
				TasksQueue.ChangesDisable()
				CreateOnStartupTasks()
				TasksQueue.ChangesEnable(true)

				// add schedules to schedule manager
				ScheduledHooks = UpdateScheduledHooks(ScheduledHooks)
				// start informers for kube events
				err := KubeEventsHooks.EnableHooks(HookManager, KubeEventsManager)
				if err != nil {
					// Something wrong with hooks configs...
					rlog.Errorf("Enable kube events for hooks error: %v", err)
					TasksQueue.Add(task.NewTask(task.Exit, "exit"))
					return
				}
			}
		case crontab := <-schedule_manager.ScheduleCh:
			scheduleHooks := ScheduledHooks.GetHooksForSchedule(crontab)
			for _, schHook := range scheduleHooks {
				var getHookErr error

				_, getHookErr = HookManager.GetHook(schHook.HookName)
				if getHookErr == nil {
					newTask := task.NewTask(task.HookRun, schHook.HookName).
						WithBinding(hook.Schedule).
						AppendBindingContext(hook.BindingContext{Binding: schHook.ConfigName}).
						WithAllowFailure(schHook.AllowFailure)
					TasksQueue.Add(newTask)
					rlog.Debugf("QUEUE add HookRun@Schedule '%s'", schHook.HookName)
				}

				rlog.Errorf("MAIN_LOOP hook '%s' scheduled but not found by hook_manager", schHook.HookName)
			}
		case kubeEvent := <-kube_events_manager.KubeEventCh:
			rlog.Infof("EVENT Kube event '%s'", kubeEvent.ConfigId)

			tasks, err := KubeEventsHooks.HandleEvent(kubeEvent)
			if err != nil {
				rlog.Errorf("MAIN_LOOP error handling kube event '%s': %s", kubeEvent.ConfigId, err)
				break
			}

			for _, resTask := range tasks {
				TasksQueue.Add(resTask)
				rlog.Infof("QUEUE add %s@%s %s", resTask.GetType(), resTask.GetBinding(), resTask.GetName())
			}
		case <-StopHandleEventsFromManagersCh:
			rlog.Infof("EVENT Stop")
			return
		}
	}
}

func TasksRunner() {
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
				rlog.Infof("TASK_RUN HookRun@%s %s", t.GetBinding(), t.GetName())
				err := HookManager.RunHook(t.GetName(), t.GetBinding(), t.GetBindingContext())
				if err != nil {
					taskHook, _ := HookManager.GetHook(t.GetName())
					hookLabel := taskHook.SafeName()

					if t.GetAllowFailure() {
						MetricsStorage.SendCounterMetric("shell_operator_hook_allowed_errors", 1.0, map[string]string{"hook": hookLabel})
						TasksQueue.Pop()
					} else {
						MetricsStorage.SendCounterMetric("shell_operator_hook_errors", 1.0, map[string]string{"hook": hookLabel})
						t.IncrementFailureCount()
						rlog.Errorf("TASK_RUN %s '%s' on '%s' failed. Will retry after delay. Failed count is %d. Error: %s", t.GetType(), t.GetName(), t.GetBinding(), t.GetFailureCount(), err)
						TasksQueue.Push(task.NewTaskDelay(FailedHookDelay))
					}

				} else {
					TasksQueue.Pop()
				}

			case task.Delay:
				rlog.Infof("TASK_RUN Delay for %s", t.GetDelay().String())
				TasksQueue.Pop()
				time.Sleep(t.GetDelay())
			case task.Stop:
				rlog.Infof("TASK_RUN Stop: Exiting TASK_RUN loop.")
				TasksQueue.Pop()
				return
			case task.Exit:
				rlog.Infof("TASK_RUN Exit: program halts.")
				TasksQueue.Pop()
				os.Exit(1)
			}

			// Breaking, if the task queue is empty to prevent the infinite loop.
			if TasksQueue.IsEmpty() {
				rlog.Debug("Task queue is empty. Will sleep now.")
				break
			}
		}
	}
}

func CreateOnStartupTasks() {
	rlog.Infof("QUEUE add all HookRun@OnStartup")

	onStartupHooks := HookManager.GetHooksInOrder(hook.OnStartup)

	for _, hookName := range onStartupHooks {
		newTask := task.NewTask(task.HookRun, hookName).
			WithBinding(hook.OnStartup).
			AppendBindingContext(hook.BindingContext{Binding: hook.ContextBindingType[hook.OnStartup]})
		TasksQueue.Add(newTask)
		rlog.Debugf("QUEUE add HookRun@OnStartup '%s'", hookName)
	}

	return
}

func UpdateScheduledHooks(storage schedule_hook.ScheduledHooksStorage) schedule_hook.ScheduledHooksStorage {
	if ScheduleManager == nil {
		return nil
	}

	// map of crontabs that should be stopped in scheduleManager
	oldCrontabs := map[string]bool{}
	if storage != nil {
		for _, crontab := range storage.GetCrontabs() {
			oldCrontabs[crontab] = false
		}
	}

	newStorage := schedule_hook.ScheduledHooksStorage{}

	hooks := HookManager.GetHooksInOrder(hook.Schedule)

	for _, hookName := range hooks {
		hmHook, _ := HookManager.GetHook(hookName)
		for _, schedule := range hmHook.Config.Schedules {
			_, err := ScheduleManager.Add(schedule.Crontab)
			if err != nil {
				rlog.Errorf("Schedule: cannot add '%s' for hook '%s': %s", schedule.Crontab, hookName, err)
				continue
			}
			rlog.Debugf("Schedule: add '%s' for hook '%s'", schedule.Crontab, hookName)
			newStorage.AddHook(hmHook.Name, schedule)
		}
	}

	if len(oldCrontabs) > 0 {
		// Creates a new set of schedules. If the schedule is in oldCrontabs, then set it to true.
		newCrontabs := newStorage.GetCrontabs()
		for _, crontab := range newCrontabs {
			if _, has_crontab := oldCrontabs[crontab]; has_crontab {
				oldCrontabs[crontab] = true
			}
		}

		// Stop crontabs that was not added to new storage.
		for crontab, _ := range oldCrontabs {
			if !oldCrontabs[crontab] {
				ScheduleManager.Remove(crontab)
			}
		}
	}

	return newStorage
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
		writer.Write([]byte(`<html>
    <head><title>Shell operator</title></head>
    <body>
    <h1>Shell operator</h1>
    <pre>go tool pprof goprofex http://SHELL_OPERATOR_IP:9115/debug/pprof/profile</pre>
    </body>
    </html>`))
	})

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/queue", func(writer http.ResponseWriter, request *http.Request) {
		io.Copy(writer, TasksQueue.DumpReader())
	})

	go func() {
		rlog.Infof("HTTP SERVER Listening on %s", listenAddr.String())
		if err := http.ListenAndServe(listenAddr.String(), nil); err != nil {
			rlog.Errorf("Error starting HTTP server: %s", err)
		}
	}()
}
