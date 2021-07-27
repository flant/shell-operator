package object_patch

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateOption interface {
	applyToCreate(operation *createOperation)
}

type DeleteOption interface {
	applyToDelete(operation *deleteOperation)
}

type PatchOption interface {
	applyToPatch(operation *patchOperation)
}

type FilterOption interface {
	applyToFilter(operation *filterOperation)
}

type subresourceHolder struct {
	subresource string
}

// WithSubresource options specifies a subresource to operate on.
func WithSubresource(s string) *subresourceHolder {
	return &subresourceHolder{subresource: s}
}
func (s *subresourceHolder) applyToCreate(operation *createOperation) {
	operation.subresource = s.subresource
}
func (s *subresourceHolder) applyToDelete(operation *deleteOperation) {
	operation.subresource = s.subresource
}
func (s *subresourceHolder) applyToPatch(operation *patchOperation) {
	operation.subresource = s.subresource
}
func (s *subresourceHolder) applyToFilter(operation *filterOperation) {
	operation.subresource = s.subresource
}

type ignoreMissingObject struct {
	ignore bool
}

// IgnoreMissingObject do not return error if object exists for Patch and Filter operations.
func IgnoreMissingObject() *ignoreMissingObject {
	return WithIgnoreMissingObject(true)
}

func WithIgnoreMissingObject(ignore bool) *ignoreMissingObject {
	return &ignoreMissingObject{ignore: ignore}
}

func (i *ignoreMissingObject) applyToPatch(operation *patchOperation) {
	operation.ignoreMissingObject = i.ignore
}

func (i *ignoreMissingObject) applyToFilter(operation *filterOperation) {
	operation.ignoreMissingObject = i.ignore
}

type ignoreIfExists struct {
	ignore bool
}

// IgnoreIfExists is an option for Create to not return error if object is already exists.
func IgnoreIfExists() CreateOption {
	return &ignoreIfExists{ignore: true}
}

func (i *ignoreIfExists) applyToCreate(operation *createOperation) {
	operation.ignoreIfExists = i.ignore
}

type updateIfExists struct {
	update bool
}

// UpdateIfExists is an option for Create to update object if it already exists.
func UpdateIfExists() CreateOption {
	return &updateIfExists{update: true}
}

func (u *updateIfExists) applyToCreate(operation *createOperation) {
	operation.updateIfExists = u.update
}

type deletePropogation struct {
	propagation metav1.DeletionPropagation
}

func (d *deletePropogation) applyToDelete(operation *deleteOperation) {
	operation.deletionPropagation = d.propagation
}

// InForeground is a default propagation option for Delete
func InForeground() DeleteOption {
	return &deletePropogation{propagation: metav1.DeletePropagationForeground}
}

// InBackground is a propagation option for Delete
func InBackground() DeleteOption {
	return &deletePropogation{propagation: metav1.DeletePropagationBackground}
}

// NonCascading is a propagation option for Delete
func NonCascading() DeleteOption {
	return &deletePropogation{propagation: metav1.DeletePropagationOrphan}
}
