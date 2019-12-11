package schedule_manager

import (
	. "github.com/flant/shell-operator/pkg/schedule_manager/types"
	log "github.com/sirupsen/logrus"
	"gopkg.in/robfig/cron.v2"
)

type ScheduleManager interface {
	Add(entry ScheduleEntry)
	Remove(entry ScheduleEntry)
	Run()
	Ch() chan string
}

type CronEntry struct {
	EntryID cron.EntryID
	Ids     map[string]bool
}

type scheduleManager struct {
	ScheduleCh chan string
	cron       *cron.Cron
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

//func (sm *scheduleManager) AddCrontab(crontab string) (string, error) {
//	logEntry := log.WithField("operator.component", "scheduleManager")
//
//	_, ok := sm.entries[crontab]
//	if !ok {
//		entryId, err := sm.cron.AddFunc(crontab, func() {
//			logEntry.Debugf("fire schedule event for entry '%s'", crontab)
//			ScheduleCh <- crontab
//		})
//		if err != nil {
//			return "", err
//		}
//
//		logEntry.Debugf("entry '%s' added", crontab)
//
//		sm.entries[crontab] = entryId
//	}
//
//	return crontab, nil
//}
//
//func (sm *scheduleManager) RemoveEntryId(crontab string) error {
//	entryID, ok := sm.entries[crontab]
//	if !ok {
//		return fmt.Errorf("schedule manager entry '%s' not found", crontab)
//	}
//
//	sm.cron.Remove(entryID)
//	log.WithField("operator.component", "scheduleManager").Debugf("entry '%s' deleted", crontab)
//
//	return nil
//}

func (sm *scheduleManager) Run() {
	sm.cron.Start()
}

func (sm *scheduleManager) stop() {
	sm.cron.Stop()
}

func (sm *scheduleManager) Ch() chan string {
	return sm.ScheduleCh
}
