package schedule

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/hook"
	"github.com/flant/shell-operator/pkg/schedule_manager"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/romana/rlog"
)

type ScheduleHooksController interface {
	WithHookManager(hook.HookManager)
	WithScheduleManager(schedule_manager.ScheduleManager)

	UpdateScheduleHooks()
	HandleEvent(crontab string) ([]task.Task, error)
}

type scheduleHooksController struct {
	Hooks scheduleHooksStorage

	// dependencies
	hookManager     hook.HookManager
	scheduleManager schedule_manager.ScheduleManager
}

var NewScheduleHooksController = func() ScheduleHooksController {
	return &scheduleHooksController{
		Hooks: make(scheduleHooksStorage, 0),
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
	// map of crontabs that should be stopped in scheduleManager
	oldCrontabs := map[string]bool{}
	for _, crontab := range c.Hooks.GetCrontabs() {
		oldCrontabs[crontab] = false
	}

	newStorage := make(scheduleHooksStorage, 0)

	hooks := c.hookManager.GetHooksInOrder(hook.Schedule)

	for _, hookName := range hooks {
		hmHook, _ := c.hookManager.GetHook(hookName)
		for _, schedule := range hmHook.Config.Schedules {
			_, err := c.scheduleManager.Add(schedule.Crontab)
			if err != nil {
				rlog.Errorf("Schedule: cannot add '%s' for hook '%s': %s", schedule.Crontab, hookName, err)
				continue
			}
			rlog.Debugf("Schedule: add '%s' for hook '%s'", schedule.Crontab, hookName)
			newStorage.AddHook(hmHook.Name, schedule)
		}
	}

	if len(oldCrontabs) > 0 {
		// Creates a new set of schedules. If the schedule is in oldCrontabs, then set it to true.
		newCrontabs := newStorage.GetCrontabs()
		for _, crontab := range newCrontabs {
			if _, has_crontab := oldCrontabs[crontab]; has_crontab {
				oldCrontabs[crontab] = true
			}
		}

		// Stop crontabs that was not added to new storage.
		for crontab, _ := range oldCrontabs {
			if !oldCrontabs[crontab] {
				c.scheduleManager.Remove(crontab)
			}
		}
	}
}

func (c *scheduleHooksController) HandleEvent(crontab string) ([]task.Task, error) {
	res := make([]task.Task, 0)

	scheduleHooks := c.Hooks.GetHooksForSchedule(crontab)
	if len(scheduleHooks) == 0 {
		return nil, fmt.Errorf("Unknown schedule event: no such crontab '%s' is registered", crontab)
	}

	for _, schHook := range scheduleHooks {
		_, err := c.hookManager.GetHook(schHook.HookName)
		if err == nil {
			newTask := task.NewTask(task.HookRun, schHook.HookName).
				WithBinding(hook.Schedule).
				AppendBindingContext(hook.BindingContext{Binding: schHook.ConfigName}).
				WithAllowFailure(schHook.AllowFailure)
			res = append(res, newTask)
		}

		rlog.Errorf("MAIN_LOOP hook '%s' scheduled but not found by hook_manager", schHook.HookName)
	}

	return res, nil
}
