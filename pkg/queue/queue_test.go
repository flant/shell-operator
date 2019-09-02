package queue

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// moved from play-queue

type QueueTestItem struct {
	failureCount int
	Name         string
}

func (t *QueueTestItem) IncrementFailureCount() {
	t.failureCount++
}

func (t *QueueTestItem) ToString() string {
	return fmt.Sprintf("Task [%s] failures=%d", t.Name, t.failureCount)
}

func FillQueue(q *Queue) {
	for i := 0; i < 5; i++ {
		t := QueueTestItem{Name: fmt.Sprintf("test_task_%04d", i)}
		q.Push(&t)
	}
	fmt.Println("Queue filled")
}

func HandleQueue(q *Queue, ch chan int, name string) {
	for !q.IsEmpty() {
		for i := 0; i < 10; i++ {
			if q.IsEmpty() {
				break
			}
			top, _ := q.Peek()
			if top != nil {
				taskText := q.WithLock(func(task interface{}) string {
					if v, ok := task.(*QueueTestItem); ok {
						return v.ToString()
					}
					return ""
				})
				if _, ok := top.(*QueueTestItem); ok {
					fmt.Printf("%s %s\n", name, taskText)
				} else {
					fmt.Printf("%s top is not Task\n", name)
				}
			} else {
				fmt.Printf("%s top is nil\n", name)
			}
		}
		q.Pop()
		time.Sleep(5 * time.Millisecond)
	}

	ch <- 1
}

// Test of parallel handling of one queue
func Test_Queue_PeekAndPopInDifferentGoRoutines(t *testing.T) {
	q := NewQueue()

	task := QueueTestItem{Name: "First one"}
	fmt.Println("Add")
	q.Push(&task)

	stopCh := make(chan int, 0)

	FillQueue(q)

	fmt.Println("Handle queue")

	go HandleQueue(q, stopCh, "---")
	go HandleQueue(q, stopCh, "+++")
	go HandleQueue(q, stopCh, "OOO")
	go HandleQueue(q, stopCh, "!!!")

	<-stopCh
	<-stopCh
	<-stopCh
	<-stopCh

	assert.Truef(t, q.IsEmpty(), "queue not empty after handling")
}
