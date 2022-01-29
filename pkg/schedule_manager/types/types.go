package types

import "time"

// ScheduleEntry is used to be able Add one crontab multiple
// times and independently Remove individual crontabs.
type ScheduleEntry struct {
	Crontab      string
	Id           string
	InitialDelay time.Duration // delay before first schedule
}
