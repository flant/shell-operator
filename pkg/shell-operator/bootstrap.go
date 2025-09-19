// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package shell_operator provides bootstrap functionality for creating and initializing ShellOperator instances.
//
// This file contains the initialization and assembly logic for ShellOperator.
// Configuration is handled via the ShellOperatorConfig struct and related functions defined in config.go.
package shell_operator

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/shell-operator/internal/metrics"
	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/config"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/filter/jq"
	"github.com/flant/shell-operator/pkg/hook"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils "github.com/flant/shell-operator/pkg/utils/file"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// NewShellOperatorWithConfig creates a fully configured ShellOperator instance with all dependencies.
// This replaces the old Init function with a more flexible constructor approach.
func NewShellOperatorWithConfig(ctx context.Context, cfg *ShellOperatorConfig) (*ShellOperator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	logger := cfg.Logger

	// Initialize runtime configuration and logging
	runtimeConfig := config.NewConfig(logger)
	app.SetupLogging(runtimeConfig, logger)

	// Log version and jq filtering implementation
	logger.Info(app.AppStartMessage)
	fl := jq.NewFilter()
	logger.Debug(fl.FilterInfo())

	// Validate and prepare directories
	hooksDir, err := utils.RequireExistingDirectory(cfg.HooksDir)
	if err != nil {
		return nil, fmt.Errorf("hooks directory validation failed: %w", err)
	}

	tempDir, err := utils.EnsureTempDirectory(cfg.TempDir)
	if err != nil {
		return nil, fmt.Errorf("temp directory setup failed: %w", err)
	}

	// Create the operator instance
	op := newShellOperator(ctx)
	op.MetricStorage = cfg.MetricStorage
	op.HookMetricStorage = cfg.HookMetricStorage
	op.logger = logger

	// Start debug server
	debugServer, err := RunDefaultDebugServer(cfg.DebugUnixSocket, cfg.DebugHttpServerAddr,
		op.logger.Named("debug-server"))
	if err != nil {
		return nil, fmt.Errorf("failed to start debug server: %w", err)
	}

	// Assemble common components
	if err := op.AssembleCommonOperator(cfg.ListenAddress, cfg.ListenPort); err != nil {
		return nil, fmt.Errorf("failed to assemble common operator: %w", err)
	}

	// Assemble shell-operator specific components
	if err := op.assembleShellOperator(hooksDir, tempDir, debugServer, runtimeConfig); err != nil {
		return nil, fmt.Errorf("failed to assemble shell operator: %w", err)
	}

	return op, nil
}

// NewShellOperator creates a ShellOperator instance using functional options.
func NewShellOperator(ctx context.Context, options ...ConfigOption) (*ShellOperator, error) {
	cfg := NewShellOperatorConfig(options...)
	return NewShellOperatorWithConfig(ctx, cfg)
}

// Init provides backward compatibility with the old initialization function.
// Deprecated: Use NewShellOperatorWithOptions for more flexibility.
func Init(logger *log.Logger) (*ShellOperator, error) {
	return NewShellOperator(context.TODO(), WithLogger(logger))
}

// AssembleCommonOperator instantiate common dependencies. These dependencies
// may be used for shell-operator derivatives, like addon-operator.
// requires listenAddress, listenPort to run http server for operator APIs
func (op *ShellOperator) AssembleCommonOperator(listenAddress, listenPort string) error {
	op.APIServer = newBaseHTTPServer(listenAddress, listenPort)

	// built-in metrics
	if err := op.setupMetricStorage(); err != nil {
		return fmt.Errorf("setup metric storage: %w", err)
	}

	// metrics from user's hooks
	op.setupHookMetricStorage()

	var err error
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
func (op *ShellOperator) assembleShellOperator(hooksDir string, tempDir string, debugServer *debug.Server, runtimeConfig *config.Config) error {
	registerRootRoute(op)
	// for shell-operator only
	if err := metrics.RegisterHookMetrics(op.HookMetricStorage); err != nil {
		return fmt.Errorf("register hook metrics: %w", err)
	}

	op.RegisterDebugQueueRoutes(debugServer)
	op.RegisterDebugHookRoutes(debugServer)
	op.RegisterDebugConfigRoutes(debugServer, runtimeConfig)

	// Create webhookManagers with dependencies.
	op.setupHookManagers(hooksDir, tempDir)

	// Search and configure all hooks.
	err := op.initHookManager()
	if err != nil {
		return fmt.Errorf("initialize HookManager fail: %w", err)
	}

	// Load validation hooks.
	err = op.initValidatingWebhookManager()
	if err != nil {
		return fmt.Errorf("initialize ValidatingWebhookManager fail: %w", err)
	}

	// Load conversion hooks.
	err = op.initConversionWebhookManager()
	if err != nil {
		return fmt.Errorf("initialize ConversionWebhookManager fail: %w", err)
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
	op.ScheduleManager = schedulemanager.NewScheduleManager(op.ctx, op.logger.Named("schedule-manager"))

	// Initialize kubernetes events manager.
	op.KubeEventsManager = kubeeventsmanager.NewKubeEventsManager(op.ctx, op.KubeClient, op.logger.Named("kube-events-manager"))
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
