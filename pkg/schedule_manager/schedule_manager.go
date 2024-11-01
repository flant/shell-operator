package schedule_manager

import (
	"context"

	"gopkg.in/robfig/cron.v2"

	"github.com/deckhouse/deckhouse/go_lib/log"
	. "github.com/flant/shell-operator/pkg/schedule_manager/types"
)

type ScheduleManager interface {
	Stop()
	Start()
	Add(entry ScheduleEntry)
	Remove(entry ScheduleEntry)
	Ch() chan string
}

type CronEntry struct {
	EntryID cron.EntryID
	Ids     map[string]bool
}

type scheduleManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	cron       *cron.Cron
	ScheduleCh chan string
	Entries    map[string]CronEntry

	logger *log.Logger
}

var _ ScheduleManager = &scheduleManager{}

func NewScheduleManager(ctx context.Context, logger *log.Logger) *scheduleManager {
	cctx, cancel := context.WithCancel(ctx)
	sm := &scheduleManager{
		ctx:        cctx,
		cancel:     cancel,
		ScheduleCh: make(chan string, 1),
		cron:       cron.New(),
		Entries:    make(map[string]CronEntry),

		logger: logger,
	}
	return sm
}

func (sm *scheduleManager) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}
}

// Add create entry for crontab and id and start scheduled function.
// Crontab string should be validated with cron.Parse
// function before pass to Add.
func (sm *scheduleManager) Add(newEntry ScheduleEntry) {
	logEntry := sm.logger.With("operator.component", "scheduleManager")

	cronEntry, hasCronEntry := sm.Entries[newEntry.Crontab]

	// If no entry, then add new scheduled function and save CronEntry.
	if !hasCronEntry {
		// The error can occur in case of bad format of crontab string.
		// All crontab strings should be validated before add.
		entryId, _ := sm.cron.AddFunc(newEntry.Crontab, func() {
			logEntry.Debugf("fire schedule event for entry '%s'", newEntry.Crontab)
			sm.ScheduleCh <- newEntry.Crontab
		})

		logEntry.Debugf("entry '%s' added", newEntry.Crontab)

		sm.Entries[newEntry.Crontab] = CronEntry{
			EntryID: entryId,
			Ids: map[string]bool{
				newEntry.Id: true,
			},
		}
	}

	// Just add id into CronEntry.Ids
	_, hasId := cronEntry.Ids[newEntry.Id]
	if !hasId {
		sm.Entries[newEntry.Crontab].Ids[newEntry.Id] = true
	}
}

func (sm *scheduleManager) Remove(delEntry ScheduleEntry) {
	cronEntry, hasCronEntry := sm.Entries[delEntry.Crontab]

	// Nothing to Remove
	if !hasCronEntry {
		return
	}

	_, hasId := cronEntry.Ids[delEntry.Id]
	if !hasId {
		// Nothing to remove
		return
	}

	// delete id from Ids map
	delete(sm.Entries[delEntry.Crontab].Ids, delEntry.Id)

	// if all ids are deleted, stop scheduled function
	if len(sm.Entries[delEntry.Crontab].Ids) == 0 {
		sm.cron.Remove(sm.Entries[delEntry.Crontab].EntryID)
		delete(sm.Entries, delEntry.Crontab)
		sm.logger.With("operator.component", "scheduleManager").Debugf("entry '%s' deleted", delEntry.Crontab)
	}
}

func (sm *scheduleManager) Start() {
	sm.cron.Start()
	go func() {
		<-sm.ctx.Done()
		sm.cron.Stop()
	}()
}

func (sm *scheduleManager) Ch() chan string {
	return sm.ScheduleCh
}
