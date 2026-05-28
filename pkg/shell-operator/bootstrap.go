package shell_operator

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/config"
	"github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/filter/jq"
	"github.com/flant/shell-operator/pkg/hook"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/metrics"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils "github.com/flant/shell-operator/pkg/utils/file"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
	webhookserver "github.com/flant/shell-operator/pkg/webhook/server"
)

// Init initializes logging, ensures directories and creates
// a ShellOperator instance with all dependencies.
// cfg must already have all configuration sources merged (NewConfig → ParseEnv → BindFlags → flags parsed).
func Init(ctx context.Context, cfg *app.Config, logger *log.Logger) (*ShellOperator, error) {
	// Propagate cfg into package-level globals (e.g. DebugUnixSocket) so
	// library consumers that skip BindFlags still get them overridden from cfg.
	app.ApplyConfig(cfg)

	// Initialize webhook settings from merged configuration.
	admission.InitFromSettings(admission.WebhookSettings{
		Settings: webhookserver.Settings{
			ServerCertPath: cfg.Admission.ServerCert,
			ServerKeyPath:  cfg.Admission.ServerKey,
			ClientCAPaths:  cfg.Admission.ClientCA,
			ServiceName:    cfg.Admission.ServiceName,
			ListenAddr:     cfg.Admission.ListenAddress,
			ListenPort:     cfg.Admission.ListenPort,
		},
		CAPath:               cfg.Admission.CA,
		ConfigurationName:    cfg.Admission.ConfigurationName,
		DefaultFailurePolicy: cfg.Admission.FailurePolicy,
	})
	conversion.InitFromSettings(conversion.WebhookSettings{
		Settings: webhookserver.Settings{
			ServerCertPath: cfg.Conversion.ServerCert,
			ServerKeyPath:  cfg.Conversion.ServerKey,
			ClientCAPaths:  cfg.Conversion.ClientCA,
			ServiceName:    cfg.Conversion.ServiceName,
			ListenAddr:     cfg.Conversion.ListenAddress,
			ListenPort:     cfg.Conversion.ListenPort,
		},
		CAPath: cfg.Conversion.CA,
	})

	runtimeConfig := config.NewConfig(logger)
	// Init logging subsystem.
	app.SetupLogging(cfg.Log.Level, runtimeConfig, logger)

	// Log version and jq filtering implementation.
	logger.Info(app.AppStartMessage)
	fl := jq.NewFilter()
	logger.Debug(fl.FilterInfo())

	hooksDir, err := utils.RequireExistingDirectory(cfg.App.HooksDir)
	if err != nil {
		logger.Log(ctx, log.LevelFatal.Level(), "hooks directory is required", log.Err(err))
		return nil, err
	}

	tempDir, err := utils.EnsureTempDirectory(cfg.App.TempDir)
	if err != nil {
		logger.Log(ctx, log.LevelFatal.Level(), "temp directory", log.Err(err))
		return nil, err
	}

	ms := metricsstorage.NewMetricStorage(
		metricsstorage.WithLogger(logger.Named("metric-storage")),
	)

	hms := metricsstorage.NewMetricStorage(
		metricsstorage.WithNewRegistry(),
		metricsstorage.WithLogger(logger.Named("hook-metric-storage")),
	)

	op := NewShellOperator(ctx, ms, hms, WithLogger(logger))

	// Debug server.
	debugServer, err := RunDefaultDebugServer(cfg.Debug.UnixSocket, cfg.Debug.HTTPServerAddr, op.logger.Named("debug-server"))
	if err != nil {
		logger.Log(ctx, log.LevelFatal.Level(), "start Debug server", log.Err(err))
		return nil, err
	}

	err = op.AssembleCommonOperatorFromConfig(cfg, []string{
		"hook",
		"binding",
		"queue",
	})
	if err != nil {
		logger.Log(ctx, log.LevelFatal.Level(), "essemble common operator", log.Err(err))
		return nil, err
	}

	err = op.assembleShellOperator(cfg, hooksDir, tempDir, debugServer, runtimeConfig)
	if err != nil {
		logger.Log(ctx, log.LevelFatal.Level(), "essemble shell operator", log.Err(err))
		return nil, err
	}

	return op, nil
}

// AssembleCommonOperatorFromConfig is the recommended assembly entry point for
// library consumers that already hold a fully populated *app.Config (for
// example, addon-operator builds its own and embeds shell-operator).
//
// It derives the HTTP server address, the main and object-patcher
// KubeClientConfigs from cfg and delegates to AssembleCommonOperator. The
// derivation reads only the supplied *app.Config — no environment variables
// are consulted on this path, so the values you put into cfg are the values
// shell-operator uses. See kubeClientConfigsFromAppConfig for the exact
// field mapping.
//
// kubeEventsManagerLabels are the metric labels for the kube-events manager;
// each embedder typically passes its own (e.g. addon-operator adds "module"
// and "kind", shell-operator passes "hook"/"binding"/"queue").
//
// Pass a nil cfg to fall back to zero-valued KubeClientConfig (in-cluster
// defaults) — useful for tests.
func (op *ShellOperator) AssembleCommonOperatorFromConfig(cfg *app.Config, kubeEventsManagerLabels []string) error {
	listenAddress, listenPort := listenAddrFromAppConfig(cfg)
	mainKubeCfg, patcherKubeCfg := kubeClientConfigsFromAppConfig(cfg)
	return op.AssembleCommonOperator(listenAddress, listenPort, kubeEventsManagerLabels, mainKubeCfg, patcherKubeCfg)
}

