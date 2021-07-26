package object_patch

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type patchCollector struct {
	patchOperations []*Operation
}

// NewPatchCollector creates OperationSpec collector which is compatible with ObjectPatcher interface
func NewPatchCollector() *patchCollector {
	return &patchCollector{
		patchOperations: make([]*Operation, 0),
	}
}

func (dop *patchCollector) Create(object interface{}, options ...OperationOption) {
	dop.add(NewCreateOperation(object, options...))
}

func (dop *patchCollector) Delete(apiVersion, kind, namespace, name string, options ...OperationOption) {
	dop.add(NewDeleteOperation(apiVersion, kind, namespace, name, options...))
}

func (dop *patchCollector) MergePatch(mergePatch []byte, apiVersion, kind, namespace, name string, options ...OperationOption) {
	dop.add(NewMergePatchOperation(mergePatch, apiVersion, kind, namespace, name, options...))
}

func (dop *patchCollector) JSONPatch(jsonPatch interface{}, apiVersion, kind, namespace, name string, options ...OperationOption) {
	dop.add(NewJSONPatchOperation(jsonPatch, apiVersion, kind, namespace, name, options...))
}

func (dop *patchCollector) Filter(
	filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
	apiVersion, kind, namespace, name string, options ...OperationOption,
) {
	dop.patchOperations = append(dop.patchOperations, NewFilterPatchOperation(filterFunc, apiVersion, kind, namespace, name, options...))
}

// Operations returns all collected operations
func (dop *patchCollector) Operations() []*Operation {
	return dop.patchOperations
}

func (dop *patchCollector) add(operation *Operation) {
	dop.patchOperations = append(dop.patchOperations, operation)
}
