package kubeeventsmanager

import (
	"sync/atomic"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// Typed informer support
//
// shell-operator builds every monitor on top of client-go's
// dynamicinformer.DynamicSharedInformerFactory, which always materialises
// objects as *unstructured.Unstructured. Unstructured stores each leaf value in
// a generic map[string]interface{}, so a Pod that fits in ~500 bytes as a typed
// struct typically occupies 1.5–2.5 KiB once it lives in the unstructured
// cache.
//
// When the typed-informer code path is enabled (see SetTypedInformerEnabled
// and the --use-typed-informers flag) and the GVR is in typedSupportedGVRs,
// the FactoryStore materialises the SharedIndexInformer via client-go's typed
// informers.SharedInformerFactory instead of the dynamic factory. The wire
// JSON is then decoded directly into typed runtime.Objects (e.g. *corev1.Pod)
// and the SharedIndexInformer's cache holds those typed values without ever
// allocating an unstructured representation.
//
// Read paths convert the typed value back to *unstructured.Unstructured on
// demand via toUnstructured, so downstream code (jq filtering, hook binding
// contexts, snapshots) keeps treating every payload uniformly.
//
// The opt-in design keeps behaviour bit-for-bit identical for CRDs and any
// kind not in typedSupportedGVRs: those continue to use the dynamic informer
// and live in the cache as *unstructured.Unstructured.

// typedInformerEnabled is read on every cache insertion and snapshot, so it is
// stored in an int32 (0/1) for atomic access. Toggle it via
// SetTypedInformerEnabled before any monitor is started.
var typedInformerEnabled int32

// SetTypedInformerEnabled toggles the typed-informer code path globally.
// It must be called before any monitor is started; runtime changes do not
// affect informers that have already been materialised.
func SetTypedInformerEnabled(v bool) {
	if v {
		atomic.StoreInt32(&typedInformerEnabled, 1)
	} else {
		atomic.StoreInt32(&typedInformerEnabled, 0)
	}
}

// IsTypedInformerEnabled reports whether the typed-informer code path is
// currently active.
func IsTypedInformerEnabled() bool {
	return atomic.LoadInt32(&typedInformerEnabled) == 1
}

// typedSupportedGVRs lists the GroupVersionResources that the FactoryStore
// will route through client-go's typed SharedInformerFactory when the
// typed-informer code path is enabled. These are the high-volume kinds where
// storing the generic unstructured.Unstructured map representation has the
// largest memory cost.
//
// CRDs and any kind not on this list continue to use the dynamic informer
// (and therefore continue to live in the cache as *unstructured.Unstructured).
var typedSupportedGVRs = map[schema.GroupVersionResource]struct{}{}

// registerTypedKind adds a GVR to the typed-cache registry. The split between
// this helper and the package-level map literal lets typed_informer_register.go
// import all of the typed K8s API packages that bring in the schemas, while
// this file stays free of those imports.
func registerTypedKind(gvr schema.GroupVersionResource) {
	typedSupportedGVRs[gvr] = struct{}{}
}

// IsTypedSupportedGVR reports whether the typed informer code path supports
// the given GVR. Exposed for tests and diagnostics.
func IsTypedSupportedGVR(gvr schema.GroupVersionResource) bool {
	_, ok := typedSupportedGVRs[gvr]
	return ok
}

// toUnstructured returns u if the value is already unstructured, otherwise
// converts a typed runtime.Object back to *unstructured.Unstructured. Used at
// every read site (event handler, snapshot, GetByKey) so downstream code can
// continue to assume *unstructured.Unstructured regardless of how the cache
// chose to store the value.
func toUnstructured(obj any) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u
	}
	ro, ok := obj.(runtime.Object)
	if !ok {
		return nil
	}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ro)
	if err != nil {
		return nil
	}
	out := &unstructured.Unstructured{Object: m}

	// runtime.DefaultUnstructuredConverter.ToUnstructured drops apiVersion/kind
	// for typed objects (their GVK is implicit). Restore them so downstream
	// code (jq filters keying off .apiVersion/.kind, the resourceId helper, ...)
	// keeps working.
	gvk := ro.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		// Typed objects from typed informers do not carry TypeMeta inline.
		// Look up the registered GVK from client-go's compiled-in scheme.
		if gvks, _, err := clientgoscheme.Scheme.ObjectKinds(ro); err == nil && len(gvks) > 0 {
			gvk = gvks[0]
		}
	}
	if !gvk.Empty() {
		out.SetAPIVersion(gvk.GroupVersion().String())
		out.SetKind(gvk.Kind)
	}
	return out
}
