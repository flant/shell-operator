package dedupclient

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/ldmonster/kubeclient/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/flant/shell-operator/pkg"
)

// SnapshotStore is a reference-counted, deduplicated object cache that
// shell-operator's kube-events-manager uses as the canonical home of full
// `*Unstructured` bodies. It wraps `*store.DedupStore` from the upstream
// kubeclient library, which gives us value-interning and subtree
// deduplication: identical scalar fields (e.g. recurring annotations,
// `securityContext` blocks, `tolerations`, `resources.limits`) are stored
// once and referenced by every object that contains them. For workloads
// where many similar objects coexist — typical for cluster-wide hooks on
// Pods or Deployments — peak RSS for the cached object population drops
// substantially compared to holding raw `*Unstructured` per consumer.
//
// Reference counting is needed because multiple `resourceInformer`s may
// independently watch the same object (e.g. two hooks on overlapping
// LabelSelectors). Each informer is treated as an *owner*: it calls
// Acquire on Add/Modified events and Release on Delete or shutdown.
// The underlying object is removed from the dedup store only when the
// last owner releases it; that keeps the per-monitor view consistent
// even though the store itself is shared globally.
//
// SnapshotStore is safe for concurrent use.
type SnapshotStore struct {
	logger *log.Logger

	dedup store.Store

	// mu guards refs. refs[key] is the set of owner IDs that currently
	// hold this object. The presence (or absence) of a key in dedup is
	// derived from refs: when refs[key] becomes empty, we Delete from
	// dedup and remove the map entry.
	mu   sync.Mutex
	refs map[store.ObjectKey]map[string]struct{}

	// stats is updated under mu to give cheap, allocation-free numbers
	// to the debug endpoint without scanning the maps.
	stats SnapshotStoreStats
}

// SnapshotStoreStats is a flat snapshot of the reference-counter state.
// It is cheap to compute and stable across concurrent updates because it
// is always read under SnapshotStore.mu.
type SnapshotStoreStats struct {
	// LiveObjects is the number of distinct (GVK, ns, name) currently in
	// the dedup store (i.e. with at least one owner).
	LiveObjects int
	// TotalAcquires counts every successful Acquire call since the
	// SnapshotStore was created. Acquires by an existing owner of the
	// same key are counted as no-ops and do not increment this counter.
	TotalAcquires uint64
	// TotalReleases counts every Release that actually removed an owner
	// (no-op releases by an unknown owner are ignored).
	TotalReleases uint64
	// TotalDeletes counts how many objects were removed from the dedup
	// store after their last owner released them.
	TotalDeletes uint64
}

// NewSnapshotStore constructs a SnapshotStore backed by a fresh
// `*store.DedupStore`. logger may be nil; in that case a default logger
// is used. The store is empty until Acquire is called.
func NewSnapshotStore(logger *log.Logger) *SnapshotStore {
	if logger == nil {
		logger = log.NewLogger()
	}
	return &SnapshotStore{
		logger: logger.With(pkg.LogKeyOperatorComponent, "dedup-snapshot-store"),
		dedup:  store.NewDedupStore(),
		refs:   make(map[store.ObjectKey]map[string]struct{}),
	}
}

