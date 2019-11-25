package schedule

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	log "github.com/sirupsen/logrus"
)

type ScheduleHooksController interface {
	WithHookManager(hook.HookManager)
	WithScheduleManager(schedule_manager.ScheduleManager)

	UpdateScheduleHooks()
	HandleEvent(crontab string) ([]task.Task, error)
}

type scheduleHooksController struct {
	Hooks ScheduleHooksStorage

	// dependencies
	hookManager     hook.HookManager
	scheduleManager schedule_manager.ScheduleManager
}

var NewScheduleHooksController = func() ScheduleHooksController {
	return &scheduleHooksController{
		Hooks: make(ScheduleHooksStorage, 0),
	}
}

func (c *scheduleHooksController) WithHookManager(hookManager hook.HookManager) {
	c.hookManager = hookManager
}

func (c *scheduleHooksController) WithScheduleManager(scheduleManager schedule_manager.ScheduleManager) {
	c.scheduleManager = scheduleManager
}

// UpdateScheduledHooks recreates a new Hooks array
func (c *scheduleHooksController) UpdateScheduleHooks() {
	logEntry := log.WithField("operator.component", "scheduleHooksController")

	newStorage := make(ScheduleHooksStorage, 0)
	hooks, err := c.hookManager.GetHooksInOrder(hook.Schedule)
	if err != nil {
		logEntry.Errorf("%v", err)
		return
	}

	for _, hookName := range hooks {
		hmHook, _ := c.hookManager.GetHook(hookName)
		for _, schedule := range hmHook.Config.Schedules {
			_, err := c.scheduleManager.Add(schedule.Crontab)
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
