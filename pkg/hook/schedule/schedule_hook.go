package schedule

import "github.com/flant/shell-operator/pkg/hook"

type ScheduleHook struct {
	// hook name
	HookName string

	Crontab string

	ConfigName   string
	AllowFailure bool
}

type ScheduleHooksStorage []*ScheduleHook

// GetCrontabs returns uniq crontabs from the storage.
func (s *ScheduleHooksStorage) GetCrontabs() []string {
	resMap := map[string]bool{}
	for _, scheduleHook := range *s {
		resMap[scheduleHook.Crontab] = true
	}

	res := make([]string, 0)
	for k := range resMap {
		res = append(res, k)
	}
	return res
}

// GetHooksForSchedule returns new array of ScheduleHook objects for specific crontab.
func (s *ScheduleHooksStorage) GetHooksForSchedule(crontab string) []ScheduleHook {
	res := make([]ScheduleHook, 0)

	for _, scheduleHook := range *s {
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
func (s *ScheduleHooksStorage) AddHook(hookName string, schedule hook.ScheduleConfig) {
	newHook := &ScheduleHook{
		HookName:     hookName,
		Crontab:      schedule.Crontab,
		ConfigName:   schedule.ConfigName,
		AllowFailure: schedule.AllowFailure,
	}
	*s = append(*s, newHook)
}

// RemoveHook removes all hooks from storage by hook name.
func (s *ScheduleHooksStorage) RemoveHook(hookName string) {
	newStorage := make([]*ScheduleHook, 0)
	for _, scheduleHook := range *s {
		if scheduleHook.HookName == hookName {
			continue
		}
		newStorage = append(newStorage, scheduleHook)
	}

	*s = newStorage
}
