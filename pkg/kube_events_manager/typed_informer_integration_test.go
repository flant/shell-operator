package kubeeventsmanager

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/kube-client/fake"
	"github.com/flant/shell-operator/pkg"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/flant/shell-operator/pkg/metric"
)

// fakeClusterWithMetrics builds a fresh fake cluster with the matching default
// namespace and a metric mock that ignores every call. The integration tests
// below don't care about exact metric values; they want to exercise the
// informer pipeline.
func newIntegrationCluster(t *testing.T) *fake.Cluster {
	t.Helper()
	fc := fake.NewFakeCluster(fake.ClusterVersionV121)

	// Create the default namespace via the typed clientset so the typed
	// namespace informer (which the Monitor always uses) finds it.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	_, _ = fc.Client.CoreV1().Namespaces().Create(context.TODO(), ns, pkg.DefaultCreateOptions())
	return fc
}

func anyMetricStorage(t *testing.T) *metric.StorageMock {
	m := metric.NewStorageMock(t)
	m.HistogramObserveMock.Set(func(_ string, _ float64, _ map[string]string, _ []float64) {})
	m.GaugeSetMock.Set(func(_ string, _ float64, _ map[string]string) {})
	return m
}

// Test_Monitor_TypedInformer_StoresTypedObjectsInCache is the end-to-end check
// that the typed-informer code path is actually wired up. With the
// --use-typed-informers flag on, the SharedIndexInformer's underlying cache
// must hold typed *corev1.ConfigMap values, while the Monitor's snapshot
// continues to expose *unstructured.Unstructured for downstream consumers.
func Test_Monitor_TypedInformer_StoresTypedObjectsInCache(t *testing.T) {
	g := NewWithT(t)
	withTypedEnabled(t, true)

	fc := newIntegrationCluster(t)

	// The typed informer reads via the typed clientset, so create the
	// ConfigMap via the typed client (the fake's typed and dynamic stores
	// don't share state).
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm-typed", Namespace: "default"},
		Data:       map[string]string{"k": "v"},
	}
	_, err := fc.Client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, pkg.DefaultCreateOptions())
	g.Expect(err).ShouldNot(HaveOccurred())

	monitorCfg := &MonitorConfig{
		ApiVersion:              "v1",
		Kind:                    "ConfigMap",
		KeepFullObjectsInMemory: true, // exercise the round-trip from typed cache → unstructured snapshot
		EventTypes:              []kemtypes.WatchEventType{kemtypes.WatchEventAdded, kemtypes.WatchEventModified, kemtypes.WatchEventDeleted},
		NamespaceSelector: &kemtypes.NamespaceSelector{
			NameSelector: &kemtypes.NameSelector{MatchNames: []string{"default"}},
		},
	}

	store := NewFactoryStore()
	mon := NewMonitor(context.Background(), fc.Client, anyMetricStorage(t), store, monitorCfg, func(_ kemtypes.KubeEvent) {}, log.NewNop())

	g.Expect(mon.CreateInformers()).Should(Succeed())
	mon.Start(context.TODO())

	// Snapshot must produce *unstructured.Unstructured regardless of how the
	// cache stores them: the FactoryStore converts on read.
	g.Eventually(func() []string {
		return snapshotResourceIDs(mon.Snapshot())
	}, "5s", "10ms").Should(ContainElement("default/ConfigMap/cm-typed"))

	snap := mon.Snapshot()
	g.Expect(snap).ShouldNot(BeEmpty())
	for _, item := range snap {
		g.Expect(item.Object).ShouldNot(BeNil(),
			"snapshot must materialise *unstructured.Unstructured for downstream consumers")
		g.Expect(item.Object.GetKind()).Should(Equal("ConfigMap"),
			"the GVK must be restored on the round-tripped unstructured object")
		g.Expect(item.Object.GetAPIVersion()).Should(Equal("v1"))
		g.Expect(item.Object.GetName()).Should(Equal("cm-typed"))
		// Data should round-trip too.
		data, _, err := unstructured.NestedStringMap(item.Object.Object, "data")
		g.Expect(err).Should(BeNil())
		g.Expect(data).Should(Equal(map[string]string{"k": "v"}))
	}

	// Reach into the FactoryStore to confirm the underlying SharedIndexInformer
	// cache actually holds the typed *corev1.ConfigMap (i.e. the typed factory
	// was used, not the dynamic factory + transform).
	store.mu.Lock()
	var found bool
	for idx, f := range store.data {
		if idx.GVR != (schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}) {
			continue
		}
		found = true
		g.Expect(f.typed).Should(BeTrue(),
			"Factory for %s should be marked typed when --use-typed-informers is on", idx.GVR.String())
		raw, exists, err := f.informer.GetStore().GetByKey("default/cm-typed")
		g.Expect(err).Should(BeNil())
		g.Expect(exists).Should(BeTrue(), "informer cache must contain the freshly-created ConfigMap")
		_, ok := raw.(*corev1.ConfigMap)
		g.Expect(ok).Should(BeTrue(),
			"informer cache must store *corev1.ConfigMap, got %T", raw)
		_, isUnstructured := raw.(*unstructured.Unstructured)
		g.Expect(isUnstructured).Should(BeFalse(),
			"informer cache must NOT store *unstructured.Unstructured when typed informers are enabled")
	}
	store.mu.Unlock()
	g.Expect(found).Should(BeTrue(), "FactoryStore should have a factory for configmaps")
}

