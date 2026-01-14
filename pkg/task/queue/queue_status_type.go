package queue

import "sync"

// QueueStatusType represents the type of queue status
type QueueStatusType int

const (
	QueueStatusUnknown QueueStatusType = iota
	QueueStatusIdle
	QueueStatusNoHandlerSet
	QueueStatusRunningTask
	QueueStatusStop
	QueueStatusRepeatTask
	QueueStatusSleeping
	QueueStatusWaiting
)

// String returns a human-readable representation of the queue status type
func (qst QueueStatusType) String() string {
	switch qst {
	case QueueStatusIdle:
		return ""
	case QueueStatusNoHandlerSet:
		return "no handler set"
	case QueueStatusRunningTask:
		return "run first task"
	case QueueStatusStop:
		return "stop"
	case QueueStatusRepeatTask:
		return "repeat head task"
	case QueueStatusSleeping:
		return "sleeping"
	case QueueStatusWaiting:
		return "waiting for task"
	default:
		return "unknown"
	}
}

// TaskQueueStatus encapsulates queue status with thread-safe access
type TaskQueueStatus struct {
	mu         sync.RWMutex
	statusType QueueStatusType
	customText string
}

// NewTaskQueueStatus creates a new TaskQueueStatus
func NewTaskQueueStatus() *TaskQueueStatus {
	return &TaskQueueStatus{
		statusType: QueueStatusIdle,
	}
}

// Get returns the current status as a string
func (tqs *TaskQueueStatus) Get() string {
	tqs.mu.RLock()
	defer tqs.mu.RUnlock()

	if tqs.customText != "" {
		return tqs.customText
	}
	return tqs.statusType.String()
}

// GetType returns the current status type
func (tqs *TaskQueueStatus) GetType() QueueStatusType {
	tqs.mu.RLock()
	defer tqs.mu.RUnlock()
	return tqs.statusType
}

// Set sets the status to a specific type and clears custom text
func (tqs *TaskQueueStatus) Set(statusType QueueStatusType) {
	tqs.mu.Lock()
	defer tqs.mu.Unlock()
	tqs.statusType = statusType
	tqs.customText = ""
}

// SetText sets a custom status text
func (tqs *TaskQueueStatus) SetText(text string) {
	tqs.mu.Lock()
	defer tqs.mu.Unlock()
	tqs.statusType = QueueStatusSleeping
	tqs.customText = text
}

// SetWithText sets the status type with custom text
func (tqs *TaskQueueStatus) SetWithText(statusType QueueStatusType, text string) {
	tqs.mu.Lock()
	defer tqs.mu.Unlock()
	tqs.statusType = statusType
	tqs.customText = text
}

// Snapshot returns both the status type and custom text atomically
func (tqs *TaskQueueStatus) Snapshot() (QueueStatusType, string) {
	tqs.mu.RLock()
	defer tqs.mu.RUnlock()
	return tqs.statusType, tqs.customText
}

// Restore sets both the status type and custom text atomically
func (tqs *TaskQueueStatus) Restore(statusType QueueStatusType, customText string) {
	tqs.mu.Lock()
	defer tqs.mu.Unlock()
	tqs.statusType = statusType
	tqs.customText = customText
}
