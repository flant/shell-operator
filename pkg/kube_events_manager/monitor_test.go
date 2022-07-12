package kube_events_manager

import (
	"context"
	"fmt"
	"testing"

	"github.com/flant/kube-client/fake"
	"github.com/flant/kube-client/manifest"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

func Test_Monitor_should_handle_dynamic_ns_events(t *testing.T) {
	g := NewWithT(t)
	fc := fake.NewFakeCluster(fake.ClusterVersionV121)

	// Initial namespace with ConfigMap.
	createNsWithLabels(fc, "default", map[string]string{"test-label": ""})
	createCM(fc, "default", testCM("default-cm-1"))

	monitorCfg := &MonitorConfig{
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		EventTypes: []WatchEventType{WatchEventAdded, WatchEventModified, WatchEventDeleted},
		NamespaceSelector: &NamespaceSelector{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "",
				},
			},
		},
	}

	mon := NewMonitor()
	mon.WithContext(context.TODO())
	mon.WithKubeClient(fc.Client)
	mon.WithConfig(monitorCfg)

	// Catch KubeEvents from monitor.
	type kubeEvent struct {
		Type  KubeEventType
		Count int
	}
	events := make([]kubeEvent, 0)
	mon.WithKubeEventCb(func(ev KubeEvent) {
		events = append(events, kubeEvent{
			Type:  ev.Type,
			Count: len(ev.Objects),
		})
	})

	// Start monitor.
	err := mon.CreateInformers()
	g.Expect(err).ShouldNot(HaveOccurred())
	mon.Start(context.TODO())

	// Get initial snapshot.
	snap := mon.Snapshot()
	g.Expect(snap).Should(HaveLen(1), "Should have only one ConfigMap on start")
	g.Expect(events).Should(HaveLen(0), "Should not call KubeEventCb until EnableKubeEventsCb")

	// Simulate object creation during Synchronization phase:
	// create new ns with matching labels and then create new ConfigMap.
	createNsWithLabels(fc, "test-ns-1", map[string]string{"test-label": ""})

	// Wait until informer appears.
	g.Eventually(func() bool {
		monImpl := mon.(*monitor)
		return len(monImpl.VaryingInformers) == 2
	}, "5s", "10ms").Should(BeTrue(), "Should create informer for new Namespace")

	createCM(fc, "test-ns-1", testCM("cm-1"))

	// Should update snapshot. Snapshot resets cached events.
	g.Eventually(func() bool {
		snap := mon.Snapshot()
		return len(snap) == 2
	}, "5s", "10ms").Should(BeTrue(), "Should not update snapshot until EnableKubeEventsCb")

	// Create more ConfigMaps to cache some events.
	createCM(fc, "test-ns-1", testCM("cm-2"))
	createCM(fc, "test-ns-1", testCM("cm-3"))

	g.Expect(events).Should(HaveLen(0), "Should not fire KubeEvents until EnableKubeEventsCb")

	// Enable Kube events, should update snapshots now.
	mon.EnableKubeEventCb()

	// Should catch 2 events for cm-2 and cm-3.
	g.Eventually(func() bool {
		return len(events) == 2
	}, "6s", "10ms").Should(BeTrue(), "Should fire cached KubeEvents after EnableKubeEventCb")

	// Now simulate NS creation and objects creation after Synchronization phase.

	// Create new ns with labels and cm there.
	createNsWithLabels(fc, "test-ns-2", map[string]string{"test-label": ""})

	// Wait until informer for new NS appears.
	g.Eventually(func() bool {
		monImpl := mon.(*monitor)
		return len(monImpl.VaryingInformers) == 3
	}, "5s", "10ms").Should(BeTrue(), "Should create informer for ns/test-ns-2")

	createCM(fc, "test-ns-2", testCM("cm-2-1"))

	// Should update snapshot.
	g.Eventually(func() bool {
		return len(mon.Snapshot()) == 4
	}, "5s", "10ms").Should(BeTrue(), "Should update snapshot after Synchronization")

	// Should catch event for cm/cm-2-1.
	g.Eventually(func() bool {
		return len(events) == 3
	}, "5s", "10ms").Should(BeTrue(), "Should fire KubeEvent after Synchronization")
}

func createNsWithLabels(fc *fake.Cluster, name string, labels map[string]string) {
	nsObj := &corev1.Namespace{}
	nsObj.SetName(name)
	nsObj.SetLabels(labels)
	_, _ = fc.Client.CoreV1().Namespaces().Create(context.TODO(), nsObj, metav1.CreateOptions{})
}

func createCM(fc *fake.Cluster, ns string, cmYAML string) {
	mft := manifest.MustFromYAML(cmYAML)
	_ = fc.Create(ns, mft)
}

func testCM(name string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: "%s"
data:
  foo: "bar"
`, name)
}
