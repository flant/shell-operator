package schedule_manager

type ScheduleConfig struct {
	Name         string `json:"name"`
	Crontab      string `json:"crontab"`
	AllowFailure bool   `json:"allowFailure"`
}
