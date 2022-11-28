package shell_operator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/config"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/jq"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	"github.com/flant/shell-operator/pkg/webhook/validating"
)

// Init initialize logging, ensures directories and creates
// a ShellOperator instance with all dependencies.
func Init() (*ShellOperator, error) {
	runtimeConfig := config.NewConfig()
	// Init logging subsystem.
	app.SetupLogging(runtimeConfig)
	// Log version and jq filtering implementation.
	log.Infof(app.AppStartMessage)
	log.Debug(jq.JqFilterInfo())

	hooksDir, err := RequireExistingDirectory(app.HooksDir)
	if err != nil {
		log.Errorf("Fatal: hooks directory is required: %s", err)
		return nil, err
	}

	tempDir, err := EnsureTempDirectory(app.TempDir)
	if err != nil {
		log.Errorf("Fatal: temp directory: %s", err)
		return nil, err
	}

	op := NewShellOperator()
	op.WithContext(context.Background())

	// Debug server.
	debugServer, err := InitDefaultDebugServer()
	if err != nil {
		log.Errorf("Fatal: start Debug server: %s", err)
		return nil, err
	}

	err = AssembleCommonOperator(op)
	if err != nil {
		log.Errorf("Fatal: %s", err)
		return nil, err
	}

	err = AssembleShellOperator(op, hooksDir, tempDir, debugServer, runtimeConfig)
	if err != nil {
		log.Errorf("Fatal: %s", err)
		return nil, err
	}

	return op, nil
}

func RequireExistingDirectory(inDir string) (dir string, err error) {
	if inDir == "" {
		return "", fmt.Errorf("path is required but not set")
	}

	dir, err = filepath.Abs(inDir)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	if exists, _ := utils_file.DirExists(dir); !exists {
		return "", fmt.Errorf("path '%s' not exist", dir)
	}

	return dir, nil
}

func EnsureTempDirectory(inDir string) (string, error) {
	// No path to temporary dir, use default temporary dir.
	if inDir == "" {
		tmpPath := app.AppName + "-*"
		dir, err := os.MkdirTemp("", tmpPath)
		if err != nil {
			return "", fmt.Errorf("create tmp dir in '%s': %s", tmpPath, err)
		}
		return dir, nil
	}

	// Get absolute path for temporary directory and create if needed.
	dir, err := filepath.Abs(inDir)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	if exists, _ := utils_file.DirExists(dir); !exists {
		err := os.Mkdir(dir, os.FileMode(0777))
		if err != nil {
			return "", fmt.Errorf("create tmp dir '%s': %s", dir, err)
		}
	}
	return dir, nil
}

// AssembleCommonOperator instantiate common dependencies. These dependencies
// may be used for shell-operator derivatives, like addon-operator.
func AssembleCommonOperator(op *ShellOperator) (err error) {
	err = StartHttpServer(app.ListenAddress, app.ListenPort, http.DefaultServeMux)
	if err != nil {
		return fmt.Errorf("start HTTP server: %s", err)
	}

	op.MetricStorage = DefaultMetricStorage(op.ctx)

	op.HookMetricStorage, err = SetupHookMetricStorageAndServer(op.ctx)
	if err != nil {
		return fmt.Errorf("start HTTP server for hook metrics: %s", err)
	}
	// Set to common metric storage if separate port is not set.
	if op.HookMetricStorage == nil {
		op.HookMetricStorage = op.MetricStorage
	}

	// 'main' Kubernetes client.
	op.KubeClient, err = InitDefaultMainKubeClient(op.MetricStorage)
	if err != nil {
		return err
	}

	// ObjectPatcher with a separate Kubernetes client.
	op.ObjectPatcher, err = InitDefaultObjectPatcher(op.MetricStorage)
	if err != nil {
		return err
	}

	SetupEventManagers(op)

	return nil
}

// AssembleShellOperator uses settings in app package to create all
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
func AssembleShellOperator(op *ShellOperator, hooksDir string, tempDir string, debugServer *debug.Server, runtimeConfig *config.Config) (err error) {
	RegisterDefaultRoutes(op)

	RegisterDebugQueueRoutes(debugServer, op)
	RegisterDebugHookRoutes(debugServer, op)
	RegisterDebugConfigRoutes(debugServer, runtimeConfig)

	RegisterShellOperatorMetrics(op.MetricStorage)

	// Create webhookManagers with dependencies.
	SetupHookManagers(op, hooksDir, tempDir)

	// Search and configure all hooks.
	err = op.InitHookManager()
	if err != nil {
		return fmt.Errorf("initialize HookManager fail: %s", err)
	}

	// Load validation hooks.
	err = op.InitValidatingWebhookManager()
	if err != nil {
		return fmt.Errorf("initialize ValidatingWebhookManager fail: %s", err)
	}

	// Load conversion hooks.
	err = op.InitConversionWebhookManager()
	if err != nil {
		return fmt.Errorf("initialize ConversionWebhookManager fail: %s", err)
	}

	return nil
}

// SetupEventManagers instantiate queues and managers for schedule and Kubernetes events.
func SetupEventManagers(op *ShellOperator) {
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
}

// SetupHookManagers instantiates different hook managers.
func SetupHookManagers(op *ShellOperator, hooksDir string, tempDir string) {
	// Initialize validating webhooks manager
	op.ValidatingWebhookManager = validating.NewWebhookManager()
	op.ValidatingWebhookManager.WithKubeClient(op.KubeClient)
	op.ValidatingWebhookManager.Settings = app.ValidatingWebhookSettings
	op.ValidatingWebhookManager.Namespace = app.Namespace

	// Initialize validating webhooks manager
	op.ConversionWebhookManager = conversion.NewWebhookManager()
	op.ConversionWebhookManager.KubeClient = op.KubeClient
	op.ConversionWebhookManager.Settings = app.ConversionWebhookSettings
	op.ConversionWebhookManager.Namespace = app.Namespace

	// Initialize Hook manager.
	op.HookManager = hook.NewHookManager()
	op.HookManager.WithDirectories(hooksDir, tempDir)
	op.HookManager.WithKubeEventManager(op.KubeEventsManager)
	op.HookManager.WithScheduleManager(op.ScheduleManager)
	op.HookManager.WithValidatingWebhookManager(op.ValidatingWebhookManager)
	op.HookManager.WithConversionWebhookManager(op.ConversionWebhookManager)
}
