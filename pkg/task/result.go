package task

import (
	"time"
)

type Status string

const (
	Success Status = "Success"
	Fail    Status = "Fail"
	Repeat  Status = "Repeat"
	Keep    Status = "Keep"
)

type Result struct {
	Status     Status
	headTasks  []Task
	tailTasks  []Task
	afterTasks []Task

	DelayBeforeNextTask time.Duration

	AfterHandle func()
}

func (res *Result) GetHeadTasks() []Task {
	return res.headTasks
}

func (res *Result) GetTailTasks() []Task {
	return res.tailTasks
}

func (res *Result) GetAfterTasks() []Task {
	return res.afterTasks
}

func (res *Result) AddHeadTasks(t ...Task) {
	if res.headTasks == nil {
		res.headTasks = make([]Task, 0, len(t))
	}

	res.headTasks = append(res.headTasks, t...)
}

func (res *Result) AddTailTasks(t ...Task) {
	if res.tailTasks == nil {
		res.tailTasks = make([]Task, 0, len(t))
	}

	res.tailTasks = append(res.tailTasks, t...)
}

func (res *Result) AddAfterTasks(t ...Task) {
	if res.afterTasks == nil {
		res.afterTasks = make([]Task, 0, len(t))
	}

	res.afterTasks = append(res.afterTasks, t...)
}
