package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/flant/shell-operator/pkg/executor"
	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

type HookManager interface {
	Init() error
	Run()
	WithDirectories(workingDir string, tempDir string)
	WorkingDir() string
	TempDir() string
	GetHook(name string) (*Hook, error)
	GetHooksInOrder(bindingType BindingType) ([]string, error)
	RunHook(hookName string, binding BindingType, bindingContext []BindingContext, logLabels map[string]string) error
}

type hookManager struct {
	workingDir       string
	tempDir          string
	hooksByName      map[string]*Hook
	hookNamesInOrder []string
	// index to search hooks by binding type
	hooksInOrder map[BindingType][]*Hook
}

// hookManager should implement HookManager
var _ HookManager = &hookManager{}

func NewHookManager() *hookManager {
	return &hookManager{
		hooksByName:      make(map[string]*Hook, 0),
		hookNamesInOrder: make([]string, 0),
		hooksInOrder:     make(map[BindingType][]*Hook, 0),
	}
}

func (hm *hookManager) WithDirectories(workingDir string, tempDir string) {
	hm.workingDir = workingDir
	hm.tempDir = tempDir
}

func (hm *hookManager) WorkingDir() string {
	return hm.workingDir
}

func (hm *hookManager) TempDir() string {
	return hm.tempDir
}

// Init finds executables in WorkingDir, execute them with --config argument and add them into indices.
func (hm *hookManager) Init() error {
	log.Info("Initialize hooks manager. Search for and load all hooks.")

	hm.hooksInOrder = make(map[BindingType][]*Hook)
	hm.hooksByName = make(map[string]*Hook)

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
func (hm *hookManager) loadHook(hookPath string) (hook *Hook, err error) {

	hookName, err := filepath.Rel(hm.workingDir, hookPath)
	if err != nil {
		return nil, err
	}
	hook = NewHook(hookName, hookPath)

	log.WithField("hook", hook.Name).
		WithField("phase", "config").
		Infof("Load config from '%s'", hookPath)

	envs := []string{}
	configOutput, err := execCommandOutput(hm.workingDir, hookPath, envs, []string{"--config"})
	if err != nil {
		log.WithField("hook", hook.Name).
			WithField("phase", "config").
			Errorf("Hook config output:\n%s", string(configOutput))
		return nil, fmt.Errorf("cannot get config for hook '%s': %s", hookPath, err)
	}

	_, err = hook.WithConfig(configOutput)
	if err != nil {
		return nil, fmt.Errorf("creating hook '%s': %s", hookName, err.Error())
	}
	hook.WithHookManager(hm)

	return hook, nil
}

func execCommandOutput(dir string, entrypoint string, envs []string, args []string) ([]byte, error) {
	envs = append(os.Environ(), envs...)
	cmd := executor.MakeCommand(dir, entrypoint, args, envs)
	log.Debugf("Executing hook in %s: '%s'", cmd.Dir, strings.Join(cmd.Args, " "))
	cmd.Stdout = nil

	output, err := executor.Output(cmd)
	if err != nil {
		return output, err
	}

	log.Debugf("Hook '%s' output:\n%s", strings.Join(cmd.Args, " "), string(output))

	return output, nil
}

// HookManager has no events for now.
func (hm *hookManager) Run() {
	panic("implement me")
}

func (hm *hookManager) GetHook(name string) (*Hook, error) {
	hook, exists := hm.hooksByName[name]
	if exists {
		return hook, nil
	} else {
		return nil, fmt.Errorf("hook '%s' not found", name)
	}
}

func (hm *hookManager) GetHooksInOrder(bindingType BindingType) ([]string, error) {
	hooks, ok := hm.hooksInOrder[bindingType]
	if !ok {
		return []string{}, nil
	}

	// OnStartup hooks are sorted by onStartup config value
	if bindingType == OnStartup {
		for _, hook := range hooks {
			if !hook.Config.HasBinding(OnStartup) {
				return nil, fmt.Errorf("possible bug: hook '%s' is registered as OnStartup but has no onStartup value", hook.Name)
			}
		}

		sort.Slice(hooks[:], func(i, j int) bool {
			return hooks[i].Config.OnStartup.Order < hooks[j].Config.OnStartup.Order
		})
	}

	var hooksNames []string
	for _, hook := range hooks {
		hooksNames = append(hooksNames, hook.Name)
	}

	return hooksNames, nil
}

func (hm *hookManager) RunHook(hookName string, binding BindingType, bindingContext []BindingContext, logLabels map[string]string) error {
	hook, err := hm.GetHook(hookName)
	if err != nil {
		return err
	}

	if err := hook.Run(binding, bindingContext, logLabels); err != nil {
		return err
	}

	return nil
}
