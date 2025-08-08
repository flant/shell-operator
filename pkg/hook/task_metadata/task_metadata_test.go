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
