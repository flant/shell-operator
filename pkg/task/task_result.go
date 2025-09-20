package task

import (
	"time"
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
	headTasks  []Task
	tailTasks  []Task
	afterTasks []Task

	DelayBeforeNextTask time.Duration

	AfterHandle func()
}

func (res *TaskResult) GetHeadTasks() []Task {
	return res.headTasks
}

func (res *TaskResult) GetTailTasks() []Task {
	return res.tailTasks
}

func (res *TaskResult) GetAfterTasks() []Task {
	return res.afterTasks
}

func (res *TaskResult) AddHeadTasks(t ...Task) {
	if res.headTasks == nil {
		res.headTasks = make([]Task, 0, len(t))
	}

	res.headTasks = append(res.headTasks, t...)
}

func (res *TaskResult) AddTailTasks(t ...Task) {
	if res.tailTasks == nil {
		res.tailTasks = make([]Task, 0, len(t))
	}

	res.tailTasks = append(res.tailTasks, t...)
}

func (res *TaskResult) AddAfterTasks(t ...Task) {
	if res.afterTasks == nil {
		res.afterTasks = make([]Task, 0, len(t))
	}

	res.afterTasks = append(res.afterTasks, t...)
}
