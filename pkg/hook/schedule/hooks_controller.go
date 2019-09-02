package schedule

import (
	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
)

type ScheduleHooksController interface {
	EnableHooks(hookManager hook.HookManager, eventsManager schedule_manager.ScheduleManager) error

	HandleEvent(kubeEvent kube_events_manager.KubeEvent) ([]task.Task, error)
}

type MainScheduleHooksController struct {
	ScheduleHooks []*ScheduleHook
}

type ScheduledHooksStorage []*ScheduleHook

// GetCrontabs returns uniq crontabs from the storage.
func (s ScheduledHooksStorage) GetCrontabs() []string {
	resMap := map[string]bool{}
	for _, scheduleHook := range s {
		resMap[scheduleHook.Crontab] = true
	}

	res := make([]string, len(resMap))
	for k := range resMap {
		res = append(res, k)
	}
	return res
}

// GetHooksForSchedule returns new array of ScheduleHook objects for specific crontab.
func (s ScheduledHooksStorage) GetHooksForSchedule(crontab string) []ScheduleHook {
	res := make([]ScheduleHook, 0)

	for _, scheduleHook := range s {
		if scheduleHook.Crontab == crontab {
			newHook := ScheduleHook{
				HookName:     scheduleHook.HookName,
				Crontab:      scheduleHook.Crontab,
				ConfigName:   scheduleHook.ConfigName,
				AllowFailure: scheduleHook.AllowFailure,
			}
			res = append(res, newHook)
		}
	}

	return res
}

// AddHook adds hook to the storage.
func (s *ScheduledHooksStorage) AddHook(hookName string, schedule hook.ScheduleConfig) {
	newHook := &ScheduleHook{
		HookName:     hookName,
		Crontab:      schedule.Crontab,
		ConfigName:   schedule.ConfigName,
		AllowFailure: schedule.AllowFailure,
	}
	*s = append(*s, newHook)
}

// RemoveHook removes all hooks from storage by hook name.
func (s *ScheduledHooksStorage) RemoveHook(hookName string) {
	newStorage := ScheduledHooksStorage{}
	for _, scheduleHook := range *s {
		if scheduleHook.HookName == hookName {
			continue
		}
		newStorage = append(newStorage, scheduleHook)
	}

	*s = newStorage
}
