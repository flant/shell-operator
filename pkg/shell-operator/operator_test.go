package shell_operator

import (
	"context"
	"testing"

	"github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/onsi/gomega"

	"github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/task_metadata"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/task"
	"github.com/flant/shell-operator/pkg/task/queue"
)

func Test_CombineBindingContext_MultipleHooks(t *testing.T) {
	g := NewWithT(t)

	op := NewShellOperator()
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue("test_multiple_hooks", func(tsk task.Task) queue.TaskResult {
		return queue.TaskResult{
			Status: "Success",
		}
	})

	var tasks = []task.Task{
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "schedule",
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook2.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
	}

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_multiple_hooks").AddLast(tsk)
	}
	g.Expect(op.TaskQueues.GetByName("test_multiple_hooks").Length()).Should(Equal(len(tasks)))

	bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), tasks[0], nil)

	// Should combine binding contexts from 4 tasks.
	g.Expect(bcs).Should(HaveLen(4))
	// Should delete 3 tasks
	g.Expect(op.TaskQueues.GetByName("test_multiple_hooks").Length()).Should(Equal(len(tasks) - 3))
}

func Test_CombineBindingContext_Nil_On_NoCombine(t *testing.T) {
	g := NewWithT(t)

	op := NewShellOperator()
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue("test_no_combine", func(tsk task.Task) queue.TaskResult {
		return queue.TaskResult{
			Status: "Success",
		}
	})

	var tasks = []task.Task{
		task.NewTask(HookRun).
			WithQueueName("test_no_combine").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_no_combine").
			WithMetadata(HookMetadata{
				HookName: "hook2.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_no_combine").
			WithMetadata(HookMetadata{
				HookName: "hook3.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "schedule",
					},
				},
			}),
	}

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_no_combine").AddLast(tsk)
	}
	g.Expect(op.TaskQueues.GetByName("test_no_combine").Length()).Should(Equal(len(tasks)))

	bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_no_combine"), tasks[0], nil)
	// Should return nil if no combine
	g.Expect(bcs).Should(BeNil())
	// Should not delete tasks
	g.Expect(op.TaskQueues.GetByName("test_no_combine").Length()).Should(Equal(len(tasks)))
}

func Test_CombineBindingContext_Group_Compaction(t *testing.T) {
	g := NewWithT(t)

	op := NewShellOperator()
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue("test_multiple_hooks", func(tsk task.Task) queue.TaskResult {
		return queue.TaskResult{
			Status: "Success",
		}
	})

	bcMeta := hook.BindingContext{}.Metadata
	bcMeta.Group = "pods"

	var tasks = []task.Task{
		// 3 tasks with Group should be compacted
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "kubernetes",
						Type:     TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "kubernetes",
						Type:     TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "schedule",
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		// Should not combine with next tasks (different hook name)
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook2.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
	}

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_multiple_hooks").AddLast(tsk)
	}
	g.Expect(op.TaskQueues.GetByName("test_multiple_hooks").Length()).Should(Equal(len(tasks)))

	bcList := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), tasks[0], nil)
	// Should compact 4 tasks into 4 binding context and combine 3 binding contexts into one.
	g.Expect(bcList).Should(HaveLen(2))

	// Should delete 3 tasks
	g.Expect(op.TaskQueues.GetByName("test_multiple_hooks").Length()).Should(Equal(len(tasks) - 3))
}

func Test_CombineBindingContext_Group_Type(t *testing.T) {
	g := NewWithT(t)

	op := NewShellOperator()
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue("test_multiple_hooks", func(tsk task.Task) queue.TaskResult {
		return queue.TaskResult{
			Status: "Success",
		}
	})

	bcMeta := hook.BindingContext{}.Metadata
	bcMeta.Group = "pods"
	bcMeta.BindingType = types.OnKubernetesEvent

	schMeta := hook.BindingContext{}.Metadata
	schMeta.Group = "pods"
	schMeta.BindingType = types.Schedule

	var tasks = []task.Task{
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "kubernetes",
						Type:     TypeSynchronization,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "kubernetes2",
						Type:     TypeSynchronization,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "schedule",
					},
				},
			}),
		// stop compaction for group

		// bcList[1] type == TypeEvent
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		// bcList[2] type == Group
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: schMeta,
						Binding:  "schedule",
					},
				},
			}),
		// bcList[3] type == Schedule
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: func() hook.BindingContext {
							bc := hook.BindingContext{}
							bc.Metadata.BindingType = types.Schedule
							return bc
						}().Metadata,
						Binding: "schedule",
					},
				},
			}),
		// bcList[4] type == Group
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Metadata: bcMeta,
						Binding:  "kubernetes",
						Type:     TypeEvent,
					},
				},
			}),
		// Should not combine with task that has different type
		task.NewTask(EnableScheduleBindings).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
		task.NewTask(HookRun).
			WithQueueName("test_multiple_hooks").
			WithMetadata(HookMetadata{
				HookName: "hook1.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "kubernetes",
						Type:    TypeEvent,
					},
				},
			}),
	}

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_multiple_hooks").AddLast(tsk)
	}
	g.Expect(op.TaskQueues.GetByName("test_multiple_hooks").Length()).Should(Equal(len(tasks)))

	bcList := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), tasks[0], nil)
	// Should leave 3 tasks in queue.
	g.Expect(op.TaskQueues.GetByName("test_multiple_hooks").Length()).Should(Equal(3))

	// Should combine binding contexts from 7 tasks. Should compact 3 binding contexts into 1.
	g.Expect(bcList).Should(HaveLen(5))

	g.Expect(string(bcList[0].Type)).Should(Equal(""), "bc: %+v", bcList[0])
	g.Expect(bcList[0].Metadata.Group).Should(Equal("pods"), "bc: %+v", bcList[0])

	g.Expect(bcList[1].Type).Should(Equal(TypeEvent))

	g.Expect(string(bcList[2].Type)).Should(Equal(""))
	g.Expect(bcList[2].Metadata.Group).Should(Equal("pods"), "bc: %+v", bcList[2])

	g.Expect(string(bcList[3].Type)).Should(Equal(""))

	g.Expect(bcList[4].Type).Should(Equal(TypeEvent))
	g.Expect(bcList[4].Metadata.Group).Should(Equal("pods"), "bc: %+v", bcList[4])
}
