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
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

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
	metricStorage.GaugeSetMock.Set(func(_ string, _ float64, _ map[string]string) {
	})

	op := NewShellOperator(context.Background())
	op.MetricStorage = metricStorage
	op.logger = log.NewNop()

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
