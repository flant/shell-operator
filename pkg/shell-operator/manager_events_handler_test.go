package shell_operator

import (
	"context"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/require"

	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

type fakeKubeEventEmitter struct {
	ch chan kemtypes.KubeEvent
}

func (f *fakeKubeEventEmitter) Ch() chan kemtypes.KubeEvent { return f.ch }

type fakeScheduleEmitter struct {
	ch chan string
}

func (f *fakeScheduleEmitter) Ch() chan string { return f.ch }

func newTestEventsHandler(kubeCh chan kemtypes.KubeEvent, schedCh chan string) *ManagerEventsHandler {
	return newManagerEventsHandler(context.Background(), &managerEventsHandlerConfig{
		tqs:    queue.NewTaskQueueSet(),
		mgr:    &fakeKubeEventEmitter{ch: kubeCh},
		smgr:   &fakeScheduleEmitter{ch: schedCh},
		logger: log.NewNop(),
	})
}

// TestManagerEventsHandlerSurvivesPanicInKubeEventCb proves the panic
// escalation bug: a panic inside the kube-event callback (in production - the
// nil HookController dereference during hook dispatch) must not kill the
// events goroutine and the whole process. The handler has to log the panic
// and keep processing subsequent events.
//
// On unfixed code this test crashes the whole test binary - that crash is the
// process-level SIGSEGV from the incident, reproduced in miniature.
func TestManagerEventsHandlerSurvivesPanicInKubeEventCb(t *testing.T) {
	kubeCh := make(chan kemtypes.KubeEvent)
	m := newTestEventsHandler(kubeCh, make(chan string))

	processed := make(chan string, 1)
	m.WithKubeEventHandler(func(_ context.Context, ev kemtypes.KubeEvent) []task.Task {
		if ev.MonitorId == "panic" {
			panic("runtime error: invalid memory address or nil pointer dereference")
		}
		processed <- ev.MonitorId
		return nil
	})

	m.Start()
	defer m.Stop()

	kubeCh <- kemtypes.KubeEvent{MonitorId: "panic"}
	kubeCh <- kemtypes.KubeEvent{MonitorId: "after-panic"}

	select {
	case got := <-processed:
		require.Equal(t, "after-panic", got)
	case <-time.After(5 * time.Second):
		t.Fatal("events goroutine died after panic: second event was never processed")
	}
}

// TestManagerEventsHandlerSurvivesPanicInScheduleCb covers the same panic
// isolation for the schedule callback branch.
func TestManagerEventsHandlerSurvivesPanicInScheduleCb(t *testing.T) {
	schedCh := make(chan string)
	m := newTestEventsHandler(make(chan kemtypes.KubeEvent), schedCh)

	processed := make(chan string, 1)
	m.WithScheduleEventHandler(func(_ context.Context, crontab string) []task.Task {
		if crontab == "panic" {
			panic("schedule hook dispatch failed")
		}
		processed <- crontab
		return nil
	})

	m.Start()
	defer m.Stop()

	schedCh <- "panic"
	schedCh <- "* * * * *"

	select {
	case got := <-processed:
		require.Equal(t, "* * * * *", got)
	case <-time.After(5 * time.Second):
		t.Fatal("events goroutine died after panic: second event was never processed")
	}
}
