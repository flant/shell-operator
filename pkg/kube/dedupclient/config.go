// Package dedupclient integrates github.com/ldmonster/kubeclient — a
// controller-runtime compatible Kubernetes client backed by a deduplicated
// cache — with shell-operator. It exposes a thin wrapper that wires the
// underlying *kubeclient.DedupClient into shell-operator's lifecycle
// (configuration, construction, start/stop) without leaking the dependency
// across the rest of the codebase.
package dedupclient

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// Config carries the parameters needed to construct a DedupClient. It is
// populated either from app.Config (production path) or directly by tests
// and library consumers that already hold the relevant Kubernetes plumbing.
//
// RESTConfig is the only mandatory field; all other fields have sensible
// defaults and may be left zero-valued.
type Config struct {
	// Context is the kubeconfig context name used when RESTConfig is nil.
	Context string

	// Config is the kubeconfig path used when RESTConfig is nil.
	Config string

	// Server is an explicit API server URL. It bypasses kubeconfig and
	// in-cluster loading when set.
	Server string

	// QPS and Burst tune the shared rest.Config rate limiter.
	QPS   float32
	Burst int

	// Timeout sets the shared rest.Config timeout. Zero leaves it unset.
	Timeout time.Duration

	// RESTConfig is the *rest.Config used by the underlying dynamic client.
	// Optional; when nil the client loads out-of-cluster or in-cluster config.
	RESTConfig *rest.Config

	// RESTMapper resolves GroupKind/Version → GroupVersionResource. When nil,
	// kubeclient falls back to meta.NewDefaultRESTMapper(nil).
	RESTMapper meta.RESTMapper

	// Scheme is the runtime.Scheme used for typed object conversion. When
	// nil, kubeclient creates an empty scheme; populate it with your types
	// if you intend to call Get/List with typed (non-Unstructured) objects.
	Scheme *runtime.Scheme

	// Namespaces restricts cached objects to a list of namespaces. An empty
	// or nil slice means "all namespaces".
	Namespaces []string

	// WatchGVKs is the set of GVKs to pre-register with the cache so that
	// informers spin up at Start() time. Additional GVKs can be added later
	// via the Cache.EnsureInformer API.
	WatchGVKs []schema.GroupVersionKind

	// ReconstructLRUSize sets the size of the per-cache LRU that memoises
	// reconstructed Unstructured objects. Zero disables reconstruction
	// caching (objects are rebuilt from the deduplicated store on every
	// access).
	ReconstructLRUSize int

	// GCInterval controls how often the deduplicated store reclaims unused
	// interned values and subtrees. Zero leaves the kubeclient default in
	// place (5 minutes at the time of writing).
	GCInterval time.Duration
}
