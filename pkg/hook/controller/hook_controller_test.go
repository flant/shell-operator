package controller

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flant/kube-client/fake"
	"github.com/flant/shell-operator/pkg/hook/binding_context"
	"github.com/flant/shell-operator/pkg/hook/config"
	"github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	types2 "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// Test updating snapshots for combined contexts.
func Test_UpdateSnapshots(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	mgr := kube_events_manager.NewKubeEventsManager()
	mgr.WithContext(context.Background())
	mgr.WithKubeClient(fc.Client)

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
	hc.InitKubernetesBindings(testCfg.OnKubernetesEvents, mgr)
	hc.EnableScheduleBindings()

	// Test case: combined binding context for binding_2 and binding_3.
	bcs := []binding_context.BindingContext{
		{
			Binding:    "binding_2",
			Type:       types2.TypeEvent,
			WatchEvent: types2.WatchEventAdded,
		},
		{
			Binding:    "binding_3",
			Type:       types2.TypeEvent,
			WatchEvent: types2.WatchEventAdded,
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
