package queue

import "sync"

// queueStorage is a thread-safe storage for task queues with basic Get/Set/Delete operations
type queueStorage struct {
	mu     sync.RWMutex
	queues map[string]*TaskQueue
}

func newQueueStorage() *queueStorage {
	return &queueStorage{
		queues: make(map[string]*TaskQueue, 1),
	}
}

// Get retrieves a queue by name, returns nil if not found
func (qs *queueStorage) Get(name string) (*TaskQueue, bool) {
	qs.mu.RLock()
	defer qs.mu.RUnlock()

	queue, exists := qs.queues[name]

	return queue, exists
}

// List retrieves all queues
func (qs *queueStorage) List() []*TaskQueue {
	qs.mu.RLock()
	defer qs.mu.RUnlock()

	queues := make([]*TaskQueue, 0, len(qs.queues))
	for _, queue := range qs.queues {
		queues = append(queues, queue)
	}

	return queues
}

// Set stores a queue with the given name
func (qs *queueStorage) Set(name string, queue *TaskQueue) {
	qs.mu.Lock()
	defer qs.mu.Unlock()

	qs.queues[name] = queue
}

// Delete removes a queue by name
func (qs *queueStorage) Delete(name string) {
	qs.mu.Lock()
	defer qs.mu.Unlock()

	delete(qs.queues, name)
}

// Len returns the number of tasks in a queue by name
func (qs *queueStorage) Len() int {
	qs.mu.RLock()
	defer qs.mu.RUnlock()

	return len(qs.queues)
}
