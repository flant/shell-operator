package schedule_manager

import (
	"context"

	. "github.com/flant/shell-operator/pkg/schedule_manager/types"
	log "github.com/sirupsen/logrus"
	"gopkg.in/robfig/cron.v2"
)

type ScheduleManager interface {
	WithContext(ctx context.Context)
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
}

var _ ScheduleManager = &scheduleManager{}

var NewScheduleManager = func() *scheduleManager {
	sm := &scheduleManager{
		ScheduleCh: make(chan string, 1),
		cron:       cron.New(),
		Entries:    make(map[string]CronEntry),
	}
	return sm
}

func (sm *scheduleManager) WithContext(ctx context.Context) {
	sm.ctx, sm.cancel = context.WithCancel(ctx)
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
	logEntry := log.WithField("operator.component", "scheduleManager")

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
		log.WithField("operator.component", "scheduleManager").Debugf("entry '%s' deleted", delEntry.Crontab)
	}

	return
}

func (sm *scheduleManager) Start() {
	sm.cron.Start()
	go func() {
		select {
		case <-sm.ctx.Done():
			sm.cron.Stop()
			return
		}
	}()
}

func (sm *scheduleManager) Ch() chan string {
	return sm.ScheduleCh
}
