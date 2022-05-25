package dump

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

// AsQueueNames implements sort.Interface for array of queue names.
// 'main' queue has the top priority. Other names are sorted as usual.
type AsQueueNames []string

func (a AsQueueNames) Len() int      { return len(a) }
func (a AsQueueNames) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a AsQueueNames) Less(i, j int) bool {
	p, q := a[i], a[j]

	switch {
	case q == queue.MainQueueName:
		return false
	case p == queue.MainQueueName:
		return true
	}
	// both names are main or both are not main, so compare as usual.
	return p < q
}

// TaskQueueMainToText dumps only the 'main' queue.
func TaskQueueMainToText(tqs *queue.TaskQueueSet) string {
	var buf strings.Builder

	q := tqs.GetMain()
	if q == nil {
		buf.WriteString(fmt.Sprintf("Queue '%s' is not created\n", queue.MainQueueName))
	} else {
		buf.WriteString(TaskQueueToText(q))
	}

	return buf.String()
}

// TaskQueueSetToText dumps all queues.
func TaskQueueSetToText(tqs *queue.TaskQueueSet) string {
	var buf strings.Builder

	names := map[string][]string{
		"active": make([]string, 0),
		"empty":  make([]string, 0),
	}
	otherQueuesCount := 0
	activeQueues := 0
	emptyQueues := 0
	tasksCount := 0
	mainTasksCount := 0
	hasMain := false
	hasOthers := false
	tqs.Iterate(func(queue *queue.TaskQueue) {
		if queue.Name == tqs.MainName {
			mainTasksCount = queue.Length()
			hasMain = true
			if queue.IsEmpty() {
				names["empty"] = append(names["empty"], queue.Name)
			} else {
				names["active"] = append(names["active"], queue.Name)
			}
			return
		}

		otherQueuesCount++
		hasOthers = true
		if queue.IsEmpty() {
			emptyQueues++
			names["empty"] = append(names["empty"], queue.Name)
		} else {
			activeQueues++
			tasksCount += queue.Length()
			names["active"] = append(names["active"], queue.Name)
		}
	})

	sort.Sort(AsQueueNames(names["active"]))
	sort.Sort(AsQueueNames(names["empty"]))

	for i, name := range names["active"] {
		q := tqs.GetByName(name)
		if i > 0 {
			buf.WriteString("\n")
		}
		if q == nil {
			buf.WriteString(fmt.Sprintf("Queue '%s' is not created\n", name))
		} else {
			buf.WriteString(TaskQueueToText(q))
		}
	}

	// Empty queues. Do not report single empty main queue.
	if emptyQueues > 0 && hasOthers {
		buf.WriteString(fmt.Sprintf("\nEmpty queues (%d):\n", len(names["empty"])))
		for _, name := range names["empty"] {
			buf.WriteString(fmt.Sprintf("- %s\n", name))
		}
	}

	// Summary:
	//   No queues.
	// or
	// Summary:
	// - 'main' queue: [empty | 1 task | 5 tasks].
	// - 24 other queues (3 active, 21 empty): 56 tasks.
	// - total 68 tasks to handle.
	if !hasMain && !hasOthers {
		buf.WriteString("Summary:\n  No queues.\n")
	} else {
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("Summary:\n")
		if hasMain {
			buf.WriteString(fmt.Sprintf("- '%s' queue: %s.\n",
				tqs.MainName,
				pluralize(mainTasksCount, "empty", "task", "tasks")))
		}
		if hasOthers {
			buf.WriteString(fmt.Sprintf("- %s (%d active, %d empty): %s.\n",
				pluralize(otherQueuesCount, "", "other queue", "other queues"),
				activeQueues, emptyQueues,
				pluralize(tasksCount, "", "task", "tasks")))
		}
		totalTasks := mainTasksCount + tasksCount
		if totalTasks == 0 {
			buf.WriteString("- no tasks to handle.\n")
		} else {
			buf.WriteString(fmt.Sprintf("- total %s to handle.\n",
				pluralize(totalTasks, "", "task", "tasks")))
		}
	}

	return buf.String()
}

func pluralize(n int, zero, one, many string) string {
	if n == 0 && zero != "" {
		return zero
	}
	description := many
	if n == 1 {
		description = one
	}
	return fmt.Sprintf("%d %s", n, description)
}

// TaskQueueToText dumps all tasks in queue.
func TaskQueueToText(q *queue.TaskQueue) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Queue '%s': length %d, status: '%s'\n", q.Name, q.Length(), q.Status))
	buf.WriteString("\n")

	var index = 1
	q.Iterate(func(task task.Task) {
		buf.WriteString(fmt.Sprintf("%2d. ", index))
		buf.WriteString(task.GetDescription())
		buf.WriteString("\n")
		index++
	})

	return buf.String()
}
