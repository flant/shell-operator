package task

import (
	"fmt"

	utils "github.com/flant/shell-operator/pkg/utils/labels"
	uuid "gopkg.in/satori/go.uuid.v1"
)

type MetadataDescriptable interface {
	GetDescription() string
}

type TaskType string

type Task interface {
	GetId() string
	GetType() TaskType
	IncrementFailureCount()
	UpdateFailureMessage(msg string)
	GetFailureCount() int
	GetLogLabels() map[string]string
	GetQueueName() string
	GetMetadata() interface{}
	GetDescription() string
}

type BaseTask struct {
	Id             string
	Type           TaskType
	LogLabels      map[string]string
	FailureCount   int // Failed executions count
	FailureMessage string
	QueueName      string

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

func (t *BaseTask) WithQueueName(name string) *BaseTask {
	t.QueueName = name
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

func (t *BaseTask) GetQueueName() string {
	return t.QueueName
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

func (t *BaseTask) UpdateFailureMessage(msg string) {
	t.FailureMessage = msg
}

func (t *BaseTask) GetDescription() string {
	metaDescription := ""
	if descriptor, ok := t.Metadata.(MetadataDescriptable); ok {
		metaDescription = ":" + descriptor.GetDescription()
	}
	failDescription := ""
	if t.FailureCount > 0 {
		failDescription = fmt.Sprintf(":failures %d", t.FailureCount)
		if t.FailureMessage != "" {
			failDescription += ":" + t.FailureMessage
		}
	}

	return fmt.Sprintf("%s:%s%s%s", t.GetType(), t.GetQueueName(), metaDescription, failDescription)
}
