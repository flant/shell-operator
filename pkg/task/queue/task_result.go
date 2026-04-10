package queue

import (
	"time"

	"github.com/flant/shell-operator/pkg/task"
)

type TaskStatus string

const (
	Success TaskStatus = "Success"
	Fail    TaskStatus = "Fail"
	Repeat  TaskStatus = "Repeat"
	Keep    TaskStatus = "Keep"
)

type TaskResult struct {
	Status     TaskStatus
	headTasks  []task.Task
	tailTasks  []task.Task
	afterTasks []task.Task

	DelayBeforeNextTask time.Duration

	AfterHandle func()
}

func (res *TaskResult) GetHeadTasks() []task.Task {
	return res.headTasks
}

func (res *TaskResult) GetTailTasks() []task.Task {
	return res.tailTasks
}

func (res *TaskResult) GetAfterTasks() []task.Task {
	return res.afterTasks
}

func (res *TaskResult) AddHeadTasks(t ...task.Task) {
	if res.headTasks == nil {
		res.headTasks = make([]task.Task, 0, len(t))
	}

	res.headTasks = append(res.headTasks, t...)
}

func (res *TaskResult) AddTailTasks(t ...task.Task) {
	if res.tailTasks == nil {
		res.tailTasks = make([]task.Task, 0, len(t))
	}

	res.tailTasks = append(res.tailTasks, t...)
}

func (res *TaskResult) AddAfterTasks(t ...task.Task) {
	if res.afterTasks == nil {
		res.afterTasks = make([]task.Task, 0, len(t))
	}

	res.afterTasks = append(res.afterTasks, t...)
}
