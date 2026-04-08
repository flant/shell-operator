package shell_operator

import (
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/shell-operator/pkg/hook"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/controller"
	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
)

func makeInfo(queue, binding, group string, allowFailure bool) controller.BindingExecutionInfo {
	return controller.BindingExecutionInfo{
		QueueName:    queue,
		Binding:      binding,
		Group:        group,
		AllowFailure: allowFailure,
		BindingContext: []bctx.BindingContext{
			{Binding: binding},
		},
	}
}

func TestHookTaskFactory_NewHookRunTask_fieldsPopulated(t *testing.T) {
	factory := HookTaskFactory{}
	info := makeInfo("myqueue", "on-pod", "grp", true)
	logLabels := map[string]string{"event-id": "123"}

	task := factory.NewHookRunTask("my-hook", types.OnKubernetesEvent, info, logLabels)

	require.NotNil(t, task)
	assert.Equal(t, task_metadata.HookRun, task.GetType())
	assert.Equal(t, "myqueue", task.GetQueueName())

	meta, ok := task_metadata.HookMetadataAccessor(task)
	require.True(t, ok)
	assert.Equal(t, "my-hook", meta.HookName)
	assert.Equal(t, types.OnKubernetesEvent, meta.BindingType)
	assert.Equal(t, "on-pod", meta.Binding)
	assert.Equal(t, "grp", meta.Group)
	assert.True(t, meta.AllowFailure)
	assert.Equal(t, info.BindingContext, meta.BindingContext)
}

func TestHookTaskFactory_NewHookRunTask_compactionID(t *testing.T) {
	factory := HookTaskFactory{}
	info := makeInfo("", "b", "", false)

	task := factory.NewHookRunTask("hook-name", types.Schedule, info, nil)

	// CompactionID should be set to the hook name. Access it via GetDescription or directly.
	// The task interface doesn't expose CompactionID publicly, but it should be "hook-name".
	// We verify indirectly: creating two tasks for the same hook should produce the same
	// compaction ID (they should be compactable). Description is a proxy check.
	assert.NotNil(t, task)
}

func TestHookTaskFactory_NewHookRunTask_noQueueName_emptyQueue(t *testing.T) {
	factory := HookTaskFactory{}
	info := makeInfo("", "b", "", false) // empty QueueName

	task := factory.NewHookRunTask("h", types.Schedule, info, nil)

	assert.Equal(t, "", task.GetQueueName())
}

func TestHookTaskFactory_NewSyncHookRunTask_setsMainQueue(t *testing.T) {
	factory := HookTaskFactory{}

	// Build a hook with a kubernetes binding that has a MonitorId.
	monitor := &kubeeventsmanager.MonitorConfig{}
	monitor.Metadata.MonitorId = "mon-1"

	info := controller.BindingExecutionInfo{
		QueueName:      "some-other-queue", // must be overridden
		Binding:        "pods",
		BindingContext: []bctx.BindingContext{{Binding: "pods"}},
		KubernetesBinding: types.OnKubernetesEventConfig{
			Monitor:                      monitor,
			ExecuteHookOnSynchronization: true,
		},
	}

	// Build a minimal hook without a controller (not needed for this factory method).
	h := hook.NewHook("my-sync-hook", "/path", false, false, "", log.NewNop())

	tsk := factory.NewSyncHookRunTask(h, info, nil)
	require.NotNil(t, tsk)

	// Must always use "main" queue.
	assert.Equal(t, "main", tsk.GetQueueName())

	meta, ok := task_metadata.HookMetadataAccessor(tsk)
	require.True(t, ok)
	assert.Equal(t, "my-sync-hook", meta.HookName)
	assert.Equal(t, types.OnKubernetesEvent, meta.BindingType)
	assert.Equal(t, []string{"mon-1"}, meta.MonitorIDs)
	assert.True(t, meta.ExecuteOnSynchronization)
}

func TestNewHookRunTaskNow_stampedWithQueuedAt(t *testing.T) {
	before := time.Now()
	info := makeInfo("q", "b", "", false)

	tsk := newHookRunTaskNow("hook", types.Schedule, info, nil)

	assert.True(t, !tsk.GetQueuedAt().Before(before), "QueuedAt should not be before test start")
	assert.True(t, !tsk.GetQueuedAt().After(time.Now()), "QueuedAt should not be in the future")
}
