package kubeeventsmanager

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// LastAppliedConfigAnnotation is the annotation set by `kubectl apply` that
// contains a full JSON copy of the object as it was last applied. For objects
// that go through `kubectl apply` (e.g. ConfigMaps with embedded files) it can
// easily be the largest single field on the object.
const LastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// stripObjectMetaNoise mutates obj in place to remove fields that are large
// and rarely needed by hooks: managedFields and the kubectl
// last-applied-configuration annotation.
//
// ManagedFields is the object's server-side-apply ownership ledger and is
// often 30-60% of an object's size in modern clusters.
// last-applied-configuration is a debugging aid for kubectl and is essentially
// a duplicate of the spec. Both are extremely rarely used from inside a hook.
//
// Works for both *unstructured.Unstructured and typed Kubernetes API objects
// (Pod, ConfigMap, ...) because both implement metav1.Object.
func stripObjectMetaNoise(obj metav1.Object) {
	obj.SetManagedFields(nil)

	annotations := obj.GetAnnotations()
	if _, ok := annotations[LastAppliedConfigAnnotation]; !ok {
		return
	}
	delete(annotations, LastAppliedConfigAnnotation)
	if len(annotations) == 0 {
		obj.SetAnnotations(nil)
	} else {
		obj.SetAnnotations(annotations)
	}
}

// stripUnstructuredNoise is a thin convenience wrapper kept for the call sites
// in resource_informer.go that already work directly with
// *unstructured.Unstructured (List() responses, defensive re-strip on watch
// events). It exists purely to avoid adding interface assertions at every
// call site.
func stripUnstructuredNoise(obj *unstructured.Unstructured) {
	stripObjectMetaNoise(obj)
}

// transformUnstructured is the bare cache.TransformFunc that strips noise
// from an unstructured cache value. Used for informers whose cache stores
// *unstructured.Unstructured.
func transformUnstructured(in any) (any, error) {
	if obj, ok := in.(*unstructured.Unstructured); ok {
		stripObjectMetaNoise(obj)
	}
	return in, nil
}

// transformTyped is the cache.TransformFunc registered on typed informers.
// It strips noise via the metav1.Object accessor methods that all typed K8s
// API objects implement.
func transformTyped(in any) (any, error) {
	if obj, ok := in.(metav1.Object); ok {
		stripObjectMetaNoise(obj)
	}
	return in, nil
}

// makeTransformFunc returns the cache.TransformFunc to register on a freshly
// created SharedIndexInformer. typed selects the dispatcher: typed informers
// store metav1.Object payloads, dynamic informers store
// *unstructured.Unstructured.
func makeTransformFunc(typed bool) cache.TransformFunc {
	if typed {
		return transformTyped
	}
	return transformUnstructured
}
