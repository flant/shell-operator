package shell_operator

import (
	"context"
	"testing"

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

	bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), currTask)
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

	bcs := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_one_hook"), currTask)
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

	bcList := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), currTask)
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

	bcList := op.CombineBindingContextForHook(op.TaskQueues.GetByName("test_multiple_hooks"), currTask)
	g.Expect(bcList).Should(HaveLen(3))
	g.Expect(bcList[0].Metadata.GroupType).Should(Equal("Synchronization"))
	g.Expect(bcList[1].Type).Should(Equal(TypeEvent))
	g.Expect(string(bcList[2].Type)).Should(Equal(""))
}
