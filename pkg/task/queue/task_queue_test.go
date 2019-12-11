package queue

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/flant/shell-operator/pkg/task"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
)

func FillQueue(q *TaskQueue) {
	for i := 0; i < 5; i++ {
		t := &task.BaseTask{Id: fmt.Sprintf("test_task_%04d", i)}
		q.AddFirst(t)
	}
	fmt.Println("Queue filled")
}

func DumpTaskIds(q *TaskQueue) string {
	var buf bytes.Buffer
	var index int
	q.Iterate(func(t task.Task) {
		buf.WriteString(fmt.Sprintf("%d: %s\n", index, t.GetId()))
		index++
	})
	return buf.String()
}

func HandleQueue(q *TaskQueue, prefix string) {
	for !q.IsEmpty() {
		for i := 0; i < 10; i++ {
			if q.IsEmpty() {
				break
			}
			top := q.GetFirst()
			if top != nil {
				fmt.Printf("%s Task [%s] failures=%d\n", prefix, top.GetId(), top.GetFailureCount())
			} else {
				fmt.Printf("%s top is nil\n", prefix)
			}
		}
		q.RemoveFirst()
		time.Sleep(5 * time.Millisecond)
	}
}

// Test of parallel handling of one queue
// TODO make more useful test, do not print to stdout, do not run multiple handlers — there is only one handler for queue by design.
func Test_TaskQueue_PeekAndPopInDifferentGoRoutines(t *testing.T) {
	q := NewTasksQueue()

	Task := &task.BaseTask{Id: "First one"}
	q.AddFirst(Task)

	wg := sync.WaitGroup{}
	wg.Add(4)

	FillQueue(q)

	prefixes := []string{"---", "+++", "OOO", "!!!"}

	for _, prefix := range prefixes {
		p := prefix
		go func() {
			HandleQueue(q, p)
			wg.Done()
		}()
	}

	wg.Wait()

	assert.Truef(t, q.IsEmpty(), "queue not empty after handling")
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

	var idsDump string

	idsDump = DumpTaskIds(q)

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
