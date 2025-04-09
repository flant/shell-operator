package controller

import (
	"sync"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
)

// ScheduleBindingToCrontabLink a link between a hook and a kube monitor
type ScheduleBindingToCrontabLink struct {
	BindingName string
	Crontab     string
	// Useful fields to create a BindingContext
	IncludeSnapshots []string
	AllowFailure     bool
	QueueName        string
	Group            string
}

// ScheduleBindingsController handles schedule bindings for one hook.
type ScheduleBindingsController interface {
	WithScheduleBindings([]htypes.ScheduleConfig)
	WithScheduleManager(schedulemanager.ScheduleManager)
	EnableScheduleBindings()
	DisableScheduleBindings()
	CanHandleEvent(crontab string) bool
	HandleEvent(crontab string) []BindingExecutionInfo
}

// scheduleBindingsController is a main implementation of KubernetesHooksController
type scheduleBindingsController struct {
	// dependencies
	scheduleManager schedulemanager.ScheduleManager

	l sync.RWMutex
	// All hooks with 'kubernetes' bindings
	ScheduleLinks map[string]*ScheduleBindingToCrontabLink

	// bindings configurations
	ScheduleBindings []htypes.ScheduleConfig
}

// NewScheduleBindingsController returns an implementation of ScheduleBindingsController
var NewScheduleBindingsController = func() ScheduleBindingsController {
	return &scheduleBindingsController{
		ScheduleLinks: make(map[string]*ScheduleBindingToCrontabLink),
	}
}

func (c *scheduleBindingsController) WithScheduleBindings(bindings []htypes.ScheduleConfig) {
	c.ScheduleBindings = bindings
}

func (c *scheduleBindingsController) WithScheduleManager(scheduleManager schedulemanager.ScheduleManager) {
	c.l.Lock()
	c.scheduleManager = scheduleManager
	c.l.Unlock()
}

func (c *scheduleBindingsController) CanHandleEvent(crontab string) bool {
	c.l.RLock()
	defer c.l.RUnlock()
	for _, link := range c.ScheduleLinks {
		if link.Crontab == crontab {
			return true
		}
	}
	return false
}

func (c *scheduleBindingsController) HandleEvent(crontab string) []BindingExecutionInfo {
	res := []BindingExecutionInfo{}

	c.l.RLock()
	for _, link := range c.ScheduleLinks {
		if link.Crontab == crontab {
			bc := bctx.BindingContext{
				Binding: link.BindingName,
			}
			bc.Metadata.BindingType = htypes.Schedule
			bc.Metadata.IncludeSnapshots = link.IncludeSnapshots
			bc.Metadata.Group = link.Group

			info := BindingExecutionInfo{
				BindingContext:   []bctx.BindingContext{bc},
				IncludeSnapshots: link.IncludeSnapshots,
				AllowFailure:     link.AllowFailure,
				QueueName:        link.QueueName,
				Binding:          link.BindingName,
				Group:            link.Group,
			}
			res = append(res, info)
		}
	}
	c.l.RUnlock()

	return res
}

func (c *scheduleBindingsController) EnableScheduleBindings() {
	c.l.Lock()
	for _, config := range c.ScheduleBindings {
		c.ScheduleLinks[config.ScheduleEntry.Id] = &ScheduleBindingToCrontabLink{
			BindingName:      config.BindingName,
			Crontab:          config.ScheduleEntry.Crontab,
			IncludeSnapshots: config.IncludeSnapshotsFrom,
			AllowFailure:     config.AllowFailure,
			QueueName:        config.Queue,
			Group:            config.Group,
		}
		c.scheduleManager.Add(config.ScheduleEntry)
	}
	c.l.Unlock()
}

func (c *scheduleBindingsController) DisableScheduleBindings() {
	c.l.Lock()
	for _, config := range c.ScheduleBindings {
		c.scheduleManager.Remove(config.ScheduleEntry)
		delete(c.ScheduleLinks, config.ScheduleEntry.Id)
	}
	c.l.Unlock()
}
