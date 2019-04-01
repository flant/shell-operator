package task

import (
	"bytes"
	"fmt"
	"io"

	"github.com/flant/shell-operator/pkg/queue"
)

/*
A working queue (a pipeline) for sequential execution of tasks.

Tasks are added to the tail and executed from the head. Also a task can be pushed
to the head to implement a meta-tasks.

Each task is executed until success. This can be controlled with allowFailure: true
config parameter.

*/

type TextDumper interface {
	DumpAsText() string
}

type FailureCountIncrementable interface {
	IncrementFailureCount()
}

type TasksQueue struct {
	*queue.Queue
}

func (tq *TasksQueue) Add(task Task) {
	tq.Queue.Add(task)
}

func (tq *TasksQueue) Push(task Task) {
	tq.Queue.Push(task)
}

func (tq *TasksQueue) Peek() (task Task, err error) {
	res, err := tq.Queue.Peek()
	if err != nil {
		return nil, err
	}

	var ok bool
	if task, ok = res.(Task); !ok {
		return nil, fmt.Errorf("bad data found in queue: %v", res)
	}

	return task, nil
}

func NewTasksQueue() *TasksQueue {
	return &TasksQueue{
		Queue: queue.NewQueue(),
	}
}

func (tq *TasksQueue) IncrementFailureCount() {
	tq.Queue.WithLock(func(topTask interface{}) string {
		if v, ok := topTask.(FailureCountIncrementable); ok {
			v.IncrementFailureCount()
		}
		return ""
	})
}

// прочитать дамп структуры для сохранения во временный файл
func (tq *TasksQueue) DumpReader() io.Reader {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Queue length %d\n", tq.Queue.Length()))
	buf.WriteString("\n")

	iterateBuf := tq.Queue.IterateWithLock(func(task interface{}, index int) string {
		if v, ok := task.(TextDumper); ok {
			return v.DumpAsText()
		}
		return fmt.Sprintf("task %d: %+v", index, task)
	})
	return io.MultiReader(&buf, iterateBuf)
}
