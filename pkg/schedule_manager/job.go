package schedule_manager

import (
	"time"

	. "github.com/flant/shell-operator/pkg/schedule_manager/types"

	"github.com/sirupsen/logrus"
	"gopkg.in/robfig/cron.v2"
)

// Schedule describes a job's duty cycle.
type scheduleWithInitDelay struct {
	firstRunDelay time.Duration
	firstSchedule bool
	scheduler     cron.Schedule
}

func (s *scheduleWithInitDelay) Next(cur time.Time) time.Time {
	next := s.scheduler.Next(cur)
	if !s.firstSchedule {
		next = next.Add(s.firstRunDelay)
		s.firstSchedule = true
	}

	return next
}

type job struct {
	scheduleCh chan string
	entry      ScheduleEntry
	logEntry   *logrus.Entry
}

func (j *job) Run() {
	j.logEntry.Debugf("fire schedule event for entry '%s'", j.entry.Crontab)
	j.scheduleCh <- j.entry.Crontab
}

func scheduleJob(cronManager *cron.Cron, entry ScheduleEntry, scheduleCh chan string) (cron.EntryID, error) {
	logEntry := logrus.WithField("operator.component", "scheduleManager")
	// The error can occur in case of bad format of crontab string.
	// All crontab strings should be validated before add.
	specSchedule, _ := cron.Parse(entry.Crontab)

	schedule := &scheduleWithInitDelay{
		firstRunDelay: entry.FirstRunDelay,
		scheduler:     specSchedule,
	}

	j := &job{
		scheduleCh: scheduleCh,
		entry:      entry,
		logEntry:   logEntry,
	}

	logEntry.Debugf("entry '%s' added with first run delay %s", entry.Crontab, entry.FirstRunDelay.String())

	return cronManager.Schedule(schedule, j), nil
}
