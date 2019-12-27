package task

import (
	utils "github.com/flant/shell-operator/pkg/utils/labels"
	uuid "gopkg.in/satori/go.uuid.v1"
)

type TaskType string

type Task interface {
	GetId() string
	GetType() TaskType
	IncrementFailureCount()
	GetFailureCount() int
	GetLogLabels() map[string]string
	GetMetadata() interface{}
}

type BaseTask struct {
	Id           string
	Type         TaskType
	LogLabels    map[string]string
	FailureCount int // Failed executions count

	Metadata interface{}
}

func NewTask(taskType TaskType) *BaseTask {
	taskId := uuid.NewV4().String()
	return &BaseTask{
		Id:           taskId,
		FailureCount: 0,
		Type:         taskType,
		LogLabels:    map[string]string{"task.id": taskId},
	}
}

func (t *BaseTask) WithLogLabels(labels map[string]string) *BaseTask {
	t.LogLabels = utils.MergeLabels(t.LogLabels, labels)
	return t
}

func (t *BaseTask) WithMetadata(metadata interface{}) *BaseTask {
	t.Metadata = metadata
	return t
}

func (t *BaseTask) GetId() string {
	return t.Id
}

func (t *BaseTask) GetType() TaskType {
	return t.Type
}

func (t *BaseTask) GetLogLabels() map[string]string {
	return t.LogLabels
}

func (t *BaseTask) GetMetadata() interface{} {
	return t.Metadata
}

func (t *BaseTask) GetFailureCount() int {
	return t.FailureCount
}

func (t *BaseTask) IncrementFailureCount() {
	t.FailureCount++
}
