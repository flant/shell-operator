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

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

// TaskHandlerFunc is the function type for handling a single task.
type TaskHandlerFunc func(ctx context.Context, t task.Task) queue.TaskResult

// TaskHandlerRegistry maps task types to their handlers.
// New task types can be registered without modifying the dispatcher.
type TaskHandlerRegistry struct {
	handlers map[task.TaskType]TaskHandlerFunc
}

// NewTaskHandlerRegistry creates an empty registry.
func NewTaskHandlerRegistry() *TaskHandlerRegistry {
	return &TaskHandlerRegistry{
		handlers: make(map[task.TaskType]TaskHandlerFunc),
	}
}

// Register associates a handler with a task type.
// Registering the same type twice overwrites the previous handler.
func (r *TaskHandlerRegistry) Register(taskType task.TaskType, handler TaskHandlerFunc) {
	r.handlers[taskType] = handler
}

// Handle dispatches the task to the registered handler.
// Returns a Fail result when no handler is found.
func (r *TaskHandlerRegistry) Handle(ctx context.Context, t task.Task) (queue.TaskResult, bool) {
	h, ok := r.handlers[t.GetType()]
	if !ok {
		return queue.TaskResult{Status: "Fail"}, false
	}
	return h(ctx, t), true
}
