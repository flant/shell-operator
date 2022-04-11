package shell_operator

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/task"
)

func Test_Operator_startup_tasks(t *testing.T) {
	g := NewWithT(t)

	hooksDir, err := RequireExistingDirectory("testdata/startup_tasks/hooks")
	g.Expect(err).ShouldNot(HaveOccurred())

	op := NewShellOperator()
	op.WithContext(context.Background())
	SetupEventManagers(op)
	SetupHookManagers(op, hooksDir, "")

	err = op.InitHookManager()
	g.Expect(err).ShouldNot(HaveOccurred())

	op.BootstrapMainQueue(op.TaskQueues)

	expectTasks := []struct {
		taskType    task.TaskType
		bindingType BindingType
		hookPrefix  string
	}{
		// OnStartup in specified order.
		// onStartup: 1
		{HookRun, OnStartup, "hook02"},
		// onStartup: 10
		{HookRun, OnStartup, "hook03"},
		// onStartup: 20
		{HookRun, OnStartup, "hook01"},
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
