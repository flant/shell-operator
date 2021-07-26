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
	log "github.com/sirupsen/logrus"
	uuid "gopkg.in/satori/go.uuid.v1"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	structured_logger "github.com/flant/shell-operator/pkg/utils/structured-logger"
	. "github.com/flant/shell-operator/pkg/webhook/validating/types"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/dump"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
	"github.com/flant/shell-operator/pkg/utils/measure"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/validating"
)

var WaitQueuesTimeout = time.Second * 10

type ShellOperator struct {
	ctx    context.Context
	cancel context.CancelFunc

	HooksDir string
	TempDir  string

	MetricStorage *metric_storage.MetricStorage
	// separate metric storage for hook metrics if separate listen port is configured
	HookMetricStorage *metric_storage.MetricStorage
	KubeClient        klient.Client
	ObjectPatcher     *object_patch.ObjectPatcher

	// Labels for kube clients
	MainKubeClientMetricLabels          map[string]string
	ObjectPatcherKubeClientMetricLabels map[string]string

	ScheduleManager   schedule_manager.ScheduleManager
	KubeEventsManager kube_events_manager.KubeEventsManager

	TaskQueues *queue.TaskQueueSet

	ManagerEventsHandler *ManagerEventsHandler

	HookManager hook.HookManager

	ValidatingWebhookManager *validating.WebhookManager
	ConversionWebhookManager *conversion.WebhookManager

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

func (op *ShellOperator) WithKubernetesClient(klient klient.Client) {
	op.KubeClient = klient
}

func (op *ShellOperator) WithMetricStorage(metricStorage *metric_storage.MetricStorage) {
	op.MetricStorage = metricStorage
}

// InitMetricStorage creates default MetricStorage object if not set earlier.
func (op *ShellOperator) InitMetricStorage() {
	if op.MetricStorage != nil {
		return
	}
	metricStorage := metric_storage.NewMetricStorage()
	metricStorage.WithContext(op.ctx)
	metricStorage.WithPrefix(app.PrometheusMetricsPrefix)
	metricStorage.Start()
	RegisterShellOperatorMetrics(metricStorage)
	op.MetricStorage = metricStorage
}

// InitHookMetricStorage creates MetricStorage object
// with new registry to scrape hook metrics on separate port.
func (op *ShellOperator) InitHookMetricStorage() {
	if op.HookMetricStorage != nil {
		return
	}
	metricStorage := metric_storage.NewMetricStorage()
	metricStorage.WithContext(op.ctx)
	metricStorage.WithPrefix(app.PrometheusMetricsPrefix)
	metricStorage.WithNewRegistry()
	metricStorage.Start()
	op.HookMetricStorage = metricStorage
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

	if op.KubeClient == nil {
		//nolint:staticcheck
		klient.RegisterKubernetesClientMetrics(op.MetricStorage, op.GetMainKubeClientMetricLabels())
		op.KubeClient, err = op.InitMainKubeClient()
		if err != nil {
			log.Errorf("MAIN Fatal: initialize 'main' Kubernetes client: %s\n", err)
			return err
		}
	}

	// Initialize ObjectPatcher with its own Kubernetes client.
	patcherKubeClient, err := op.InitObjectPatcherKubeClient()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize 'object_patcher' Kubernetes client: %s\n", err)
		return err
	}
	op.ObjectPatcher = object_patch.NewObjectPatcher(patcherKubeClient)

	// Initialize the task queues set with the "main" queue.
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(op.ctx)
	op.TaskQueues.WithMetricStorage(op.MetricStorage)

	// Initialize schedule manager.
	op.ScheduleManager = schedule_manager.NewScheduleManager()
	op.ScheduleManager.WithContext(op.ctx)

	// Initialize kubernetes events manager.
	op.KubeEventsManager = kube_events_manager.NewKubeEventsManager()
	op.KubeEventsManager.WithKubeClient(op.KubeClient)
	op.KubeEventsManager.WithContext(op.ctx)
	op.KubeEventsManager.WithMetricStorage(op.MetricStorage)

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
	op.HookManager.WithValidatingWebhookManager(op.ValidatingWebhookManager)
	op.HookManager.WithConversionWebhookManager(op.ConversionWebhookManager)
	// Search hooks and load their configurations
	err = op.HookManager.Init()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize hook manager: %s\n", err)
		return err
	}

	// Define event handlers for schedule event and kubernetes event.
	op.ManagerEventsHandler.WithKubeEventHandler(func(kubeEvent KubeEvent) []task.Task {
		logLabels := map[string]string{
			"event.id": uuid.NewV4().String(),
			"binding":  string(OnKubernetesEvent),
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
			"binding":  string(Schedule),
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

// InitWebhookManagers adds kubernetesValidating hooks
// to a WebhookManager and set a validating event handler.
func (op *ShellOperator) InitValidatingWebhookManager() (err error) {
	// Do not init ValidatingWebhook if there are no KubernetesValidating hooks.
	hookNames, _ := op.HookManager.GetHooksInOrder(KubernetesValidating)
	if len(hookNames) == 0 {
		return
	}

	// Initialize validating webhooks manager
	op.ValidatingWebhookManager.WithKubeClient(op.KubeClient)
	op.ValidatingWebhookManager.Settings = app.ValidatingWebhookSettings
	op.ValidatingWebhookManager.Namespace = app.Namespace

	err = op.ValidatingWebhookManager.Init()
	if err != nil {
		log.Errorf("ValidatingWebhookManager init: %v", err)
		return err
	}

	for _, hookName := range hookNames {
		h := op.HookManager.GetHook(hookName)
		h.HookController.EnableValidatingBindings()
	}

	// Define handler for ValidatingEvent
	op.ValidatingWebhookManager.WithValidatingEventHandler(func(event ValidatingEvent) (*ValidatingResponse, error) {
		logLabels := map[string]string{
			"event.id": uuid.NewV4().String(),
			"binding":  string(KubernetesValidating),
		}
		logEntry := log.WithFields(utils.LabelsToLogFields(logLabels))
		logEntry.Debugf("Handle '%s' event '%s' '%s'", string(KubernetesValidating), event.ConfigurationId, event.WebhookId)

		var tasks []task.Task
		op.HookManager.HandleValidatingEvent(event, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(HookRun).
				WithMetadata(HookMetadata{
					HookName:       hook.Name,
					BindingType:    KubernetesValidating,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(logLabels)
			tasks = append(tasks, newTask)
		})

		// Assert exactly one task is created.
		if len(tasks) == 0 {
			logEntry.Errorf("Possible bug!!! No hook found for '%s' event '%s' '%s'", string(KubernetesValidating), event.ConfigurationId, event.WebhookId)
			return nil, fmt.Errorf("no hook found for '%s' '%s'", event.ConfigurationId, event.WebhookId)
		}

		if len(tasks) > 1 {
			logEntry.Errorf("Possible bug!!! %d hooks found for '%s' event '%s' '%s'", len(tasks), string(KubernetesValidating), event.ConfigurationId, event.WebhookId)
		}

		res := op.TaskHandler(tasks[0])

		if res.Status == "Fail" {
			return &ValidatingResponse{
				Allowed: false,
				Message: "Hook failed",
			}, nil
		}

		validatingProp := tasks[0].GetProp("validatingResponse")
		validatingResponse, ok := validatingProp.(*ValidatingResponse)
		if !ok {
			logEntry.Errorf("'validatingResponse' task prop is not of type *ValidatingResponse: %T", validatingProp)
			return nil, fmt.Errorf("hook task prop error")
		}
		return validatingResponse, nil
	})

	err = op.ValidatingWebhookManager.Start()
	if err != nil {
		log.Errorf("ValidatingWebhookManager start: %v", err)
	}
	return err
}

// InitConversionWebhookManager sets a conversions webhook manager.
func (op *ShellOperator) InitConversionWebhookManager() (err error) {
	// Do not init ConversionWebhook if there are no KubernetesConversion hooks.
	hookNames, _ := op.HookManager.GetHooksInOrder(KubernetesConversion)
	if len(hookNames) == 0 {
		return
	}

	// Initialize validating webhooks manager
	op.ConversionWebhookManager.KubeClient = op.KubeClient
	op.ConversionWebhookManager.Settings = app.ConversionWebhookSettings
	op.ConversionWebhookManager.Namespace = app.Namespace
	// This handler is called when Kubernetes requests a conversion.
	op.ConversionWebhookManager.EventHandlerFn = op.ConversionEventHandler

	err = op.ConversionWebhookManager.Init()
	if err != nil {
		log.Errorf("ConversionWebhookManager init: %v", err)
		return err
	}

	for _, hookName := range hookNames {
		h := op.HookManager.GetHook(hookName)
		h.HookController.EnableConversionBindings()
	}

	err = op.ConversionWebhookManager.Start()
	if err != nil {
		log.Errorf("ConversionWebhookManager Start: %v", err)
	}
	return err
}

// ConversionEventHandler is called when Kubernetes requests a conversion.
func (op *ShellOperator) ConversionEventHandler(event conversion.Event) (*conversion.Response, error) {
	logLabels := map[string]string{
		"event.id": uuid.NewV4().String(),
		"binding":  string(KubernetesConversion),
	}
	logEntry := log.WithFields(utils.LabelsToLogFields(logLabels))

	sourceVersions := conversion.ExtractAPIVersions(event.Objects)
	logEntry.Infof("Handle '%s' event for crd/%s: %d objects with versions %v", string(KubernetesConversion), event.CrdName, len(event.Objects), sourceVersions)

	done := false
	for _, srcVer := range sourceVersions {
		rule := conversion.Rule{
			FromVersion: srcVer,
			ToVersion:   event.Review.Request.DesiredAPIVersion,
		}
		convPath := op.HookManager.FindConversionChain(event.CrdName, rule)
		if len(convPath) == 0 {
			continue
		}
		logEntry.Infof("Find conversion path for %s: %v", rule.String(), convPath)

		for _, rule := range convPath {
			var tasks []task.Task
			op.HookManager.HandleConversionEvent(event, rule, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
				newTask := task.NewTask(HookRun).
					WithMetadata(HookMetadata{
						HookName:       hook.Name,
						BindingType:    KubernetesConversion,
						BindingContext: info.BindingContext,
						AllowFailure:   info.AllowFailure,
						Binding:        info.Binding,
						Group:          info.Group,
					}).
					WithLogLabels(logLabels)
				tasks = append(tasks, newTask)
			})

			// Assert exactly one task is created.
			if len(tasks) == 0 {
				logEntry.Errorf("Possible bug!!! No hook found for '%s' event for crd/%s", string(KubernetesConversion), event.CrdName)
				return nil, fmt.Errorf("no hook found for '%s' event for crd/%s", string(KubernetesConversion), event.CrdName)
			}
			if len(tasks) > 1 {
				logEntry.Errorf("Possible bug!!! %d hooks found for '%s' event for crd/%s", len(tasks), string(KubernetesValidating), event.CrdName)
			}

			res := op.TaskHandler(tasks[0])

			if res.Status == "Fail" {
				return &conversion.Response{
					FailedMessage:    fmt.Sprintf("Hook failed to convert to %s", event.Review.Request.DesiredAPIVersion),
					ConvertedObjects: nil,
				}, nil
			}

			prop := tasks[0].GetProp("conversionResponse")
			response, ok := prop.(*conversion.Response)
			if !ok {
				logEntry.Errorf("'conversionResponse' task prop is not of type *conversion.Response: %T", prop)
				return nil, fmt.Errorf("hook task prop error")
			}

			// Set response objects as new objects for a next round.
			event.Objects = response.ConvertedObjects

			// Stop iterating if hook has converted all objects to a desiredAPIVersions.
			newSourceVersions := conversion.ExtractAPIVersions(event.Objects)
			//logEntry.Infof("Hook return conversion response: failMsg=%s, %d convertedObjects, versions:%v, desired: %s", response.FailedMessage, len(response.ConvertedObjects), newSourceVersions, event.Review.Request.DesiredAPIVersion)

			if len(newSourceVersions) == 1 && newSourceVersions[0] == event.Review.Request.DesiredAPIVersion {
				// success
				done = true
				break
			}
		}

		if done {
			break
		}
	}

	if done {
		return &conversion.Response{
			ConvertedObjects: event.Objects,
		}, nil
	}

	return &conversion.Response{
		FailedMessage: fmt.Sprintf("Conversion to %s was not successuful", event.Review.Request.DesiredAPIVersion),
	}, nil
}

// Start
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
		res = op.TaskHandleHookRun(t)

	case EnableKubernetesBindings:
		res = op.TaskHandleEnableKubernetesBindings(t)

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

// TaskHandleEnableKubernetesBindings creates task for each Kubernetes binding in the hook and queues them.
func (op *ShellOperator) TaskHandleEnableKubernetesBindings(t task.Task) queue.TaskResult {
	var hookMeta = HookMetadataAccessor(t)

	metricLabels := map[string]string{
		"hook": hookMeta.HookName,
	}
	defer measure.Duration(func(d time.Duration) {
		op.MetricStorage.GaugeSet("{PREFIX}hook_enable_kubernetes_bindings_seconds", d.Seconds(), metricLabels)
	})()

	var res queue.TaskResult
	hookLogLabels := map[string]string{}
	hookLogLabels["hook"] = hookMeta.HookName
	hookLogLabels["binding"] = ""
	hookLogLabels["task"] = "EnableKubernetesBindings"
	hookLogLabels["queue"] = "main"

	taskLogEntry := log.WithFields(utils.LabelsToLogFields(hookLogLabels))

	taskLogEntry.Info("Enable kubernetes binding for hook")

	taskHook := op.HookManager.GetHook(hookMeta.HookName)

	hookRunTasks := []task.Task{}

	// Run hook for each binding with Synchronization binding context. Ignore queue name here, execute in main queue.
	err := taskHook.HookController.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		newTask := task.NewTask(HookRun).
			WithMetadata(HookMetadata{
				HookName:                 taskHook.Name,
				BindingType:              OnKubernetesEvent,
				BindingContext:           info.BindingContext,
				AllowFailure:             info.AllowFailure,
				Binding:                  info.Binding,
				Group:                    info.Group,
				MonitorIDs:               []string{info.KubernetesBinding.Monitor.Metadata.MonitorId},
				ExecuteOnSynchronization: info.KubernetesBinding.ExecuteHookOnSynchronization,
			}).
			WithLogLabels(hookLogLabels).
			WithQueueName("main")
		hookRunTasks = append(hookRunTasks, newTask)
	})

	success := 0.0
	errors := 0.0
	if err != nil {
		errors = 1.0
		t.UpdateFailureMessage(err.Error())
		taskLogEntry.Errorf("Enable Kubernetes binding for hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount()+1, err)
		res.Status = "Fail"
	} else {
		success = 1.0
		taskLogEntry.Infof("Kubernetes bindings for hook are enabled successfully, %d tasks generated", len(hookRunTasks))
		res.Status = "Success"
		now := time.Now()
		for _, t := range hookRunTasks {
			t.WithQueuedAt(now)
		}
		res.HeadTasks = hookRunTasks
	}

	op.MetricStorage.CounterAdd("{PREFIX}hook_enable_kubernetes_bindings_errors_total", errors, metricLabels)
	op.MetricStorage.GaugeAdd("{PREFIX}hook_enable_kubernetes_bindings_success", success, metricLabels)

	return res
}

// TODO use Context to pass labels and a queue name
func (op *ShellOperator) TaskHandleHookRun(t task.Task) queue.TaskResult {
	var hookMeta = HookMetadataAccessor(t)
	taskHook := op.HookManager.GetHook(hookMeta.HookName)

	err := taskHook.RateLimitWait(context.Background())
	if err != nil {
		// This could happen when the Context is canceled, so just repeat the task until the queue is stopped.
		return queue.TaskResult{
			Status: "Repeat",
		}
	}

	metricLabels := map[string]string{
		"hook":    hookMeta.HookName,
		"binding": hookMeta.Binding,
		"queue":   t.GetQueueName(),
	}
	taskWaitTime := time.Since(t.GetQueuedAt()).Seconds()
	op.MetricStorage.CounterAdd("{PREFIX}task_wait_in_queue_seconds_total", taskWaitTime, metricLabels)

	defer measure.Duration(func(d time.Duration) {
		op.MetricStorage.HistogramObserve("{PREFIX}hook_run_seconds", d.Seconds(), metricLabels, nil)
	})()

	hookLogLabels := map[string]string{}
	hookLogLabels["hook"] = hookMeta.HookName
	hookLogLabels["binding"] = hookMeta.Binding
	hookLogLabels["event"] = string(hookMeta.BindingType)
	hookLogLabels["task"] = "HookRun"
	hookLogLabels["queue"] = t.GetQueueName()

	taskLogEntry := log.WithFields(utils.LabelsToLogFields(hookLogLabels))

	isSynchronization := hookMeta.IsSynchronization()
	shouldRunHook := true
	if isSynchronization {
		// There were no Synchronization for v0 hooks, skip hook execution.
		if taskHook.Config.Version == "v0" {
			shouldRunHook = false
		}
		// Explicit "executeOnSynchronization: false"
		if !hookMeta.ExecuteOnSynchronization {
			shouldRunHook = false
		}
	}

	if shouldRunHook && taskHook.Config.Version == "v1" {
		// Do not combine Synchronization with Event
		shouldCombine := true
		if hookMeta.BindingType == OnKubernetesEvent {
			// Do not combine Synchronizations without group
			if hookMeta.BindingContext[0].Type == TypeSynchronization && hookMeta.Group == "" {
				shouldCombine = false
			}
		}
		if shouldCombine {
			combineResult := op.CombineBindingContextForHook(op.TaskQueues.GetByName(t.GetQueueName()), t, nil)
			if combineResult != nil {
				hookMeta.BindingContext = combineResult.BindingContexts
				// Extra monitor IDs can be returned if several Synchronization for Group are combined.
				if len(combineResult.MonitorIDs) > 0 {
					hookMeta.MonitorIDs = combineResult.MonitorIDs
				}
				t.UpdateMetadata(hookMeta)
			}
		}
	}

	var res queue.TaskResult
	// Default when shouldRunHook is false.
	res.Status = "Success"

	if shouldRunHook {
		taskLogEntry.Info("Execute hook")

		success := 0.0
		errors := 0.0
		allowed := 0.0
		err = op.HandleRunHook(t, taskHook, hookMeta, taskLogEntry, hookLogLabels, metricLabels)
		if err != nil {
			if hookMeta.AllowFailure {
				allowed = 1.0
				taskLogEntry.Infof("Hook failed, but allowed to fail: %v", err)
				res.Status = "Success"
			} else {
				errors = 1.0
				t.UpdateFailureMessage(err.Error())
				t.WithQueuedAt(time.Now()) // Reset queueAt for correct results in 'task_wait_in_queue' metric.
				taskLogEntry.Errorf("Hook failed. Will retry after delay. Failed count is %d. Error: %s", t.GetFailureCount()+1, err)
				res.Status = "Fail"
			}
		} else {
			success = 1.0
			taskLogEntry.Infof("Hook executed successfully")
			res.Status = "Success"
		}
		op.MetricStorage.CounterAdd("{PREFIX}hook_run_allowed_errors_total", allowed, metricLabels)
		op.MetricStorage.CounterAdd("{PREFIX}hook_run_errors_total", errors, metricLabels)
		op.MetricStorage.CounterAdd("{PREFIX}hook_run_success_total", success, metricLabels)
	}

	// Unlock Kubernetes events for all monitors when Synchronization task is done.
	if isSynchronization && res.Status == "Success" {
		taskLogEntry.Info("Unlock kubernetes.Event tasks")
		for _, monitorID := range hookMeta.MonitorIDs {
			taskHook.HookController.UnlockKubernetesEventsFor(monitorID)
		}
	}

	return res
}

func (op *ShellOperator) HandleRunHook(t task.Task, taskHook *hook.Hook, hookMeta HookMetadata, taskLogEntry *log.Entry, hookLogLabels map[string]string, metricLabels map[string]string) error {
	result, err := taskHook.Run(hookMeta.BindingType, hookMeta.BindingContext, hookLogLabels)
	if err != nil {
		return err
	}

	if result != nil && result.Usage != nil {
		taskLogEntry.Infof("Usage: %+v", result.Usage)
		op.MetricStorage.HistogramObserve("{PREFIX}hook_run_sys_seconds", result.Usage.Sys.Seconds(), metricLabels, nil)
		op.MetricStorage.HistogramObserve("{PREFIX}hook_run_user_seconds", result.Usage.User.Seconds(), metricLabels, nil)
		op.MetricStorage.GaugeSet("{PREFIX}hook_run_max_rss_bytes", float64(result.Usage.MaxRss)*1024, metricLabels)
	}

	// Try to apply Kubernetes actions.
	if len(result.KubernetesPatchBytes) > 0 {
		var operations []*object_patch.Operation
		operations, err = object_patch.ParseOperations(result.KubernetesPatchBytes)
		if err != nil {
			return err
		}
		err = op.ObjectPatcher.ExecuteOperations(operations)
		if err != nil {
			return err
		}
	}

	// Try to update custom metrics
	err = op.HookMetricStorage.SendBatch(result.Metrics, map[string]string{
		"hook": hookMeta.HookName,
	})
	if err != nil {
		return err
	}

	// Save validatingResponse in task props for future use.
	if result.ValidatingResponse != nil {
		t.SetProp("validatingResponse", result.ValidatingResponse)
		taskLogEntry.Infof("ValidatingResponse from hook: %s", result.ValidatingResponse.Dump())
	}

	// Save conversionResponse in task props for future use.
	if result.ConversionResponse != nil {
		t.SetProp("conversionResponse", result.ConversionResponse)
		taskLogEntry.Infof("ConversionResponse from hook: %s", result.ConversionResponse.Dump())
	}

	return nil
}

type CombineResult struct {
	BindingContexts []BindingContext
	MonitorIDs      []string
}

// CombineBindingContextForHook combines binding contexts from a sequence of task with similar
// hook name and task type into array of binding context and delete excess tasks from queue.
//
// Also, sequences of binding contexts with similar group are compacted in one binding context.
//
// If input task has no metadata, result will be nil.
// Metadata should implement HookNameAccessor, BindingContextAccessor and MonitorIDAccessor interfaces.
// DEV WARNING! Do not use HookMetadataAccessor here. Use only *Accessor interfaces because this method is used from addon-operator.
func (op *ShellOperator) CombineBindingContextForHook(q *queue.TaskQueue, t task.Task, stopCombineFn func(tsk task.Task) bool) *CombineResult {
	if q == nil {
		return nil
	}
	var taskMeta = t.GetMetadata()
	if taskMeta == nil {
		// Ignore task without metadata
		return nil
	}
	var hookName = taskMeta.(HookNameAccessor).GetHookName()

	var res = new(CombineResult)

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
		// Stop on task without metadata
		if hm == nil {
			stopIterate = true
			return
		}
		nextHookName := hm.(HookNameAccessor).GetHookName()
		// Only tasks for the same hook and of the same type can be combined (HookRun cannot be combined with OnStartup).
		// Using stopCombineFn function more stricter combine rules can be defined.
		if nextHookName == hookName && t.GetType() == tsk.GetType() {
			if stopCombineFn != nil {
				stopIterate = stopCombineFn(tsk)
			}
		} else {
			stopIterate = true
		}
		if !stopIterate {
			otherTasks = append(otherTasks, tsk)
		}
	})

	// no tasks found to combine
	if len(otherTasks) == 0 {
		return nil
	}

	// Combine binding context and make a map to delete excess tasks
	var combinedContext = make([]BindingContext, 0)
	var monitorIDs = taskMeta.(MonitorIDAccessor).GetMonitorIDs()
	var tasksFilter = make(map[string]bool)
	// current task always remain in queue
	combinedContext = append(combinedContext, taskMeta.(BindingContextAccessor).GetBindingContext()...)
	tasksFilter[t.GetId()] = true
	for _, tsk := range otherTasks {
		combinedContext = append(combinedContext, tsk.GetMetadata().(BindingContextAccessor).GetBindingContext()...)
		tskMonitorIDs := tsk.GetMetadata().(MonitorIDAccessor).GetMonitorIDs()
		if len(tskMonitorIDs) > 0 {
			monitorIDs = append(monitorIDs, tskMonitorIDs...)
		}
		tasksFilter[tsk.GetId()] = false
	}

	// Delete tasks with false in tasksFilter map
	op.TaskQueues.GetByName(t.GetQueueName()).Filter(func(tsk task.Task) bool {
		if v, ok := tasksFilter[tsk.GetId()]; ok {
			return v
		}
		return true
	})

	// group is used to compact binding contexts when only snapshots are needed
	var compactedContext = make([]BindingContext, 0)
	for i := 0; i < len(combinedContext); i++ {
		var keep = true

		// Binding context is ignored if next binding context has the similar group.
		var groupName = combinedContext[i].Metadata.Group
		if groupName != "" && (i+1 <= len(combinedContext)-1) && combinedContext[i+1].Metadata.Group == groupName {
			keep = false
		}

		if keep {
			compactedContext = append(compactedContext, combinedContext[i])
		}
	}

	// Describe what was done.
	compactMsg := ""
	if len(compactedContext) < len(combinedContext) {
		compactMsg = fmt.Sprintf("are combined and compacted to %d contexts", len(compactedContext))
	} else {
		compactMsg = fmt.Sprintf("are combined to %d contexts", len(combinedContext))
	}
	log.Infof("Binding contexts from %d tasks %s. %d tasks are dropped from queue '%s'", len(otherTasks)+1, compactMsg, len(tasksFilter)-1, t.GetQueueName())

	res.BindingContexts = compactedContext
	res.MonitorIDs = monitorIDs
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

	// Add tasks to run OnStartup bindings
	onStartupHooks, err := op.HookManager.GetHooksInOrder(OnStartup)
	if err != nil {
		logEntry.Errorf("%v", err)
		return
	}

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
			}).
			WithQueuedAt(time.Now())
		mainQueue.AddLast(newTask)
		logEntry.Infof("queue task %s with hook %s", newTask.GetDescription(), hookName)
	}

	// Add tasks to enable kubernetes monitors and schedules for each hook
	for _, hookName := range op.HookManager.GetHookNames() {
		h := op.HookManager.GetHook(hookName)

		if h.GetConfig().HasBinding(OnKubernetesEvent) {
			newTask := task.NewTask(EnableKubernetesBindings).
				WithMetadata(HookMetadata{
					HookName: hookName,
					Binding:  string(EnableKubernetesBindings),
				}).
				WithQueuedAt(time.Now())
			mainQueue.AddLast(newTask)
			logEntry.Infof("queue task %s for hook %s", newTask.GetDescription(), hookName)
		}

		if h.GetConfig().HasBinding(Schedule) {
			newTask := task.NewTask(EnableScheduleBindings).
				WithMetadata(HookMetadata{
					HookName: hookName,
					Binding:  string(EnableScheduleBindings),
				}).
				WithQueuedAt(time.Now())
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
			op.MetricStorage.CounterAdd("{PREFIX}live_ticks", 1.0, map[string]string{})
			time.Sleep(10 * time.Second)
		}
	}()

	// task queue length
	go func() {
		for {
			op.TaskQueues.Iterate(func(queue *queue.TaskQueue) {
				queueLen := float64(queue.Length())
				op.MetricStorage.GaugeSet("{PREFIX}tasks_queue_length", queueLen, map[string]string{"queue": queue.Name})
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
		structured_logger.GetLogEntry(request).Debugf("queue list using format %s", format)
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

	http.Handle("/metrics", op.MetricStorage.Handler())
}

func (op *ShellOperator) StartHttpServer(ip string, port string, mux *http.ServeMux) error {
	address := fmt.Sprintf("%s:%s", ip, port)

	// Check if port is available
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Errorf("Fail to listen on '%s': %v", address, err)
		return err
	}

	log.Infof("Listen on %s", address)

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Errorf("Error starting HTTP server: %s", err)
			os.Exit(1)
		}
	}()

	return nil
}

