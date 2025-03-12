package shell_operator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/gofrs/uuid/v5"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/hook"
	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/hook/types"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemTypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
	"github.com/flant/shell-operator/pkg/utils/measure"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

var WaitQueuesTimeout = time.Second * 10

type ShellOperator struct {
	ctx    context.Context
	cancel context.CancelFunc

	logger *log.Logger

	// APIServer common http server for liveness and metrics endpoints
	APIServer *baseHTTPServer

	// MetricStorage collects and store metrics for built-in operator primitives, hook execution
	MetricStorage metric.Storage
	// HookMetricStorage separate metric storage for metrics, which are returned by user hooks
	HookMetricStorage metric.Storage
	KubeClient        *klient.Client
	ObjectPatcher     *objectpatch.ObjectPatcher

	ScheduleManager   schedulemanager.ScheduleManager
	KubeEventsManager kubeeventsmanager.KubeEventsManager

	TaskQueues *queue.TaskQueueSet

	ManagerEventsHandler *ManagerEventsHandler

	HookManager *hook.Manager

	AdmissionWebhookManager  *admission.WebhookManager
	ConversionWebhookManager *conversion.WebhookManager
}

type Option func(operator *ShellOperator)

func WithLogger(logger *log.Logger) Option {
	return func(operator *ShellOperator) {
		operator.logger = logger
	}
}

func NewShellOperator(ctx context.Context, opts ...Option) *ShellOperator {
	if ctx == nil {
		ctx = context.Background()
	}

	cctx, cancel := context.WithCancel(ctx)

	so := &ShellOperator{
		ctx:    cctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		opt(so)
	}

	if so.logger == nil {
		so.logger = log.NewLogger(log.Options{}).Named("shell-operator")
	}

	return so
}

// Start run the operator
func (op *ShellOperator) Start() {
	log.Info("start shell-operator")

	op.APIServer.Start(op.ctx)

	// Create 'main' queue and add onStartup tasks and enable bindings tasks.
	op.bootstrapMainQueue(op.TaskQueues)
	// Start main task queue handler
	op.TaskQueues.StartMain()
	op.initAndStartHookQueues()

	// Start emit "live" metrics
	op.runMetrics()

	// Managers are generating events. This go-routine handles all events and converts them into queued tasks.
	// Start it before start all informers to catch all kubernetes events (#42)
	op.ManagerEventsHandler.Start()

	// Unlike KubeEventsManager, ScheduleManager has one go-routine.
	op.ScheduleManager.Start()
}

func (op *ShellOperator) Stop() {
	if op.cancel != nil {
		op.cancel()
	}
}

