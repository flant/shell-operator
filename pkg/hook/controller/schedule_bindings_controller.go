package controller

import (
	log "github.com/sirupsen/logrus"

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
	logEntry        *log.Entry
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
		}
		c.scheduleManager.Add(config.ScheduleEntry)
	}
}

func (c *scheduleBindingsController) DisableScheduleBindings() {
	for _, config := range c.ScheduleBindings {
		c.scheduleManager.Remove(config.ScheduleEntry)
		delete(c.ScheduleLinks, config.ScheduleEntry.Id)
	}
	return
}

/*
// UpdateScheduledHooks recreates a new Hooks array
func (c *scheduleHooksController) UpdateScheduleHooks() {
	logEntry := log.WithField("operator.component", "scheduleHooksController")

	newStorage := make(ScheduleHooksStorage, 0)
	hooks, err := c.hookManager.GetHooksInOrder(Schedule)
	if err != nil {
		logEntry.Errorf("%v", err)
		return
	}

	for _, hookName := range hooks {
		hmHook := c.hookManager.GetHook(hookName)
		for _, schedule := range hmHook.Config.Schedules {
			err := c.scheduleManager.Add(schedule.ScheduleEntry)
			if err != nil {
				// TODO add crontab string validation and this error can be ignored
				logEntry.Errorf("Schedule: cannot add '%s' for hook '%s': %s", schedule.Crontab, hookName, err)
				continue
			}
			logEntry.Debugf("Schedule: add '%s' for hook '%s'", schedule.Crontab, hookName)
			newStorage.AddHook(hmHook.Name, schedule)
		}
	}

	// Calculate obsolete crontab strings
	oldCrontabs := map[string]bool{}
	for _, crontab := range c.Hooks.GetCrontabs() {
		oldCrontabs[crontab] = false
	}
	for _, crontab := range newStorage.GetCrontabs() {
		oldCrontabs[crontab] = true
	}

	// Stop crontabs that is not in new storage.
	for crontab, isActive := range oldCrontabs {
		if !isActive {
			c.scheduleManager.Remove(crontab)
		}
	}

	c.Hooks = newStorage
}

func (c *scheduleHooksController) HandleEvent(crontab string) ([]task.Task, error) {
	res := make([]task.Task, 0)

	scheduleHooks := c.Hooks.GetHooksForSchedule(crontab)
	if len(scheduleHooks) == 0 {
		return nil, fmt.Errorf("Unknown schedule event: no such crontab '%s' is registered", crontab)
	}

	for _, schHook := range scheduleHooks {
		_, err := c.hookManager.GetHook(schHook.HookName)
		if err != nil {
			// This should not happen
			log.WithField("operator.component", "scheduleHooksController").
				Errorf("Possible bug: hook '%s' is scheduled as '%s' but not found by hook_manager", schHook.HookName, crontab)
			continue
		}
		newTask := task.NewTask(task.HookRun, schHook.HookName).
			WithBinding(hook.Schedule).
			AppendBindingContext(hook.BindingContext{Binding: schHook.ConfigName}).
			WithAllowFailure(schHook.AllowFailure)
		res = append(res, newTask)
	}

	return res, nil
}
*/
