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
	. "github.com/flant/shell-operator/pkg/validating_webhook/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
)

type CommonHook interface {
	Name() string
}

type HookResult struct {
	Usage              *executor.CmdUsage
	Metrics            []operation.MetricOperation
	ValidatingResponse *ValidatingResponse
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

func (h *Hook) Run(bindingType BindingType, context []BindingContext, logLabels map[string]string) (*HookResult, error) {
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

	validatingPath, err := h.prepareValidatingResponseFile()
	if err != nil {
		return nil, err
	}

	// remove tmp file on hook exit
	defer func() {
		if app.DebugKeepTmpFiles != "yes" {
			os.Remove(contextPath)
			os.Remove(metricsPath)
			os.Remove(validatingPath)
		}
	}()

	envs := []string{}
	envs = append(envs, os.Environ()...)
	if contextPath != "" {
		envs = append(envs, fmt.Sprintf("BINDING_CONTEXT_PATH=%s", contextPath))
		envs = append(envs, fmt.Sprintf("METRICS_PATH=%s", metricsPath))
		envs = append(envs, fmt.Sprintf("VALIDATING_RESPONSE_PATH=%s", validatingPath))
	}

	hookCmd := executor.MakeCommand(path.Dir(h.Path), h.Path, []string{}, envs)

	result := &HookResult{}

	result.Usage, err = executor.RunAndLogLines(hookCmd, logLabels)
	if err != nil {
		return result, fmt.Errorf("%s FAILED: %s", h.Name, err)
	}

	result.Metrics, err = operation.MetricOperationsFromFile(metricsPath)
	if err != nil {
		return result, fmt.Errorf("got bad metrics: %s", err)
	}

	result.ValidatingResponse, err = ValidatingResponseFromFile(validatingPath)
	if err != nil {
		return result, fmt.Errorf("got bad validating response: %s", err)
	}

	return result, nil
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
	if len(h.Config.KubernetesValidating) > 0 {
		kinds := map[string]bool{}
		for _, validating := range h.Config.KubernetesValidating {
			if validating.Webhook == nil {
				continue
			}
			for _, rule := range validating.Webhook.Rules {
				for _, resource := range rule.Resources {
					kinds[strings.ToLower(resource)] = true
				}
			}
		}
		kindList := []string{}
		for kind := range kinds {
			kindList = append(kindList, kind)
		}
		msgs = append(msgs, fmt.Sprintf("Validate k8s kinds: '%s'", strings.Join(kindList, "', '")))
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

func (h *Hook) prepareValidatingResponseFile() (string, error) {
	validatingPath := filepath.Join(h.TmpDir, fmt.Sprintf("hook-%s-validating-response-%s.json", h.SafeName(), uuid.NewV4().String()))

	err := ioutil.WriteFile(validatingPath, []byte{}, 0644)
	if err != nil {
		return "", err
	}

	return validatingPath, nil
}
