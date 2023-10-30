package shell_operator

import (
	"context"
	"fmt"
	app2 "github.com/flant/shell-operator/internal/app"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/config"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/jq"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils "github.com/flant/shell-operator/pkg/utils/file"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// Init initialize logging, ensures directories and creates
// a ShellOperator instance with all dependencies.
func Init() (*ShellOperator, error) {
	runtimeConfig := config.NewConfig()
	// Init logging subsystem.
	app2.SetupLogging(runtimeConfig)
	// Log version and jq filtering implementation.
	log.Infof(app2.AppStartMessage)
	log.Debug(jq.FilterInfo())

	hooksDir, err := utils.RequireExistingDirectory(app2.HooksDir)
	if err != nil {
		log.Errorf("Fatal: hooks directory is required: %s", err)
		return nil, err
	}

	tempDir, err := utils.EnsureTempDirectory(app2.TempDir)
	if err != nil {
		log.Errorf("Fatal: temp directory: %s", err)
		return nil, err
	}

	op := NewShellOperator(context.Background())

	// Debug server.
	debugServer, err := RunDefaultDebugServer(app2.DebugUnixSocket, app2.DebugHttpServerAddr)
	if err != nil {
		log.Errorf("Fatal: start Debug server: %s", err)
		return nil, err
	}

	err = op.AssembleCommonOperator(app2.ListenAddress, app2.ListenPort)
	if err != nil {
		log.Errorf("Fatal: %s", err)
		return nil, err
	}

	err = op.assembleShellOperator(hooksDir, tempDir, debugServer, runtimeConfig)
	if err != nil {
		log.Errorf("Fatal: %s", err)
		return nil, err
	}

	return op, nil
}

// AssembleCommonOperator instantiate common dependencies. These dependencies
// may be used for shell-operator derivatives, like addon-operator.
// requires listenAddress, listenPort to run http server for operator APIs
func (op *ShellOperator) AssembleCommonOperator(listenAddress, listenPort string) (err error) {
	op.APIServer = newBaseHTTPServer(listenAddress, listenPort)

	op.MetricStorage = defaultMetricStorage(op.ctx)

	op.setupHookMetricStorage()
	if err != nil {
		return fmt.Errorf("start HTTP server for hook metrics: %s", err)
	}
	// Set to common metric storage if separate port is not set.
	if op.HookMetricStorage == nil {
		op.HookMetricStorage = op.MetricStorage
	}

	// 'main' Kubernetes client.
	op.KubeClient, err = initDefaultMainKubeClient(op.MetricStorage)
	if err != nil {
		return err
	}

	// ObjectPatcher with a separate Kubernetes client.
	op.ObjectPatcher, err = initDefaultObjectPatcher(op.MetricStorage)
	if err != nil {
		return err
	}

	op.SetupEventManagers()

	return nil
}

// assembleShellOperator uses settings in app package to create all
// dependencies needed for the full-fledged ShellOperator.
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
func (op *ShellOperator) assembleShellOperator(hooksDir string, tempDir string, debugServer *debug.Server, runtimeConfig *config.Config) (err error) {
	registerDefaultRoutes(op)

	op.RegisterDebugQueueRoutes(debugServer)
	op.RegisterDebugHookRoutes(debugServer)
	op.RegisterDebugConfigRoutes(debugServer, runtimeConfig)

	registerShellOperatorMetrics(op.MetricStorage)

	// Create webhookManagers with dependencies.
	op.setupHookManagers(hooksDir, tempDir)

	// Search and configure all hooks.
	err = op.initHookManager()
	if err != nil {
		return fmt.Errorf("initialize HookManager fail: %s", err)
	}

	// Load validation hooks.
	err = op.initValidatingWebhookManager()
	if err != nil {
		return fmt.Errorf("initialize ValidatingWebhookManager fail: %s", err)
	}

	// Load conversion hooks.
	err = op.initConversionWebhookManager()
	if err != nil {
		return fmt.Errorf("initialize ConversionWebhookManager fail: %s", err)
	}

	return nil
}

// SetupEventManagers instantiate queues and managers for schedule and Kubernetes events.
// This function is also used in the addon-operator
func (op *ShellOperator) SetupEventManagers() {
	// Initialize the task queues set with the "main" queue.
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(op.ctx)
	op.TaskQueues.WithMetricStorage(op.MetricStorage)

	// Initialize schedule manager.
	op.ScheduleManager = schedule_manager.NewScheduleManager(op.ctx)

	// Initialize kubernetes events manager.
	op.KubeEventsManager = kube_events_manager.NewKubeEventsManager(op.ctx, op.KubeClient)
	op.KubeEventsManager.WithMetricStorage(op.MetricStorage)

	// Initialize events handler that emit tasks to run hooks
	cfg := &managerEventsHandlerConfig{
		tqs:  op.TaskQueues,
		mgr:  op.KubeEventsManager,
		smgr: op.ScheduleManager,
	}
	op.ManagerEventsHandler = newManagerEventsHandler(op.ctx, cfg)
}

// setupHookManagers instantiates different hook managers.
func (op *ShellOperator) setupHookManagers(hooksDir string, tempDir string) {
	// Initialize admission webhooks manager.
	op.AdmissionWebhookManager = admission.NewWebhookManager(op.KubeClient)
	op.AdmissionWebhookManager.Settings = app2.ValidatingWebhookSettings
	op.AdmissionWebhookManager.Namespace = app2.Namespace

	// Initialize conversion webhooks manager.
	op.ConversionWebhookManager = conversion.NewWebhookManager()
	op.ConversionWebhookManager.KubeClient = op.KubeClient
	op.ConversionWebhookManager.Settings = app2.ConversionWebhookSettings
	op.ConversionWebhookManager.Namespace = app2.Namespace

	// Initialize Hook manager.
	cfg := &hook.HookManagerConfig{
		WorkingDir: hooksDir,
		TempDir:    tempDir,
		Kmgr:       op.KubeEventsManager,
		Smgr:       op.ScheduleManager,
		Wmgr:       op.AdmissionWebhookManager,
		Cmgr:       op.ConversionWebhookManager,
	}
	op.HookManager = hook.NewHookManager(cfg)
}
