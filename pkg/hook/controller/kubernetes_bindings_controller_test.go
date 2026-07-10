package controller

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"

	"github.com/flant/kube-client/fake"
	pkg "github.com/flant/shell-operator/pkg"
	"github.com/flant/shell-operator/pkg/hook/config"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
)

const twoBindingsHookConfig = `
configVersion: v1
kubernetes:
- name: binding_1
  apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
- name: binding_2
  apiVersion: v1
  kind: Pod
  executeHookOnEvent: ["Added"]
`

func newTestKubeController(t *testing.T) (*HookController, *config.HookConfig, kubeeventsmanager.KubeEventsManager) {
	t.Helper()
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	mgr := kubeeventsmanager.NewKubeEventsManager(context.Background(), fc.Client, log.NewNop())

	cfg := &config.HookConfig{}
	g.Expect(cfg.LoadAndValidate([]byte(twoBindingsHookConfig))).ShouldNot(HaveOccurred())

	hc := NewHookController()
	hc.InitKubernetesBindings(cfg.OnKubernetesEvents, mgr, log.NewNop())

	return hc, cfg, mgr
}

// TestEnableKubernetesBindings_RepeatCallStillEmitsSynchronization is the direct
// regression test for the incident: on the second call the old code hit the
// alreadyEnabled early-return and yielded ZERO Synchronization contexts, so a
// ModuleRun retry lost the values-populating Synchronization run for the module's
// hooks. The fix must return a Synchronization context for every binding on every
// call.
func TestEnableKubernetesBindings_RepeatCallStillEmitsSynchronization(t *testing.T) {
	g := NewWithT(t)
	hc, cfg, _ := newTestKubeController(t)
	want := len(cfg.OnKubernetesEvents)

	first, err := hc.KubernetesController.EnableKubernetesBindings()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(first).To(HaveLen(want), "first call must emit a Synchronization context per binding")

	second, err := hc.KubernetesController.EnableKubernetesBindings()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(second).To(HaveLen(want), "repeat call must still emit Synchronization contexts (regression: was 0)")
}

// TestEnableKubernetesBindings_SelfHealsMissingMonitor reproduces the permanent
// "link present, monitor gone" state left by a Disable/Enable race and asserts
// that a subsequent EnableKubernetesBindings repairs it — restoring the
// self-healing behavior that existed before the idempotent early-return was added.
func TestEnableKubernetesBindings_SelfHealsMissingMonitor(t *testing.T) {
	g := NewWithT(t)
	hc, cfg, mgr := newTestKubeController(t)

	_, err := hc.KubernetesController.EnableKubernetesBindings()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Drop a monitor out of band while its binding link stays — this is the state
	// DisableKubernetesBindings racing with the queue worker leaves behind.
	victim := cfg.OnKubernetesEvents[0].Monitor.Metadata.MonitorId
	g.Expect(mgr.HasMonitor(victim)).To(BeTrue())
	g.Expect(mgr.StopMonitor(victim)).ShouldNot(HaveOccurred())
	g.Expect(mgr.HasMonitor(victim)).To(BeFalse())

	// Snapshot for the orphaned binding is empty here — the silent-failure state.
	g.Expect(hc.KubernetesController.SnapshotsFor(cfg.OnKubernetesEvents[0].BindingName)).To(BeNil())

	// Next enable must re-create the monitor rather than early-return.
	_, err = hc.KubernetesController.EnableKubernetesBindings()
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(mgr.HasMonitor(victim)).To(BeTrue(), "missing monitor must be repaired on next EnableKubernetesBindings")
}

// TestSnapshotsFor_MissingMonitorIsCounted ensures the empty-snapshot state is
// observable: reading a snapshot for a configured binding whose monitor is not
// running must increment binding_monitor_missing_total with the binding label.
func TestSnapshotsFor_MissingMonitorIsCounted(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	mgr := kubeeventsmanager.NewKubeEventsManager(context.Background(), fc.Client, log.NewNop())

	var (
		calls     int
		gotMetric string
		gotValue  float64
		gotLabels map[string]string
	)
	storage := metric.NewStorageMock(t)
	storage.CounterAddMock.Set(func(name string, value float64, labels map[string]string) {
		calls++
		gotMetric, gotValue, gotLabels = name, value, labels
	})
	// The storage must be set before InitKubernetesBindings: the controller
	// captures it from the manager at init time.
	mgr.WithMetricStorage(storage)

	cfg := &config.HookConfig{}
	g.Expect(cfg.LoadAndValidate([]byte(twoBindingsHookConfig))).ShouldNot(HaveOccurred())

	hc := NewHookController()
	hc.InitKubernetesBindings(cfg.OnKubernetesEvents, mgr, log.NewNop())

	// Bindings are configured but never enabled: no monitor is running.
	binding := cfg.OnKubernetesEvents[0].BindingName
	g.Expect(hc.KubernetesController.SnapshotsFor(binding)).To(BeNil())

	g.Expect(calls).To(Equal(1))
	g.Expect(gotMetric).To(Equal(metrics.BindingMonitorMissingTotal))
	g.Expect(gotValue).To(Equal(1.0))
	g.Expect(gotLabels).To(Equal(map[string]string{pkg.MetricKeyBinding: binding}))
}
