package shell_operator

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/hook/controller"
	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/task"
	utils "github.com/flant/shell-operator/pkg/utils/file"
)

func Test_Operator_startup_tasks(t *testing.T) {
	g := NewWithT(t)

	hooksDir, err := utils.RequireExistingDirectory("testdata/startup_tasks/hooks")
	g.Expect(err).ShouldNot(HaveOccurred())

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		assert.Equal(t, metric, metrics.TasksQueueActionDurationSeconds)
		assert.NotZero(t, value)
		assert.Equal(t, map[string]string{
			"queue_action": "AddLast",
			"queue_name":   "main",
		}, labels)
		assert.Nil(t, buckets)
	})
	metricStorage.GaugeSetMock.Optional().Set(func(_ string, _ float64, _ map[string]string) {
	})

	op := NewShellOperator(context.Background(), nil, nil, WithLogger(log.NewNop()))
	op.MetricStorage = metricStorage

	op.SetupEventManagers()
	op.setupHookManagers(app.NewConfig(), hooksDir, "")

	err = op.initHookManager()
	g.Expect(err).ShouldNot(HaveOccurred())

	op.bootstrapMainQueue(op.TaskQueues)

	expectTasks := []struct {
		taskType    task.TaskType
		bindingType htypes.BindingType
		hookPrefix  string
	}{
		// OnStartup in specified order.
		// onStartup: 1
		{HookRun, htypes.OnStartup, "hook02"},
		// onStartup: 10
		{HookRun, htypes.OnStartup, "hook03"},
		// onStartup: 20
		{HookRun, htypes.OnStartup, "hook01"},
		// EnableKubernetes and EnableSchedule in alphabet order.
		{EnableKubernetesBindings, "", "hook01"},
		{EnableScheduleBindings, "", "hook02"},
		{EnableKubernetesBindings, "", "hook03"},
		{EnableScheduleBindings, "", "hook03"},
	}

	i := 0
	op.TaskQueues.GetMain().IterateSnapshot(func(tsk task.Task) {
		// Stop checking if no expects left.
		if i >= len(expectTasks) {
			return
		}

		expect := expectTasks[i]
		hm, ok := HookMetadataAccessor(tsk)
		g.Expect(ok).To(BeTrue(), "HookMetadataAccessor should succeed for task %d", i)
		g.Expect(tsk.GetType()).To(Equal(expect.taskType), "task type should match for task %d, got %+v %+v", i, tsk, hm)
		g.Expect(hm.BindingType).To(Equal(expect.bindingType), "binding should match for task %d, got %+v %+v", i, tsk, hm)
		g.Expect(hm.HookName).To(HavePrefix(expect.hookPrefix), "hook name should match for task %d, got %+v %+v", i, tsk, hm)
		i++
	})
}

// newTestOperator builds a minimal ShellOperator from the startup_tasks testdata hooks,
// ready for Reinit testing. The main queue is bootstrapped but not started.
func newTestOperator(t *testing.T) *ShellOperator {
	t.Helper()
	hooksDir, err := utils.RequireExistingDirectory("testdata/startup_tasks/hooks")
	require.NoError(t, err)

	op := NewShellOperator(context.Background(), nil, nil, WithLogger(log.NewNop()))
	op.SetupEventManagers()
	op.setupHookManagers(app.NewConfig(), hooksDir, t.TempDir())
	require.NoError(t, op.initHookManager())
	op.bootstrapMainQueue(op.TaskQueues)
	return op
}

// collectTasks returns all tasks currently in the main queue as a slice.
func collectTasks(op *ShellOperator) []task.Task {
	var tasks []task.Task
	op.TaskQueues.GetMain().IterateSnapshot(func(tsk task.Task) {
		tasks = append(tasks, tsk)
	})
	return tasks
}

func Test_Reinit_softReinit_doesNotRequeueOnStartup(t *testing.T) {
	op := newTestOperator(t)

	initialCount := len(collectTasks(op))

	op.Reinit(context.Background(), false)

	all := collectTasks(op)
	reinitTasks := all[initialCount:]

	for _, tsk := range reinitTasks {
		hm, ok := HookMetadataAccessor(tsk)
		require.True(t, ok)
		assert.NotEqual(t, htypes.OnStartup, hm.BindingType, "soft reinit must not enqueue onStartup tasks")
	}

	// Binding-enable tasks must be present for hooks that have those bindings.
	types := make(map[task.TaskType]bool)
	for _, tsk := range reinitTasks {
		types[tsk.GetType()] = true
	}
	assert.True(t, types[EnableKubernetesBindings], "EnableKubernetesBindings tasks must be enqueued")
	assert.True(t, types[EnableScheduleBindings], "EnableScheduleBindings tasks must be enqueued")
}

func Test_Reinit_fullReset_requeuesOnStartup(t *testing.T) {
	op := newTestOperator(t)

	// Capture old controller pointers before reinit to verify they are replaced.
	oldControllers := map[string]*controller.HookController{}
	for _, name := range op.HookManager.GetHookNames() {
		oldControllers[name] = op.HookManager.GetHook(name).HookController
	}

	initialCount := len(collectTasks(op))

	op.Reinit(context.Background(), true)

	all := collectTasks(op)
	reinitTasks := all[initialCount:]

	taskTypes := make(map[task.TaskType]bool)
	hasOnStartup := false
	for _, tsk := range reinitTasks {
		taskTypes[tsk.GetType()] = true
		hm, ok := HookMetadataAccessor(tsk)
		require.True(t, ok)
		if hm.BindingType == htypes.OnStartup {
			hasOnStartup = true
		}
	}
	assert.True(t, hasOnStartup, "full reset must re-enqueue onStartup tasks")
	assert.True(t, taskTypes[EnableKubernetesBindings], "EnableKubernetesBindings tasks must be enqueued")
	assert.True(t, taskTypes[EnableScheduleBindings], "EnableScheduleBindings tasks must be enqueued")

	// Verify that old hook controllers were replaced, proving the disable+rebuild cycle ran.
	for _, name := range op.HookManager.GetHookNames() {
		newCtrl := op.HookManager.GetHook(name).HookController
		assert.NotSame(t, oldControllers[name], newCtrl, "hook %s: controller must be a new instance after full reset", name)
	}
}
