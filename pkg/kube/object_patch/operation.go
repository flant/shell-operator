package object_patch

import (
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/log"
	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	"github.com/hashicorp/go-multierror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/filter/jq"
)

// OperationSpec a JSON and YAML representation of the operation for shell hooks
type OperationSpec struct {
	Operation   OperationType `json:"operation" yaml:"operation"`
	ApiVersion  string        `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind        string        `json:"kind,omitempty" yaml:"kind,omitempty"`
	Namespace   string        `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name        string        `json:"name,omitempty" yaml:"name,omitempty"`
	Subresource string        `json:"subresource,omitempty" yaml:"subresource,omitempty"`

	Object     any    `json:"object,omitempty" yaml:"object,omitempty"`
	JQFilter   string `json:"jqFilter,omitempty" yaml:"jqFilter,omitempty"`
	MergePatch any    `json:"mergePatch,omitempty" yaml:"mergePatch,omitempty"`
	JSONPatch  any    `json:"jsonPatch,omitempty" yaml:"jsonPatch,omitempty"`

	IgnoreMissingObject bool `json:"ignoreMissingObject" yaml:"ignoreMissingObject"`
	IgnoreHookError     bool `json:"ignoreHookError" yaml:"ignoreHookError"`
}

type OperationType string

const (
	CreateOrUpdate    OperationType = "CreateOrUpdate"
	Create            OperationType = "Create"
	CreateIfNotExists OperationType = "CreateIfNotExists"

	Delete             OperationType = "Delete"
	DeleteInBackground OperationType = "DeleteInBackground"
	DeleteNonCascading OperationType = "DeleteNonCascading"

	JQPatch    OperationType = "JQPatch"
	MergePatch OperationType = "MergePatch"
	JSONPatch  OperationType = "JSONPatch"
)

// GetPatchStatusOperationsOnHookError returns list of Patch/Filter operations eligible for execution on Hook Error
func GetPatchStatusOperationsOnHookError(operations []Operation) []Operation {
	patchStatusOperations := make([]Operation, 0)
	for _, op := range operations {
		switch operation := op.(type) {
		case *FilterOperation:
			if operation.subresource == "/status" && operation.ignoreHookError {
				patchStatusOperations = append(patchStatusOperations, operation)
			}
		case *PatchOperation:
			if operation.subresource == "/status" && operation.ignoreHookError {
				patchStatusOperations = append(patchStatusOperations, operation)
			}
		}
	}

	return patchStatusOperations
}

func ParseOperations(specBytes []byte) ([]Operation, error) {
	log.Debug("parsing patcher operations", slog.String("value", string(specBytes)))

	specs, err := unmarshalFromJSONOrYAML(specBytes)
	if err != nil {
		return nil, err
	}

	validationErrors := &multierror.Error{}
	ops := make([]Operation, 0)
	for _, spec := range specs {
		err = ValidateOperationSpec(spec, GetSchema("v0"), "")
		if err != nil {
			validationErrors = multierror.Append(validationErrors, err)
			break
		}
		ops = append(ops, NewFromOperationSpec(spec))
	}

	return ops, validationErrors.ErrorOrNil()
}

// Operation is a command for ObjectPatcher.
//
// There are 4 types of operations:
//
// - createOperation to create or update object via Create and Update API calls. Unstructured, map[string]any or runtime.Object is required.
//
// - deleteOperation to delete object via Delete API call. deletionPropagation should be set, default is Foregound.
//
// - patchOperation to modify object via Patch API call. patchType should be set. patch can be string, []byte or map[string]any
//
// - filterOperation to modify object via Get-filter-Update process. filterFunc should be set.
type Operation interface {
	Description() string
}

type CreateOperation struct {
	object      any
	subresource string

	ignoreIfExists bool
	updateIfExists bool
}

func (op *CreateOperation) Description() string {
	return "Create object"
}

