package schedulemanager

import (
	"context"
	"log/slog"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"gopkg.in/robfig/cron.v2"

	pkg "github.com/flant/shell-operator/pkg"
	smtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"
)

// ScheduleRegistry manages cron schedule entries.
// ScheduleBindingsController only needs this subset of ScheduleManager.
type ScheduleRegistry interface {
	Add(entry smtypes.ScheduleEntry)
	Remove(entry smtypes.ScheduleEntry)
}

// ScheduleEmitter emits crontab schedule events.
// ManagerEventsHandler only needs this subset of ScheduleManager.
type ScheduleEmitter interface {
	Ch() chan string
}

type ScheduleManager interface {
	ScheduleRegistry
	ScheduleEmitter
	Stop()
	Start()
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
	Entries    map[string]*CronEntry

	logger *log.Logger
	mu     sync.Mutex
}

var _ ScheduleManager = (*scheduleManager)(nil)

func NewScheduleManager(ctx context.Context, logger *log.Logger) *scheduleManager {
	cctx, cancel := context.WithCancel(ctx)
	sm := &scheduleManager{
		ctx:        cctx,
		cancel:     cancel,
		ScheduleCh: make(chan string, 1),
		cron:       cron.New(),
		Entries:    make(map[string]*CronEntry),

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
func (sm *scheduleManager) Add(newEntry smtypes.ScheduleEntry) {
	logEntry := sm.logger.With(pkg.LogKeyOperatorComponent, "scheduleManager")
	if newEntry.Crontab == "" {
		logEntry.Error("crontab is empty")
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, hasCronEntry := sm.Entries[newEntry.Crontab]

	// If no entry, then add new scheduled function and save CronEntry.
	if !hasCronEntry {
		entryId, err := sm.cron.AddFunc(newEntry.Crontab, func() {
			logEntry.Debug("fire schedule event for entry", slog.String(pkg.LogKeyCrontab, newEntry.Crontab))
			sm.ScheduleCh <- newEntry.Crontab
		})
		if err != nil {
			logEntry.Error("invalid crontab", slog.String(pkg.LogKeyCrontab, newEntry.Crontab), slog.Any(pkg.LogKeyError, err))
			return
		}

		logEntry.Debug("entry added", slog.String(pkg.LogKeyCrontab, newEntry.Crontab))

		sm.Entries[newEntry.Crontab] = &CronEntry{
			EntryID: entryId,
			Ids: map[string]bool{
				newEntry.Id: true,
			},
		}
	}

	// Just add id into CronEntry.Ids
	if hasCronEntry {
		if _, hasId := sm.Entries[newEntry.Crontab].Ids[newEntry.Id]; !hasId {
			sm.Entries[newEntry.Crontab].Ids[newEntry.Id] = true
		}
	}
}

func (sm *scheduleManager) Remove(delEntry smtypes.ScheduleEntry) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
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
		sm.logger.With(pkg.LogKeyOperatorComponent, "scheduleManager").
			Debug("entry deleted", slog.String(pkg.LogKeyName, delEntry.Crontab))
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
