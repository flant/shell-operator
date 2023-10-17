package dump

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

// asQueueNames implements sort.Interface for array of queue names.
// 'main' queue has the top priority. Other names are sorted as usual.
type asQueueNames []dumpQueue

func (a asQueueNames) Len() int      { return len(a) }
func (a asQueueNames) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a asQueueNames) Less(i, j int) bool {
	p, q := a[i], a[j]

	switch {
	case q.Name == queue.MainQueueName:
		return false
	case p.Name == queue.MainQueueName:
		return true
	}
	// both names are main or both are not main, so compare as usual.
	return p.Name < q.Name
}

// TaskMainQueue dumps 'main' queue with the given
func TaskMainQueue(tqs *queue.TaskQueueSet, format string) interface{} {
	var dq dumpQueue

	q := tqs.GetMain()
	if q == nil {
		dq.Name = queue.MainQueueName
		dq.Status = "Queue is not created"
	} else {
		tasks := getTasksForQueue(q)
		dq = dumpQueue{
			Name:       q.Name,
			TasksCount: q.Length(),
			Status:     q.Status,
			Tasks:      tasks,
		}
	}

	if format == "text" {
		var buf strings.Builder
		buf.WriteString(fmt.Sprintf("Queue '%s': length %d, status: '%s'\n", dq.Name, dq.TasksCount, dq.Status))
		buf.WriteString("\n")

		for _, ts := range dq.Tasks {
			buf.WriteString(fmt.Sprintf("%2d. ", ts.Index))
			buf.WriteString(ts.Description)
			buf.WriteString("\n")
		}

		return buf.String()
	}

	return dq
}

