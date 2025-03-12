package task_metadata

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/mock"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

func Test_HookMetadata_Access(t *testing.T) {
	g := NewWithT(t)

	Task := task.NewTask(HookRun).
		WithMetadata(HookMetadata{
			HookName:    "test-hook",
			BindingType: htypes.Schedule,
			BindingContext: []bctx.BindingContext{
				{Binding: "each_1_min"},
				{Binding: "each_5_min"},
			},
			AllowFailure: true,
		})

	hm := HookMetadataAccessor(Task)

	g.Expect(hm.HookName).Should(Equal("test-hook"))
	g.Expect(hm.BindingType).Should(Equal(htypes.Schedule))
	g.Expect(hm.AllowFailure).Should(BeTrue())
	g.Expect(hm.BindingContext).Should(HaveLen(2))
	g.Expect(hm.BindingContext[0].Binding).Should(Equal("each_1_min"))
	g.Expect(hm.BindingContext[1].Binding).Should(Equal("each_5_min"))
}

func Test_HookMetadata_QueueDump_Task_Description(t *testing.T) {
	g := NewWithT(t)

	logLabels := map[string]string{
		"hook": "hook1.sh",
	}

	metricStorage := mock.NewMetricStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		assert.Equal(t, metric, "{PREFIX}tasks_queue_action_duration_seconds")
		assert.NotZero(t, value)
		assert.Equal(t, map[string]string{
			"queue_action": "AddLast",
			"queue_name":   "",
		}, labels)
		assert.Nil(t, buckets)
	})

	q := queue.NewTasksQueue().WithMetricStorage(metricStorage)

	q.AddLast(task.NewTask(EnableKubernetesBindings).
		WithMetadata(HookMetadata{
			HookName: "hook1.sh",
			Binding:  string(EnableKubernetesBindings),
		}))

	q.AddLast(task.NewTask(HookRun).
		WithMetadata(HookMetadata{
			HookName:    "hook1.sh",
			BindingType: htypes.OnKubernetesEvent,
			Binding:     "monitor_pods",
		}).
		WithLogLabels(logLabels).
		WithQueueName("main"))

	q.AddLast(task.NewTask(HookRun).
		WithMetadata(HookMetadata{
			HookName:     "hook1.sh",
			BindingType:  htypes.Schedule,
			AllowFailure: true,
			Binding:      "every 1 sec",
			Group:        "monitor_pods",
		}).
		WithLogLabels(logLabels).
		WithQueueName("main"))

	queueDump := taskQueueToText(q)

	g.Expect(queueDump).Should(ContainSubstring("hook1.sh"), "Queue dump should reveal a hook name.")
	g.Expect(queueDump).Should(ContainSubstring("EnableKubernetesBindings"), "Queue dump should reveal EnableKubernetesBindings.")
	g.Expect(queueDump).Should(ContainSubstring(":kubernetes:"), "Queue dump should show kubernetes binding.")
	g.Expect(queueDump).Should(ContainSubstring(":schedule:"), "Queue dump should show schedule binding.")
	g.Expect(queueDump).Should(ContainSubstring("group=monitor_pods"), "Queue dump should show group name.")
}

func taskQueueToText(q *queue.TaskQueue) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Queue '%s': length %d, status: '%s'\n", q.Name, q.Length(), q.Status))
	buf.WriteString("\n")

	index := 1
	q.Iterate(func(task task.Task) {
		buf.WriteString(fmt.Sprintf("%2d. ", index))
		buf.WriteString(task.GetDescription())
		buf.WriteString("\n")
		index++
	})

	return buf.String()
}
