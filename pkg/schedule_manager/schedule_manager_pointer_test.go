package schedulemanager

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	smtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"
)

// TestPointerEntries_AddSecondIdPersists is a regression test for the CronEntry.Ids
// pointer-vs-value bug described in review.md §9.2 / Phase 2.7.
// Before the fix, adding a second ID to an existing crontab entry was silently dropped
// because the map stored CronEntry by value (a copy) and mutations to the copy did not
// persist in the map.
func TestPointerEntries_AddSecondIdPersists(t *testing.T) {
	sm := NewScheduleManager(context.Background(), log.NewNop())

	const crontab = "* * * * *"
	entry1 := smtypes.ScheduleEntry{Crontab: crontab, Id: "id-alpha"}
	entry2 := smtypes.ScheduleEntry{Crontab: crontab, Id: "id-beta"}

	sm.Add(entry1)
	sm.Add(entry2)

	// Both IDs must be present in the shared CronEntry.
	require.Contains(t, sm.Entries, crontab)
	entry := sm.Entries[crontab]
	assert.True(t, entry.Ids["id-alpha"], "id-alpha must be in Ids map")
	assert.True(t, entry.Ids["id-beta"], "id-beta must be in Ids map (regression: pointer fix)")
	assert.Len(t, entry.Ids, 2)
}

// TestPointerEntries_RemoveOneOfTwo verifies that removing one of two entries
// leaves the other intact.
func TestPointerEntries_RemoveOneOfTwo(t *testing.T) {
	sm := NewScheduleManager(context.Background(), log.NewNop())

	const crontab = "* * * * *"
	e1 := smtypes.ScheduleEntry{Crontab: crontab, Id: "a"}
	e2 := smtypes.ScheduleEntry{Crontab: crontab, Id: "b"}

	sm.Add(e1)
	sm.Add(e2)
	sm.Remove(e1)

	require.Contains(t, sm.Entries, crontab, "crontab should still exist after removing one ID")
	assert.False(t, sm.Entries[crontab].Ids["a"], "removed id should not be present")
	assert.True(t, sm.Entries[crontab].Ids["b"], "remaining id should still be present")
}

// TestSubinterfaces_Satisfy verifies that *scheduleManager satisfies both
// focused sub-interfaces defined in Phase 2.5.
func TestSubinterfaces_Satisfy(_ *testing.T) {
	sm := NewScheduleManager(context.Background(), log.NewNop())

	// Compile-time checks expressed as interface assignments.
	var _ ScheduleRegistry = sm
	var _ ScheduleEmitter = sm
	var _ ScheduleManager = sm
}
