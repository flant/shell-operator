package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/romana/rlog"

	"github.com/flant/shell-operator/pkg/executor"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

type HookManager interface {
	Run()
	GetHook(name string) (*Hook, error)
	GetHooksInOrder(bindingType BindingType) []string
	RunHook(hookName string, binding BindingType, bindingContext []BindingContext) error
}

type MainHookManager struct {
	hooksByName      map[string]*Hook
	hookNamesInOrder []string
	// index to search hooks by binding type
	hooksInOrder map[BindingType][]*Hook
}

var (
	WorkingDir string
	TempDir    string
	EventCh    chan Event
)

type EventType string

const (
	HooksLoaded EventType = "HOOKS_LOADED"
)

type Event struct {
	Type EventType
}

// Additional info from schedule and kube events
type BindingContext struct {
	Binding string `json:"binding"`
	// event type from kube API
	WatchEvent string `json:"watchEvent,omitempty"`
	// lower cased event type
	ResourceEvent     string `json:"resourceEvent,omitempty"` // deprecated
	ResourceNamespace string `json:"resourceNamespace,omitempty"`
	ResourceKind      string `json:"resourceKind,omitempty"`
	ResourceName      string `json:"resourceName,omitempty"`
}

func Init(workingDir string, tempDir string) (HookManager, error) {
	rlog.Info("Initialize hooks manager ...")

	TempDir = tempDir
	WorkingDir = workingDir

	hm := NewMainHookManager()

	EventCh = make(chan Event, 2)

	if err := hm.loadAllHooks(); err != nil {
		return nil, err
	}

	EventCh <- Event{Type: HooksLoaded}

	return hm, nil
}

func NewMainHookManager() *MainHookManager {
	return &MainHookManager{
		hooksByName:      make(map[string]*Hook, 0),
		hookNamesInOrder: make([]string, 0),
		hooksInOrder:     make(map[BindingType][]*Hook, 0),
	}
}

// loadAllHooks finds executables in WorkingDir, execute them with --config argument and add them into indices.
func (hm *MainHookManager) loadAllHooks() error {
	rlog.Info("Search and load hooks ...")

	hm.hooksInOrder = make(map[BindingType][]*Hook)
	hm.hooksByName = make(map[string]*Hook)

	hooksDir := WorkingDir

	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return nil
	}

	hooksRelativePaths, err := utils_file.RecursiveGetExecutablePaths(hooksDir)
	if err != nil {
		return err
	}

	// sort hooks by path
	sort.Strings(hooksRelativePaths)
	rlog.Debugf("  Hook paths: %+v", hooksRelativePaths)

	for _, hookPath := range hooksRelativePaths {
		hook, err := hm.loadHook(hookPath)
		if err != nil {
			return err
		}

		// register hook in indexes
		for _, binding := range hook.Config.Bindings() {
			hm.hooksInOrder[binding] = append(hm.hooksInOrder[binding], hook)
		}
		hm.hooksByName[hook.Name] = hook
		hm.hookNamesInOrder = append(hm.hookNamesInOrder, hook.Name)
	}

	return nil
}

// TODO move --config execution to a Hook method
func (hm *MainHookManager) loadHook(hookPath string) (hook *Hook, err error) {
	rlog.Infof("Load hook config from '%s'", hookPath)

	envs := []string{}
	envs = append(envs, fmt.Sprintf("WORKING_DIR=%s", WorkingDir))

	configOutput, err := execCommandOutput(WorkingDir, hookPath, envs, []string{"--config"})
	if err != nil {
		return nil, fmt.Errorf("cannot get config for hook '%s': %s", hookPath, err)
	}

	hookName, err := filepath.Rel(WorkingDir, hookPath)
	if err != nil {
		return nil, err
	}

	hook, err = NewHook(hookName, hookPath).WithConfig(configOutput)
	if err != nil {
		return nil, fmt.Errorf("creating hook '%s' failed: %s", hookName, err.Error())
	}

	return hook, nil
}

func execCommandOutput(dir string, entrypoint string, envs []string, args []string) ([]byte, error) {
	envs = append(os.Environ(), envs...)
	cmd := executor.MakeCommand(dir, entrypoint, args, envs)
	rlog.Debugf("Executing hook in %s: '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	cmd.Stdout = nil

	output, err := executor.Output(cmd)
	if err != nil {
		rlog.Errorf("Hook '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))
		return output, err
	}

	rlog.Debugf("Hook '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))

	return output, nil
}

// HookManager has no events for now unlike addon-operator
func (hm *MainHookManager) Run() {
	panic("implement me")
}

func (hm *MainHookManager) GetHook(name string) (*Hook, error) {
	hook, exists := hm.hooksByName[name]
	if exists {
		return hook, nil
	} else {
		return nil, fmt.Errorf("hook '%s' not found", name)
	}
}

func (hm *MainHookManager) GetHooksInOrder(bindingType BindingType) []string {
	hooks, ok := hm.hooksInOrder[bindingType]
	if !ok {
		return []string{}
	}

	// OnStartup hooks are sorted by onStartup config value
	if bindingType == OnStartup {
		sort.Slice(hooks[:], func(i, j int) bool {
			if !hooks[i].Config.HasBinding(OnStartup) {
				rlog.Errorf("hook '%s' is registered as OnStartup but has no onStartup value", hooks[i].Name)
			}
			if !hooks[j].Config.HasBinding(OnStartup) {
				rlog.Errorf("hook '%s' is registered as OnStartup but has no onStartup value", hooks[j].Name)
			}

			return hooks[i].Config.OnStartup.Order < hooks[j].Config.OnStartup.Order
		})
	}

	var hooksNames []string
	for _, hook := range hooks {
		hooksNames = append(hooksNames, hook.Name)
	}

	return hooksNames
}

func (hm *MainHookManager) RunHook(hookName string, binding BindingType, bindingContext []BindingContext) error {
	hook, err := hm.GetHook(hookName)
	if err != nil {
		return err
	}

	if err := hook.Run(binding, bindingContext); err != nil {
		return err
	}

	return nil
}
