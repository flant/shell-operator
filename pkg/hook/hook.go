package hook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/kennygrant/sanitize"
	"github.com/romana/rlog"

	"github.com/flant/shell-operator/pkg/executor"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

type Hook struct {
	Name   string // The unique name like '002-prometheus-hooks/startup_hook'.
	Path   string // The absolute path to the executable file.
	Config *HookConfig

	hookManager HookManager
}

func NewHook(name, path string) *Hook {
	return &Hook{
		Name:   name,
		Path:   path,
		Config: &HookConfig{},
	}
}

func (h *Hook) WithConfig(configOutput []byte) (hook *Hook, err error) {
	if err := json.Unmarshal(configOutput, h.Config); err != nil {
		return h, fmt.Errorf("unmarshaling hook '%s' config failed: %s\nhook --config output: %s", h.Name, err.Error(), configOutput)
	}

	err = h.Config.Convert()
	if err != nil {
		return h, fmt.Errorf("load hook config: %s", err)
	}

	return h, nil
}

func (h *Hook) Run(bindingType BindingType, context []BindingContext) error {
	rlog.Infof("Running hook '%s' binding '%s' ...", h.Name, bindingType)

	var versionedContext = ConvertBindingContextList(h.Config.Version, context)

	contextPath, err := h.prepareBindingContextJsonFile(versionedContext)
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

	err = executor.Run(hookCmd)
	if err != nil {
		return fmt.Errorf("%s FAILED: %s", h.Name, err)
	}

	return nil
}

func (h *Hook) SafeName() string {
	return sanitize.BaseName(h.Name)
}

func (h *Hook) prepareBindingContextJsonFile(context interface{}) (string, error) {
	data, _ := json.Marshal(context)
	bindingContextPath := filepath.Join(TempDir, fmt.Sprintf("hook-%s-binding-context.json", h.SafeName()))

	err := ioutil.WriteFile(bindingContextPath, data, 0644)
	if err != nil {
		return "", err
	}

	rlog.Debugf("Prepared hook '%s' binding context:\n%s", h.Name, utils_data.YamlToString(context))

	return bindingContextPath, nil
}
