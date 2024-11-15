package controller

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"

	"github.com/flant/kube-client/fake"
	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding-context"
	"github.com/flant/shell-operator/pkg/hook/config"
	"github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// Test updating snapshots for combined contexts.
func Test_UpdateSnapshots(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	mgr := kubeeventsmanager.NewKubeEventsManager(context.Background(), fc.Client, log.NewNop())

	testHookConfig := `
configVersion: v1
kubernetes:
- name: binding_1
  apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
  includeSnapshotsFrom: ["binding_1"]
- name: binding_2
  apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
  includeSnapshotsFrom: ["binding_1", "binding_2"]
- name: binding_3
  apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
  includeSnapshotsFrom: ["binding_2", "binding_3", "binding_1"]
`
	// Use HookConfig as importing Hook lead to import cycle.
	testCfg := &config.HookConfig{}
	err := testCfg.LoadAndValidate([]byte(testHookConfig))
	g.Expect(err).ShouldNot(HaveOccurred())

	hc := NewHookController()
	hc.InitKubernetesBindings(testCfg.OnKubernetesEvents, mgr, log.NewNop())
	hc.EnableScheduleBindings()

	// Test case: combined binding context for binding_2 and binding_3.
	bcs := []bindingcontext.BindingContext{
		{
			Binding:    "binding_2",
			Type:       kemtypes.TypeEvent,
			WatchEvent: kemtypes.WatchEventAdded,
		},
		{
			Binding:    "binding_3",
			Type:       kemtypes.TypeEvent,
			WatchEvent: kemtypes.WatchEventAdded,
		},
	}
	bcs[0].Metadata.BindingType = types.OnKubernetesEvent
	bcs[1].Metadata.BindingType = types.OnKubernetesEvent

	newBcs := hc.UpdateSnapshots(bcs)

	g.Expect(newBcs).To(HaveLen(len(bcs)))

	bc := newBcs[0]
	g.Expect(bc.Snapshots).Should(HaveKey("binding_1"))
	g.Expect(bc.Snapshots).Should(HaveKey("binding_2"))

	bc = newBcs[1]
	g.Expect(bc.Snapshots).Should(HaveKey("binding_1"))
	g.Expect(bc.Snapshots).Should(HaveKey("binding_2"))
	g.Expect(bc.Snapshots).Should(HaveKey("binding_3"))
}
