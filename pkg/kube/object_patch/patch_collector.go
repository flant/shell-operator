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

func (dop *patchCollector) Create(object interface{}, options ...CreateOption) {
	dop.add(NewCreateOperation(object, options...))
}

func (dop *patchCollector) Delete(apiVersion, kind, namespace, name string, options ...DeleteOption) {
	dop.add(NewDeleteOperation(apiVersion, kind, namespace, name, options...))
}

func (dop *patchCollector) MergePatch(mergePatch interface{}, apiVersion, kind, namespace, name string, options ...PatchOption) {
	dop.add(NewMergePatchOperation(mergePatch, apiVersion, kind, namespace, name, options...))
}

func (dop *patchCollector) JSONPatch(jsonPatch interface{}, apiVersion, kind, namespace, name string, options ...PatchOption) {
	dop.add(NewJSONPatchOperation(jsonPatch, apiVersion, kind, namespace, name, options...))
}

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
