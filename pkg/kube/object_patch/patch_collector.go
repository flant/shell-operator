package object_patch

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type patchCollector struct {
	patchOperations []Operation
}

// NewPatchCollector creates Operation collector to use within Go hooks.
func NewPatchCollector() *patchCollector {
	return &patchCollector{
		patchOperations: make([]Operation, 0),
	}
}

// Create or update an object.
//
// Options:
//   - WithSubresource - create a specified subresource
//   - IgnoreIfExists - do not return error if the specified object exists
//   - UpdateIfExists - call Update if the specified object exists
func (dop *patchCollector) Create(object interface{}, options ...CreateOption) {
	dop.add(NewCreateOperation(object, options...))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
//
// Options:
//   - WithSubresource - delete a specified subresource
//   - InForeground -  remove object when all dependants are removed (default)
//   - InBackground - remove object immediately, dependants remove in background
//   - NonCascading - remove object, dependants become orphan
//
// Missing object is ignored by default.
func (dop *patchCollector) Delete(apiVersion, kind, namespace, name string, options ...DeleteOption) {
	dop.add(NewDeleteOperation(apiVersion, kind, namespace, name, options...))
}

// MergePatch applies a merge patch to the specified object using API call Patch.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
func (dop *patchCollector) MergePatch(mergePatch interface{}, apiVersion, kind, namespace, name string, options ...PatchOption) {
	dop.add(NewMergePatchOperation(mergePatch, apiVersion, kind, namespace, name, options...))
}

// JSONPatch applies a json patch to the specified object using API call Patch.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
func (dop *patchCollector) JSONPatch(jsonPatch interface{}, apiVersion, kind, namespace, name string, options ...PatchOption) {
	dop.add(NewJSONPatchOperation(jsonPatch, apiVersion, kind, namespace, name, options...))
}

// Filter retrieves a specified object, modified it with
// filterFunc and calls update.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
func (dop *patchCollector) Filter(
	filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
	apiVersion, kind, namespace, name string, options ...FilterOption,
) {
	dop.add(NewFilterPatchOperation(filterFunc, apiVersion, kind, namespace, name, options...))
}

// Operations returns all collected operations
func (dop *patchCollector) Operations() []Operation {
	return dop.patchOperations
}

func (dop *patchCollector) add(operation Operation) {
	dop.patchOperations = append(dop.patchOperations, operation)
}
