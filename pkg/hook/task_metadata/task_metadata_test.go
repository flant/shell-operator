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

	hm, ok := HookMetadataAccessor(Task)

	g.Expect(ok).Should(BeTrue())

	g.Expect(hm.HookName).Should(Equal("test-hook"))
	g.Expect(hm.BindingType).Should(Equal(htypes.Schedule))
	g.Expect(hm.AllowFailure).Should(BeTrue())
	g.Expect(hm.BindingContext).Should(HaveLen(2))
	g.Expect(hm.BindingContext[0].Binding).Should(Equal("each_1_min"))
	g.Expect(hm.BindingContext[1].Binding).Should(Equal("each_5_min"))
}

func Test_HookMetadataAccessor_NilMetadata(t *testing.T) {
	g := NewWithT(t)

	// A task with nil metadata should return ok=false and a zero HookMetadata.
	nilMetaTask := task.NewTask(HookRun) // no WithMetadata call

	hm, ok := HookMetadataAccessor(nilMetaTask)

	g.Expect(ok).Should(BeFalse())
	g.Expect(hm).Should(Equal(HookMetadata{}))
}

func Test_HookMetadataAccessor_WrongMetadataType(t *testing.T) {
	g := NewWithT(t)

	// A task with metadata of the wrong type should return ok=false.
	type otherMeta struct{ Value string }
	wrongTypeTask := task.NewTask(HookRun).WithMetadata(otherMeta{Value: "unexpected"})

	hm, ok := HookMetadataAccessor(wrongTypeTask)

	g.Expect(ok).Should(BeFalse())
	g.Expect(hm).Should(Equal(HookMetadata{}))
}
