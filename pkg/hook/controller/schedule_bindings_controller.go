package controller

import (
	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/flant/shell-operator/pkg/schedule_manager"
)

// A link between a hook and a kube monitor
type ScheduleBindingToCrontabLink struct {
	BindingName string
	Crontab     string
	// Useful fields to create a BindingContext
	IncludeSnapshots []string
	AllowFailure     bool
	QueueName        string
	SkipKey          string
}

// ScheduleBindingsController handles schedule bindings for one hook.
type ScheduleBindingsController interface {
	WithScheduleBindings([]ScheduleConfig)
	WithScheduleManager(schedule_manager.ScheduleManager)
	EnableScheduleBindings()
	DisableScheduleBindings()
	CanHandleEvent(crontab string) bool
	HandleEvent(crontab string) []BindingExecutionInfo
}

// scheduleHooksController is a main implementation of KubernetesHooksController
type scheduleBindingsController struct {
	// All hooks with 'kubernetes' bindings
	ScheduleLinks map[string]*ScheduleBindingToCrontabLink

	// bindings configurations
	ScheduleBindings []ScheduleConfig

	// dependencies
	scheduleManager schedule_manager.ScheduleManager
}

// kubernetesHooksController should implement the KubernetesHooksController
var _ ScheduleBindingsController = &scheduleBindingsController{}

// NewKubernetesHooksController returns an implementation of KubernetesHooksController
var NewScheduleBindingsController = func() *scheduleBindingsController {
	return &scheduleBindingsController{
		ScheduleLinks: make(map[string]*ScheduleBindingToCrontabLink),
	}
}

func (c *scheduleBindingsController) WithScheduleBindings(bindings []ScheduleConfig) {
	c.ScheduleBindings = bindings
}

func (c *scheduleBindingsController) WithScheduleManager(scheduleManager schedule_manager.ScheduleManager) {
	c.scheduleManager = scheduleManager
}

func (c *scheduleBindingsController) CanHandleEvent(crontab string) bool {
	for _, link := range c.ScheduleLinks {
		if link.Crontab == crontab {
			return true
		}
	}
	return false
}

func (c *scheduleBindingsController) HandleEvent(crontab string) []BindingExecutionInfo {
	res := []BindingExecutionInfo{}

	for _, link := range c.ScheduleLinks {
		if link.Crontab == crontab {
			bc := BindingContext{
				Binding: link.BindingName,
			}
			bc.Metadata.BindingType = Schedule
			bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
			bc.Metadata.SkipKey = link.SkipKey

			info := BindingExecutionInfo{
				BindingContext:   []BindingContext{bc},
				IncludeSnapshots: link.IncludeSnapshots,
				AllowFailure:     link.AllowFailure,
				QueueName:        link.QueueName,
			}
			res = append(res, info)
		}
	}

	return res
}

func (c *scheduleBindingsController) EnableScheduleBindings() {
	for _, config := range c.ScheduleBindings {
		c.ScheduleLinks[config.ScheduleEntry.Id] = &ScheduleBindingToCrontabLink{
			BindingName:      config.BindingName,
			Crontab:          config.ScheduleEntry.Crontab,
			IncludeSnapshots: config.IncludeSnapshotsFrom,
			AllowFailure:     config.AllowFailure,
			QueueName:        config.Queue,
			SkipKey:          config.SkipKey,
		}
		c.scheduleManager.Add(config.ScheduleEntry)
	}
}

func (c *scheduleBindingsController) DisableScheduleBindings() {
	for _, config := range c.ScheduleBindings {
		c.scheduleManager.Remove(config.ScheduleEntry)
		delete(c.ScheduleLinks, config.ScheduleEntry.Id)
	}
}
