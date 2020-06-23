package hook

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kennygrant/sanitize"
	uuid "gopkg.in/satori/go.uuid.v1"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
)

type CommonHook interface {
	Name() string
}

type Hook struct {
	Name   string // The unique name like '002-prometheus-hooks/startup_hook'.
	Path   string // The absolute path to the executable file.
	Config *HookConfig

	HookController controller.HookController

	TmpDir string
}

func NewHook(name, path string) *Hook {
	return &Hook{
		Name:   name,
		Path:   path,
		Config: &HookConfig{},
	}
}

func (h *Hook) WithTmpDir(dir string) {
	h.TmpDir = dir
}

func (h *Hook) WithConfig(configOutput []byte) (hook *Hook, err error) {
	err = h.Config.LoadAndValidate(configOutput)
	if err != nil {
		return h, fmt.Errorf("load hook '%s' config: %s\nhook --config output: %s", h.Name, err.Error(), configOutput)
	}

	return h, nil
}

func (h *Hook) GetConfig() *HookConfig {
	return h.Config
}

func (h *Hook) WithHookController(hookController controller.HookController) {
	h.HookController = hookController
}

func (h *Hook) GetHookController() controller.HookController {
	return h.HookController
}

func (h *Hook) Run(bindingType BindingType, context []BindingContext, logLabels map[string]string) ([]operation.MetricOperation, error) {
	// Refresh snapshots
	freshBindingContext := h.HookController.UpdateSnapshots(context)

	versionedContextList := ConvertBindingContextList(h.Config.Version, freshBindingContext)

	contextPath, err := h.prepareBindingContextJsonFile(versionedContextList)
	if err != nil {
		return nil, err
	}

	metricsPath, err := h.prepareMetricsFile()
	if err != nil {
		return nil, err
	}

	// remove tmp file on hook exit
	defer func() {
		if app.DebugKeepTmpFiles != "yes" {
			os.Remove(contextPath)
			os.Remove(metricsPath)
		}
	}()

	envs := []string{}
	envs = append(envs, os.Environ()...)
	if contextPath != "" {
		envs = append(envs, fmt.Sprintf("BINDING_CONTEXT_PATH=%s", contextPath))
		envs = append(envs, fmt.Sprintf("METRICS_PATH=%s", metricsPath))
	}

	hookCmd := executor.MakeCommand(path.Dir(h.Path), h.Path, []string{}, envs)

	err = executor.RunAndLogLines(hookCmd, logLabels)
	if err != nil {
		return nil, fmt.Errorf("%s FAILED: %s", h.Name, err)
	}

	metrics, err := operation.MetricOperationsFromFile(metricsPath)
	if err != nil {
		return nil, fmt.Errorf("got bad metrics: %s", err)
	}

	return metrics, nil
}

func (h *Hook) SafeName() string {
	return sanitize.BaseName(h.Name)
}

func (h *Hook) GetConfigDescription() string {
	msgs := []string{}
	if h.Config.OnStartup != nil {
		msgs = append(msgs, fmt.Sprintf("OnStartup:%d", int64(h.Config.OnStartup.Order)))
	}
	if len(h.Config.Schedules) > 0 {
		crontabs := map[string]bool{}
		for _, schCfg := range h.Config.Schedules {
			crontabs[schCfg.ScheduleEntry.Crontab] = true
		}
		crontabList := []string{}
		for crontab := range crontabs {
			crontabList = append(crontabList, crontab)
		}
		msgs = append(msgs, fmt.Sprintf("Schedules: '%s'", strings.Join(crontabList, "', '")))
	}
	if len(h.Config.OnKubernetesEvents) > 0 {
		kinds := map[string]bool{}
		for _, kubeCfg := range h.Config.OnKubernetesEvents {
			kinds[kubeCfg.Monitor.Kind] = true
		}
		kindList := []string{}
		for kind := range kinds {
			kindList = append(kindList, kind)
		}
		msgs = append(msgs, fmt.Sprintf("Watch k8s kinds: '%s'", strings.Join(kindList, "', '")))
	}
	return strings.Join(msgs, ", ")
}

func (h *Hook) prepareBindingContextJsonFile(context BindingContextList) (string, error) {
	var err error
	data, err := context.Json()
	if err != nil {
		return "", err
	}

	bindingContextPath := filepath.Join(h.TmpDir, fmt.Sprintf("hook-%s-binding-context-%s.json", h.SafeName(), uuid.NewV4().String()))

	err = ioutil.WriteFile(bindingContextPath, data, 0644)
	if err != nil {
		return "", err
	}

	return bindingContextPath, nil
}

func (h *Hook) prepareMetricsFile() (string, error) {
	metricsPath := filepath.Join(h.TmpDir, fmt.Sprintf("hook-%s-metrics-%s.json", h.SafeName(), uuid.NewV4().String()))

	err := ioutil.WriteFile(metricsPath, []byte{}, 0644)
	if err != nil {
		return "", err
	}

	return metricsPath, nil
}
