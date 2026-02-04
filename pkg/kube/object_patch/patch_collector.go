package object_patch

import (
	"io"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type iPatchCollector interface {
	sdkpkg.PatchCollector

	PatchWithMutatingFunc(fn func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption)
}

var _ iPatchCollector = (*PatchCollector)(nil)

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
func (dop *PatchCollector) Create(object any) {
	dop.add(NewCreateOperation(object))
}

// Create or update an object.
//
// Options:
//   - WithSubresource - create a specified subresource
func (dop *PatchCollector) CreateOrUpdate(object any) {
	dop.add(NewCreateOrUpdateOperation(object))
}

// Create if not exists an object.
//
// Options:
//   - WithSubresource - create a specified subresource
func (dop *PatchCollector) CreateIfNotExists(object any) {
	dop.add(NewCreateIfNotExistsOperation(object))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
// remove object when all dependants are removed
//
// Options:
//   - WithSubresource - delete a specified subresource
//
// Missing object is ignored by default.
func (dop *PatchCollector) Delete(apiVersion, kind, namespace, name string) {
	dop.add(NewDeleteOperation(apiVersion, kind, namespace, name))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
// remove object immediately, dependants remove in background
//
// Options:
//   - WithSubresource - delete a specified subresource
//
// Missing object is ignored by default.
func (dop *PatchCollector) DeleteInBackground(apiVersion, kind, namespace, name string) {
	dop.add(NewDeleteInBackgroundOperation(apiVersion, kind, namespace, name))
}

// Delete uses apiVersion, kind, namespace and name to delete object from cluster.
// remove object, dependants become orphan
//
// Options:
//   - WithSubresource - delete a specified subresource
//
// Missing object is ignored by default.
func (dop *PatchCollector) DeleteNonCascading(apiVersion, kind, namespace, name string) {
	dop.add(NewDeleteNonCascadingOperation(apiVersion, kind, namespace, name))
}

// MergePatch applies a merge patch to the specified object using API call Patch.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
func (dop *PatchCollector) MergePatch(mergePatch any, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorOption) {
	dop.add(NewMergePatchOperation(mergePatch, apiVersion, kind, namespace, name, opts...))
}

// JSONPatch applies a json patch to the specified object using API call Patch.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
func (dop *PatchCollector) JSONPatch(jsonPatch any, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorOption) {
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
func (dop *PatchCollector) PatchWithMutatingFunc(
	fn func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
	apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorOption,
) {
	dop.add(NewPatchWithMutatingFuncOperation(fn, apiVersion, kind, namespace, name, opts...))
}

// Filter retrieves a specified object, modified it with
// filterFunc and calls update.
//
// Options:
//   - WithSubresource — a subresource argument for Patch call.
//   - IgnoreMissingObject — do not return error if the specified object is missing.
//   - IgnoreHookError — allows applying patches for a Status subresource even if the hook fails
func (dop *PatchCollector) JQFilter(
	jqfilter, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorOption,
) {
	dop.add(NewPatchWithJQOperation(jqfilter, apiVersion, kind, namespace, name, opts...))
}

// Operations returns all collected operations
func (dop *PatchCollector) Operations() []sdkpkg.PatchCollectorOperation {
	return dop.patchOperations
}

func (dop *PatchCollector) add(operation sdkpkg.PatchCollectorOperation) {
	dop.patchOperations = append(dop.patchOperations, operation)
}

func (dop *PatchCollector) PatchWithJQ(jqfilter string, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) {
	dop.add(NewPatchWithJQOperation(jqfilter, apiVersion, kind, namespace, name, opts...))
}

func (dop *PatchCollector) PatchWithJSON(jsonPatch any, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) {
	dop.add(NewJSONPatchOperation(jsonPatch, apiVersion, kind, namespace, name, opts...))
}

func (dop *PatchCollector) PatchWithMerge(mergePatch any, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) {
	dop.add(NewMergePatchOperation(mergePatch, apiVersion, kind, namespace, name, opts...))
}

func (dop *PatchCollector) WriteOutput(w io.Writer) error {
	//TODO implement me
	return nil
}
