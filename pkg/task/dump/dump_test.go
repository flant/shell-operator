package dump

import (
	"context"
	"fmt"
	"sort"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

func Test_Sort_ByNamespaceAndName(t *testing.T) {
	g := NewWithT(t)

	sorted := []string{
		"main",
		"asd",
		"hook.kube",
		"sched-queue",
		"str",
	}

	g.Expect(sort.IsSorted(AsQueueNames(sorted))).To(BeTrue())

	input := []string{
		"str",
		"asd",
		"hook.kube",
		"main",
		"sched-queue",
	}

	sort.Sort(AsQueueNames(input))

	for i, s := range sorted {
		g.Expect(input[i]).Should(Equal(s), "fail on 'input' index %d", i)
	}
}

func fillQueue(q *queue.TaskQueue, n int) {
	for i := 0; i < n; i++ {
		t := &task.BaseTask{Id: fmt.Sprintf("test_task_%s_%04d", q.Name, i)}
		t.WithMetadata(task_metadata.HookMetadata{
			HookName: fmt.Sprintf("test_task_%s_%04d", q.Name, i),
		})
		q.AddFirst(t)
	}
}

func Test_Dump(t *testing.T) {
	g := NewWithT(t)

	const (
		mainTasks   = 5
		activeTasks = 4
	)
	tqs := queue.NewTaskQueueSet()
	tqs.WithMainName("main")
	tqs.WithContext(context.Background())

	// Empty set, no queues, dump should not fail.
	dump := TaskQueueSetToText(tqs)
	g.Expect(dump).To(ContainSubstring("No queues"))

	// Create and fill main queue.
	tqs.NewNamedQueue("main", nil)

	// Single empty main should be reported only as summary.
	dump = TaskQueueSetToText(tqs)
	t.Log(dump)
	g.Expect(dump).ToNot(ContainSubstring("Empty queues"))
	g.Expect(dump).To(ContainSubstring(": empty"))

	fillQueue(tqs.GetMain(), mainTasks)

	// Main queue is not counted as active or empty queue.
	dump = TaskQueueSetToText(tqs)
	g.Expect(dump).ToNot(ContainSubstring("other"))
	g.Expect(dump).To(ContainSubstring(": %d tasks", mainTasks))

	// Create and fill active queue.
	tqs.NewNamedQueue("active-queue", nil)
	fillQueue(tqs.GetByName("active-queue"), activeTasks)

	dump = TaskQueueSetToText(tqs)
	g.Expect(dump).To(ContainSubstring("1 active"))
	g.Expect(dump).To(ContainSubstring(": %d tasks", mainTasks))
	g.Expect(dump).To(ContainSubstring(": %d tasks", activeTasks))

	// Create empty queue.
	tqs.NewNamedQueue("empty", nil)

	dump = TaskQueueSetToText(tqs)
	t.Log(dump)
	g.Expect(dump).To(ContainSubstring("1 active"))
	g.Expect(dump).To(ContainSubstring("1 empty"))
	g.Expect(dump).To(ContainSubstring("total %d tasks", mainTasks+activeTasks))
}
