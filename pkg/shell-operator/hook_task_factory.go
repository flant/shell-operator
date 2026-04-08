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
	"time"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/task"
)

// HookTaskFactory builds HookRun tasks, deduplicating the repeated boilerplate
// across kube event, schedule, admission, and conversion event handlers.
//
// The factory does not set WithQueuedAt; callers should stamp the task with
// task.WithQueuedAt(time.Now()) when the task is ready to be enqueued.
type HookTaskFactory struct{}

// NewHookRunTask creates a HookRun task populated from a hook and a BindingExecutionInfo.
// It sets the CompactionID to hook.Name and copies QueueName from info.QueueName.
func (HookTaskFactory) NewHookRunTask(hookName string, bindingType types.BindingType, info controller.BindingExecutionInfo, logLabels map[string]string) task.Task {
	return task.NewTask(task_metadata.HookRun).
		WithMetadata(task_metadata.HookMetadata{
			HookName:       hookName,
			BindingType:    bindingType,
			BindingContext: info.BindingContext,
			AllowFailure:   info.AllowFailure,
			Binding:        info.Binding,
			Group:          info.Group,
		}).
		WithLogLabels(logLabels).
		WithQueueName(info.QueueName).
		WithCompactionID(hookName)
}

// NewSyncHookRunTask creates a HookRun task for the Kubernetes synchronization flow.
// It sets extra MonitorIDs and ExecuteOnSynchronization fields, and always uses
// the "main" queue regardless of info.QueueName.
func (HookTaskFactory) NewSyncHookRunTask(h *hook.Hook, info controller.BindingExecutionInfo, logLabels map[string]string) task.Task {
	return task.NewTask(task_metadata.HookRun).
		WithMetadata(task_metadata.HookMetadata{
			HookName:                 h.Name,
			BindingType:              types.OnKubernetesEvent,
			BindingContext:           info.BindingContext,
			AllowFailure:             info.AllowFailure,
			Binding:                  info.Binding,
			Group:                    info.Group,
			MonitorIDs:               []string{info.KubernetesBinding.Monitor.Metadata.MonitorId},
			ExecuteOnSynchronization: info.KubernetesBinding.ExecuteHookOnSynchronization,
		}).
		WithLogLabels(logLabels).
		WithQueueName("main").
		WithCompactionID(h.Name)
}

// globalHookTaskFactory is the package-level factory used by operator event handlers.
var globalHookTaskFactory HookTaskFactory

// newHookRunTaskNow is a convenience wrapper that also stamps WithQueuedAt(time.Now()).
func newHookRunTaskNow(hookName string, bindingType types.BindingType, info controller.BindingExecutionInfo, logLabels map[string]string) task.Task {
	return globalHookTaskFactory.NewHookRunTask(hookName, bindingType, info, logLabels).
		WithQueuedAt(time.Now())
}
