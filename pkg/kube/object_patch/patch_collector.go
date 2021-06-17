package object_patch

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type patchCollector struct {
	patchOperations []OperationSpec
}

// NewPatchCollector creates OperationSpec collector which is compatible with ObjectPatcher interface
func NewPatchCollector() *patchCollector {
	return &patchCollector{
		patchOperations: make([]OperationSpec, 0),
	}
}

func (dop *patchCollector) CreateOrUpdateObject(object *unstructured.Unstructured, subresource string) error {
	return dop.create(CreateOrUpdate, object, subresource)
}

func (dop *patchCollector) CreateObject(object *unstructured.Unstructured, subresource string) error {
	return dop.create(Create, object, subresource)
}

// Deprecated: use MergePatch instead
func (dop *patchCollector) JQPatchObject(jqPatch, apiVersion, kind, namespace, name, subresource string) error {
	patch := OperationSpec{
		Operation:   JQPatch,
		ApiVersion:  apiVersion,
		Kind:        kind,
		Namespace:   namespace,
		Name:        name,
		Subresource: subresource,
		JQFilter:    jqPatch,
	}

	dop.patchOperations = append(dop.patchOperations, patch)

	return nil
}

func (dop *patchCollector) MergePatchObject(mergePatch []byte, apiVersion, kind, namespace, name, subresource string) error {
	var patch map[string]interface{}
	err := json.Unmarshal(mergePatch, &patch)
	if err != nil {
		return err
	}

	op := OperationSpec{
		Operation:   MergePatch,
		ApiVersion:  apiVersion,
		Kind:        kind,
		Namespace:   namespace,
		Name:        name,
		Subresource: subresource,
		MergePatch:  patch,
	}

	dop.patchOperations = append(dop.patchOperations, op)
	return nil
}

func (dop *patchCollector) JSONPatchObject(jsonPatch []byte, apiVersion, kind, namespace, name, subresource string) error {
	var patch []interface{}
	err := json.Unmarshal(jsonPatch, &patch)
	if err != nil {
		return err
	}

	op := OperationSpec{
		Operation:   MergePatch,
		ApiVersion:  apiVersion,
		Kind:        kind,
		Namespace:   namespace,
		Name:        name,
		Subresource: subresource,
		JSONPatch:   patch,
	}

	dop.patchOperations = append(dop.patchOperations, op)
	return nil
}

func (dop *patchCollector) DeleteObject(apiVersion, kind, namespace, name, subresource string) error {
	return dop.delete(Delete, apiVersion, kind, namespace, name, subresource)
}

func (dop *patchCollector) DeleteObjectInBackground(apiVersion, kind, namespace, name, subresource string) error {
	return dop.delete(DeleteInBackground, apiVersion, kind, namespace, name, subresource)
}

func (dop *patchCollector) DeleteObjectNonCascading(apiVersion, kind, namespace, name, subresource string) error {
	return dop.delete(DeleteNonCascading, apiVersion, kind, namespace, name, subresource)
}

func (dop *patchCollector) FilterObject(
	filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error),
	apiVersion, kind, namespace, name, subresource string,
) error {

	if filterFunc == nil {
		return fmt.Errorf("FilterFunc could not be nil")
	}
	op := OperationSpec{
		Operation:   Filter,
		ApiVersion:  apiVersion,
		Kind:        kind,
		Namespace:   namespace,
		Name:        name,
		Subresource: subresource,
		FilterFunc:  filterFunc,
	}
	dop.patchOperations = append(dop.patchOperations, op)

	return nil
}

// Operations returns all stored operations
func (dop *patchCollector) Operations() []OperationSpec {
	return dop.patchOperations
}

func (dop *patchCollector) delete(operation OperationType, apiVersion, kind, namespace, name, subresource string) error {
	op := OperationSpec{
		Operation:   operation,
		ApiVersion:  apiVersion,
		Kind:        kind,
		Namespace:   namespace,
		Name:        name,
		Subresource: subresource,
	}

	dop.patchOperations = append(dop.patchOperations, op)

	return nil
}

func (dop *patchCollector) create(operation OperationType, object *unstructured.Unstructured, subresource string) error {
	op := OperationSpec{
		Operation:   operation,
		ApiVersion:  object.GetAPIVersion(),
		Kind:        object.GetKind(),
		Namespace:   object.GetNamespace(),
		Name:        object.GetName(),
		Subresource: subresource,
		Object:      object.Object,
	}

	dop.patchOperations = append(dop.patchOperations, op)

	return nil
}
