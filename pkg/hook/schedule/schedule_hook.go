package schedule

import "github.com/flant/shell-operator/pkg/schedule_manager"

type ScheduleHook struct {
	Name     string
	Schedule []schedule_manager.ScheduleConfig
}
