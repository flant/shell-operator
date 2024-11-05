package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/flant/shell-operator/pkg/executor"
	"github.com/flant/shell-operator/pkg/hook/controller"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

type Manager struct {
	// dependencies
	workingDir               string
	tempDir                  string
	kubeEventsManager        kube_events_manager.KubeEventsManager
	scheduleManager          schedule_manager.ScheduleManager
	conversionWebhookManager *conversion.WebhookManager
	admissionWebhookManager  *admission.WebhookManager

	// sorted hook names
	hookNamesInOrder []string

	// index by name
	hooksByName map[string]*Hook
	// index to search hooks by binding type
	hooksInOrder map[BindingType][]*Hook

	// Index crdName -> fromVersion -> conversionLink
	conversionChains *conversion.ChainStorage

	logger *log.Logger
}

// ManagerConfig sets configuration for Manager
type ManagerConfig struct {
	WorkingDir string
	TempDir    string
	Kmgr       kube_events_manager.KubeEventsManager
	Smgr       schedule_manager.ScheduleManager
	Wmgr       *admission.WebhookManager
	Cmgr       *conversion.WebhookManager

	Logger *log.Logger
}

func NewHookManager(config *ManagerConfig) *Manager {
	return &Manager{
		hooksByName:      make(map[string]*Hook),
		hookNamesInOrder: make([]string, 0),
		hooksInOrder:     make(map[BindingType][]*Hook),
		conversionChains: conversion.NewChainStorage(),

		workingDir:               config.WorkingDir,
		tempDir:                  config.TempDir,
		kubeEventsManager:        config.Kmgr,
		scheduleManager:          config.Smgr,
		admissionWebhookManager:  config.Wmgr,
		conversionWebhookManager: config.Cmgr,

		logger: config.Logger,
	}
}

func (hm *Manager) WorkingDir() string {
	return hm.workingDir
}

func (hm *Manager) TempDir() string {
	return hm.tempDir
}

// Init finds executables in WorkingDir, execute them with --config argument and add them into indices.
func (hm *Manager) Init() error {
	log.Info("Initialize hooks manager. Search for and load all hooks.")

	hm.hooksInOrder = make(map[BindingType][]*Hook)
	hm.hooksByName = make(map[string]*Hook)

	if err := utils_file.RecursiveCheckLibDirectory(hm.workingDir); err != nil {
		log.Errorf("failed to check lib directory %s: %v", hm.workingDir, err)
	}

	hooksRelativePaths, err := utils_file.RecursiveGetExecutablePaths(hm.workingDir)
	if err != nil {
		return err
	}

	// sort hooks by path
	sort.Strings(hooksRelativePaths)
	log.Debugf("  Search hooks in this paths: %+v", hooksRelativePaths)

	for _, hookPath := range hooksRelativePaths {
		hook, err := hm.loadHook(hookPath)
		if err != nil {
			return err
		}

		// register hook in indices
		for _, binding := range hook.Config.Bindings() {
			hm.hooksInOrder[binding] = append(hm.hooksInOrder[binding], hook)
		}
		hm.hooksByName[hook.Name] = hook
		hm.hookNamesInOrder = append(hm.hookNamesInOrder, hook.Name)
	}

	// Validate conversion chains and create index with conversion paths.
	err = hm.UpdateConversionChains()
	if err != nil {
		return fmt.Errorf("check conversion configs: %v", err)
	}

	return nil
}

