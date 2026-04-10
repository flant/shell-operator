package queue

import (
	"sync"

	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/utils/list"
)

// TaskStorage encapsulates the queue data structures with their own mutex
type TaskStorage struct {
	mu      sync.RWMutex
	items   *list.List[task.Task]
	idIndex map[string]*list.Element[task.Task]
}

// newTaskStorage creates a new TaskStorage instance
func newTaskStorage() *TaskStorage {
	return &TaskStorage{
		items:   list.New[task.Task](),
		idIndex: make(map[string]*list.Element[task.Task]),
	}
}

// Private methods (without locking)

// addFirst adds tasks to the front of the queue (internal implementation)
func (ts *TaskStorage) addFirst(tasks ...task.Task) {
	// Add tasks in reverse order so the first task becomes the new head
	for i := len(tasks) - 1; i >= 0; i-- {
		element := ts.items.PushFront(tasks[i])
		ts.idIndex[tasks[i].GetId()] = element
	}
}

// removeFirst removes and returns the first task (internal implementation)
func (ts *TaskStorage) removeFirst() task.Task {
	if ts.items.Len() > 0 {
		element := ts.items.Front()
		t := ts.items.Remove(element)
		delete(ts.idIndex, t.GetId())
		return t
	}
	return nil
}

// addLast adds tasks to the back of the queue (internal implementation)
func (ts *TaskStorage) addLast(tasks ...task.Task) {
	for _, t := range tasks {
		element := ts.items.PushBack(t)
		ts.idIndex[t.GetId()] = element
	}
}

// removeLast removes and returns the last task (internal implementation)
func (ts *TaskStorage) removeLast() task.Task {
	if ts.items.Len() > 0 {
		element := ts.items.Back()
		t := ts.items.Remove(element)
		delete(ts.idIndex, t.GetId())
		return t
	}
	return nil
}

// addAfter inserts tasks after the task with the specified ID (internal implementation)
func (ts *TaskStorage) addAfter(id string, tasks ...task.Task) {
	if element, ok := ts.idIndex[id]; ok {
		// Insert tasks in reverse order so they appear in the correct sequence
		for i := len(tasks) - 1; i >= 0; i-- {
			newElement := ts.items.InsertAfter(tasks[i], element)
			ts.idIndex[tasks[i].GetId()] = newElement
		}
	}
}

// addBefore inserts a task before the task with the specified ID (internal implementation)
func (ts *TaskStorage) addBefore(id string, newTask task.Task) {
	if element, ok := ts.idIndex[id]; ok {
		newElement := ts.items.InsertBefore(newTask, element)
		ts.idIndex[newTask.GetId()] = newElement
	}
}

// remove removes and returns a task by ID (internal implementation)
func (ts *TaskStorage) remove(id string) task.Task {
	if element, ok := ts.idIndex[id]; ok {
		t := ts.items.Remove(element)
		delete(ts.idIndex, id)
		return t
	}
	return nil
}

// removeElement removes an element from the list and updates the index (internal implementation)
func (ts *TaskStorage) removeElement(element *list.Element[task.Task]) task.Task {
	t := ts.items.Remove(element)
	delete(ts.idIndex, t.GetId())
	return t
}

// Public methods (with locking)

// Length returns the number of items in the queue
func (ts *TaskStorage) Length() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.items.Len()
}

// GetFirst returns the first task without removing it
func (ts *TaskStorage) GetFirst() task.Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if ts.items.Len() > 0 {
		return ts.items.Front().Value
	}

	return nil
}

// Get returns a task by ID
func (ts *TaskStorage) Get(id string) task.Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if element, ok := ts.idIndex[id]; ok {
		return element.Value
	}

	return nil
}

// AddFirst adds tasks to the front of the queue
func (ts *TaskStorage) AddFirst(tasks ...task.Task) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.addFirst(tasks...)
}

// RemoveFirst removes and returns the first task
func (ts *TaskStorage) RemoveFirst() task.Task {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	return ts.removeFirst()
}

// AddLast adds tasks to the back of the queue
func (ts *TaskStorage) AddLast(tasks ...task.Task) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.addLast(tasks...)
}

// RemoveLast removes and returns the last task
func (ts *TaskStorage) RemoveLast() task.Task {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	return ts.removeLast()
}

// AddAfter inserts tasks after the task with the specified ID
func (ts *TaskStorage) AddAfter(id string, tasks ...task.Task) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.addAfter(id, tasks...)
}

// AddBefore inserts a task before the task with the specified ID
func (ts *TaskStorage) AddBefore(id string, newTask task.Task) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.addBefore(id, newTask)
}

// Remove removes and returns a task by ID
func (ts *TaskStorage) Remove(id string) task.Task {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	return ts.remove(id)
}

// GetSnapshot returns a copy of all tasks in the queue
func (ts *TaskStorage) GetSnapshot() []task.Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	snapshot := make([]task.Task, 0, ts.items.Len())
	for e := ts.items.Front(); e != nil; e = e.Next() {
		snapshot = append(snapshot, e.Value)
	}

	return snapshot
}

// Iterate iterates over all tasks in the queue with a callback
func (ts *TaskStorage) Iterate(fn func(*list.Element[task.Task])) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	for e := ts.items.Front(); e != nil; e = e.Next() {
		fn(e)
	}
}

// RemoveElement removes an element from the list and updates the index
func (ts *TaskStorage) RemoveElement(element *list.Element[task.Task]) task.Task {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	return ts.removeElement(element)
}

// GetLast returns the last task in the queue
func (ts *TaskStorage) GetLast() task.Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if ts.items.Len() > 0 {
		return ts.items.Back().Value
	}

	return nil
}

// DeleteFunc runs fn on every task and removes each task for which fn returns false
func (ts *TaskStorage) DeleteFunc(fn func(task.Task) bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for e := ts.items.Front(); e != nil; {
		next := e.Next()
		task := e.Value
		if !fn(task) {
			ts.items.Remove(e)
			delete(ts.idIndex, task.GetId())
		}
		e = next
	}
}

func (ts *TaskStorage) ProcessResult(taskRes TaskResult, t task.Task) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Add after tasks
	ts.addAfter(t.GetId(), taskRes.GetAfterTasks()...)

	// Remove the current task if successful
	if taskRes.Status == Success {
		ts.remove(t.GetId())
	}

	// Add head tasks
	ts.addFirst(taskRes.GetHeadTasks()...)

	// Add tail tasks to the end
	ts.addLast(taskRes.GetTailTasks()...)
}
