package schedule_manager

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gopkg.in/robfig/cron.v2"
)

var (
	ScheduleCh chan string
)

type ScheduleManager interface {
	Add(crontab string) (string, error)
	Remove(entryId string) error
	Run()
}

type scheduleManager struct {
	cron    *cron.Cron
	entries map[string]cron.EntryID
}

var NewScheduleManager = func() *scheduleManager {
	sm := &scheduleManager{}
	sm.cron = cron.New()
	sm.entries = make(map[string]cron.EntryID)
	return sm
}

func (sm *scheduleManager) Add(crontab string) (string, error) {
	logEntry := log.WithField("operator.component", "scheduleManager")

	_, ok := sm.entries[crontab]
	if !ok {
		entryId, err := sm.cron.AddFunc(crontab, func() {
			logEntry.Debugf("fire schedule event for entry '%s'", crontab)
			ScheduleCh <- crontab
		})
		if err != nil {
			return "", err
		}

		logEntry.Debugf("entry '%s' added", crontab)

		sm.entries[crontab] = entryId
	}

	return crontab, nil
}

func (sm *scheduleManager) Remove(crontab string) error {
	entryID, ok := sm.entries[crontab]
	if !ok {
		return fmt.Errorf("schedule manager entry '%s' not found", crontab)
	}

	sm.cron.Remove(entryID)
	log.WithField("operator.component", "scheduleManager").Debugf("entry '%s' deleted", crontab)

	return nil
}

func (sm *scheduleManager) Run() {
	sm.cron.Start()
}

func (sm *scheduleManager) stop() {
	sm.cron.Stop()
}

func Init() (ScheduleManager, error) {
	ScheduleCh = make(chan string, 1)

	sm := NewScheduleManager()

	return sm, nil
}