// TODO move --config execution to a Hook method
func (hm *Manager) loadHook(hookPath string) (hook *Hook, err error) {
	hookName, err := filepath.Rel(hm.workingDir, hookPath)
	if err != nil {
		return nil, err
	}
	hook = NewHook(hookName, hookPath, hm.logger.Named("hook"))

	hookEntry := hm.logger.With("hook", hook.Name).
		With("phase", "config")

	hookEntry.Infof("Load config from '%s'", hookPath)

	envs := make([]string, 0)
	configOutput, err := hm.execCommandOutput(hook.Name, hm.workingDir, hookPath, envs, []string{"--config"})
	if err != nil {
		hookEntry.Errorf("Hook config output:\n%s", string(configOutput))
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			hookEntry.Errorf("Hook config stderr:\n%s", string(ee.Stderr))
		}
		return nil, fmt.Errorf("cannot get config for hook '%s': %s", hookPath, err)
	}

	_, err = hook.LoadConfig(configOutput)
	if err != nil {
		return nil, fmt.Errorf("creating hook '%s': %s", hookName, err.Error())
	}

	// Add hook info as log labels, update MetricLabels
	for _, kubeCfg := range hook.GetConfig().OnKubernetesEvents {
		kubeCfg.Monitor.Metadata.LogLabels["hook"] = hook.Name
		kubeCfg.Monitor.Metadata.MetricLabels = map[string]string{
			"hook":    hook.Name,
			"binding": kubeCfg.BindingName,
			"queue":   kubeCfg.Queue,
		}
	}
	for _, conversionCfg := range hook.GetConfig().KubernetesConversion {
		conversionCfg.Webhook.Metadata.LogLabels["hook"] = hook.Name
		conversionCfg.Webhook.Metadata.MetricLabels = map[string]string{
			"hook":    hook.Name,
			"binding": conversionCfg.BindingName,
		}
	}

	for _, validatingCfg := range hook.GetConfig().KubernetesValidating {
		validatingCfg.Webhook.Metadata.LogLabels["hook"] = hook.Name
		validatingCfg.Webhook.Metadata.MetricLabels = map[string]string{
			"hook":    hook.Name,
			"binding": validatingCfg.BindingName,
		}
		validatingCfg.Webhook.UpdateIds("", validatingCfg.BindingName)
	}
	for _, mutatingCfg := range hook.GetConfig().KubernetesMutating {
		mutatingCfg.Webhook.Metadata.LogLabels["hook"] = hook.Name
		mutatingCfg.Webhook.Metadata.MetricLabels = map[string]string{
			"hook":    hook.Name,
			"binding": mutatingCfg.BindingName,
		}
		mutatingCfg.Webhook.UpdateIds("", mutatingCfg.BindingName)
	}

	hookCtrl := controller.NewHookController()
	hookCtrl.InitKubernetesBindings(hook.GetConfig().OnKubernetesEvents, hm.kubeEventsManager, hm.logger.Named("kubernetes-bindings"))
	hookCtrl.InitScheduleBindings(hook.GetConfig().Schedules, hm.scheduleManager)
	hookCtrl.InitConversionBindings(hook.GetConfig().KubernetesConversion, hm.conversionWebhookManager)
	hookCtrl.InitAdmissionBindings(hook.GetConfig().KubernetesValidating, hook.GetConfig().KubernetesMutating, hm.admissionWebhookManager)
	// TODO
	// hookCtrl.InitMutatingBindings(hook.GetConfig().KubernetesMutating, hm.admissionWebhookManager)

	hook.WithHookController(hookCtrl)
	hook.WithTmpDir(hm.TempDir())

	if hook.Config == nil {
		return nil, fmt.Errorf("hook %q is marked as executable but doesn't contain config section", hook.Path)
	}

	hookEntry.Infof("Loaded config: %s", hook.GetConfigDescription())

	return hook, nil
}

func (hm *Manager) execCommandOutput(hookName string, dir string, entrypoint string, envs []string, args []string) ([]byte, error) {
	envs = append(os.Environ(), envs...)
	cmd := executor.MakeCommand(dir, entrypoint, args, envs)
	cmd.Stdout = nil
	cmd.Stderr = nil

	debugEntry := hm.logger.With("hook", hookName).
		With("cmd", strings.Join(cmd.Args, " "))

	debugEntry.Debugf("Executing hook in %s", cmd.Dir)

	output, err := executor.Output(cmd)
	if err != nil {
		return output, err
	}

	debugEntry.Debugf("output:\n%s", string(output))

	return output, nil
}

func (hm *Manager) GetHook(name string) *Hook {
	hook, exists := hm.hooksByName[name]
	if exists {
		return hook
	}
	log.Errorf("Possible bug!!! Hook '%s' not found in hook manager", name)
	return nil
}

func (hm *Manager) GetHookNames() []string {
	return hm.hookNamesInOrder
}

func (hm *Manager) GetHooksInOrder(bindingType BindingType) ([]string, error) {
	hooks, ok := hm.hooksInOrder[bindingType]
	if !ok {
		return []string{}, nil
	}

	// OnStartup hooks are sorted by onStartup config value
	// FIXME: onStartup value is now a config validating error, no need to check it here again.
	if bindingType == OnStartup {
		for _, hook := range hooks {
			if !hook.Config.HasBinding(OnStartup) {
				return nil, fmt.Errorf("possible bug: hook '%s' is registered as OnStartup but has no onStartup value", hook.Name)
			}
		}

		sort.Slice(hooks, func(i, j int) bool {
			return hooks[i].Config.OnStartup.Order < hooks[j].Config.OnStartup.Order
		})
	}

	hooksNames := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		hooksNames = append(hooksNames, hook.Name)
	}

	return hooksNames, nil
}

