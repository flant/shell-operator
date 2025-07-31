package task_metadata

import (
	"testing"

	. "github.com/onsi/gomega"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	htypes "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/task"
)

func Test_HookMetadata_Access(t *testing.T) {
	g := NewWithT(t)

	Task := task.NewTask(HookRun).
		WithMetadata(HookMetadata{
			HookName:    "test-hook",
			BindingType: htypes.Schedule,
			BindingContext: []bctx.BindingContext{
				{Binding: "each_1_min"},
				{Binding: "each_5_min"},
			},
			AllowFailure: true,
		})

	hm := HookMetadataAccessor(Task)

	g.Expect(hm.HookName).Should(Equal("test-hook"))
	g.Expect(hm.BindingType).Should(Equal(htypes.Schedule))
	g.Expect(hm.AllowFailure).Should(BeTrue())
	g.Expect(hm.BindingContext).Should(HaveLen(2))
	g.Expect(hm.BindingContext[0].Binding).Should(Equal("each_1_min"))
	g.Expect(hm.BindingContext[1].Binding).Should(Equal("each_5_min"))
}

func Test_HookMetadata_GetDescription(t *testing.T) {
	g := NewWithT(t)

	hm1 := HookMetadata{
		HookName:    "hook1.sh",
		BindingType: htypes.OnKubernetesEvent,
		Binding:     "monitor_pods",
	}
	g.Expect(hm1.GetDescription()).Should(Equal(":kubernetes:hook1.sh:monitor_pods"))

	hm2 := HookMetadata{
		HookName:    "hook1.sh",
		BindingType: htypes.Schedule,
		Binding:     "every 1 sec",
		Group:       "monitor_pods",
	}
	g.Expect(hm2.GetDescription()).Should(Equal(":schedule:hook1.sh:group=monitor_pods:every 1 sec"))

	hm3 := HookMetadata{
		HookName:    "hook1.sh",
		BindingType: htypes.OnStartup,
	}
	g.Expect(hm3.GetDescription()).Should(Equal(":onstartup:hook1.sh"))

	hm4 := HookMetadata{
		HookName:    "hook1.sh",
		BindingType: htypes.Schedule,
		Group:       "monitor_pods",
	}
	g.Expect(hm4.GetDescription()).Should(Equal(":schedule:hook1.sh:group=monitor_pods"))

	hm5 := HookMetadata{
		HookName:    "hook1.sh",
		BindingType: htypes.Schedule,
		Binding:     "every 1 sec",
	}
	g.Expect(hm5.GetDescription()).Should(Equal(":schedule:hook1.sh:every 1 sec"))
}

func Test_HookMetadata_IsSynchronization(t *testing.T) {
	g := NewWithT(t)

	hm1 := HookMetadata{
		HookName: "hook1.sh",
		BindingContext: []bctx.BindingContext{
			{
				Binding: "monitor_pods",
				Type:    "Synchronization",
			},
		},
	}
	g.Expect(hm1.IsSynchronization()).Should(BeTrue())

	hm2 := HookMetadata{
		HookName: "hook1.sh",
		BindingContext: []bctx.BindingContext{
			{
				Binding: "monitor_pods",
				Type:    "Event",
			},
		},
	}
	g.Expect(hm2.IsSynchronization()).Should(BeFalse())

	hm3 := HookMetadata{
		HookName: "hook1.sh",
	}
	g.Expect(hm3.IsSynchronization()).Should(BeFalse())
}
