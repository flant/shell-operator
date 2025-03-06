package object_patch

import (
	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type IPatchCollector interface {
	sdkpkg.PatchCollector

	Filter(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorFilterOption)
}

var _ IPatchCollector = (*PatchCollector)(nil)

type PatchCollector struct {
	patchOperations []sdkpkg.PatchCollectorOperation
}

// NewPatchCollector creates Operation collector to use within Go hooks.
func NewPatchCollector() *PatchCollector {
	return &PatchCollector{
		patchOperations: make([]sdkpkg.PatchCollectorOperation, 0),
	}
}

// Create an object.
//
// Options:
//   - WithSubresource - create a specified subresource
func (dop *PatchCollector) Create(object any, opts ...sdkpkg.PatchCollectorCreateOption) {
	dop.add(NewCreateOperation(object, opts...))
}

// Create or update an object.
//
// Options:
//   - WithSubresource - create a specified subresource
func (dop *PatchCollector) CreateOrUpdate(object any, opts ...sdkpkg.PatchCollectorCreateOption) {
	dop.add(NewCreateOrUpdateOperation(object, opts...))
}

// Create if not exists an object.
//
// Options:
//   - WithSubresource - create a specified subresource
func (dop *PatchCollector) CreateIfNotExists(object any, opts ...sdkpkg.PatchCollectorCreateOption) {
	dop.add(NewCreateIfNotExistsOperation(object, opts...))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
// remove object when all dependants are removed
//
// Options:
//   - WithSubresource - delete a specified subresource
//
// Missing object is ignored by default.
func (dop *PatchCollector) Delete(apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorDeleteOption) {
	dop.add(NewDeleteOperation(apiVersion, kind, namespace, name, opts...))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
// remove object immediately, dependants remove in background
//
// Options:
//   - WithSubresource - delete a specified subresource
//
// Missing object is ignored by default.
func (dop *PatchCollector) DeleteInBackground(apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorDeleteOption) {
	dop.add(NewDeleteInBackgroundOperation(apiVersion, kind, namespace, name, opts...))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
// remove object, dependants become orphan
//
// Options:
//   - WithSubresource - delete a specified subresource
//
// Missing object is ignored by default.
func (dop *PatchCollector) DeleteNonCascading(apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorDeleteOption) {
	dop.add(NewDeleteNonCascadingOperation(apiVersion, kind, namespace, name, opts...))
}

// MergePatch applies a merge patch to the specified object using API call Patch.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
func (dop *PatchCollector) MergePatch(mergePatch any, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorPatchOption) {
	dop.add(NewMergePatchOperation(mergePatch, apiVersion, kind, namespace, name, opts...))
}

// JSONPatch applies a json patch to the specified object using API call Patch.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
func (dop *PatchCollector) JSONPatch(jsonPatch any, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorPatchOption) {
	dop.add(NewJSONPatchOperation(jsonPatch, apiVersion, kind, namespace, name, opts...))
}

// Filter retrieves a specified object, modified it with
// filterFunc and calls update.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
//
// Note: do not modify and return argument in filterFunc,
// use FromUnstructured to instantiate a concrete type or modify after DeepCopy.
func (dop *PatchCollector) Filter(
	filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
	apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorFilterOption,
) {
	dop.add(NewFilterPatchOperation(filterFunc, apiVersion, kind, namespace, name, opts...))
}

// Filter retrieves a specified object, modified it with
// filterFunc and calls update.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
func (dop *PatchCollector) JQFilter(
	jqfilter, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorFilterOption,
) {
	dop.add(NewJQPatchOperation(jqfilter, apiVersion, kind, namespace, name, opts...))
}

// Operations returns all collected operations
func (dop *PatchCollector) Operations() []sdkpkg.PatchCollectorOperation {
	return dop.patchOperations
}

func (dop *PatchCollector) add(operation sdkpkg.PatchCollectorOperation) {
	dop.patchOperations = append(dop.patchOperations, operation)
}
