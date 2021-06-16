package object_patch

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type specGenerator struct {
	patchOperations []OperationSpec
}

// NewSpecGenerator creates OperationSpec generator which is compatible with ObjectPatcher interface
func NewSpecGenerator() *specGenerator {
	return &specGenerator{
		patchOperations: make([]OperationSpec, 0),
	}
}

func (dop *specGenerator) CreateOrUpdateObject(object *unstructured.Unstructured, subresource string) error {
	return dop.create(CreateOrUpdate, object, subresource)
}

func (dop *specGenerator) CreateObject(object *unstructured.Unstructured, subresource string) error {
	return dop.create(Create, object, subresource)
}

// Deprecated: use MergePatch instead
func (dop *specGenerator) JQPatchObject(jqPatch, apiVersion, kind, namespace, name, subresource string) error {
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

func (dop *specGenerator) MergePatchObject(mergePatch []byte, apiVersion, kind, namespace, name, subresource string) error {
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

func (dop *specGenerator) JSONPatchObject(jsonPatch []byte, apiVersion, kind, namespace, name, subresource string) error {
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

func (dop *specGenerator) DeleteObject(apiVersion, kind, namespace, name, subresource string) error {
	return dop.delete(Delete, apiVersion, kind, namespace, name, subresource)
}

func (dop *specGenerator) DeleteObjectInBackground(apiVersion, kind, namespace, name, subresource string) error {
	return dop.delete(DeleteInBackground, apiVersion, kind, namespace, name, subresource)
}

func (dop *specGenerator) DeleteObjectNonCascading(apiVersion, kind, namespace, name, subresource string) error {
	return dop.delete(DeleteNonCascading, apiVersion, kind, namespace, name, subresource)
}

func (dop *specGenerator) FilterObject(
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
func (dop *specGenerator) Operations() []OperationSpec {
	return dop.patchOperations
}

func (dop *specGenerator) delete(operation OperationType, apiVersion, kind, namespace, name, subresource string) error {
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

func (dop *specGenerator) create(operation OperationType, object *unstructured.Unstructured, subresource string) error {
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
