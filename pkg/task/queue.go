package task

import (
	"context"
	"time"
)

type TaskQueue interface {
	// Status and lifecycle methods
	GetStatus() string
	SetStatus(status string)
	GetName() string
	GetHandler() func(ctx context.Context, t Task) Result
	SetHandler(handler func(ctx context.Context, t Task) Result)
	Start(ctx context.Context)
	Stop()

	// Queue operations
	IsEmpty() bool
	Length() int

	// Add operations
	AddFirst(tasks ...Task)
	AddLast(tasks ...Task)
	AddAfter(id string, tasks ...Task)
	AddBefore(id string, newTask Task)

	// Remove operations
	RemoveFirst() Task
	RemoveLast() Task
	Remove(id string) Task

	// Get operations
	GetFirst() Task
	GetLast() Task
	Get(id string) Task

	// Iteration and filtering
	Iterate(doFn func(Task))
	Filter(filterFn func(Task) bool)

	// Utility methods
	CancelTaskDelay()
	MeasureActionTime(action string) func()
	String() string
}

type TaskQueueSet interface {
	Stop()
	StartMain(ctx context.Context)
	Start(ctx context.Context)
	Add(queue TaskQueue)
	GetByName(name string) TaskQueue
	GetMain() TaskQueue
	DoWithLock(fn func(tqs TaskQueueSet))
	Iterate(doFn func(queue TaskQueue))
	Remove(name string)
	WaitStopWithTimeout(timeout time.Duration)
}
