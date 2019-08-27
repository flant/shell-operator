package schedule

type ScheduleHook struct {
	// hook name
	HookName string

	Crontab string

	ConfigName   string
	AllowFailure bool
}