// Acquire upserts obj under key for the given owner. The owner string is
// any opaque identifier — `resourceInformer` uses its UUID. Calling
// Acquire repeatedly with the same (owner, key) is safe: the refcount
// stays at one and only the object body is refreshed.
//
// Returning an error from the underlying dedup store leaves the refs
// map untouched, so the caller can retry without leaking ownership.
func (s *SnapshotStore) Acquire(owner string, key store.ObjectKey, obj *unstructured.Unstructured) error {
	if obj == nil {
		return fmt.Errorf("dedupclient: SnapshotStore.Acquire: obj is nil")
	}
	if owner == "" {
		return fmt.Errorf("dedupclient: SnapshotStore.Acquire: owner is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Always Upsert so the latest object body wins. The dedup store is
	// content-addressable, so Upserting an unchanged object is cheap.
	if err := s.dedup.Upsert(key, obj); err != nil {
		return fmt.Errorf("dedupclient: SnapshotStore.Acquire: upsert: %w", err)
	}

	owners, ok := s.refs[key]
	if !ok {
		owners = make(map[string]struct{}, 1)
		s.refs[key] = owners
		s.stats.LiveObjects++
	}
	if _, already := owners[owner]; !already {
		owners[owner] = struct{}{}
		s.stats.TotalAcquires++
	}
	return nil
}

// Release decrements the reference for (owner, key). When the last owner
// releases, the underlying object is removed from the dedup store. A
// Release for an unknown (owner, key) pair is silently dropped, which
// makes the caller's bookkeeping robust to dropped/repeated events.
func (s *SnapshotStore) Release(owner string, key store.ObjectKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	owners, ok := s.refs[key]
	if !ok {
		return nil
	}
	if _, hadOwner := owners[owner]; !hadOwner {
		return nil
	}
	delete(owners, owner)
	s.stats.TotalReleases++

	if len(owners) > 0 {
		return nil
	}

	delete(s.refs, key)
	s.stats.LiveObjects--
	if err := s.dedup.Delete(key); err != nil {
		// The map entry is already gone; the most useful action is to
		// surface the error to the caller. Subsequent Acquires for the
		// same key will simply overwrite whatever zombie state remains
		// in the underlying store.
		return fmt.Errorf("dedupclient: SnapshotStore.Release: delete: %w", err)
	}
	s.stats.TotalDeletes++
	return nil
}

// ReleaseOwner releases every key currently held by owner. It is meant
// for use during informer shutdown to reclaim store space deterministically.
// Errors from individual Delete calls are logged at warn level and do not
// abort the loop, so a stuck key cannot prevent the rest of the owner's
// references from being released.
func (s *SnapshotStore) ReleaseOwner(owner string) {
	if owner == "" {
		return
	}
	s.mu.Lock()
	heldKeys := make([]store.ObjectKey, 0)
	for key, owners := range s.refs {
		if _, ok := owners[owner]; ok {
			heldKeys = append(heldKeys, key)
		}
	}
	s.mu.Unlock()

	for _, key := range heldKeys {
		if err := s.Release(owner, key); err != nil {
			s.logger.Warn("release on shutdown failed",
				slog.String("owner", owner),
				slog.String("gvk", key.GVK.String()),
				slog.String("namespace", key.Namespace),
				slog.String("name", key.Name),
				log.Err(err))
		}
	}
}

// Get returns a freshly reconstructed `*unstructured.Unstructured` for
// key, or (nil, false) when the object is not currently held. The
// returned object is a new allocation owned by the caller — mutating it
// does not affect the dedup store.
func (s *SnapshotStore) Get(key store.ObjectKey) (*unstructured.Unstructured, bool) {
	return s.dedup.Get(key)
}

// HasOwner reports whether owner currently holds key. Used by tests and
// the debug endpoint; not part of the hot path.
func (s *SnapshotStore) HasOwner(owner string, key store.ObjectKey) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	owners, ok := s.refs[key]
	if !ok {
		return false
	}
	_, has := owners[owner]
	return has
}

// Stats returns a copy of the current statistics. Cheap, lock-bounded.
func (s *SnapshotStore) Stats() SnapshotStoreStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// KeyFor is a convenience helper that builds a `store.ObjectKey` from an
// already-typed `*unstructured.Unstructured`. Callers that already have
// GVK on hand can construct ObjectKey themselves; this helper exists
// solely so the hot path in `resourceInformer` can stay short.
func KeyFor(obj *unstructured.Unstructured) store.ObjectKey {
	if obj == nil {
		return store.ObjectKey{}
	}
	return store.ObjectKey{
		GVK:       obj.GroupVersionKind(),
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// KeyForGVK builds a key when the GVK is already known (e.g. derived
// from a Monitor configuration) and the object's GroupVersionKind() may
// not be reliable — for instance because the dynamic informer strips it.
func KeyForGVK(gvk schema.GroupVersionKind, namespace, name string) store.ObjectKey {
	return store.ObjectKey{GVK: gvk, Namespace: namespace, Name: name}
}
