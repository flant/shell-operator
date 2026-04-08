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

package shell_operator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/gofrs/uuid/v5"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	pkg "github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
	utils "github.com/flant/shell-operator/pkg/utils/labels"
	"github.com/flant/shell-operator/pkg/webhook/admission"
	"github.com/flant/shell-operator/pkg/webhook/conversion"
)

// TaskRunner executes a task synchronously and returns its result.
// ShellOperator.taskHandler satisfies this type.
type TaskRunner func(ctx context.Context, t task.Task) queue.TaskResult

// AdmissionEventHandler handles admission webhook events by creating a hook task,
// running it synchronously, and returning the hook's admission response.
type AdmissionEventHandler struct {
	hookManager hook.HookManager
	taskRunner  TaskRunner
	logger      *log.Logger
}

// NewAdmissionEventHandler creates a new AdmissionEventHandler.
func NewAdmissionEventHandler(hm hook.HookManager, runner TaskRunner, logger *log.Logger) *AdmissionEventHandler {
	return &AdmissionEventHandler{
		hookManager: hm,
		taskRunner:  runner,
		logger:      logger,
	}
}

// Handle implements the admission event handler func signature expected by
// admission.WebhookManager.WithAdmissionEventHandler.
func (h *AdmissionEventHandler) Handle(ctx context.Context, event admission.Event) (*admission.Response, error) {
	eventBindingType := h.hookManager.DetectAdmissionEventType(event)
	logLabels := map[string]string{
		pkg.LogKeyEventID: uuid.Must(uuid.NewV4()).String(),
		pkg.LogKeyEvent:   string(eventBindingType),
	}
	logEntry := utils.EnrichLoggerWithLabels(h.logger, logLabels)
	logEntry = logEntry.With(
		slog.String(pkg.LogKeyType, string(eventBindingType)),
		slog.String(pkg.LogKeyConfigurationId, event.ConfigurationId),
		slog.String(pkg.LogKeyWebhookID, event.WebhookId))
	logEntry.Debug("Handle event")

	var admissionTask task.Task
	h.hookManager.HandleAdmissionEvent(ctx, event, func(h *hook.Hook, info controller.BindingExecutionInfo) {
		admissionTask = globalHookTaskFactory.NewHookRunTask(h.Name, eventBindingType, info, logLabels)
	})

	if admissionTask == nil {
		logEntry.Error("Possible bug!!! No hook found for event")
		return nil, fmt.Errorf("no hook found for '%s' '%s'", event.ConfigurationId, event.WebhookId)
	}

	res := h.taskRunner(ctx, admissionTask)

	if res.Status == "Fail" {
		return &admission.Response{
			Allowed: false,
			Message: "Hook failed",
		}, nil
	}

	admissionProp := admissionTask.GetProp("admissionResponse")
	admissionResponse, ok := admissionProp.(*admission.Response)
	if !ok {
		logEntry.Error("'admissionResponse' task prop is not of type *AdmissionResponse",
			slog.String(pkg.LogKeyType, fmt.Sprintf("%T", admissionProp)))
		return nil, fmt.Errorf("hook task prop error")
	}
	return admissionResponse, nil
}

// ConversionEventHandler handles conversion webhook events by finding the conversion
// path, running the appropriate hook tasks, and returning the converted objects.
type ConversionEventHandler struct {
	hookManager hook.HookManager
	taskRunner  TaskRunner
	logger      *log.Logger
}

// NewConversionEventHandler creates a new ConversionEventHandler.
func NewConversionEventHandler(hm hook.HookManager, runner TaskRunner, logger *log.Logger) *ConversionEventHandler {
	return &ConversionEventHandler{
		hookManager: hm,
		taskRunner:  runner,
		logger:      logger,
	}
}

// Handle implements the conversion event handler func signature expected by
// conversion.WebhookManager.EventHandlerFn.
func (h *ConversionEventHandler) Handle(ctx context.Context, crdName string, request *v1.ConversionRequest) (*conversion.Response, error) {
	logLabels := map[string]string{
		pkg.LogKeyEventID: uuid.Must(uuid.NewV4()).String(),
		pkg.LogKeyBinding: string(types.KubernetesConversion),
	}
	logEntry := utils.EnrichLoggerWithLabels(h.logger, logLabels)

	sourceVersions := conversion.ExtractAPIVersions(request.Objects)
	logEntry.Info("Handle kubernetesCustomResourceConversion event for crd",
		slog.String(pkg.LogKeyName, crdName),
		slog.Int(pkg.LogKeyLen, len(request.Objects)),
		slog.Any(pkg.LogKeyVersions, sourceVersions))

	done := false
	for _, srcVer := range sourceVersions {
		rule := conversion.Rule{
			FromVersion: srcVer,
			ToVersion:   request.DesiredAPIVersion,
		}
		convPath := h.hookManager.FindConversionChain(crdName, rule)
		if len(convPath) == 0 {
			continue
		}
		logEntry.Info("Find conversion path for rule",
			slog.String(pkg.LogKeyName, rule.String()),
			slog.Any(pkg.LogKeyValue, convPath))

		for _, convRule := range convPath {
			var convTask task.Task
			h.hookManager.HandleConversionEvent(ctx, crdName, request, convRule, func(hk *hook.Hook, info controller.BindingExecutionInfo) {
				convTask = globalHookTaskFactory.NewHookRunTask(hk.Name, types.KubernetesConversion, info, logLabels)
			})

			if convTask == nil {
				return nil, fmt.Errorf("no hook found for '%s' event for crd/%s", string(types.KubernetesConversion), crdName)
			}

			res := h.taskRunner(ctx, convTask)

			if res.Status == "Fail" {
				return &conversion.Response{
					FailedMessage:    fmt.Sprintf("Hook failed to convert to %s", request.DesiredAPIVersion),
					ConvertedObjects: nil,
				}, nil
			}

			prop := convTask.GetProp("conversionResponse")
			response, ok := prop.(*conversion.Response)
			if !ok {
				logEntry.Error("'conversionResponse' task prop is not of type *conversion.Response",
					slog.String(pkg.LogKeyType, fmt.Sprintf("%T", prop)))
				return nil, fmt.Errorf("hook task prop error")
			}

			// Set response objects as new objects for a next round.
			request.Objects = response.ConvertedObjects

			// Stop iterating if hook has converted all objects to a desiredAPIVersion.
			newSourceVersions := conversion.ExtractAPIVersions(request.Objects)

			if len(newSourceVersions) == 1 && newSourceVersions[0] == request.DesiredAPIVersion {
				done = true
				break
			}
		}

		if done {
			break
		}
	}

	if done {
		return &conversion.Response{
			ConvertedObjects: request.Objects,
		}, nil
	}

	return &conversion.Response{
		FailedMessage: fmt.Sprintf("Conversion %s to %s was not successful", crdName, request.DesiredAPIVersion),
	}, nil
}