func (hm *Manager) HandleKubeEvent(kubeEvent KubeEvent, createTaskFn func(*Hook, controller.BindingExecutionInfo)) {
	kubeHooks, _ := hm.GetHooksInOrder(OnKubernetesEvent)

	for _, hookName := range kubeHooks {
		h := hm.GetHook(hookName)

		if h.HookController.CanHandleKubeEvent(kubeEvent) {
			h.HookController.HandleKubeEvent(kubeEvent, func(info controller.BindingExecutionInfo) {
				if createTaskFn != nil {
					createTaskFn(h, info)
				}
			})
		}
	}
}

func (hm *Manager) HandleScheduleEvent(crontab string, createTaskFn func(*Hook, controller.BindingExecutionInfo)) {
	schHooks, _ := hm.GetHooksInOrder(Schedule)
	for _, hookName := range schHooks {
		h := hm.GetHook(hookName)
		if h.HookController.CanHandleScheduleEvent(crontab) {
			h.HookController.HandleScheduleEvent(crontab, func(info controller.BindingExecutionInfo) {
				if createTaskFn != nil {
					createTaskFn(h, info)
				}
			})
		}
	}
}

func (hm *Manager) HandleAdmissionEvent(event admission.Event, createTaskFn func(*Hook, controller.BindingExecutionInfo)) {
	vHooks, _ := hm.GetHooksInOrder(KubernetesValidating)
	for _, hookName := range vHooks {
		h := hm.GetHook(hookName)
		if h.HookController.CanHandleAdmissionEvent(event) {
			h.HookController.HandleAdmissionEvent(event, func(info controller.BindingExecutionInfo) {
				if createTaskFn != nil {
					createTaskFn(h, info)
				}
			})
		}
	}

	mHooks, _ := hm.GetHooksInOrder(KubernetesMutating)
	for _, hookName := range mHooks {
		h := hm.GetHook(hookName)
		if h.HookController.CanHandleAdmissionEvent(event) {
			h.HookController.HandleAdmissionEvent(event, func(info controller.BindingExecutionInfo) {
				if createTaskFn != nil {
					createTaskFn(h, info)
				}
			})
		}
	}
}

func (hm *Manager) DetectAdmissionEventType(event admission.Event) BindingType {
	vHooks, _ := hm.GetHooksInOrder(KubernetesValidating)
	for _, hookName := range vHooks {
		h := hm.GetHook(hookName)
		if h.HookController.CanHandleAdmissionEvent(event) {
			return KubernetesValidating
		}
	}

	mHooks, _ := hm.GetHooksInOrder(KubernetesMutating)
	for _, hookName := range mHooks {
		h := hm.GetHook(hookName)
		if h.HookController.CanHandleAdmissionEvent(event) {
			return KubernetesMutating
		}
	}

	log.Errorf("Possible bug!!! No linked hook for admission event %s %s kind=%s name=%s ns=%s", event.ConfigurationId, event.WebhookId, event.Request.Kind, event.Request.Name, event.Request.Namespace)
	return ""
}

// HandleConversionEvent receives a crdName and calculates a sequence of hooks to run.
func (hm *Manager) HandleConversionEvent(crdName string, request *v1.ConversionRequest, rule conversion.Rule, createTaskFn func(*Hook, controller.BindingExecutionInfo)) {
	vHooks, _ := hm.GetHooksInOrder(KubernetesConversion)

	for _, hookName := range vHooks {
		h := hm.GetHook(hookName)
		if h.HookController.CanHandleConversionEvent(crdName, request, rule) {
			h.HookController.HandleConversionEvent(crdName, request, rule, func(info controller.BindingExecutionInfo) {
				if createTaskFn != nil {
					createTaskFn(h, info)
				}
			})
		}
	}
}

func (hm *Manager) UpdateConversionChains() error {
	vHooks, _ := hm.GetHooksInOrder(KubernetesConversion)

	// Update conversionChains.
	for _, hookName := range vHooks {
		h := hm.GetHook(hookName)

		for _, cfg := range h.Config.KubernetesConversion {
			crdName := cfg.Webhook.CrdName
			chain := hm.conversionChains.Get(crdName)
			for _, conversionRule := range cfg.Webhook.Rules {
				chain.Put(conversionRule)
			}
		}
	}

	return nil
}

func (hm *Manager) FindConversionChain(crdName string, rule conversion.Rule) []conversion.Rule {
	return hm.conversionChains.FindConversionChain(crdName, rule)
}
