package metrics

const (
	TasksQueueActionDurationSeconds = "{PREFIX}tasks_queue_action_duration_seconds"
	TasksQueueCompactionCounter     = "d8_telemetry_{PREFIX}tasks_queue_compaction_counter"
	TasksQueueCompactionReached     = "d8_telemetry_{PREFIX}tasks_queue_compaction_reached"
)
