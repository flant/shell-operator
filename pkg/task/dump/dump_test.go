package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

func Test_Sort_ByNamespaceAndName(t *testing.T) {
	g := NewWithT(t)

	sortedStr := []string{
		"main",
		"asd",
		"hook.kube",
		"sched-queue",
		"str",
	}
	sorted := make([]dumpQueue, 0, len(sortedStr))
	for _, ss := range sortedStr {
		sorted = append(sorted, dumpQueue{Name: ss})
	}

	g.Expect(sort.IsSorted(asQueueNames(sorted))).To(BeTrue())

	inputStr := []string{
		"str",
		"asd",
		"hook.kube",
		"main",
		"sched-queue",
	}

	input := make([]dumpQueue, 0, len(inputStr))
	for _, ss := range inputStr {
		input = append(input, dumpQueue{Name: ss})
	}

	sort.Sort(asQueueNames(input))

	for i, s := range sorted {
		g.Expect(input[i]).Should(Equal(s), "fail on 'input' index %d", i)
	}
}

func fillQueue(q task.TaskQueue, n int) {
	for i := 0; i < n; i++ {
		t := &task.BaseTask{Id: fmt.Sprintf("test_task_%s_%04d", q.GetName(), i)}
		t.WithMetadata(task_metadata.HookMetadata{
			HookName: fmt.Sprintf("test_task_%s_%04d", q.GetName(), i),
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

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		assert.Equal(t, metric, metrics.TasksQueueActionDurationSeconds)
		assert.NotZero(t, value)

		mapSlice := []map[string]string{
			{
				"queue_action": "Length",
				"queue_name":   "main",
			},
			{
				"queue_action": "AddFirst",
				"queue_name":   "active-queue",
			},
			{
				"queue_action": "IsEmpty",
				"queue_name":   "empty",
			},
		}
		assert.Contains(t, mapSlice, labels)
		assert.Nil(t, buckets)
	})
	metricStorage.GaugeSetMock.Set(func(_ string, _ float64, _ map[string]string) {
	})

	tqs := queue.NewTaskQueueSet().WithMetricStorage(metricStorage)
	tqs.WithMainName("main")
	tqs.WithContext(context.Background())

	// Empty set, no queues, dump should not fail.
	t.Run("empty set", func(_ *testing.T) {
		dump := testDumpQueuesWrapper(tqs, "text", true)
		g.Expect(dump).To(ContainSubstring("No queues"))
	})

	// Create and fill main queue.
	t.Run("single main queue", func(t *testing.T) {
		tqs.NewNamedQueue("main", nil, queue.WithCompactableTypes(task_metadata.HookRun))

		// Single empty main should be reported only as summary.
		dump := testDumpQueuesWrapper(tqs, "text", true)
		t.Log(dump)
		g.Expect(dump).ToNot(ContainSubstring("Empty queues"))
		g.Expect(dump).To(ContainSubstring(": empty"))

		dump = testDumpQueuesWrapper(tqs, "json", true)
		t.Log(dump)
		g.Expect(dump).To(MatchJSON(`{"active":[],"summary":{"mainQueueTasks":0,"otherQueues":{"active":0,"empty":0,"tasks":0},"totalTasks":0}}`))
	})

	t.Run("main queue as active", func(_ *testing.T) {
		fillQueue(tqs.GetMain(), mainTasks)

		// Main queue is not counted as active or empty queue.
		dump := testDumpQueuesWrapper(tqs, "text", true)
		g.Expect(dump).ToNot(ContainSubstring("other"))
		g.Expect(dump).To(ContainSubstring(": %d tasks", mainTasks))
	})

	// Create and fill active queue.
	t.Run("fill active queue", func(_ *testing.T) {
		tqs.NewNamedQueue("active-queue", nil, queue.WithCompactableTypes(task_metadata.HookRun))

		fillQueue(tqs.GetByName("active-queue"), activeTasks)

		dump := testDumpQueuesWrapper(tqs, "text", true)
		g.Expect(dump).To(ContainSubstring("1 active"))
		g.Expect(dump).To(ContainSubstring(": %d tasks", mainTasks))
		g.Expect(dump).To(ContainSubstring(": %d tasks", activeTasks))
	})

	// Create empty queue.
	t.Run("create empty queue", func(t *testing.T) {
		tqs.NewNamedQueue("empty", nil, queue.WithCompactableTypes(task_metadata.HookRun))

		dump := testDumpQueuesWrapper(tqs, "text", true)
		t.Log(dump)
		g.Expect(dump).To(ContainSubstring("1 active"))
		g.Expect(dump).To(ContainSubstring("1 empty"))
		g.Expect(dump).To(ContainSubstring("total %d tasks", mainTasks+activeTasks))

		dump = testDumpQueuesWrapper(tqs, "json", true)
		g.Expect(dump).To(MatchJSON(`{"active":[{"name":"main","tasksCount":5,"tasks":[{"index":1,"description":":::test_task_main_0004"},{"index":2,"description":":::test_task_main_0003"},{"index":3,"description":":::test_task_main_0002"},{"index":4,"description":":::test_task_main_0001"},{"index":5,"description":":::test_task_main_0000"}]},{"name":"active-queue","tasksCount":4,"tasks":[{"index":1,"description":":::test_task_active-queue_0003"},{"index":2,"description":":::test_task_active-queue_0002"},{"index":3,"description":":::test_task_active-queue_0001"},{"index":4,"description":":::test_task_active-queue_0000"}]}],"empty":[{"name":"empty","tasksCount":0}],"summary":{"mainQueueTasks":5,"otherQueues":{"active":1,"empty":1,"tasks":4},"totalTasks":9}}`))
	})

	t.Run("omit empty queue", func(_ *testing.T) {
		dump := testDumpQueuesWrapper(tqs, "text", false)
		g.Expect(dump).To(ContainSubstring("1 active"))
		g.Expect(dump).ToNot(ContainSubstring("Empty queues (1):"))
		g.Expect(dump).To(ContainSubstring("total %d tasks", mainTasks+activeTasks))

		dump = testDumpQueuesWrapper(tqs, "json", false)
		g.Expect(dump).To(MatchJSON(`{"active":[{"name":"main","tasksCount":5,"tasks":[{"index":1,"description":":::test_task_main_0004"},{"index":2,"description":":::test_task_main_0003"},{"index":3,"description":":::test_task_main_0002"},{"index":4,"description":":::test_task_main_0001"},{"index":5,"description":":::test_task_main_0000"}]},{"name":"active-queue","tasksCount":4,"tasks":[{"index":1,"description":":::test_task_active-queue_0003"},{"index":2,"description":":::test_task_active-queue_0002"},{"index":3,"description":":::test_task_active-queue_0001"},{"index":4,"description":":::test_task_active-queue_0000"}]}],"summary":{"mainQueueTasks":5,"otherQueues":{"active":1,"empty":1,"tasks":4},"totalTasks":9}}`))
	})
}

func testDumpQueuesWrapper(tqs *queue.TaskQueueSet, format string, showEmpty bool) interface{} {
	dump := TaskQueues(tqs, format, showEmpty)

	switch format {
	case "text":
		return dump.(string)

	case "json":
		data, _ := json.Marshal(dump)
		return string(data)

	case "yaml":
		data, _ := yaml.Marshal(dump)
		return string(data)
	}

	return nil
}
