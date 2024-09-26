package task

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"

	utils "github.com/flant/shell-operator/pkg/utils/labels"
)

type TaskType string

type Task interface {
	GetId() string
	GetType() TaskType
	IncrementFailureCount()
	GetFailureMessage() string
	UpdateFailureMessage(msg string)
	GetFailureCount() int
	GetLogLabels() map[string]string
	GetQueueName() string
	GetQueuedAt() time.Time
	WithQueuedAt(time.Time) Task
	GetMetadata() interface{}
	UpdateMetadata(interface{})
	GetDescription() string
	GetProp(key string) interface{}
	SetProp(key string, value interface{})
}

type BaseTask struct {
	Id             string
	Type           TaskType
	LogLabels      map[string]string
	FailureCount   int // Failed executions count
	FailureMessage string
	QueueName      string
	QueuedAt       time.Time

	Props map[string]interface{}

	lock     sync.RWMutex
	Metadata interface{}
}

func NewTask(taskType TaskType) *BaseTask {
	taskId := uuid.Must(uuid.NewV4()).String()
	return &BaseTask{
		Id:           taskId,
		FailureCount: 0,
		Type:         taskType,
		LogLabels:    map[string]string{"task.id": taskId},
		Props:        make(map[string]interface{}),
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
	t.lock.Lock()
	t.Metadata = metadata
	t.lock.Unlock()
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

func (t *BaseTask) GetQueuedAt() time.Time {
	return t.QueuedAt
}

func (t *BaseTask) WithQueuedAt(queuedAt time.Time) Task {
	t.QueuedAt = queuedAt
	return t
}

func (t *BaseTask) GetMetadata() interface{} {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.Metadata
}

func (t *BaseTask) UpdateMetadata(meta interface{}) {
	t.lock.Lock()
	t.Metadata = meta
	t.lock.Unlock()
}

func (t *BaseTask) GetProp(key string) interface{} {
	return t.Props[key]
}

func (t *BaseTask) SetProp(key string, value interface{}) {
	t.Props[key] = value
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

func (t *BaseTask) GetFailureMessage() string {
	return t.FailureMessage
}

type MetadataDescriptionGetter interface {
	GetDescription() string
}

func (t *BaseTask) GetDescription() string {
	metaDescription := ""
	t.lock.RLock()
	if descriptor, ok := t.Metadata.(MetadataDescriptionGetter); ok {
		metaDescription = ":" + descriptor.GetDescription()
	}
	t.lock.RUnlock()
	failDescription := ""
	if t.FailureCount > 0 {
		failDescription = fmt.Sprintf(":failures %d", t.FailureCount)
		if t.FailureMessage != "" {
			failDescription += ":" + t.FailureMessage
		}
	}

	return fmt.Sprintf("%s:%s%s%s", t.GetType(), t.GetQueueName(), metaDescription, failDescription)
}
