package hook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kennygrant/sanitize"
	"github.com/romana/rlog"

	"github.com/flant/shell-operator/pkg/executor"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
	"path"
)

type HookConfig struct {
	OnStartup         interface{}                                   `json:"onStartup"`
	Schedule          []schedule_manager.ScheduleConfig             `json:"schedule"`
	OnKubernetesEvent []kube_events_manager.OnKubernetesEventConfig `json:"onKubernetesEvent"`
}

type Hook struct {
	Name           string // The unique name like '002-prometheus-hooks/startup_hook'.
	Path           string // The absolute path to the executable file.
	Bindings       []BindingType
	OrderByBinding map[BindingType]float64
	Config         *HookConfig

	hookManager HookManager
}

func NewHook(name, path string) *Hook {
	return &Hook{
		Name:           name,
		Path:           path,
		OrderByBinding: make(map[BindingType]float64),
		Bindings:       make([]BindingType, 0),
		Config:         &HookConfig{},
	}
}

func (h *Hook) WithConfig(configOutput []byte) (hook *Hook, err error) {
	if err := json.Unmarshal(configOutput, h.Config); err != nil {
		return h, fmt.Errorf("unmarshaling hook '%s' config failed: %s\nhook --config output: %s", h.Name, err.Error(), configOutput)
	}

	for i := range h.Config.OnKubernetesEvent {
		config := &h.Config.OnKubernetesEvent[i]

		if config.EventTypes == nil {
			h.Config.OnKubernetesEvent[i].EventTypes = []kube_events_manager.OnKubernetesEventType{
				kube_events_manager.KubernetesEventOnAdd,
				kube_events_manager.KubernetesEventOnUpdate,
				kube_events_manager.KubernetesEventOnDelete,
			}
		}

		if config.NamespaceSelector == nil {
			h.Config.OnKubernetesEvent[i].NamespaceSelector = &kube_events_manager.KubeNamespaceSelector{Any: true}
		}
	}

	var ok bool

	if h.Config.OnStartup != nil {
		h.Bindings = append(h.Bindings, OnStartup)
		if h.OrderByBinding[OnStartup], ok = h.Config.OnStartup.(float64); !ok {
			return h, fmt.Errorf("unsuported value '%v' for binding '%s'", h.Config.OnStartup, OnStartup)
		}
	}

	if len(h.Config.Schedule) != 0 {
		h.Bindings = append(h.Bindings, Schedule)
	}

	if len(h.Config.OnKubernetesEvent) != 0 {
		h.Bindings = append(h.Bindings, KubeEvents)
	}

	return h, nil
}

func (h *Hook) Run(bindingType BindingType, context []BindingContext) error {
	rlog.Infof("Running hook '%s' binding '%s' ...", h.Name, bindingType)

	contextPath, err := h.prepareBindingContextJsonFile(context)
	if err != nil {
		return err
	}

	envs := []string{}
	envs = append(envs, os.Environ()...)
	if contextPath != "" {
		envs = append(envs, fmt.Sprintf("BINDING_CONTEXT_PATH=%s", contextPath))
	}
	envs = append(envs, fmt.Sprintf("WORKING_DIR=%s", WorkingDir))

	hookCmd := executor.MakeCommand(path.Dir(h.Path), h.Path, []string{}, envs)

	err = executor.Run(hookCmd, true)
	if err != nil {
		return fmt.Errorf("%s FAILED: %s", h.Name, err)
	}

	return nil
}

func (h *Hook) SafeName() string {
	return sanitize.BaseName(h.Name)
}

func (h *Hook) prepareBindingContextJsonFile(context []BindingContext) (string, error) {
	data, _ := json.Marshal(context)
	//data := utils.MustDump(utils.DumpValuesJson(context))
	path := filepath.Join(TempDir, fmt.Sprintf("global-hook-%s-binding-context.json", h.SafeName()))
	err := dumpData(path, data)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared global hook %s binding context:\n%s", h.Name, utils_data.YamlToString(context))

	return path, nil
}

func dumpData(filePath string, data []byte) error {
	err := ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
