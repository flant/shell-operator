package kubeeventsmanager

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/kube-client/fake"
	"github.com/flant/kube-client/manifest"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
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
		EventTypes: []kemtypes.WatchEventType{kemtypes.WatchEventAdded, kemtypes.WatchEventModified, kemtypes.WatchEventDeleted},
		NamespaceSelector: &kemtypes.NamespaceSelector{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "",
				},
			},
		},
	}
	objsFromEvents := make([]string, 0)
	var objsMutex sync.Mutex

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(metric string, value float64, labels map[string]string, buckets []float64) {
		metrics := []string{
			"{PREFIX}kube_event_duration_seconds",
			"{PREFIX}kube_jq_filter_duration_seconds",
		}
		assert.Contains(t, metrics, metric)
		assert.NotZero(t, value)
		assert.Equal(t, map[string]string(nil), labels)
		assert.Nil(t, buckets)
	})
	metricStorage.GaugeSetMock.When("{PREFIX}kube_snapshot_objects", 1, map[string]string(nil)).Then()
	metricStorage.GaugeSetMock.When("{PREFIX}kube_snapshot_objects", 2, map[string]string(nil)).Then()
	metricStorage.GaugeSetMock.When("{PREFIX}kube_snapshot_objects", 3, map[string]string(nil)).Then()

	mon := NewMonitor(context.Background(), fc.Client, metricStorage, monitorCfg, func(ev kemtypes.KubeEvent) {
		objsMutex.Lock()
		objsFromEvents = append(objsFromEvents, snapshotResourceIDs(ev.Objects)...)
		objsMutex.Unlock()
	}, log.NewNop())

	// Start monitor.
	err := mon.CreateInformers()
	g.Expect(err).ShouldNot(HaveOccurred())
	mon.Start(context.TODO())

	// Get initial snapshot.
	g.Expect(snapshotResourceIDs(mon.Snapshot())).Should(ContainElement("default/ConfigMap/default-cm-1"), "Should have only one ConfigMap on start")
	g.Expect(objsFromEvents).Should(BeEmpty(), "Should not call KubeEventCb until EnableKubeEventsCb")

	// Simulate object creation during Synchronization phase:
	// create new ns with matching labels and then create new ConfigMap.
	createNsWithLabels(fc, "test-ns-1", map[string]string{"test-label": ""})

	// Wait until informers appears.
	g.Eventually(func() bool {
		_, ok := mon.VaryingInformers.Load("test-ns-1")
		return ok
	}, "5s", "10ms").
		Should(BeTrue(), "Should create informer for new namespace")

	createCM(fc, "test-ns-1", testCM("cm-1"))

	// Should update snapshot with new objects.
	g.Eventually(func() []string {
		return snapshotResourceIDs(mon.Snapshot())
	}, "5s", "10ms").Should(ContainElement("test-ns-1/ConfigMap/cm-1"), "Should update snapshot before EnableKubeEventsCb")

	// Create more ConfigMaps to cache some events.
	createCM(fc, "test-ns-1", testCM("cm-2"))
	createCM(fc, "test-ns-1", testCM("cm-3"))

	g.Expect(objsFromEvents).Should(BeEmpty(), "Should not fire KubeEvents until EnableKubeEventsCb")

	// Enable Kube events, should update snapshots now.
	mon.EnableKubeEventCb()

	// Should catch 2 events for cm-2 and cm-3.
	g.Eventually(func() []string {
		objsMutex.Lock()
		defer objsMutex.Unlock()
		return objsFromEvents
	}, "6s", "10ms").
		Should(SatisfyAll(
			ContainElement("test-ns-1/ConfigMap/cm-2"),
			ContainElement("test-ns-1/ConfigMap/cm-3"),
		), "Should fire cached KubeEvents after enableKubeEventCb")

	g.Expect(snapshotResourceIDs(mon.Snapshot())).
		Should(SatisfyAll(
			ContainElement("test-ns-1/ConfigMap/cm-2"),
			ContainElement("test-ns-1/ConfigMap/cm-3"),
		), "Snapshot should have cm-2 and cm-3")

	// Simulate NS creation and objects creation after Synchronization phase.

	// Create new ns with labels and cm there.
	createNsWithLabels(fc, "test-ns-2", map[string]string{"test-label": ""})

	// Monitor should create new configmap informer for new namespace.
	g.Eventually(func() bool {
		_, ok := mon.VaryingInformers.Load("test-ns-2")
		return ok
	}, "5s", "10ms").
		Should(BeTrue(), "Should create informer for ns/test-ns-2")

	// Create new ConfigMap after Synchronization.
	createCM(fc, "test-ns-2", testCM("cm-2-1"))

	// Should update snapshot.
	g.Eventually(func() []string {
		return snapshotResourceIDs(mon.Snapshot())
	}, "5s", "10ms").Should(ContainElement("test-ns-2/ConfigMap/cm-2-1"), "Should update snapshot on new ConfigMap after Synchronization")

	// Should catch event for cm-2-1.
	g.Eventually(func() []string {
		objsMutex.Lock()
		defer objsMutex.Unlock()
		return objsFromEvents
	}, "5s", "10ms").
		Should(ContainElement("test-ns-2/ConfigMap/cm-2-1"), "Should fire KubeEvent for new ConfigMap after Synchronization")

	// Add non-matched Namespace.
	createNsWithLabels(fc, "test-ns-non-matched", map[string]string{"non-matched-label": ""})

	// Monitor should create new configmap informer for new namespace.
	g.Eventually(func() bool {
		_, ok := mon.VaryingInformers.Load("test-ns-non-matched")
		return ok
	}, "5s", "10ms").
		ShouldNot(BeTrue(), "Should not create informer for non-mathed Namespace")
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

func snapshotResourceIDs(snap []kemtypes.ObjectAndFilterResult) []string {
	ids := make([]string, 0)
	for _, obj := range snap {
		ids = append(ids, obj.Metadata.ResourceId)
	}
	return ids
}
