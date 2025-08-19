package queue

const taskCap = 100

type TaskCounter struct {
	counter    map[string]uint
	reachedCap []string
}

func NewTaskCounter() *TaskCounter {
	return &TaskCounter{
		counter:    make(map[string]uint, 32),
		reachedCap: make([]string, 0, 32),
	}
}

func (tc *TaskCounter) Add(taskID string) {
	counter, ok := tc.counter[taskID]
	if !ok {
		tc.counter[taskID] = 0
	}

	counter++

	tc.counter[taskID] = counter

	if counter == taskCap {
		tc.reachedCap = append(tc.reachedCap, taskID)
	}
}

func (tc *TaskCounter) Remove(taskID string) {
	counter, ok := tc.counter[taskID]
	if !ok {
		return
	}

	counter--

	if counter == 0 {
		delete(tc.counter, taskID)
	} else {
		tc.counter[taskID] = counter
	}
}

func (tc *TaskCounter) GetReachedCap() []string {
	return tc.reachedCap
}

func (tc *TaskCounter) IsAnyCapReached() bool {
	return len(tc.reachedCap) > 0
}

func (tc *TaskCounter) ResetReachedCap() {
	tc.reachedCap = make([]string, 0, 32)
}
