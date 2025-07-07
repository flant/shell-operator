package schedulemanager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"

	smtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"
)

func newTestManager() *scheduleManager {
	return NewScheduleManager(context.Background(), log.NewNop())
}

func TestAddAndRemoveEntry(t *testing.T) {
	mgr := newTestManager()
	entry := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}

	mgr.Add(entry)
	assert.Contains(t, mgr.Entries, entry.Crontab)
	assert.Contains(t, mgr.Entries[entry.Crontab].Ids, entry.Id)

	mgr.Remove(entry)
	assert.NotContains(t, mgr.Entries, entry.Crontab)
}

func TestAddDuplicateId(t *testing.T) {
	mgr := newTestManager()
	entry := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	mgr.Add(entry)
	mgr.Add(entry) // duplicate
	assert.Equal(t, 1, len(mgr.Entries[entry.Crontab].Ids))
}

func TestAddMultipleIdsSameCrontab(t *testing.T) {
	mgr := newTestManager()
	entry1 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	entry2 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id2"}
	mgr.Add(entry1)
	mgr.Add(entry2)
	assert.Contains(t, mgr.Entries[entry1.Crontab].Ids, entry1.Id)
	assert.Contains(t, mgr.Entries[entry2.Crontab].Ids, entry2.Id)
	assert.Equal(t, 2, len(mgr.Entries[entry1.Crontab].Ids))

	mgr.Remove(entry1)
	assert.NotContains(t, mgr.Entries[entry1.Crontab].Ids, entry1.Id)
	assert.Contains(t, mgr.Entries[entry1.Crontab].Ids, entry2.Id)
	mgr.Remove(entry2)
	assert.NotContains(t, mgr.Entries, entry1.Crontab)
}

func TestRemoveNonExistentEntry(_ *testing.T) {
	mgr := newTestManager()
	entry := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	mgr.Remove(entry) // should not panic or error
}

func TestRemoveNonExistentId(t *testing.T) {
	mgr := newTestManager()
	entry1 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	entry2 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id2"}
	mgr.Add(entry1)
	mgr.Remove(entry2) // should not remove entry1
	assert.Contains(t, mgr.Entries[entry1.Crontab].Ids, entry1.Id)
}

func TestAddEmptyCrontab(t *testing.T) {
	mgr := newTestManager()
	entry := smtypes.ScheduleEntry{Crontab: "", Id: "id1"}
	mgr.Add(entry)
	// Should not panic, and entry is not added
	assert.NotContains(t, mgr.Entries, "")
}

func TestStartAndStop(_ *testing.T) {
	mgr := newTestManager()
	mgr.Start()
	mgr.Stop()
}

func TestChReturnsChannel(t *testing.T) {
	mgr := newTestManager()
	ch := mgr.Ch()
	assert.NotNil(t, ch)
}

func TestScheduleFires(t *testing.T) {
	mgr := newTestManager()
	entry := smtypes.ScheduleEntry{Crontab: "@every 0.1s", Id: "id1"}
	mgr.Add(entry)
	mgr.Start()
	defer mgr.Stop()

	select {
	case v := <-mgr.Ch():
		assert.Equal(t, entry.Crontab, v)
	case <-time.After(2 * time.Second):
		t.Fatal("schedule did not fire in time")
	}
}

func TestConcurrentAddRemove(_ *testing.T) {
	mgr := newTestManager()
	mgr.Start()
	defer mgr.Stop()
	entry := smtypes.ScheduleEntry{Crontab: "@every 0.2s", Id: "id1"}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for i := 0; i < 10; i++ {
			mgr.Add(smtypes.ScheduleEntry{Crontab: entry.Crontab, Id: "id" + string(rune(i))})
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 10; i++ {
			mgr.Remove(smtypes.ScheduleEntry{Crontab: entry.Crontab, Id: "id" + string(rune(i))})
		}
		wg.Done()
	}()
	wg.Wait()
}
