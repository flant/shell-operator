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

	var tasks = make([]task.Task, 0)
	currTask := task.NewTask(HookRun).
		WithQueueName("test_multiple_hooks").
		WithMetadata(HookMetadata{
			HookName: "hook1.sh",
			BindingContext: []hook.BindingContext{
				{
					Binding: "kubernetes",
					Type:    TypeEvent,
				},
			},
		})

	tasks = append(tasks, currTask)

	tasks = append(tasks, []task.Task{
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
	}...)

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_multiple_hooks").AddLast(tsk)
	}

	bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), currTask, nil)
	g.Expect(bcs).Should(HaveLen(4))
}

func Test_CombineBindingContext_OneHook(t *testing.T) {
	g := NewWithT(t)

	op := NewShellOperator()
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue("test_one_hook", func(tsk task.Task) queue.TaskResult {
		return queue.TaskResult{
			Status: "Success",
		}
	})

	var tasks = make([]task.Task, 0)
	currTask := task.NewTask(HookRun).
		WithQueueName("test_one_hook").
		WithMetadata(HookMetadata{
			HookName: "hook1.sh",
			BindingContext: []hook.BindingContext{
				{
					Binding: "kubernetes",
					Type:    TypeEvent,
				},
			},
		})

	tasks = append(tasks, currTask)

	tasks = append(tasks, []task.Task{
		task.NewTask(HookRun).
			WithQueueName("test_one_hook").
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
			WithQueueName("test_one_hook").
			WithMetadata(HookMetadata{
				HookName: "hook3.sh",
				BindingContext: []hook.BindingContext{
					{
						Binding: "schedule",
					},
				},
			}),
	}...)

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_one_hook").AddLast(tsk)
	}

	bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_one_hook"), currTask, nil)
	g.Expect(bcs).Should(BeNil())
}

func Test_CombineBindingContext_Group(t *testing.T) {
	g := NewWithT(t)

	op := NewShellOperator()
	op.TaskQueues = queue.NewTaskQueueSet()
	op.TaskQueues.WithContext(context.Background())
	op.TaskQueues.NewNamedQueue("test_multiple_hooks", func(tsk task.Task) queue.TaskResult {
		return queue.TaskResult{
			Status: "Success",
		}
	})

	var tasks = make([]task.Task, 0)

	bcMeta := hook.BindingContext{}.Metadata
	bcMeta.Group = "pods"

	currTask := task.NewTask(HookRun).
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
		})

	tasks = append(tasks, currTask)

	tasks = append(tasks, []task.Task{
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
	}...)

	//hm.BindingContext[0].Metadata.Group = "pods"

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_multiple_hooks").AddLast(tsk)
	}

	bcList := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), currTask, nil)
	g.Expect(bcList).Should(HaveLen(2))
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

	var tasks = make([]task.Task, 0)

	bcMeta := hook.BindingContext{}.Metadata
	bcMeta.Group = "pods"
	bcMeta.BindingType = types.OnKubernetesEvent

	schMeta := hook.BindingContext{}.Metadata
	schMeta.Group = "pods"
	schMeta.BindingType = types.Schedule

	currTask := task.NewTask(HookRun).
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
		})

	tasks = append(tasks, currTask)

	tasks = append(tasks, []task.Task{
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
						Binding:  "schedule",
					},
				},
			}),
		// stop grouping

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
	}...)

	for _, tsk := range tasks {
		op.TaskQueues.GetByName("test_multiple_hooks").AddLast(tsk)
	}

	bcList := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), currTask, nil)

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
