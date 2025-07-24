package kubeeventsmanager

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"

	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

type ResourceID struct {
	NamespaceIDX uint16
	KindIDX      uint16
	Name         string

	store *ResourceIDStore
}

func (rid ResourceID) String() string {
	if rid.store == nil {
		// This is a rare/debug path, but let's optimize it too.
		// Using strings.Builder is significantly faster than fmt.Sprintf.
		var sb strings.Builder
		sb.WriteString("uninitialized_resource_id:ns_idx=")
		sb.WriteString(strconv.FormatUint(uint64(rid.NamespaceIDX), 10))
		sb.WriteString("/kind_idx=")
		sb.WriteString(strconv.FormatUint(uint64(rid.KindIDX), 10))
		sb.WriteByte('/')
		sb.WriteString(rid.Name)
		return sb.String()
	}

	ns := rid.store.GetNSByID(rid.NamespaceIDX)
	kind := rid.store.GetKindByID(rid.KindIDX)

	// Pre-allocating the buffer for the string builder is a key performance optimization.
	// It avoids intermediate allocations while building the string.
	size := len(kind) + len(rid.Name) + 1 // kind + "/" + name
	if ns != "" {
		size += len(ns) + 1 // ns + "/"
	}

	var sb strings.Builder
	sb.Grow(size)

	if ns != "" {
		sb.WriteString(ns)
		sb.WriteByte('/')
	}
	sb.WriteString(kind)
	sb.WriteByte('/')
	sb.WriteString(rid.Name)

	return sb.String()
}

// ResourceIDStore is a thread-safe storage for "interning" Kind and Namespace strings
type ResourceIDStore struct {
	mu        sync.RWMutex
	kindStore map[string]uint16
	kindRev   []string // For reverse mapping (index -> string)
	nsStore   map[string]uint16
	nsRev     []string
}

// NewResourceIDStore creates a new instance of the store
func NewResourceIDStore() *ResourceIDStore {
	return &ResourceIDStore{
		kindStore: make(map[string]uint16),
		kindRev:   make([]string, 0),
		nsStore:   make(map[string]uint16),
		nsRev:     make([]string, 0),
	}
}

// GetKindIDX gets the numeric index for a Kind string
// If the Kind has not been seen before, it will be added
func (r *ResourceIDStore) GetKindIDX(kind string) uint16 {
	// 1. Fast path: check for existence with a read lock.
	r.mu.RLock()
	idx, ok := r.kindStore[kind]
	r.mu.RUnlock()
	if ok {
		return idx
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check to prevent a race condition in case another goroutine added the kind while we were waiting for the lock.
	idx, ok = r.kindStore[kind]
	if !ok {
		idx = uint16(len(r.kindStore))
		r.kindStore[kind] = idx
		r.kindRev = append(r.kindRev, kind)
	}
	return idx
}

// GetKindByID returns the Kind string for its index
func (r *ResourceIDStore) GetKindByID(idx uint16) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(idx) < len(r.kindRev) {
		return r.kindRev[idx]
	}
	return ""
}

// GetNSIDX gets the numeric index for a Namespace string
func (r *ResourceIDStore) GetNSIDX(ns string) uint16 {
	r.mu.RLock()
	idx, ok := r.nsStore[ns]
	r.mu.RUnlock()
	if ok {
		return idx
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx, ok = r.nsStore[ns]
	if !ok {
		idx = uint16(len(r.nsStore))
		r.nsStore[ns] = idx
		r.nsRev = append(r.nsRev, ns)
	}
	return idx
}

// GetNSByID returns the Namespace string for its index
func (r *ResourceIDStore) GetNSByID(idx uint16) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if int(idx) < len(r.nsRev) {
		return r.nsRev[idx]
	}
	return ""
}

func (r *ResourceIDStore) GetResourceID(obj *unstructured.Unstructured) ResourceID {
	return ResourceID{
		NamespaceIDX: r.GetNSIDX(obj.GetNamespace()),
		KindIDX:      r.GetKindIDX(obj.GetKind()),
		Name:         obj.GetName(),
		store:        r,
	}
}

// resourceId describes object with namespace, kind and name
//
// Change with caution, as this string is used for sorting objects and snapshots
// func resourceId(obj *unstructured.Unstructured) string {
// 	return fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName())
// }

func FormatLabelSelector(selector *metav1.LabelSelector) (string, error) {
	res, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return "", err
	}

	return res.String(), nil
}

func FormatFieldSelector(selector *kemtypes.FieldSelector) (string, error) {
	if selector == nil || selector.MatchExpressions == nil {
		return "", nil
	}

	requirements := make([]fields.Selector, 0)

	for _, req := range selector.MatchExpressions {
		switch req.Operator {
		case "=", "==", "Equals":
			requirements = append(requirements, fields.OneTermEqualSelector(req.Field, req.Value))
		case "!=", "NotEquals":
			requirements = append(requirements, fields.OneTermNotEqualSelector(req.Field, req.Value))
		default:
			return "", fmt.Errorf("%s%s%s: operator '%s' is not recognized", req.Field, req.Operator, req.Value, req.Operator)
		}
	}

	return fields.AndSelectors(requirements...).String(), nil
}

const (
	ResyncPeriodMedian            = time.Duration(3) * time.Hour
	ResyncPeriodSpread            = time.Duration(2) * time.Hour
	ResyncPeriodGranularity       = time.Duration(5) * time.Minute
	ResyncPeriodJitterGranularity = time.Duration(15) * time.Second
)

// randomizedResyncPeriod returns a time.Duration between 2 hours and 4 hours with jitter and granularity
func randomizedResyncPeriod() time.Duration {
	spreadCount := ResyncPeriodSpread.Nanoseconds() / ResyncPeriodGranularity.Nanoseconds()
	rndSpreadDelta := time.Duration(rand.Int64N(spreadCount)) * ResyncPeriodGranularity
	jitterCount := ResyncPeriodGranularity.Nanoseconds() / ResyncPeriodJitterGranularity.Nanoseconds()
	rndJitterDelta := time.Duration(rand.Int64N(jitterCount)) * ResyncPeriodJitterGranularity

	return ResyncPeriodMedian - (ResyncPeriodSpread / 2) + rndSpreadDelta + rndJitterDelta
}

// CachedObjectsInfo stores counters of operations over resources in Monitors and Informers.
type CachedObjectsInfo struct {
	Count    uint64 `json:"count"`
	Added    uint64 `json:"added"`
	Deleted  uint64 `json:"deleted"`
	Modified uint64 `json:"modified"`
	Cleaned  uint64 `json:"cleaned"`
}

func (c *CachedObjectsInfo) add(in CachedObjectsInfo) {
	c.Count += in.Count
	c.Added += in.Added
	c.Deleted += in.Deleted
	c.Modified += in.Modified
	c.Cleaned += in.Cleaned
}
