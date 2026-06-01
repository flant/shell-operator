package kubeeventsmanager

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flant/kube-client/fake"
	"github.com/flant/shell-operator/pkg/kube/dedupclient"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
	"github.com/flant/shell-operator/pkg/metrics"
)

// Test_Monitor_SnapshotStore_BackingPath exercises the dedup-store path
// end-to-end on a fake cluster: the resourceInformer must
//   - upsert the body into the shared SnapshotStore on initial list,
//   - keep its in-memory `*ObjectAndFilterResult` entry with Object == nil,
//   - reconstitute Object lazily when Snapshot() is called,
//   - count the object in the store's live-objects gauge.
//
// This is the critical regression test: without the dedup wiring, Snapshot()
// would still work (the object stays in cachedObjects), but the store would
// stay empty — which is exactly the "memory didn't drop" failure mode that
// motivated this feature.
func Test_Monitor_SnapshotStore_BackingPath(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	createNsWithLabels(fc, "default", nil)
	createCM(fc, "default", testCM("dedup-cm"))

	monitorCfg := &MonitorConfig{
		ApiVersion:              "v1",
		Kind:                    "ConfigMap",
		EventTypes:              []kemtypes.WatchEventType{kemtypes.WatchEventAdded, kemtypes.WatchEventModified, kemtypes.WatchEventDeleted},
		KeepFullObjectsInMemory: true,
		NamespaceSelector: &kemtypes.NamespaceSelector{
			NameSelector: &kemtypes.NameSelector{MatchNames: []string{"default"}},
		},
	}

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(_ string, _ float64, _ map[string]string, _ []float64) {})
	metricStorage.GaugeSetMock.When(metrics.KubeSnapshotObjects, 1, map[string]string(nil)).Then()

	snapshotStore := dedupclient.NewSnapshotStore(log.NewNop())

	mon := NewMonitor(context.Background(), testDedupClientFromLegacy(fc.Client), metricStorage, NewFactoryStore(), monitorCfg, func(_ kemtypes.KubeEvent) {}, log.NewNop())
	mon.WithSnapshotStore(snapshotStore)

	require.NoError(t, mon.CreateInformers())
	mon.Start(context.TODO())

	// After initial list completes, the snapshot store must own exactly one
	// live object — the ConfigMap.
	g.Eventually(func() int {
		return snapshotStore.Stats().LiveObjects
	}, "3s", "10ms").
		Should(Equal(1), "snapshot store must observe the initial-list upsert")

	// Snapshot() must return Object populated (reconstituted from the
	// dedup store), but the underlying cachedObjects entry must not hold
	// any `*Unstructured` pointer of its own.
	snap := mon.Snapshot()
	require.Len(t, snap, 1)
	assert.NotNil(t, snap[0].Object, "Snapshot must reconstitute Object from the dedup store")
	assert.Equal(t, "dedup-cm", snap[0].Object.GetName())
	assert.Equal(t, "default", snap[0].Object.GetNamespace())

	for _, ri := range mon.ResourceInformers {
		ri.cacheLock.RLock()
		for _, entry := range ri.cachedObjects {
			assert.Nil(t, entry.Object,
				"cachedObjects entries must not hold `*Unstructured` when SnapshotStore is active")
		}
		assert.NotEmpty(t, ri.cachedObjectKeys, "cachedObjectKeys must mirror cachedObjects")
		ri.cacheLock.RUnlock()
	}
}

// Test_Monitor_SnapshotStore_DefaultPath confirms backwards compatibility:
// when no SnapshotStore is wired in, the original behaviour is preserved —
// the per-monitor cache holds `*Unstructured` pointers itself and the
// Snapshot() output is identical in shape.
func Test_Monitor_SnapshotStore_DefaultPath(t *testing.T) {
	g := NewWithT(t)
	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	createNsWithLabels(fc, "default", nil)
	createCM(fc, "default", testCM("plain-cm"))

	monitorCfg := &MonitorConfig{
		ApiVersion:              "v1",
		Kind:                    "ConfigMap",
		EventTypes:              []kemtypes.WatchEventType{kemtypes.WatchEventAdded},
		KeepFullObjectsInMemory: true,
		NamespaceSelector: &kemtypes.NamespaceSelector{
			NameSelector: &kemtypes.NameSelector{MatchNames: []string{"default"}},
		},
	}

	metricStorage := metric.NewStorageMock(t)
	metricStorage.HistogramObserveMock.Set(func(_ string, _ float64, _ map[string]string, _ []float64) {})
	metricStorage.GaugeSetMock.When(metrics.KubeSnapshotObjects, 1, map[string]string(nil)).Then()

	mon := NewMonitor(context.Background(), testDedupClientFromLegacy(fc.Client), metricStorage, NewFactoryStore(), monitorCfg, func(_ kemtypes.KubeEvent) {}, log.NewNop())
	require.NoError(t, mon.CreateInformers())
	mon.Start(context.TODO())

	g.Eventually(func() []string {
		return snapshotResourceIDs(mon.Snapshot())
	}, "3s", "10ms").Should(ContainElement("default/ConfigMap/plain-cm"))

	for _, ri := range mon.ResourceInformers {
		ri.cacheLock.RLock()
		assert.Empty(t, ri.cachedObjectKeys,
			"cachedObjectKeys must remain empty when SnapshotStore is not configured")
		for _, entry := range ri.cachedObjects {
			assert.NotNil(t, entry.Object,
				"cachedObjects entries must keep their `*Unstructured` pointer in the legacy path")
		}
		ri.cacheLock.RUnlock()
	}
}