// TaskQueues dumps all queues.
// returns empty interface because we have a formatter on the router level, which will marshal data or return it as a string
func TaskQueues(tqs *queue.TaskQueueSet, format string, showEmpty bool) interface{} {
	result := dumpTaskQueues{
		Empty:  make([]dumpQueue, 0),
		Active: make([]dumpQueue, 0),
	}

	otherQueuesCount := 0
	activeQueues := 0
	emptyQueues := 0
	tasksCount := 0
	mainTasksCount := 0

	tqs.Iterate(func(queue *queue.TaskQueue) {
		if queue == nil {
			return
		}

		if queue.Name == tqs.MainName {
			mainTasksCount = queue.Length()
			if queue.IsEmpty() {
				mainQueue := dumpQueue{
					Name:       queue.Name,
					TasksCount: queue.Length(),
				}
				result.Empty = append(result.Empty, mainQueue)
				result.MainQueue = &mainQueue
			} else {
				tasks := getTasksForQueue(queue)
				mainQueue := dumpQueue{
					Name:       queue.Name,
					TasksCount: queue.Length(),
					Status:     queue.Status,
					Tasks:      tasks,
				}
				result.Active = append(result.Active, mainQueue)
				result.MainQueue = &mainQueue
			}
			return
		}

		otherQueuesCount++
		if queue.IsEmpty() {
			emptyQueues++
			result.Empty = append(result.Empty, dumpQueue{
				Name: queue.Name,
			})
		} else {
			activeQueues++
			tasksCount += queue.Length()
			tasks := getTasksForQueue(queue)
			result.Active = append(result.Active, dumpQueue{
				Name:       queue.Name,
				TasksCount: queue.Length(),
				Status:     queue.Status,
				Tasks:      tasks,
			})
		}
	})

	result.SortByName()

	result.Summary = dumpSummary{
		mainQueueTasksCount:    mainTasksCount,
		otherQueuesActiveCount: activeQueues,
		otherQueuesEmptyCount:  emptyQueues,
		otherQueuesTasksCount:  tasksCount,
		totalTasksCount:        mainTasksCount + tasksCount,
	}

	if showEmpty {
		// Empty queues. Do not report single empty main queue.
		if len(result.Empty) > 0 && otherQueuesCount == 0 {
			showEmpty = false
		}
	}

	return result.format(format, showEmpty)
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

func getTasksForQueue(q *queue.TaskQueue) []dumpTask {
	tasks := make([]dumpTask, 0, q.Length())

	index := 1
	q.Iterate(func(task task.Task) {
		tasks = append(tasks, dumpTask{
			Index:       index,
			Description: task.GetDescription(),
		})
		index++
	})

	return tasks
}

type dumpTaskQueues struct {
	MainQueue *dumpQueue  `json:"-"`
	Active    []dumpQueue `json:"active" yaml:"active"`
	Empty     []dumpQueue `json:"empty,omitempty" yaml:"empty,omitempty"`

	Summary dumpSummary `json:"summary" yaml:"summary"`
}

type dumpQueue struct {
	Name       string     `json:"name" yaml:"name"`
	TasksCount int        `json:"tasksCount" yaml:"tasksCount"`
	Status     string     `json:"status,omitempty" yaml:"status,omitempty"`
	Tasks      []dumpTask `json:"tasks,omitempty" yaml:"tasks,omitempty"`
}

type dumpTask struct {
	Index       int    `json:"index" yaml:"index"`
	Description string `json:"description" yaml:"description"`
}

type dumpSummary struct {
	// - 'main' queue: [empty | 1 task | 5 tasks].
	// - 24 other queues (3 active, 21 empty): 56 tasks.
	// - total 68 tasks to handle.

	mainQueueTasksCount    int
	otherQueuesActiveCount int
	otherQueuesEmptyCount  int
	otherQueuesTasksCount  int
	totalTasksCount        int
}

func (ds dumpSummary) MarshalJSON() ([]byte, error) {
	s := struct {
		Main   int `json:"mainQueueTasks"`
		Others struct {
			Active int `json:"active"`
			Empty  int `json:"empty"`
			Tasks  int `json:"tasks"`
		} `json:"otherQueues"`
		TotalTasks int `json:"totalTasks"`
	}{
		Main:       ds.mainQueueTasksCount,
		TotalTasks: ds.totalTasksCount,
	}

	s.Others.Tasks = ds.otherQueuesTasksCount
	s.Others.Active = ds.otherQueuesActiveCount
	s.Others.Empty = ds.otherQueuesEmptyCount

	return json.Marshal(s)
}

func (dtq dumpTaskQueues) SortByName() {
	sort.Sort(asQueueNames(dtq.Active))

	sort.Sort(asQueueNames(dtq.Empty))
}

func (dtq dumpTaskQueues) format(format string, showEmpty bool) interface{} {
	switch format {
	case "text":
		return dtq.asText(showEmpty)

	case "json", "yaml":
		if !showEmpty {
			dtq.Empty = make([]dumpQueue, 0)
		}
		return dtq
	}

	return ""
}

func (dtq dumpTaskQueues) asText(showEmpty bool) string {
	var buf strings.Builder

	for i, taskQueue := range dtq.Active {
		if i > 0 {
			buf.WriteString("\n")
		}

		buf.WriteString(fmt.Sprintf("Queue '%s': length %d, status: '%s'\n", taskQueue.Name, taskQueue.TasksCount, taskQueue.Status))
		buf.WriteString("\n")

		for _, tk := range taskQueue.Tasks {
			buf.WriteString(fmt.Sprintf("%2d. ", tk.Index))
			buf.WriteString(tk.Description)
			buf.WriteString("\n")
		}
	}

	// Empty queues. Do not report single empty main queue.
	if showEmpty {
		buf.WriteString(fmt.Sprintf("\nEmpty queues (%d):\n", len(dtq.Empty)))
		for _, taskQueue := range dtq.Empty {
			buf.WriteString(fmt.Sprintf("- %s\n", taskQueue.Name))
		}
	}

	// Summary:
	//   No queues.
	// or
	// Summary:
	// - 'main' queue: [empty | 1 task | 5 tasks].
	// - 24 other queues (3 active, 21 empty): 56 tasks.
	// - total 68 tasks to handle.
	if len(dtq.Active) == 0 && len(dtq.Empty) == 0 {
		buf.WriteString("Summary:\n  No queues.\n")
	} else {
		otherQueuesCount := len(dtq.Active) + len(dtq.Empty)

		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("Summary:\n")
		if dtq.MainQueue != nil {
			otherQueuesCount-- // minus main queue
			buf.WriteString(fmt.Sprintf("- '%s' queue: %s.\n",
				dtq.MainQueue.Name,
				pluralize(len(dtq.MainQueue.Tasks), "empty", "task", "tasks")))
		}

		if otherQueuesCount > 0 {
			buf.WriteString(fmt.Sprintf("- %s (%d active, %d empty): %s.\n",
				pluralize(otherQueuesCount, "", "other queue", "other queues"),
				dtq.Summary.otherQueuesActiveCount, dtq.Summary.otherQueuesEmptyCount,
				pluralize(dtq.Summary.otherQueuesTasksCount, "", "task", "tasks")))
		}
		if dtq.Summary.totalTasksCount == 0 {
			buf.WriteString("- no tasks to handle.\n")
		} else {
			buf.WriteString(fmt.Sprintf("- total %s to handle.\n",
				pluralize(dtq.Summary.totalTasksCount, "", "task", "tasks")))
		}
	}

	return buf.String()
}