// listenAddrFromAppConfig returns the HTTP server listen address/port from cfg
// or empty strings when cfg is nil. Extracted as a helper so unit tests can
// assert that no environment variable is consulted during derivation.
func listenAddrFromAppConfig(cfg *app.Config) (string, string) {
	if cfg == nil {
		return "", ""
	}
	return cfg.App.ListenAddress, cfg.App.ListenPort
}

// kubeClientConfigsFromAppConfig derives the main and object-patcher
// KubeClientConfigs from an *app.Config. The function is pure: it does not
// touch the process environment, so any value present in cfg is used as-is
// and library consumers can rely on env vars never overriding their config.
// A nil cfg yields two zero KubeClientConfig values (in-cluster defaults).
func kubeClientConfigsFromAppConfig(cfg *app.Config) (KubeClientConfig, KubeClientConfig) {
	if cfg == nil {
		return KubeClientConfig{}, KubeClientConfig{}
	}
	mainKubeCfg := KubeClientConfig{
		Context:      cfg.Kube.Context,
		Config:       cfg.Kube.Config,
		QPS:          cfg.Kube.ClientQPS,
		Burst:        cfg.Kube.ClientBurst,
		MetricPrefix: cfg.App.PrometheusMetricsPrefix,
	}
	patcherKubeCfg := KubeClientConfig{
		Context:      cfg.Kube.Context,
		Config:       cfg.Kube.Config,
		QPS:          cfg.ObjectPatcher.KubeClientQPS,
		Burst:        cfg.ObjectPatcher.KubeClientBurst,
		Timeout:      cfg.ObjectPatcher.KubeClientTimeout,
		MetricPrefix: "object_patcher_",
	}
	return mainKubeCfg, patcherKubeCfg
}

// AssembleCommonOperator instantiates common dependencies used by both
// shell-operator and its derivatives (e.g. addon-operator).
// Requires listenAddress and listenPort to run the HTTP server for operator APIs.
// kubeCfg provides Kubernetes connection settings for the main client and
// object patcher; pass KubeClientConfig{} to fall back to in-cluster defaults.
//
// For library consumers that already hold an *app.Config, prefer
// AssembleCommonOperatorFromConfig instead of unpacking fields by hand.
func (op *ShellOperator) AssembleCommonOperator(listenAddress, listenPort string, kubeEventsManagerLabels []string, mainKubeCfg, patcherKubeCfg KubeClientConfig) error {
	op.APIServer = newBaseHTTPServer(listenAddress, listenPort)

	// built-in metrics
	err := op.setupMetricStorage(kubeEventsManagerLabels)
	if err != nil {
		return fmt.Errorf("setup metric storage: %w", err)
	}

	// metrics from user's hooks
	op.setupHookMetricStorage()

	// 'main' Kubernetes client.
	op.KubeClient, err = initDefaultMainKubeClient(mainKubeCfg, op.MetricStorage, op.logger)
	if err != nil {
		return err
	}

	// ObjectPatcher with a separate Kubernetes client.
	op.ObjectPatcher, err = initDefaultObjectPatcher(patcherKubeCfg, op.MetricStorage, op.logger.Named("object-patcher"))
	if err != nil {
		return err
	}

	op.SetupEventManagers()

	return nil
}

// assembleShellOperator creates all dependencies needed for the full-fledged
// ShellOperator using values from cfg.
func (op *ShellOperator) assembleShellOperator(cfg *app.Config, hooksDir string, tempDir string, debugServer *debug.Server, runtimeConfig *config.Config) error {
	registerRootRoute(op)
	// for shell-operator only
	err := metrics.RegisterHookMetrics(op.HookMetricStorage)
	if err != nil {
		return fmt.Errorf("register hook metrics: %w", err)
	}

	op.RegisterDebugQueueRoutes(debugServer)
	op.RegisterDebugHookRoutes(debugServer)
	op.RegisterDebugConfigRoutes(debugServer, runtimeConfig)

	// Create webhookManagers with dependencies.
	op.setupHookManagers(cfg, hooksDir, tempDir)

	// Register the three built-in task type handlers. Extenders may add more
	// handlers via op.taskHandlerRegistry.Register() after this call.
	op.RegisterBuiltinTaskHandlers()

	// Search and configure all hooks.
	err = op.initHookManager()
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
func (op *ShellOperator) setupHookManagers(cfg *app.Config, hooksDir string, tempDir string) {
	// Initialize admission webhooks manager.
	op.AdmissionWebhookManager = admission.NewWebhookManager(op.KubeClient, admission.WithLogger(op.logger.Named("admission-webhook-manager")))
	op.AdmissionWebhookManager.Settings = admission.DefaultSettings
	op.AdmissionWebhookManager.Namespace = cfg.App.Namespace

	// Initialize conversion webhooks manager.
	op.ConversionWebhookManager = conversion.NewWebhookManager(conversion.WithLogger(op.logger.Named("conversion-webhook-manager")))
	op.ConversionWebhookManager.KubeClient = op.KubeClient
	op.ConversionWebhookManager.Settings = conversion.DefaultSettings
	op.ConversionWebhookManager.Namespace = cfg.App.Namespace

	// Initialize Hook manager.
	hookCfg := &hook.ManagerConfig{
		WorkingDir:               hooksDir,
		TempDir:                  tempDir,
		KubeEventsManager:        op.KubeEventsManager,
		ScheduleManager:          op.ScheduleManager,
		AdmissionWebhookManager:  op.AdmissionWebhookManager,
		ConversionWebhookManager: op.ConversionWebhookManager,
		KeepTemporaryHookFiles:   cfg.Debug.KeepTempFiles,
		LogProxyHookJSON:         cfg.Log.ProxyHookJSON,
		LogProxyHookJSONKey:      app.ProxyJsonLogKey,
		Logger:                   op.logger.Named("hook-manager"),
	}
	op.HookManager = hook.NewHookManager(hookCfg)
}
