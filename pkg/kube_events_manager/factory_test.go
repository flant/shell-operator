package kubeeventsmanager

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/kube-client/fake"
	"github.com/flant/shell-operator/pkg/metric"
)

// configMapIndex is the shared FactoryIndex used by the tests below: cluster
// "core/v1 ConfigMaps in the default namespace", no selectors. Two consumers
// with this identical index share a single factory in the store.
func configMapIndex() FactoryIndex {
	return FactoryIndex{
		GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		Namespace: "default",
	}
}

// Test_FactoryStore_SharedInformer_SurvivesFirstConsumerCancel covers the
// context-ownership fix (defect #1).
//
// Two consumers (A then B) share one shared informer for the same index. A
// registers first. On the buggy code the factory's context descended from A's
// context, so cancelling A (as StopMonitor does at module reload) tore down the
// shared informer while B was still registered — B's snapshot froze until the
// process restarted. With the fix the informer's lifetime is anchored to the
// store's baseCtx and must survive A's cancellation, keeping B live.
func Test_FactoryStore_SharedInformer_SurvivesFirstConsumerCancel(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	createNsWithLabels(fc, "default", nil)
	createCM(fc, "default", testCM("cm-initial"))

	baseCtx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()

	store := NewFactoryStore(baseCtx)
	index := configMapIndex()
	errH := newWatchErrorHandler("test", "ConfigMap", nil, metric.NewStorageMock(t), log.NewNop())

	var bCount atomic.Int64
	handlerA := cache.ResourceEventHandlerFuncs{AddFunc: func(_ interface{}) {}}
	handlerB := cache.ResourceEventHandlerFuncs{AddFunc: func(_ interface{}) { bCount.Add(1) }}

	// Consumer A registers first -> creates the shared factory.
	ctxA, cancelA := context.WithCancel(baseCtx)
	g.Expect(store.Start(ctxA, "consumer-A", fc.Client.Dynamic(), index, handlerA, errH)).
		ShouldNot(HaveOccurred())

	// Consumer B reuses the same shared factory.
	ctxB, cancelB := context.WithCancel(baseCtx)
	defer cancelB()
	g.Expect(store.Start(ctxB, "consumer-B", fc.Client.Dynamic(), index, handlerB, errH)).
		ShouldNot(HaveOccurred())

	g.Eventually(bCount.Load, "5s", "10ms").Should(BeNumerically(">=", int64(1)),
		"consumer B must receive the initial snapshot")

	// Simulate monitor A being stopped: its context is cancelled (resource_informer
	// cancels ei.ctx on stop) AND its handler is removed (factoryStore.Stop drops
	// the refcount from 2 to 1).
	cancelA()
	store.Stop("consumer-A", index)

	// Give a cancelled shared informer time to actually exit on the buggy code so
	// the assertion below is not decided by a lucky in-flight delivery. On the
	// fixed code nothing is torn down, so this settle is a no-op.
	time.Sleep(500 * time.Millisecond)

	before := bCount.Load()

	// A new object arrives AFTER A is gone. B is still registered, so B must see it.
	createCM(fc, "default", testCM("cm-after-a-stopped"))

	g.Eventually(bCount.Load, "5s", "10ms").Should(BeNumerically(">", before),
		"shared informer must keep delivering to consumer B after consumer A is stopped")
}

// Test_FactoryStore_WaitStopped_BlocksUntilInformerExits guards the graceful
// shutdown barrier. Before the fix WaitStopped read a per-index stoppedCh that
// was never populated, so it built a pre-closed channel and returned
// immediately — monitor.Wait() never actually waited for informers to stop.
// The fix waits on the factory's own done channel, which is closed only when
// the informer's Run goroutine exits.
func Test_FactoryStore_WaitStopped_BlocksUntilInformerExits(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	createNsWithLabels(fc, "default", nil)

	baseCtx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()

	store := NewFactoryStore(baseCtx)
	index := configMapIndex()
	errH := newWatchErrorHandler("test", "ConfigMap", nil, metric.NewStorageMock(t), log.NewNop())

	// WaitStopped on an unknown index must return immediately.
	g.Eventually(func() bool { store.WaitStopped(index); return true }, "1s", "10ms").
		Should(BeTrue(), "WaitStopped must not block when no factory exists for the index")

	handler := cache.ResourceEventHandlerFuncs{AddFunc: func(_ interface{}) {}}
	ctx, cancel := context.WithCancel(baseCtx)
	g.Expect(store.Start(ctx, "consumer", fc.Client.Dynamic(), index, handler, errH)).
		ShouldNot(HaveOccurred())

	var returned atomic.Bool
	go func() {
		store.WaitStopped(index)
		returned.Store(true)
	}()

	// The informer is still running, so WaitStopped must keep blocking.
	g.Consistently(returned.Load, "500ms", "20ms").Should(BeFalse(),
		"WaitStopped must block while the shared informer is still running")

	// Stopping the last consumer cancels the factory; its Run goroutine exits
	// and closes done, which must release WaitStopped.
	cancel()
	store.Stop("consumer", index)

	g.Eventually(returned.Load, "5s", "10ms").Should(BeTrue(),
		"WaitStopped must return once the shared informer has stopped")
}

// Test_Factory_isDead covers the liveness predicate (defect #2).
func Test_Factory_isDead(t *testing.T) {
	g := NewWithT(t)

	// Never started: Run has not been launched, done is nil.
	g.Expect((&Factory{done: nil}).isDead()).To(BeFalse(), "a never-started factory is not dead")

	// Started and still running: done is open.
	running := make(chan struct{})
	g.Expect((&Factory{done: running}).isDead()).To(BeFalse(), "a running factory is not dead")

	// Started and exited: informer.Run returned and closed done.
	exited := make(chan struct{})
	close(exited)
	g.Expect((&Factory{done: exited}).isDead()).To(BeTrue(), "a factory whose Run exited is dead")
}

// Test_FactoryStore_Get_RecreatesDeadFactory covers the corpse-recreation fix
// (defect #2): get() must never hand back a factory whose informer goroutine has
// already exited. On the buggy code get() returned the cached factory as-is, so
// a new consumer attached to a dead informer and its snapshot never updated. The
// fix cancels and discards the corpse and builds a fresh factory instead.
func Test_FactoryStore_Get_RecreatesDeadFactory(t *testing.T) {
	g := NewWithT(t)

	fc := fake.NewFakeCluster(fake.ClusterVersionV121)
	store := NewFactoryStore(context.Background())
	index := configMapIndex()

	// Inject a dead factory (its Run has exited -> done closed) under the index.
	dead := make(chan struct{})
	close(dead)
	var cancelled atomic.Bool
	corpse := &Factory{
		handlerRegistrations: make(map[string]cache.ResourceEventHandlerRegistration),
		done:                 dead,
		cancel:               func() { cancelled.Store(true) },
	}
	store.data[index] = corpse

	got := store.get(fc.Client.Dynamic(), index)

	g.Expect(got).ShouldNot(BeIdenticalTo(corpse), "get must not reuse the dead factory")
	g.Expect(cancelled.Load()).To(BeTrue(), "the dead factory must be cancelled before discarding")
	g.Expect(got.isDead()).To(BeFalse(), "the recreated factory must be alive")
	g.Expect(store.data[index]).To(BeIdenticalTo(got), "the store must cache the recreated factory under the index")
}
