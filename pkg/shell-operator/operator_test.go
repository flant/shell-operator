package shell_operator

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/task"
	utils "github.com/flant/shell-operator/pkg/utils/file"
)

func Test_Operator_startup_tasks(t *testing.T) {
	g := NewWithT(t)

	hooksDir, err := utils.RequireExistingDirectory("testdata/startup_tasks/hooks")
	g.Expect(err).ShouldNot(HaveOccurred())

	op := NewShellOperator(context.Background(), WithLogger(log.NewNop()))
	op.SetupEventManagers()
	op.setupHookManagers(hooksDir, "")

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
	op.TaskQueues.GetMain().Iterate(func(tsk task.Task) {
		// Stop checking if no expects left.
		if i >= len(expectTasks) {
			return
		}

		expect := expectTasks[i]
		hm := HookMetadataAccessor(tsk)
		g.Expect(tsk.GetType()).To(Equal(expect.taskType), "task type should match for task %d, got %+v %+v", i, tsk, hm)
		g.Expect(hm.BindingType).To(Equal(expect.bindingType), "binding should match for task %d, got %+v %+v", i, tsk, hm)
		g.Expect(hm.HookName).To(HavePrefix(expect.hookPrefix), "hook name should match for task %d, got %+v %+v", i, tsk, hm)
		i++
	})
}
