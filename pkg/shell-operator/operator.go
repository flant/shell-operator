package shell_operator

import (
	"context"
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

	ScheduleManager         schedule_manager.ScheduleManager
	ScheduleHooksController schedule_hook.ScheduleHooksController

	KubeEventsManager         kube_events_manager.KubeEventsManager
	KubernetesHooksController kube_event_hook.KubernetesHooksController

	MetricsStorage *metrics_storage.MetricStorage

	// StopHandleEventsFromManagersCh channel is used to stop loop of the HandleEventsFromManagers.
	StopHandleEventsFromManagersCh chan struct{}
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
	rlog.Debug("MAIN Init")

	if app.WorkingDir != "" {
		WorkingDir = app.WorkingDir
	} else {
		WorkingDir, err = os.Getwd()
		if err != nil {
			rlog.Errorf("MAIN Fatal: Cannot determine a working dir: %s", err)
			return err
		}
	}
	if exists, _ := utils_file.DirExists(WorkingDir); !exists {
		rlog.Errorf("MAIN Fatal: working dir '%s' is not exists", WorkingDir)
		return fmt.Errorf("no working dir")
	}
	rlog.Infof("Working dir: %s", WorkingDir)

	TempDir = app.TempDir
	if exists, _ := utils_file.DirExists(TempDir); !exists {
		err = os.Mkdir(TempDir, os.FileMode(0777))
		if err != nil {
			rlog.Errorf("MAIN Fatal: Cannot create a temporary dir: %s", err)
			return err
		}
	}
	rlog.Infof("Use temporary dir: %s", TempDir)

	// Initializing hook manager (load hooks from WorkingDir)
	HookManager = hook.NewHookManager()
	HookManager.WithDirectories(WorkingDir, TempDir)
	err = HookManager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: initialize hook manager: %s\n", err)
		return err
	}

	// Initializing the empty task queue.
	TasksQueue = task.NewTasksQueue()

	// Initializing the hooks schedule.
	ScheduleManager, err = schedule_manager.Init()
	if err != nil {
		rlog.Errorf("MAIN Fatal: initialize schedule manager: %s", err)
		return err
	}

	ScheduleHooksController = schedule_hook.NewScheduleHooksController()
	ScheduleHooksController.WithHookManager(HookManager)
	ScheduleHooksController.WithScheduleManager(ScheduleManager)

	// Initialize kube client for kube events hooks.
	err = kube.Init(kube.InitOptions{KubeContext: app.KubeContext, KubeConfig: app.KubeConfig})
	if err != nil {
		rlog.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}

	KubeEventsManager = kube_events_manager.NewKubeEventsManager()
	KubeEventsManager.WithContext(context.Background())

	KubernetesHooksController = kube_event_hook.NewKubernetesHooksController()
	KubernetesHooksController.WithHookManager(HookManager)
	KubernetesHooksController.WithKubeEventsManager(KubeEventsManager)

	// Initialiaze prometheus client
	MetricsStorage = metrics_storage.Init()

	return nil
}

func Run() {
	rlog.Info("MAIN: run main loop")

	// Metric storage and live metrics
	go MetricsStorage.Run()
	RunMetrics()

	// Load queue with onStartup tasks
	TasksQueue.ChangesDisable()
	CreateOnStartupTasks()
	TasksQueue.ChangesEnable(true)

	// Managers are generating events. This go-routine handles all events and converts them into queued tasks.
	// Start it before start all informers to catch all kubernetes events (#42)
	go HandleEventsFromManagers()

	// create informers for all kubernetes hooks
	err := KubernetesHooksController.EnableHooks()
	if err != nil {
		// Something wrong with hook configs, cannot start informers.
		rlog.Errorf("Start informers for kubernetes hooks: %v", err)
		return
	}
	// Start all created informers
	KubeEventsManager.Start()

	// add schedules to schedule manager
	ScheduleHooksController.UpdateScheduleHooks()
	go ScheduleManager.Run()

	// TasksRunner runs tasks from the queue.
	go TasksRunner()
}

func HandleEventsFromManagers() {
	for {
		select {
		case crontab := <-schedule_manager.ScheduleCh:
			rlog.Infof("EVENT Schedule event '%s'", crontab)

			tasks, err := ScheduleHooksController.HandleEvent(crontab)
			if err != nil {
				rlog.Errorf("MAIN_LOOP error handling Schedule event '%s': %s", crontab, err)
				break
			}

			for _, resTask := range tasks {
				TasksQueue.Add(resTask)
				rlog.Infof("QUEUE add %s@%s %s", resTask.GetType(), resTask.GetBinding(), resTask.GetName())
			}

		case kubeEvent := <-kube_events_manager.KubeEventCh:
			rlog.Infof("EVENT Kube event '%s'", kubeEvent.ConfigId)

			tasks, err := KubernetesHooksController.HandleEvent(kubeEvent)
			if err != nil {
				rlog.Errorf("MAIN_LOOP error handling kubernetes event '%s': %s", kubeEvent.ConfigId, err)
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
		rlog.Infof("HTTP SERVER Listening on %s", listenAddr.String())
		if err := http.ListenAndServe(listenAddr.String(), nil); err != nil {
			rlog.Errorf("Error starting HTTP server: %s", err)
		}
	}()
}
