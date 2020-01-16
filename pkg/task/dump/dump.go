package dump

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flant/shell-operator/pkg/hook/task_metadata"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

func TaskToString(task task.Task) string {
	// Implement TextDumper in Metadata objects!
	var buf bytes.Buffer
	//buf.WriteString(fmt.Sprintf("%s '%s'", t.Type, t.Name))
	hm := task_metadata.HookMetadataAccessor(task)
	buf.WriteString(fmt.Sprintf("%s %s %s", task.GetType(), hm.HookName, hm.BindingType))

	if task.GetFailureCount() > 0 {
		buf.WriteString(fmt.Sprintf(" failed %d times. ", task.GetFailureCount()))
	}
	return buf.String()
}

// Dump tasks in queue
func TaskQueueToReader(q *queue.TaskQueue) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Queue '%s': length %d\n", q.Name, q.Length()))
	buf.WriteString("\n")

	var index int
	q.Iterate(func(task task.Task) {
		buf.WriteString(TaskToString(task))
		buf.WriteString("\n")
		index++
	})

	return buf.String()
}
