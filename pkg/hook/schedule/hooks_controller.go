package schedule

import "github.com/flant/shell-operator/pkg/schedule_manager"

type ScheduledHooksStorage []*ScheduleHook

// GetCrontabs returns all schedules from the hook store.
func (s ScheduledHooksStorage) GetCrontabs() []string {
	resMap := map[string]bool{}
	for _, hook := range s {
		for _, schedule := range hook.Schedule {
			resMap[schedule.Crontab] = true
		}
	}

	res := make([]string, len(resMap))
	for k := range resMap {
		res = append(res, k)
	}
	return res
}

// GetHooksForSchedule returns hooks for specific schedule.
func (s ScheduledHooksStorage) GetHooksForSchedule(crontab string) []*ScheduleHook {
	res := []*ScheduleHook{}

	for _, hook := range s {
		newHook := &ScheduleHook{
			Name:     hook.Name,
			Schedule: []schedule_manager.ScheduleConfig{},
		}
		for _, schedule := range hook.Schedule {
			if schedule.Crontab == crontab {
				newHook.Schedule = append(newHook.Schedule, schedule)
			}
		}

		if len(newHook.Schedule) > 0 {
			res = append(res, newHook)
		}
	}

	return res
}

// AddHook adds hook to the hook schedule.
func (s *ScheduledHooksStorage) AddHook(hookName string, config []schedule_manager.ScheduleConfig) {
	for i, hook := range *s {
		if hook.Name == hookName {
			// Changes hook config and exit if the hook already exists.
			(*s)[i].Schedule = []schedule_manager.ScheduleConfig{}
			for _, item := range config {
				(*s)[i].Schedule = append((*s)[i].Schedule, item)
			}
			return
		}
	}

	newHook := &ScheduleHook{
		Name:     hookName,
		Schedule: []schedule_manager.ScheduleConfig{},
	}
	for _, item := range config {
		newHook.Schedule = append(newHook.Schedule, item)
	}
	*s = append(*s, newHook)

}

// RemoveHook removes hook from the hook storage.
func (s *ScheduledHooksStorage) RemoveHook(hookName string) {
	tmp := ScheduledHooksStorage{}
	for _, hook := range *s {
		if hook.Name == hookName {
			continue
		}
		tmp = append(tmp, hook)
	}

	*s = tmp
}
