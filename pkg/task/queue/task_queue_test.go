package queue

import (
	"bytes"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/task"
)

func DumpTaskIds(q *TaskQueue) string {
	var buf bytes.Buffer
	var index int
	q.Iterate(func(t task.Task) {
		buf.WriteString(fmt.Sprintf("%d: %s\n", index, t.GetId()))
		index++
	})
	return buf.String()
}

func Test_TasksQueue_Remove(t *testing.T) {
	g := NewWithT(t)
	q := NewTasksQueue()

	// Remove just one element
	Task := &task.BaseTask{Id: "First one"}
	q.AddFirst(Task)
	g.Expect(q.Length()).To(Equal(1))
	q.Remove("First one")
	g.Expect(q.Length()).To(Equal(0))

	// Remove element in the middle
	for i := 0; i < 5; i++ {
		Task := &task.BaseTask{Id: fmt.Sprintf("task_%02d", i)}
		q.AddFirst(Task)
	}
	g.Expect(q.Length()).To(Equal(5))
	q.Remove("task_02")
	g.Expect(q.Length()).To(Equal(4))

	idsDump := DumpTaskIds(q)

	g.Expect(idsDump).To(And(
		ContainSubstring("task_00"),
		ContainSubstring("task_01"),
		ContainSubstring("task_03"),
		ContainSubstring("task_04"),
	))

	// Remove last element
	q.Remove("task_04")
	g.Expect(q.Length()).To(Equal(3))

	idsDump = DumpTaskIds(q)

	g.Expect(idsDump).To(And(
		ContainSubstring("task_00"),
		ContainSubstring("task_01"),
		ContainSubstring("task_03"),
	))

	// Remove first element by id
	q.Remove("task_00")
	g.Expect(q.Length()).To(Equal(2))

	idsDump = DumpTaskIds(q)

	g.Expect(idsDump).To(And(
		ContainSubstring("task_01"),
		ContainSubstring("task_03"),
	))
}
