package shell_operator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

func makeTask(typ task.TaskType) task.Task {
	return task.NewTask(typ)
}

func TestTaskHandlerRegistry_NewTaskHandlerRegistry_empty(t *testing.T) {
	r := NewTaskHandlerRegistry()
	require.NotNil(t, r)
	_, handled := r.Handle(context.Background(), makeTask("any"))
	assert.False(t, handled, "empty registry should not handle any task")
}

func TestTaskHandlerRegistry_Register_and_Handle_found(t *testing.T) {
	r := NewTaskHandlerRegistry()
	called := false
	want := queue.TaskResult{Status: queue.Success}

	r.Register("my-task", func(_ context.Context, _ task.Task) queue.TaskResult {
		called = true
		return want
	})

	got, handled := r.Handle(context.Background(), makeTask("my-task"))
	assert.True(t, handled)
	assert.True(t, called)
	assert.Equal(t, want, got)
}

func TestTaskHandlerRegistry_Handle_notFound_returnsFail(t *testing.T) {
	r := NewTaskHandlerRegistry()
	r.Register("registered", func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Success}
	})

	got, handled := r.Handle(context.Background(), makeTask("unregistered"))
	assert.False(t, handled)
	assert.Equal(t, queue.Fail, got.Status)
}

func TestTaskHandlerRegistry_Register_overwrites(t *testing.T) {
	r := NewTaskHandlerRegistry()
	r.Register("t", func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Success}
	})
	r.Register("t", func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Fail}
	})

	got, handled := r.Handle(context.Background(), makeTask("t"))
	assert.True(t, handled)
	assert.Equal(t, queue.Fail, got.Status)
}

func TestTaskHandlerRegistry_MultipleTypes(t *testing.T) {
	r := NewTaskHandlerRegistry()
	r.Register("type-a", func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Success}
	})
	r.Register("type-b", func(_ context.Context, _ task.Task) queue.TaskResult {
		return queue.TaskResult{Status: queue.Repeat}
	})

	gotA, handledA := r.Handle(context.Background(), makeTask("type-a"))
	gotB, handledB := r.Handle(context.Background(), makeTask("type-b"))
	_, handledC := r.Handle(context.Background(), makeTask("type-c"))

	assert.True(t, handledA)
	assert.Equal(t, queue.Success, gotA.Status)
	assert.True(t, handledB)
	assert.Equal(t, queue.Repeat, gotB.Status)
	assert.False(t, handledC)
}

func TestTaskHandlerRegistry_Handle_passesTaskToHandler(t *testing.T) {
	r := NewTaskHandlerRegistry()
	var receivedTask task.Task

	r.Register("my-task", func(_ context.Context, t task.Task) queue.TaskResult {
		receivedTask = t
		return queue.TaskResult{Status: queue.Success}
	})

	input := makeTask("my-task")
	r.Handle(context.Background(), input)

	assert.Equal(t, input.GetId(), receivedTask.GetId())
}