// initHookManager load hooks from HooksDir and defines event handlers that emit tasks.
func (op *ShellOperator) initHookManager() error {
	if op.HookManager == nil {
		return nil
	}
	// Search hooks and load their configurations
	if err := op.HookManager.Init(); err != nil {
		return fmt.Errorf("MAIN Fatal: initialize hook manager: %w", err)
	}

	// Define event handlers for schedule event and kubernetes event.
	op.ManagerEventsHandler.WithKubeEventHandler(func(kubeEvent kemTypes.KubeEvent) []task.Task {
		logLabels := map[string]string{
			"event.id": uuid.Must(uuid.NewV4()).String(),
			"binding":  string(types.OnKubernetesEvent),
		}
		logEntry := utils.EnrichLoggerWithLabels(op.logger, logLabels)
		logEntry.Debug("Create tasks for 'kubernetes' event", slog.String("name", kubeEvent.String()))

		var tasks []task.Task
		op.HookManager.HandleKubeEvent(kubeEvent, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(task_metadata.HookRun).
				WithMetadata(task_metadata.HookMetadata{
					HookName:       hook.Name,
					BindingType:    types.OnKubernetesEvent,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(logLabels).
				WithQueueName(info.QueueName)
			tasks = append(tasks, newTask.WithQueuedAt(time.Now()))

			logEntry.With("queue", info.QueueName).
				Info("queue task", slog.String("name", newTask.GetDescription()))
		})

		return tasks
	})
	op.ManagerEventsHandler.WithScheduleEventHandler(func(crontab string) []task.Task {
		logLabels := map[string]string{
			"event.id": uuid.Must(uuid.NewV4()).String(),
			"binding":  string(types.Schedule),
		}
		logEntry := utils.EnrichLoggerWithLabels(op.logger, logLabels)
		logEntry.Debug("Create tasks for 'schedule' event", slog.String("name", crontab))

		var tasks []task.Task
		op.HookManager.HandleScheduleEvent(crontab, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(task_metadata.HookRun).
				WithMetadata(task_metadata.HookMetadata{
					HookName:       hook.Name,
					BindingType:    types.Schedule,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(logLabels).
				WithQueueName(info.QueueName)
			tasks = append(tasks, newTask.WithQueuedAt(time.Now()))

			logEntry.With("queue", info.QueueName).
				Info("queue task", slog.String("name", newTask.GetDescription()))
		})

		return tasks
	})

	return nil
}

// initValidatingWebhookManager adds kubernetesValidating hooks
// to a WebhookManager and set a validating event handler.
func (op *ShellOperator) initValidatingWebhookManager() error {
	if op.HookManager == nil || op.AdmissionWebhookManager == nil {
		return nil
	}
	// Do not init ValidatingWebhook if there are no KubernetesValidating hooks.
	hookNamesV, _ := op.HookManager.GetHooksInOrder(types.KubernetesValidating)
	hookNamesM, _ := op.HookManager.GetHooksInOrder(types.KubernetesMutating)

	var hookNames []string

	hookNames = append(hookNames, hookNamesV...)
	hookNames = append(hookNames, hookNamesM...)

	if len(hookNames) == 0 {
		return nil
	}

	err := op.AdmissionWebhookManager.Init()
	if err != nil {
		return fmt.Errorf("ValidatingWebhookManager init: %w", err)
	}

	for _, hookName := range hookNames {
		h := op.HookManager.GetHook(hookName)
		h.HookController.EnableAdmissionBindings()
	}

	// Define handler for AdmissionEvent
	op.AdmissionWebhookManager.WithAdmissionEventHandler(func(event admission.Event) (*admission.Response, error) {
		eventBindingType := op.HookManager.DetectAdmissionEventType(event)
		logLabels := map[string]string{
			"event.id": uuid.Must(uuid.NewV4()).String(),
			"event":    string(eventBindingType),
		}
		logEntry := utils.EnrichLoggerWithLabels(op.logger, logLabels)
		logEntry.Debug("Handle event",
			slog.String("type", string(eventBindingType)),
			slog.String("configurationId", event.ConfigurationId),
			slog.String("webhookID", event.WebhookId))

		var admissionTask task.Task
		op.HookManager.HandleAdmissionEvent(event, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
			newTask := task.NewTask(task_metadata.HookRun).
				WithMetadata(task_metadata.HookMetadata{
					HookName:       hook.Name,
					BindingType:    eventBindingType,
					BindingContext: info.BindingContext,
					AllowFailure:   info.AllowFailure,
					Binding:        info.Binding,
					Group:          info.Group,
				}).
				WithLogLabels(logLabels)
			admissionTask = newTask
		})

		// Assert exactly one task is created.
		if admissionTask == nil {
			logEntry.Error("Possible bug!!! No hook found for event",
				slog.String("type", string(types.KubernetesValidating)),
				slog.String("configurationId", event.ConfigurationId),
				slog.String("webhookID", event.WebhookId))
			return nil, fmt.Errorf("no hook found for '%s' '%s'", event.ConfigurationId, event.WebhookId)
		}

		res := op.taskHandler(admissionTask)

		if res.Status == "Fail" {
			return &admission.Response{
				Allowed: false,
				Message: "Hook failed",
			}, nil
		}

		admissionProp := admissionTask.GetProp("admissionResponse")
		admissionResponse, ok := admissionProp.(*admission.Response)
		if !ok {
			logEntry.Error("'admissionResponse' task prop is not of type *AdmissionResponse",
				slog.String("type", fmt.Sprintf("%T", admissionProp)))
			return nil, fmt.Errorf("hook task prop error")
		}
		return admissionResponse, nil
	})

	if err := op.AdmissionWebhookManager.Start(); err != nil {
		return fmt.Errorf("ValidatingWebhookManager start: %w", err)
	}

	return nil
}

// initConversionWebhookManager creates and starts a conversion webhook manager.
func (op *ShellOperator) initConversionWebhookManager() error {
	if op.HookManager == nil || op.ConversionWebhookManager == nil {
		return nil
	}

	// Do not init ConversionWebhook if there are no KubernetesConversion hooks.
	hookNames, _ := op.HookManager.GetHooksInOrder(types.KubernetesConversion)
	if len(hookNames) == 0 {
		return nil
	}

	// This handler is called when Kubernetes requests a conversion.
	op.ConversionWebhookManager.EventHandlerFn = op.conversionEventHandler

	err := op.ConversionWebhookManager.Init()
	if err != nil {
		return fmt.Errorf("ConversionWebhookManager init: %w", err)
	}

	for _, hookName := range hookNames {
		h := op.HookManager.GetHook(hookName)
		h.HookController.EnableConversionBindings()
	}

	if err = op.ConversionWebhookManager.Start(); err != nil {
		return fmt.Errorf("ConversionWebhookManager Start: %w", err)
	}

	return nil
}

// conversionEventHandler is called when Kubernetes requests a conversion.
func (op *ShellOperator) conversionEventHandler(crdName string, request *v1.ConversionRequest) (*conversion.Response, error) {
	logLabels := map[string]string{
		"event.id": uuid.Must(uuid.NewV4()).String(),
		"binding":  string(types.KubernetesConversion),
	}
	logEntry := utils.EnrichLoggerWithLabels(op.logger, logLabels)

	sourceVersions := conversion.ExtractAPIVersions(request.Objects)
	logEntry.Info("Handle kubernetesCustomResourceConversion event for crd",
		slog.String("name", crdName),
		slog.Int("len", len(request.Objects)),
		slog.Any("versions", sourceVersions))

	done := false
	for _, srcVer := range sourceVersions {
		rule := conversion.Rule{
			FromVersion: srcVer,
			ToVersion:   request.DesiredAPIVersion,
		}
		convPath := op.HookManager.FindConversionChain(crdName, rule)
		if len(convPath) == 0 {
			continue
		}
		logEntry.Info("Find conversion path for rule",
			slog.String("name", rule.String()),
			slog.Any("value", convPath))

		for _, convRule := range convPath {
			var convTask task.Task
			op.HookManager.HandleConversionEvent(crdName, request, convRule, func(hook *hook.Hook, info controller.BindingExecutionInfo) {
				newTask := task.NewTask(task_metadata.HookRun).
					WithMetadata(task_metadata.HookMetadata{
						HookName:       hook.Name,
						BindingType:    types.KubernetesConversion,
						BindingContext: info.BindingContext,
						AllowFailure:   info.AllowFailure,
						Binding:        info.Binding,
						Group:          info.Group,
					}).
					WithLogLabels(logLabels)
				convTask = newTask
			})

			if convTask == nil {
				return nil, fmt.Errorf("no hook found for '%s' event for crd/%s", string(types.KubernetesConversion), crdName)
			}

			res := op.taskHandler(convTask)

			if res.Status == "Fail" {
				return &conversion.Response{
					FailedMessage:    fmt.Sprintf("Hook failed to convert to %s", request.DesiredAPIVersion),
					ConvertedObjects: nil,
				}, nil
			}

			prop := convTask.GetProp("conversionResponse")
			response, ok := prop.(*conversion.Response)
			if !ok {
				logEntry.Error("'conversionResponse' task prop is not of type *conversion.Response",
					slog.String("type", fmt.Sprintf("%T", prop)))
				return nil, fmt.Errorf("hook task prop error")
			}

			// Set response objects as new objects for a next round.
			request.Objects = response.ConvertedObjects

			// Stop iterating if hook has converted all objects to a desiredAPIVersions.
			newSourceVersions := conversion.ExtractAPIVersions(request.Objects)
			// logEntry.Infof("Hook return conversion response: failMsg=%s, %d convertedObjects, versions:%v, desired: %s", response.FailedMessage, len(response.ConvertedObjects), newSourceVersions, event.Review.Request.DesiredAPIVersion)

			if len(newSourceVersions) == 1 && newSourceVersions[0] == request.DesiredAPIVersion {
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
			ConvertedObjects: request.Objects,
		}, nil
	}

	return &conversion.Response{
		FailedMessage: fmt.Sprintf("Conversion to %s was not successuful", request.DesiredAPIVersion),
	}, nil
}

// taskHandler
func (op *ShellOperator) taskHandler(t task.Task) queue.TaskResult {
	logEntry := op.logger.With("operator.component", "taskRunner")
	hookMeta := task_metadata.HookMetadataAccessor(t)
	var res queue.TaskResult

	switch t.GetType() {
	case task_metadata.HookRun:
		res = op.taskHandleHookRun(t)

	case task_metadata.EnableKubernetesBindings:
		res = op.taskHandleEnableKubernetesBindings(t)

	case task_metadata.EnableScheduleBindings:
		hookLogLabels := map[string]string{}
		hookLogLabels["hook"] = hookMeta.HookName
		hookLogLabels["binding"] = string(types.Schedule)
		hookLogLabels["task"] = "EnableScheduleBindings"
		hookLogLabels["queue"] = "main"

		taskLogEntry := utils.EnrichLoggerWithLabels(logEntry, hookLogLabels)

		taskHook := op.HookManager.GetHook(hookMeta.HookName)
		taskHook.HookController.EnableScheduleBindings()
		taskLogEntry.Info("Schedule binding for hook enabled successfully")
		res.Status = "Success"
	}

	return res
}

// taskHandleEnableKubernetesBindings creates task for each Kubernetes binding in the hook and queues them.
func (op *ShellOperator) taskHandleEnableKubernetesBindings(t task.Task) queue.TaskResult {
	hookMeta := task_metadata.HookMetadataAccessor(t)

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

	taskLogEntry := utils.EnrichLoggerWithLabels(op.logger, hookLogLabels)

	taskLogEntry.Info("Enable kubernetes binding for hook")

	taskHook := op.HookManager.GetHook(hookMeta.HookName)

	hookRunTasks := make([]task.Task, 0)

	// Run hook for each binding with Synchronization binding context. Ignore queue name here, execute in main queue.
	err := taskHook.HookController.HandleEnableKubernetesBindings(func(info controller.BindingExecutionInfo) {
		newTask := task.NewTask(task_metadata.HookRun).
			WithMetadata(task_metadata.HookMetadata{
				HookName:                 taskHook.Name,
				BindingType:              types.OnKubernetesEvent,
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
		taskLogEntry.Error("Enable Kubernetes binding for hook failed. Will retry after delay.",
			slog.Int("failedCount", t.GetFailureCount()+1),
			log.Err(err))
		res.Status = "Fail"
	} else {
		success = 1.0
		taskLogEntry.Info("Kubernetes bindings for hook are enabled successfully",
			slog.Int("count", len(hookRunTasks)))
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
func (op *ShellOperator) taskHandleHookRun(t task.Task) queue.TaskResult {
	hookMeta := task_metadata.HookMetadataAccessor(t)
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

	taskLogEntry := utils.EnrichLoggerWithLabels(op.logger, hookLogLabels)

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
		if hookMeta.BindingType == types.OnKubernetesEvent {
			// Do not combine Synchronizations without group
			if hookMeta.BindingContext[0].Type == kemTypes.TypeSynchronization && hookMeta.Group == "" {
				shouldCombine = false
			}
		}
		if shouldCombine {
			combineResult := op.combineBindingContextForHook(op.TaskQueues, op.TaskQueues.GetByName(t.GetQueueName()), t, nil)
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
		err = op.handleRunHook(t, taskHook, hookMeta, taskLogEntry, hookLogLabels, metricLabels)
		if err != nil {
			if hookMeta.AllowFailure {
				allowed = 1.0
				taskLogEntry.Info("Hook failed, but allowed to fail", log.Err(err))
				res.Status = "Success"
			} else {
				errors = 1.0
				t.UpdateFailureMessage(err.Error())
				t.WithQueuedAt(time.Now()) // Reset queueAt for correct results in 'task_wait_in_queue' metric.
				taskLogEntry.Error("Hook failed. Will retry after delay.",
					slog.Int("failedCount", t.GetFailureCount()+1),
					log.Err(err))
				res.Status = "Fail"
			}
		} else {
			success = 1.0
			taskLogEntry.Info("Hook executed successfully")
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

func (op *ShellOperator) handleRunHook(t task.Task, taskHook *hook.Hook, hookMeta task_metadata.HookMetadata, taskLogEntry *log.Logger, hookLogLabels map[string]string, metricLabels map[string]string) error {
	for _, info := range taskHook.HookController.SnapshotsInfo() {
		taskLogEntry.Debug("snapshot info", slog.String("info", info))
	}

	result, err := taskHook.Run(hookMeta.BindingType, hookMeta.BindingContext, hookLogLabels)
	if err != nil {
		if result != nil && len(result.KubernetesPatchBytes) > 0 {
			operations, patchStatusErr := objectpatch.ParseOperations(result.KubernetesPatchBytes)
			if patchStatusErr != nil {
				return fmt.Errorf("%s: couldn't patch status: %s", err, patchStatusErr)
			}

			patchStatusErr = op.ObjectPatcher.ExecuteOperations(objectpatch.GetPatchStatusOperationsOnHookError(operations))
			if patchStatusErr != nil {
				return fmt.Errorf("%s: couldn't patch status: %s", err, patchStatusErr)
			}
		}
		return err
	}

	if result.Usage != nil {
		taskLogEntry.Debug("Usage", slog.String("value", fmt.Sprintf("%+v", result.Usage)))
		op.MetricStorage.HistogramObserve("{PREFIX}hook_run_sys_seconds", result.Usage.Sys.Seconds(), metricLabels, nil)
		op.MetricStorage.HistogramObserve("{PREFIX}hook_run_user_seconds", result.Usage.User.Seconds(), metricLabels, nil)
		op.MetricStorage.GaugeSet("{PREFIX}hook_run_max_rss_bytes", float64(result.Usage.MaxRss)*1024, metricLabels)
	}

	// Try to apply Kubernetes actions.
	if len(result.KubernetesPatchBytes) > 0 {
		operations, err := objectpatch.ParseOperations(result.KubernetesPatchBytes)
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
	if result.AdmissionResponse != nil {
		t.SetProp("admissionResponse", result.AdmissionResponse)
		taskLogEntry.Info("AdmissionResponse from hook",
			slog.String("value", result.AdmissionResponse.Dump()))
	}

	// Save conversionResponse in task props for future use.
	if result.ConversionResponse != nil {
		t.SetProp("conversionResponse", result.ConversionResponse)
		taskLogEntry.Info("ConversionResponse from hook",
			slog.String("value", result.ConversionResponse.Dump()))
	}

	return nil
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
	taskMeta := t.GetMetadata()
	if taskMeta == nil {
		// Ignore task without metadata
		return nil
	}
	hookName := taskMeta.(task_metadata.HookNameAccessor).GetHookName()

	res := new(CombineResult)

	otherTasks := make([]task.Task, 0)
	stopIterate := false
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
		nextHookName := hm.(task_metadata.HookNameAccessor).GetHookName()
		// Only tasks for the same hook and of the same type can be combined (HookRun cannot be combined with OnStartup).
		// Using stopCombineFn function stricter combine rules can be defined.
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
	combinedContext := make([]bindingcontext.BindingContext, 0)
	monitorIDs := taskMeta.(task_metadata.MonitorIDAccessor).GetMonitorIDs()
	tasksFilter := make(map[string]bool)
	// current task always remain in queue
	combinedContext = append(combinedContext, taskMeta.(task_metadata.BindingContextAccessor).GetBindingContext()...)
	tasksFilter[t.GetId()] = true
	for _, tsk := range otherTasks {
		combinedContext = append(combinedContext, tsk.GetMetadata().(task_metadata.BindingContextAccessor).GetBindingContext()...)
		tskMonitorIDs := tsk.GetMetadata().(task_metadata.MonitorIDAccessor).GetMonitorIDs()
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
	compactedContext := make([]bindingcontext.BindingContext, 0)
	for i := 0; i < len(combinedContext); i++ {
		keep := true

		// Binding context is ignored if next binding context has the similar group.
		groupName := combinedContext[i].Metadata.Group
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
	log.Info("Binding contexts from tasks. Tasks are dropped from queue",
		slog.Int("count", len(otherTasks)+1),
		slog.String("tasks", compactMsg),
		slog.Int("filteredCount", len(tasksFilter)-1),
		slog.String("queueName", t.GetQueueName()))

	res.BindingContexts = compactedContext
	res.MonitorIDs = monitorIDs
	return res
}

// bootstrapMainQueue adds tasks to run hooks with OnStartup bindings
// and tasks to enable kubernetes bindings.
func (op *ShellOperator) bootstrapMainQueue(tqs *queue.TaskQueueSet) {
	logEntry := op.logger.With("operator.component", "initMainQueue")

	// Prepopulate main queue with 'onStartup' tasks and 'enable kubernetes bindings' tasks.
	tqs.WithMainName("main")
	tqs.NewNamedQueue("main", op.taskHandler)

	mainQueue := tqs.GetMain()

	// Add tasks to run OnStartup bindings
	onStartupHooks, err := op.HookManager.GetHooksInOrder(types.OnStartup)
	if err != nil {
		logEntry.Error("add tasks to run OnStartup bindings", log.Err(err))
		return
	}

	for _, hookName := range onStartupHooks {
		bc := bindingcontext.BindingContext{
			Binding: string(types.OnStartup),
		}
		bc.Metadata.BindingType = types.OnStartup

		newTask := task.NewTask(task_metadata.HookRun).
			WithMetadata(task_metadata.HookMetadata{
				HookName:       hookName,
				BindingType:    types.OnStartup,
				BindingContext: []bindingcontext.BindingContext{bc},
			}).
			WithQueuedAt(time.Now())
		mainQueue.AddLast(newTask)
		logEntry.Info("queue task with hook",
			slog.String("task", newTask.GetDescription()),
			slog.String("hook", hookName))
	}

	// Add tasks to enable kubernetes monitors and schedules for each hook
	for _, hookName := range op.HookManager.GetHookNames() {
		h := op.HookManager.GetHook(hookName)

		if h.GetConfig().HasBinding(types.OnKubernetesEvent) {
			newTask := task.NewTask(task_metadata.EnableKubernetesBindings).
				WithMetadata(task_metadata.HookMetadata{
					HookName: hookName,
					Binding:  string(task_metadata.EnableKubernetesBindings),
				}).
				WithQueuedAt(time.Now())
			mainQueue.AddLast(newTask)
			logEntry.Info("queue task with hook",
				slog.String("task", newTask.GetDescription()),
				slog.String("hook", hookName))
		}

		if h.GetConfig().HasBinding(types.Schedule) {
			newTask := task.NewTask(task_metadata.EnableScheduleBindings).
				WithMetadata(task_metadata.HookMetadata{
					HookName: hookName,
					Binding:  string(task_metadata.EnableScheduleBindings),
				}).
				WithQueuedAt(time.Now())
			mainQueue.AddLast(newTask)
			logEntry.Info("queue task with hook",
				slog.String("task", newTask.GetDescription()),
				slog.String("hook", hookName))
		}
	}
}

// initAndStartHookQueues create all queues defined in hooks
func (op *ShellOperator) initAndStartHookQueues() {
	schHooks, _ := op.HookManager.GetHooksInOrder(types.Schedule)
	for _, hookName := range schHooks {
		h := op.HookManager.GetHook(hookName)
		for _, hookBinding := range h.Config.Schedules {
			if op.TaskQueues.GetByName(hookBinding.Queue) == nil {
				op.TaskQueues.NewNamedQueue(hookBinding.Queue, op.taskHandler)
				op.TaskQueues.GetByName(hookBinding.Queue).Start()
			}
		}
	}

	kubeHooks, _ := op.HookManager.GetHooksInOrder(types.OnKubernetesEvent)
	for _, hookName := range kubeHooks {
		h := op.HookManager.GetHook(hookName)
		for _, hookBinding := range h.Config.OnKubernetesEvents {
			if op.TaskQueues.GetByName(hookBinding.Queue) == nil {
				op.TaskQueues.NewNamedQueue(hookBinding.Queue, op.taskHandler)
				op.TaskQueues.GetByName(hookBinding.Queue).Start()
			}
		}
	}
}

func (op *ShellOperator) runMetrics() {
	if op.MetricStorage == nil {
		return
	}

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

// Shutdown pause kubernetes events handling and stop queues. Wait for queues to stop.
func (op *ShellOperator) Shutdown() {
	op.ScheduleManager.Stop()
	op.KubeEventsManager.PauseHandleEvents()
	op.TaskQueues.Stop()
	// Wait for queues to stop, but no more than 10 seconds
	op.TaskQueues.WaitStopWithTimeout(WaitQueuesTimeout)
}