func (op *CreateOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

func (op *CreateOperation) WithIgnoreIfExists(ignore bool) {
	op.ignoreIfExists = ignore
}

func (op *CreateOperation) WithUpdateIfExists(update bool) {
	op.updateIfExists = update
}

type DeleteOperation struct {
	// Object coordinates.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Delete options.
	deletionPropagation metav1.DeletionPropagation
}

func (op *DeleteOperation) Description() string {
	return fmt.Sprintf("Delete object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
}

func (op *DeleteOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

type PatchOperation struct {
	// Object coordinates for patch and delete.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Patch options.
	patchType           types.PatchType
	patch               any
	ignoreMissingObject bool
	ignoreHookError     bool
}

func (op *PatchOperation) Description() string {
	return fmt.Sprintf("Patch object %s/%s/%s/%s using %s patch", op.apiVersion, op.kind, op.namespace, op.name, op.patchType)
}

func (op *PatchOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

func (op *PatchOperation) WithIgnoreMissingObject(ignore bool) {
	op.ignoreMissingObject = ignore
}

func (op *PatchOperation) WithIgnoreHookError(ignore bool) {
	op.ignoreHookError = ignore
}

type FilterOperation struct {
	// Object coordinates for patch and delete.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Patch options.
	filterFunc          func(*unstructured.Unstructured) (*unstructured.Unstructured, error)
	ignoreMissingObject bool
	ignoreHookError     bool
}

func (op *FilterOperation) Description() string {
	return fmt.Sprintf("Filter object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
}

func (op *FilterOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

func (op *FilterOperation) WithIgnoreMissingObject(ignore bool) {
	op.ignoreMissingObject = ignore
}

func (op *FilterOperation) WithIgnoreHookError(ignore bool) {
	op.ignoreHookError = ignore
}

func NewFromOperationSpec(spec OperationSpec) Operation {
	switch spec.Operation {
	case Create:
		return NewCreateOperation(spec.Object,
			CreateWithSubresource(spec.Subresource))
	case CreateIfNotExists:
		return NewCreateIfNotExistsOperation(spec.Object,
			CreateWithSubresource(spec.Subresource),
			CreateWithIgnoreIfExists(true))
	case CreateOrUpdate:
		return NewCreateOrUpdateOperation(spec.Object,
			CreateWithSubresource(spec.Subresource),
			CreateWithUpdateIfExists(true))
	case Delete:
		return NewDeleteOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			DeleteWithSubresource(spec.Subresource))
	case DeleteInBackground:
		return NewDeleteInBackgroundOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			DeleteWithSubresource(spec.Subresource))
	case DeleteNonCascading:
		return NewDeleteNonCascadingOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			DeleteWithSubresource(spec.Subresource))
	case JQPatch:
		return NewJQPatchOperation(
			spec.JQFilter,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			FilterWithSubresource(spec.Subresource),
			FilterWithIgnoreMissingObject(spec.IgnoreMissingObject),
			FilterWithIgnoreHookError(spec.IgnoreHookError),
		)
	case MergePatch:
		return NewMergePatchOperation(spec.MergePatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			PatchWithSubresource(spec.Subresource),
			PatchWithIgnoreMissingObject(spec.IgnoreMissingObject),
			PatchWithIgnoreHookError(spec.IgnoreHookError),
		)
	case JSONPatch:
		return NewJSONPatchOperation(spec.JSONPatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			PatchWithSubresource(spec.Subresource),
			PatchWithIgnoreMissingObject(spec.IgnoreMissingObject),
			PatchWithIgnoreHookError(spec.IgnoreHookError),
		)
	}

	// Should not be reached!
	return nil
}

func NewCreateOperation(obj any, opts ...sdkpkg.PatchCollectorCreateOption) Operation {
	return newCreateOperation(Create, obj, opts...)
}

func NewCreateOrUpdateOperation(obj any, opts ...sdkpkg.PatchCollectorCreateOption) Operation {
	return newCreateOperation(CreateOrUpdate, obj, opts...)
}

func NewCreateIfNotExistsOperation(obj any, opts ...sdkpkg.PatchCollectorCreateOption) Operation {
	return newCreateOperation(CreateIfNotExists, obj, opts...)
}

func newCreateOperation(operation OperationType, obj any, opts ...sdkpkg.PatchCollectorCreateOption) Operation {
	op := &CreateOperation{
		object: obj,
	}

	switch operation {
	case Create:
		// pass
	case CreateOrUpdate:
		op.updateIfExists = true
	case CreateIfNotExists:
		op.ignoreIfExists = true
	}

	for _, opt := range opts {
		opt.Apply(op)
	}

	return op
}

func NewDeleteOperation(apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorDeleteOption) Operation {
	return newDeleteOperation(metav1.DeletePropagationForeground, apiVersion, kind, namespace, name, opts...)
}

func NewDeleteInBackgroundOperation(apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorDeleteOption) Operation {
	return newDeleteOperation(metav1.DeletePropagationBackground, apiVersion, kind, namespace, name, opts...)
}

func NewDeleteNonCascadingOperation(apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorDeleteOption) Operation {
	return newDeleteOperation(metav1.DeletePropagationOrphan, apiVersion, kind, namespace, name, opts...)
}

func newDeleteOperation(propagation metav1.DeletionPropagation, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorDeleteOption) Operation {
	op := &DeleteOperation{
		apiVersion:          apiVersion,
		kind:                kind,
		namespace:           namespace,
		name:                name,
		deletionPropagation: propagation,
	}

	for _, opt := range opts {
		opt.Apply(op)
	}

	return op
}

func NewMergePatchOperation(mergePatch any, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorPatchOption) Operation {
	return newPatchOperation(types.MergePatchType, mergePatch, apiVersion, kind, namespace, name, opts...)
}

func NewJSONPatchOperation(jsonpatch any, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorPatchOption) Operation {
	return newPatchOperation(types.JSONPatchType, jsonpatch, apiVersion, kind, namespace, name, opts...)
}

func newPatchOperation(patchType types.PatchType, patch any, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorPatchOption) Operation {
	op := &PatchOperation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		patch:      patch,
		patchType:  patchType,
	}

	for _, opt := range opts {
		opt.Apply(op)
	}

	return op
}

func NewJQPatchOperation(jqfilter string, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorFilterOption) Operation {
	return newFilterOperation(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		filter := jq.NewFilter(app.JqLibraryPath)
		return applyJQPatch(jqfilter, filter, u)
	}, apiVersion, kind, namespace, name, opts...)
}

func NewFilterPatchOperation(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorFilterOption) Operation {
	return newFilterOperation(filterFunc, apiVersion, kind, namespace, name, opts...)
}

func newFilterOperation(filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorFilterOption) Operation {
	op := &FilterOperation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		filterFunc: filterFunc,
	}

	for _, opt := range opts {
		opt.Apply(op)
	}

	return op
}
