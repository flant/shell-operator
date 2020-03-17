package dump

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

// ByNamespaceAndName implements sort.Interface for []ObjectAndFilterResult
// based on Namespace and Name of Object field.
type AsQueueNames []string

func (a AsQueueNames) Len() int      { return len(a) }
func (a AsQueueNames) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a AsQueueNames) Less(i, j int) bool {
	p, q := a[i], a[j]

	switch {
	case q == "main":
		return false
	case p == "main":
		return true
	}
	// both names are main or both are not main, so compare as usual.
	return p < q
}

func TaskQueueSetToText(tqs *queue.TaskQueueSet) string {
	var buf strings.Builder

	var names []string
	tqs.Iterate(func(queue *queue.TaskQueue) {
		names = append(names, queue.Name)
	})

	sort.Sort(AsQueueNames(names))

	for i, name := range names {
		q := tqs.GetByName(name)
		if i > 0 {
			buf.WriteString("==========\n\n")
		}
		if q == nil {
			buf.WriteString(fmt.Sprintf("Queue '%s' is not created\n", name))
		} else {
			buf.WriteString(TaskQueueToText(q))
		}
	}

	return buf.String()
}

// Dump tasks in queue
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
