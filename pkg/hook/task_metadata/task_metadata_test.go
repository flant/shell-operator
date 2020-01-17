package task_metadata

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/flant/shell-operator/pkg/task"
)

func Test_HookMetadata_Access(t *testing.T) {
	var g = NewWithT(t)

	Task := task.NewTask(HookRun).
		WithMetadata(HookMetadata{
			HookName:    "test-hook",
			BindingType: Schedule,
			BindingContext: []BindingContext{
				{Binding: "each_1_min"},
				{Binding: "each_5_min"},
			},
			AllowFailure: true,
		})

	hm := HookMetadataAccessor(Task)

	g.Expect(hm.HookName).Should(Equal("test-hook"))
	g.Expect(hm.BindingType).Should(Equal(Schedule))
	g.Expect(hm.AllowFailure).Should(BeTrue())
	g.Expect(hm.BindingContext).Should(HaveLen(2))
	g.Expect(hm.BindingContext[0].Binding).Should(Equal("each_1_min"))
	g.Expect(hm.BindingContext[1].Binding).Should(Equal("each_5_min"))

}
