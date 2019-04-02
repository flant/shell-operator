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

type BindingType string

const (
	Schedule   BindingType = "SCHEDULE"
	OnStartup  BindingType = "ON_STARTUP"
	KubeEvents BindingType = "KUBE_EVENTS"
)

var ContextBindingType = map[BindingType]string{
	Schedule:   "schedule",
	OnStartup:  "onStartup",
	KubeEvents: "onKubernetesEvent",
}

// Additional info from schedule and kube events
type BindingContext struct {
	Binding           string `json:"binding"`
	ResourceEvent     string `json:"resourceEvent,omitempty"`
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

// loadAllHooks finds executables in WorkDir, execute them with --config argument and add them into indices.
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
		if err := hm.loadHook(hookPath); err != nil {
			return err
		}
	}

	return nil
}

func (hm *MainHookManager) loadHook(hookPath string) (err error) {
	rlog.Infof("Load hook config from '%s'", hookPath)

	configOutput, err := execCommandOutput(WorkingDir, hookPath, []string{}, []string{"--config"})
	if err != nil {
		return fmt.Errorf("cannot get config for hook '%s': %s", hookPath, err)
	}

	hookName, err := filepath.Rel(WorkingDir, hookPath)
	if err != nil {
		return err
	}

	hook, err := NewHook(hookName, hookPath).WithConfig(configOutput)
	if err != nil {
		return fmt.Errorf("creating hook '%s' failed: %s", hookName, err.Error())
	}

	hm.addHook(hook)

	return nil
}

func (hm *MainHookManager) addHook(hook *Hook) {
	if hook.Config.OnStartup != nil {
		hm.hooksInOrder[OnStartup] = append(hm.hooksInOrder[OnStartup], hook)
	}

	if len(hook.Config.Schedule) != 0 {
		hm.hooksInOrder[Schedule] = append(hm.hooksInOrder[Schedule], hook)
	}

	if len(hook.Config.OnKubernetesEvent) != 0 {
		hm.hooksInOrder[KubeEvents] = append(hm.hooksInOrder[KubeEvents], hook)
	}

	hm.hooksByName[hook.Name] = hook
	hm.hookNamesInOrder = append(hm.hookNamesInOrder, hook.Name)
	return
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

// HookManager has no events for now unlike antiopa
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

	sort.Slice(hooks[:], func(i, j int) bool {
		return hooks[i].OrderByBinding[bindingType] < hooks[j].OrderByBinding[bindingType]
	})

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
