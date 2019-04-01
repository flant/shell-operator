package queue

import (
	"bytes"
	"io"
	"sync"
)

// TODO transform into gen/main.go to generate a working queues for different types

type QueueWatcher interface {
	QueueChangeCallback()
}

type Queue struct {
	m              sync.Mutex
	items          []interface{}
	changesEnabled bool           // turns on and off callbacks execution
	changesCount   int            // counts changes when callbacks turned off
	queueWatchers  []QueueWatcher // callbacks to be executed on items change
}

// NewQueue creates a new queue.
func NewQueue() *Queue {
	return &Queue{
		m:              sync.Mutex{},
		items:          make([]interface{}, 0),
		changesCount:   0,
		changesEnabled: false,
		queueWatchers:  make([]QueueWatcher, 0),
	}
}

// Add adds the last element.
func (q *Queue) Add(task interface{}) {
	q.m.Lock()
	q.items = append(q.items, task)
	q.m.Unlock()
	q.queueChanged()
}

// Peek examines the first element (get without delete).
func (q *Queue) Peek() (task interface{}, err error) {
	if q.IsEmpty() {
		return nil, err
	}
	return q.items[0], err
}

// Pop pops the first element (delete).
func (q *Queue) Pop() (task interface{}) {
	q.m.Lock()
	if q.isEmpty() {
		q.m.Unlock()
		return
	}
	task = q.items[0]
	q.items = q.items[1:]
	q.m.Unlock()
	q.queueChanged()
	return task
}

// Push pushes the element as the first element.
func (q *Queue) Push(task interface{}) {
	q.m.Lock()
	q.items = append([]interface{}{task}, q.items...)
	q.m.Unlock()
	q.queueChanged()
}

func (q *Queue) IsEmpty() bool {
	q.m.Lock()
	defer q.m.Unlock()
	return q.isEmpty()
}

func (q *Queue) isEmpty() bool {
	return len(q.items) == 0
}

func (q *Queue) Length() int {
	return len(q.items)
}

// Watcher functions

// AddWatcher adds queue watcher.
func (q *Queue) AddWatcher(queueWatcher QueueWatcher) {
	q.queueWatchers = append(q.queueWatchers, queueWatcher)
}

// queueChanged must be called every time the queue changes.
func (q *Queue) queueChanged() {
	if len(q.queueWatchers) == 0 {
		return
	}

	if q.changesEnabled {
		for _, watcher := range q.queueWatchers {
			watcher.QueueChangeCallback()
		}
	} else {
		q.changesCount++
	}
}

// Включить вызов QueueChangeCallback при каждом изменении
// В паре с ChangesDisabled могут быть использованы, чтобы
// производить массовые изменения. Если runCallbackOnPreviousChanges true,
// то будет вызвана QueueChangeCallback
func (q *Queue) ChangesEnable(runCallbackOnPreviousChanges bool) {
	q.changesEnabled = true
	if runCallbackOnPreviousChanges && q.changesCount > 0 {
		q.changesCount = 0
		q.queueChanged()
	}
}

func (q *Queue) ChangesDisable() {
	q.changesEnabled = false
	q.changesCount = 0
}

// Тип метода — операция над первым элементом очереди
type TaskOperation func(topTask interface{}) string
type IterateOperation func(task interface{}, index int) string

// WithLock executes operation on the first element of the queue with a queue lock.
func (q *Queue) WithLock(operation TaskOperation) string {
	q.m.Lock()
	defer q.m.Unlock()
	if !q.isEmpty() {
		return operation(q.items[0])
	}
	return ""
}

// IterateWithLock executes operation on all elements of the queue with a queue lock.
func (q *Queue) IterateWithLock(operation IterateOperation) io.Reader {
	q.m.Lock()
	defer q.m.Unlock()
	var buf bytes.Buffer
	for i := 0; i < len(q.items); i++ {
		buf.WriteString(operation(q.items[i], i))
		buf.WriteString("\n")
	}
	return &buf
}