// Test_Monitor_DefaultPath_StoresUnstructured complements the typed test:
// with the typed-informer flag off (the default), the cache must still hold
// *unstructured.Unstructured exactly like before this refactor, so behaviour
// is bit-for-bit unchanged for users that don't opt in.
func Test_Monitor_DefaultPath_StoresUnstructured(t *testing.T) {
	g := NewWithT(t)
	withTypedEnabled(t, false)

	fc := newIntegrationCluster(t)

	// The dynamic informer reads via the dynamic client, so create the
	// ConfigMap there.
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	cmU := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "cm-dyn",
			"namespace": "default",
		},
		"data": map[string]any{"k": "v"},
	}}
	_, err := fc.Client.Dynamic().Resource(gvr).Namespace("default").Create(context.TODO(), cmU, pkg.DefaultCreateOptions())
	g.Expect(err).ShouldNot(HaveOccurred())

	monitorCfg := &MonitorConfig{
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		EventTypes: []kemtypes.WatchEventType{kemtypes.WatchEventAdded, kemtypes.WatchEventModified, kemtypes.WatchEventDeleted},
		NamespaceSelector: &kemtypes.NamespaceSelector{
			NameSelector: &kemtypes.NameSelector{MatchNames: []string{"default"}},
		},
	}

	store := NewFactoryStore()
	mon := NewMonitor(context.Background(), fc.Client, anyMetricStorage(t), store, monitorCfg, func(_ kemtypes.KubeEvent) {}, log.NewNop())

	g.Expect(mon.CreateInformers()).Should(Succeed())
	mon.Start(context.TODO())

	g.Eventually(func() []string {
		return snapshotResourceIDs(mon.Snapshot())
	}, "5s", "10ms").Should(ContainElement("default/ConfigMap/cm-dyn"))

	store.mu.Lock()
	for idx, f := range store.data {
		if idx.GVR != gvr {
			continue
		}
		g.Expect(f.typed).Should(BeFalse(),
			"Factory for %s must NOT be typed when the flag is off", idx.GVR.String())
		raw, exists, err := f.informer.GetStore().GetByKey("default/cm-dyn")
		g.Expect(err).Should(BeNil())
		g.Expect(exists).Should(BeTrue())
		_, ok := raw.(*unstructured.Unstructured)
		g.Expect(ok).Should(BeTrue(),
			"informer cache must store *unstructured.Unstructured by default, got %T", raw)
	}
	store.mu.Unlock()
}
