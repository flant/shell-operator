package dedupclient

import (
	"sync"
	"testing"

	"github.com/ldmonster/kubeclient/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newPod(ns, name string, extraLabel string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Pod"})
	u.SetNamespace(ns)
	u.SetName(name)
	u.SetLabels(map[string]string{"app": "demo", "extra": extraLabel})
	return u
}

func TestSnapshotStore_AcquireGetRelease(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	pod := newPod("default", "pod-a", "v1")
	key := KeyFor(pod)

	require.NoError(t, s.Acquire("informer-1", key, pod))

	got, ok := s.Get(key)
	require.True(t, ok, "Get must succeed after Acquire")
	require.NotNil(t, got)
	assert.Equal(t, "pod-a", got.GetName())
	assert.Equal(t, "default", got.GetNamespace())

	stats := s.Stats()
	assert.Equal(t, 1, stats.LiveObjects)
	assert.Equal(t, uint64(1), stats.TotalAcquires)

	require.NoError(t, s.Release("informer-1", key))
	_, ok = s.Get(key)
	assert.False(t, ok, "Get must fail after the last owner releases")

	stats = s.Stats()
	assert.Equal(t, 0, stats.LiveObjects)
	assert.Equal(t, uint64(1), stats.TotalReleases)
	assert.Equal(t, uint64(1), stats.TotalDeletes)
}

func TestSnapshotStore_RefcountAcrossOwners(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	pod := newPod("default", "pod-shared", "x")
	key := KeyFor(pod)

	require.NoError(t, s.Acquire("informer-A", key, pod))
	require.NoError(t, s.Acquire("informer-B", key, pod))

	assert.Equal(t, 1, s.Stats().LiveObjects, "two acquires for the same key must not double-count live objects")
	assert.Equal(t, uint64(2), s.Stats().TotalAcquires)

	require.NoError(t, s.Release("informer-A", key))
	_, ok := s.Get(key)
	assert.True(t, ok, "object must remain in the store while informer-B still owns it")

	require.NoError(t, s.Release("informer-B", key))
	_, ok = s.Get(key)
	assert.False(t, ok, "object must be evicted after the last owner releases")
}

func TestSnapshotStore_RepeatedAcquireBySameOwner(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	pod := newPod("default", "pod-r", "x")
	key := KeyFor(pod)

	require.NoError(t, s.Acquire("informer-X", key, pod))
	require.NoError(t, s.Acquire("informer-X", key, pod))
	assert.Equal(t, uint64(1), s.Stats().TotalAcquires,
		"acquiring twice with the same owner must not increment the counter")

	// One Release is enough to fully evict because the owner only counts once.
	require.NoError(t, s.Release("informer-X", key))
	_, ok := s.Get(key)
	assert.False(t, ok)
}

func TestSnapshotStore_UnknownReleaseIsNoOp(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	pod := newPod("default", "pod-u", "x")
	key := KeyFor(pod)

	require.NoError(t, s.Acquire("informer-1", key, pod))

	// Release by an owner that never acquired this key — should not affect anything.
	require.NoError(t, s.Release("informer-2", key))
	_, ok := s.Get(key)
	assert.True(t, ok, "object must still be present after a no-op release")

	require.NoError(t, s.Release("informer-1", key))
	_, ok = s.Get(key)
	assert.False(t, ok)
}

func TestSnapshotStore_ReleaseOwner_DropsAllKeys(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	pods := []*unstructured.Unstructured{
		newPod("default", "p1", "a"),
		newPod("default", "p2", "b"),
		newPod("kube-system", "p3", "c"),
	}
	keys := make([]store.ObjectKey, len(pods))
	for i, p := range pods {
		keys[i] = KeyFor(p)
		require.NoError(t, s.Acquire("informer-Z", keys[i], p))
	}

	// One key co-owned with another informer must NOT be removed by ReleaseOwner.
	require.NoError(t, s.Acquire("informer-W", keys[0], pods[0]))

	s.ReleaseOwner("informer-Z")

	_, ok := s.Get(keys[0])
	assert.True(t, ok, "co-owned key must survive informer-Z's owner release")
	for _, k := range keys[1:] {
		_, ok := s.Get(k)
		assert.False(t, ok, "key only owned by informer-Z must be evicted")
	}
}

func TestSnapshotStore_AcquireRejectsNil(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	err := s.Acquire("o", store.ObjectKey{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "obj is nil")

	pod := newPod("ns", "n", "x")
	err = s.Acquire("", KeyFor(pod), pod)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owner is empty")
}

func TestSnapshotStore_ConcurrentAcquireRelease(t *testing.T) {
	t.Parallel()

	s := NewSnapshotStore(nil)
	pod := newPod("default", "stress", "x")
	key := KeyFor(pod)

	const ownersN = 32
	const cycles = 100

	var wg sync.WaitGroup
	for i := 0; i < ownersN; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			owner := "informer-" + string(rune('A'+(id%26)))
			for j := 0; j < cycles; j++ {
				_ = s.Acquire(owner, key, pod)
				_ = s.Release(owner, key)
			}
		}(i)
	}
	wg.Wait()

	// After every owner has released the same number of times it acquired,
	// the live-object count must collapse back to zero.
	assert.Equal(t, 0, s.Stats().LiveObjects)
}
