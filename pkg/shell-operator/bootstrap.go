package shell_operator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/shell-operator/pkg/app"
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
func Init(logger *log.Logger) (*ShellOperator, error) {
	runtimeConfig := config.NewConfig(logger)
	// Init logging subsystem.
	app.SetupLogging(runtimeConfig, logger)

	// Log version and jq filtering implementation.
	logger.Info(app.AppStartMessage)
	logger.Debug(jq.FilterInfo())

	hooksDir, err := utils.RequireExistingDirectory(app.HooksDir)
	if err != nil {
		logger.Log(context.TODO(), log.LevelFatal.Level(), "hooks directory is required", slog.String("error", err.Error()))
		return nil, err
	}

	tempDir, err := utils.EnsureTempDirectory(app.TempDir)
	if err != nil {
		logger.Log(context.TODO(), log.LevelFatal.Level(), "temp directory", slog.String("error", err.Error()))
		return nil, err
	}

	op := NewShellOperator(context.Background(), WithLogger(logger))

	// Debug server.
	debugServer, err := RunDefaultDebugServer(app.DebugUnixSocket, app.DebugHttpServerAddr, op.logger.Named("debug-server"))
	if err != nil {
		logger.Log(context.TODO(), log.LevelFatal.Level(), "start Debug server", slog.String("error", err.Error()))
		return nil, err
	}

	err = op.AssembleCommonOperator(app.ListenAddress, app.ListenPort, map[string]string{
		"hook":    "",
		"binding": "",
		"queue":   "",
	})
	if err != nil {
		logger.Log(context.TODO(), log.LevelFatal.Level(), "essemble common operator", slog.String("error", err.Error()))
		return nil, err
	}

	err = op.assembleShellOperator(hooksDir, tempDir, debugServer, runtimeConfig)
	if err != nil {
		logger.Log(context.TODO(), log.LevelFatal.Level(), "essemble shell operator", slog.String("error", err.Error()))
		return nil, err
	}

	return op, nil
}

// AssembleCommonOperator instantiate common dependencies. These dependencies
// may be used for shell-operator derivatives, like addon-operator.
// requires listenAddress, listenPort to run http server for operator APIs
func (op *ShellOperator) AssembleCommonOperator(listenAddress, listenPort string, kubeEventsManagerLabels map[string]string) (err error) {
	op.APIServer = newBaseHTTPServer(listenAddress, listenPort)

	// built-in metrics
	op.setupMetricStorage(kubeEventsManagerLabels)

	// metrics from user's hooks
	op.setupHookMetricStorage()

	// 'main' Kubernetes client.
	op.KubeClient, err = initDefaultMainKubeClient(op.MetricStorage, op.logger)
	if err != nil {
		return err
	}

	// ObjectPatcher with a separate Kubernetes client.
	op.ObjectPatcher, err = initDefaultObjectPatcher(op.MetricStorage, op.logger.Named("object-patcher"))
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
	registerRootRoute(op)
	// for shell-operator only
	registerHookMetrics(op.HookMetricStorage)

	op.RegisterDebugQueueRoutes(debugServer)
	op.RegisterDebugHookRoutes(debugServer)
	op.RegisterDebugConfigRoutes(debugServer, runtimeConfig)

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
	op.ScheduleManager = schedule_manager.NewScheduleManager(op.ctx, op.logger.Named("schedule-manager"))

	// Initialize kubernetes events manager.
	op.KubeEventsManager = kube_events_manager.NewKubeEventsManager(op.ctx, op.KubeClient, op.logger.Named("kube-events-manager"))
	op.KubeEventsManager.WithMetricStorage(op.MetricStorage)

	// Initialize events handler that emit tasks to run hooks
	cfg := &managerEventsHandlerConfig{
		tqs:    op.TaskQueues,
		mgr:    op.KubeEventsManager,
		smgr:   op.ScheduleManager,
		logger: op.logger.Named("manager-events-handler"),
	}
	op.ManagerEventsHandler = newManagerEventsHandler(op.ctx, cfg)
}

// setupHookManagers instantiates different hook managers.
func (op *ShellOperator) setupHookManagers(hooksDir string, tempDir string) {
	// Initialize admission webhooks manager.
	op.AdmissionWebhookManager = admission.NewWebhookManager(op.KubeClient)
	op.AdmissionWebhookManager.Settings = app.ValidatingWebhookSettings
	op.AdmissionWebhookManager.Namespace = app.Namespace

	// Initialize conversion webhooks manager.
	op.ConversionWebhookManager = conversion.NewWebhookManager()
	op.ConversionWebhookManager.KubeClient = op.KubeClient
	op.ConversionWebhookManager.Settings = app.ConversionWebhookSettings
	op.ConversionWebhookManager.Namespace = app.Namespace

	// Initialize Hook manager.
	cfg := &hook.ManagerConfig{
		WorkingDir: hooksDir,
		TempDir:    tempDir,
		Kmgr:       op.KubeEventsManager,
		Smgr:       op.ScheduleManager,
		Wmgr:       op.AdmissionWebhookManager,
		Cmgr:       op.ConversionWebhookManager,
		Logger:     op.logger.Named("hook-manager"),
	}
	op.HookManager = hook.NewHookManager(cfg)
}