func (op *ShellOperator) SetupHookMetricStorageAndServer() error {
	if op.HookMetricStorage != nil {
		return nil
	}
	if app.HookMetricsListenPort == "" || app.HookMetricsListenPort == app.ListenPort {
		// register default prom handler in DefaultServeMux
		op.HookMetricStorage = op.MetricStorage
	} else {
		// create new metric storage for hooks
		op.InitHookMetricStorage()
		// Create new ServeMux, serve on custom port
		mux := http.NewServeMux()
		err := op.StartHttpServer(app.ListenAddress, app.HookMetricsListenPort, mux)
		if err != nil {
			return err
		}
		// register scrape handler
		mux.Handle("/metrics", op.HookMetricStorage.Handler())
	}
	return nil
}

func DefaultOperator() *ShellOperator {
	operator := NewShellOperator()
	operator.WithContext(context.Background())
	return operator
}

func InitAndStart(operator *ShellOperator) error {
	err := operator.StartHttpServer(app.ListenAddress, app.ListenPort, http.DefaultServeMux)
	if err != nil {
		log.Errorf("HTTP SERVER start failed: %v", err)
		return err
	}
	operator.InitMetricStorage()
	operator.SetupHttpServerHandles()

	err = operator.SetupHookMetricStorageAndServer()
	if err != nil {
		log.Errorf("HTTP SERVER for hook metrics start failed: %v", err)
		return err
	}

	err = operator.Init()
	if err != nil {
		log.Errorf("INIT failed: %s", err)
		return err
	}

	operator.SetupDebugServerHandles()

	// Create webhookManagers before hook loading to set them on HookControllers.
	operator.ValidatingWebhookManager = validating.NewWebhookManager()
	operator.ConversionWebhookManager = conversion.NewWebhookManager()

	err = operator.InitHookManager()
	if err != nil {
		log.Errorf("INIT HookManager failed: %s", err)
		return err
	}

	// Setup validating configs from hooks.
	err = operator.InitValidatingWebhookManager()
	if err != nil {
		log.Errorf("INIT ValidatingWebhookManager failed: %s", err)
		return err
	}

	// Setup conversion configs from hooks.
	err = operator.InitConversionWebhookManager()
	if err != nil {
		log.Errorf("INIT ConversionWebhookManager failed: %s", err)
		return err
	}

	operator.Start()

	return nil
}

// Shutdown pause kubernetes events handling and stop queues. Wait for queues to stop.
func (op *ShellOperator) Shutdown() {
	op.ScheduleManager.Stop()
	op.KubeEventsManager.PauseHandleEvents()
	op.TaskQueues.Stop()
	// Wait for queues to stop, but no more than 10 seconds
	op.TaskQueues.WaitStopWithTimeout(WaitQueuesTimeout)
}
