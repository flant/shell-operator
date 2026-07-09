package kubeeventsmanager

import (
	"context"
	"sync"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/kube-client/fake"
)

var configMapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

// recordingHandler is a minimal cache.ResourceEventHandler that records names of
// added objects.
type recordingHandler struct {
	mu    sync.Mutex
	added []string
}

func (h *recordingHandler) OnAdd(obj interface{}, _ bool) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	h.mu.Lock()
	h.added = append(h.added, u.GetName())
	h.mu.Unlock()
}

func (h *recordingHandler) OnUpdate(_, _ interface{}) {}

func (h *recordingHandler) OnDelete(_ interface{}) {}

func (h *recordingHandler) addedNames() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.added...)
}

func testWatchErrorHandler() *WatchErrorHandler {
	return newWatchErrorHandler("factory-test", "ConfigMap", nil, nil, log.NewNop())
}

// TestFactoryStore_SurvivesConsumerContextCancel is the regression test for the
// shared-informer lifetime bug: the factory context must derive from the store's
// base context, not from the context of whichever consumer created the factory
// first. Before the fix, cancelling the first consumer's context killed the
// shared informer for every other consumer, silently freezing their snapshots.
func TestFactoryStore_SurvivesConsumerContextCancel(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	createNsWithLabels(fc, "default", map[string]string{})

	store := NewFactoryStore(context.Background())
	index := FactoryIndex{GVR: configMapsGVR, Namespace: "default"}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	handler1 := &recordingHandler{}
	g.Expect(store.Start(ctx1, "informer-1", fc.Client.Dynamic(), index, handler1, testWatchErrorHandler())).To(Succeed())

	handler2 := &recordingHandler{}
	g.Expect(store.Start(context.Background(), "informer-2", fc.Client.Dynamic(), index, handler2, testWatchErrorHandler())).To(Succeed())

	// The first consumer goes away: its context is cancelled and its handler is
	// removed — exactly what StopMonitor does for one of two hooks sharing an index.
	cancel1()
	store.Stop("informer-1", index)

	// The shared informer must still be running for the second consumer.
	store.mu.Lock()
	factory, ok := store.data[index]
	store.mu.Unlock()
	g.Expect(ok).To(BeTrue(), "factory must stay in the store while a consumer remains")
	select {
	case <-factory.done:
		t.Fatal("shared informer died together with the first consumer's context")
	default:
	}

	// And events must still be delivered to the second consumer.
	createCM(fc, "default", testCM("after-first-consumer-gone"))
	g.Eventually(handler2.addedNames, "5s", "10ms").
		Should(ContainElement("after-first-consumer-gone"),
			"second consumer must keep receiving events after the first consumer is gone")
}

// TestFactoryStore_RecreatesDeadFactory is the regression test for the
// success-on-corpse bug: Start on an index whose informer goroutine has already
// exited must build a fresh factory instead of silently reusing the stopped one
// (whose HasSynced() still returns true and which never delivers events).
func TestFactoryStore_RecreatesDeadFactory(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	createNsWithLabels(fc, "default", map[string]string{})

	store := NewFactoryStore(context.Background())
	index := FactoryIndex{GVR: configMapsGVR, Namespace: "default"}

	handler1 := &recordingHandler{}
	g.Expect(store.Start(context.Background(), "informer-1", fc.Client.Dynamic(), index, handler1, testWatchErrorHandler())).To(Succeed())

	// Kill the informer out of band, keeping the factory in the store — the state
	// the pre-fix lifetime bug used to leave behind.
	store.mu.Lock()
	dead := store.data[index]
	store.mu.Unlock()
	dead.cancel()
	<-dead.done

	handler2 := &recordingHandler{}
	g.Expect(store.Start(context.Background(), "informer-2", fc.Client.Dynamic(), index, handler2, testWatchErrorHandler())).To(Succeed())

	store.mu.Lock()
	fresh, ok := store.data[index]
	store.mu.Unlock()
	g.Expect(ok).To(BeTrue())
	g.Expect(fresh).ToNot(BeIdenticalTo(dead), "a dead factory must be replaced, not reused")
	select {
	case <-fresh.done:
		t.Fatal("recreated factory is not running")
	default:
	}

	createCM(fc, "default", testCM("after-recreate"))
	g.Eventually(handler2.addedNames, "5s", "10ms").
		Should(ContainElement("after-recreate"), "consumer on the recreated factory must receive events")
}
