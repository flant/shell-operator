package object_patch

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type OperationOption func(operation *Operation)

// WithSubresource options specifies a subresource to operate on.
func WithSubresource(subresource string) OperationOption {
	return func(operation *Operation) {
		operation.subresource = subresource
	}
}

func UseFilterFunc(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error)) OperationOption {
	return func(operation *Operation) {
		operation.filterFunc = filterFunc
	}
}

func UseJQPatch(jqPatch string) OperationOption {
	return func(operation *Operation) {
		operation.filterFunc = func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return applyJQPatch(jqPatch, obj)
		}
	}
}

func UseMergePatch(patch []byte) OperationOption {
	return func(operation *Operation) {
		operation.patch = patch
		operation.patchType = types.MergePatchType
	}
}

func UseJSONPatch(patch []byte) OperationOption {
	return func(operation *Operation) {
		operation.patch = patch
		operation.patchType = types.JSONPatchType
	}
}

func IgnoreMissingObject() OperationOption {
	return func(operation *Operation) {
		operation.ignoreMissingObject = true
	}
}

func WithIgnoreMissingObject(ignore bool) OperationOption {
	return func(operation *Operation) {
		operation.ignoreMissingObject = ignore
	}
}

// IgnoreIfExists is an option for Create to not return error if object is already exists.
func IgnoreIfExists() OperationOption {
	return func(operation *Operation) {
		operation.ignoreIfExists = true
	}
}

// UpdateIfExists is an option for Create to update object if it already exists.
func UpdateIfExists() OperationOption {
	return func(operation *Operation) {
		operation.updateIfExists = true
	}
}

// InForeground is a default propagation option for Delete
func InForeground() OperationOption {
	return func(operation *Operation) {
		operation.deletionPropagation = metav1.DeletePropagationForeground
	}
}

// InBackground is a propagation option for Delete
func InBackground() OperationOption {
	return func(operation *Operation) {
		operation.deletionPropagation = metav1.DeletePropagationBackground
	}
}

// NonCascading is a propagation option for Delete
func NonCascading() OperationOption {
	return func(operation *Operation) {
		operation.deletionPropagation = metav1.DeletePropagationOrphan
	}
}
