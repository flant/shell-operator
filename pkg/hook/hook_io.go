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

package hook

import (
	"fmt"
	"os"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// HookEnv holds the temporary file paths created for a single hook execution.
// The file paths are passed to the hook subprocess via environment variables.
type HookEnv struct {
	ContextPath         string
	MetricsPath         string
	AdmissionPath       string
	ConversionPath      string
	KubernetesPatchPath string
}

// Envs returns the environment variable pairs that expose the file paths
// to the hook subprocess. Returns nil when ContextPath is empty.
func (e *HookEnv) Envs() []string {
	envs := os.Environ()
	if e.ContextPath != "" {
		envs = append(envs,
			"BINDING_CONTEXT_PATH="+e.ContextPath,
			"METRICS_PATH="+e.MetricsPath,
			"CONVERSION_RESPONSE_PATH="+e.ConversionPath,
			"VALIDATING_RESPONSE_PATH="+e.AdmissionPath,
			"ADMISSION_RESPONSE_PATH="+e.AdmissionPath,
			"KUBERNETES_PATCH_PATH="+e.KubernetesPatchPath,
		)
	}
	return envs
}

// IOProvider prepares temporary files before hook execution and cleans them up after.
// The default implementation (hookIOProvider) writes to the OS temp directory.
// Tests may supply a different implementation backed by t.TempDir() or in-memory buffers.
type IOProvider interface {
	Prepare(versionedCtx bctx.BindingContextList) (*HookEnv, error)
	Cleanup(env *HookEnv)
}

// ResponseParser reads the hook output files and populates a Result.
// The default implementation (hookResponseParser) reads from the real filesystem.
// Tests may supply a stub that returns pre-canned values without touching disk.
type ResponseParser interface {
	ParseResult(hookName string, env *HookEnv, result *Result) error
}

// hookIOProvider is the default IOProvider that writes to the hook's TmpDir.
type hookIOProvider struct {
	hook *Hook
}

func (p *hookIOProvider) Prepare(versionedCtx bctx.BindingContextList) (*HookEnv, error) {
	contextPath, err := p.hook.prepareBindingContextJsonFile(versionedCtx)
	if err != nil {
		return nil, err
	}

	metricsPath, err := p.hook.prepareMetricsFile()
	if err != nil {
		return nil, err
	}

	admissionPath, err := p.hook.prepareAdmissionResponseFile()
	if err != nil {
		return nil, err
	}

	conversionPath, err := p.hook.prepareConversionResponseFile()
	if err != nil {
		return nil, err
	}

	kubernetesPatchPath, err := p.hook.prepareObjectPatchFile()
	if err != nil {
		return nil, err
	}

	return &HookEnv{
		ContextPath:         contextPath,
		MetricsPath:         metricsPath,
		AdmissionPath:       admissionPath,
		ConversionPath:      conversionPath,
		KubernetesPatchPath: kubernetesPatchPath,
	}, nil
}

func (p *hookIOProvider) Cleanup(env *HookEnv) {
	if p.hook.KeepTemporaryHookFiles {
		return
	}
	_ = os.Remove(env.ContextPath)
	_ = os.Remove(env.MetricsPath)
	_ = os.Remove(env.ConversionPath)
	_ = os.Remove(env.AdmissionPath)
	_ = os.Remove(env.KubernetesPatchPath)
}

// hookResponseParser is the default ResponseParser that reads the real output files.
type hookResponseParser struct {
	hook *Hook
}

func (p *hookResponseParser) ParseResult(hookName string, env *HookEnv, result *Result) error {
	operations, err := MetricOperationsFromFile(env.MetricsPath, hookName)
	if err != nil {
		return fmt.Errorf("got bad metrics: %s", err)
	}
	result.Metrics = p.hook.remapOperationsToOperations(operations)

	result.AdmissionResponse, err = admission.ResponseFromFile(env.AdmissionPath)
	if err != nil {
		return fmt.Errorf("got bad validating response: %s", err)
	}

	result.ConversionResponse, err = conversion.ResponseFromFile(env.ConversionPath)
	if err != nil {
		return fmt.Errorf("got bad conversion response: %s", err)
	}

	result.KubernetesPatchBytes, err = os.ReadFile(env.KubernetesPatchPath)
	if err != nil {
		return fmt.Errorf("can't read object patch file: %s", err)
	}

	return nil
}
