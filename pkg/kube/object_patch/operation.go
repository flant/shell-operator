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
func GetPatchStatusOperationsOnHookError(operations []sdkpkg.PatchCollectorOperation) []sdkpkg.PatchCollectorOperation {
	patchStatusOperations := make([]sdkpkg.PatchCollectorOperation, 0)
	for _, op := range operations {
		operation, ok := op.(*patchOperation)
		if ok && operation.subresource == "/status" && operation.ignoreHookError {
			patchStatusOperations = append(patchStatusOperations, operation)
		}
	}

	return patchStatusOperations
}

func ParseOperations(specBytes []byte) ([]sdkpkg.PatchCollectorOperation, error) {
	log.Debug("parsing patcher operations", slog.String("value", string(specBytes)))

	specs, err := unmarshalFromJSONOrYAML(specBytes)
	if err != nil {
		return nil, err
	}

	validationErrors := &multierror.Error{}
	ops := make([]sdkpkg.PatchCollectorOperation, 0)
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

type createOperation struct {
	object      any
	subresource string

	ignoreIfExists bool
	updateIfExists bool
}

func (op *createOperation) Description() string {
	return "Create object"
}

func (op *createOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

func (op *createOperation) WithIgnoreIfExists(ignore bool) {
	op.ignoreIfExists = ignore
}

func (op *createOperation) WithUpdateIfExists(update bool) {
	op.updateIfExists = update
}

type deleteOperation struct {
	// Object coordinates.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Delete options.
	deletionPropagation metav1.DeletionPropagation
}

func (op *deleteOperation) Description() string {
	return fmt.Sprintf("Delete object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
}

func (op *deleteOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

type patchOperation struct {
	// Object coordinates for patch and delete.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Patch options.
	patchType types.PatchType
	patch     any

	// using only if not JSON or Merge patch type
	filterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error)

	ignoreMissingObject bool
	ignoreHookError     bool
}

func (op *patchOperation) Description() string {
	return fmt.Sprintf("Filter object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
}

func (op *patchOperation) hasFilterFn() bool {
	return op.filterFunc != nil
}

func (op *patchOperation) WithSubresource(subresource string) {
	op.subresource = subresource
}

func (op *patchOperation) WithIgnoreMissingObject(ignore bool) {
	op.ignoreMissingObject = ignore
}

func (op *patchOperation) WithIgnoreHookError(ignore bool) {
	op.ignoreHookError = ignore
}

func NewFromOperationSpec(spec OperationSpec) sdkpkg.PatchCollectorOperation {
	switch spec.Operation {
	case Create:
		return NewCreateOperation(spec.Object)
	case CreateIfNotExists:
		return NewCreateIfNotExistsOperation(spec.Object)
	case CreateOrUpdate:
		return NewCreateOrUpdateOperation(spec.Object)
	case Delete:
		return NewDeleteOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name)
	case DeleteInBackground:
		return NewDeleteInBackgroundOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name)
	case DeleteNonCascading:
		return NewDeleteNonCascadingOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name)
	case JQPatch:
		return NewPatchWithJQOperation(
			spec.JQFilter,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			withIgnoreMissingObject(spec.IgnoreMissingObject),
			withIgnoreHookError(spec.IgnoreHookError),
		)
	case MergePatch:
		return NewMergePatchOperation(spec.MergePatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			withIgnoreMissingObject(spec.IgnoreMissingObject),
			withIgnoreHookError(spec.IgnoreHookError),
		)
	case JSONPatch:
		return NewJSONPatchOperation(spec.JSONPatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			withIgnoreMissingObject(spec.IgnoreMissingObject),
			withIgnoreHookError(spec.IgnoreHookError),
		)
	}

	// Should not be reached!
	return nil
}

func NewCreateOperation(obj any) sdkpkg.PatchCollectorOperation {
	return newCreateOperation(Create, obj)
}

func NewCreateOrUpdateOperation(obj any) sdkpkg.PatchCollectorOperation {
	return newCreateOperation(CreateOrUpdate, obj)
}

func NewCreateIfNotExistsOperation(obj any) sdkpkg.PatchCollectorOperation {
	return newCreateOperation(CreateIfNotExists, obj)
}

func newCreateOperation(operation OperationType, obj any) sdkpkg.PatchCollectorOperation {
	op := &createOperation{
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

	return op
}

func NewDeleteOperation(apiVersion string, kind string, namespace string, name string) sdkpkg.PatchCollectorOperation {
	return newDeleteOperation(metav1.DeletePropagationForeground, apiVersion, kind, namespace, name)
}

func NewDeleteInBackgroundOperation(apiVersion string, kind string, namespace string, name string) sdkpkg.PatchCollectorOperation {
	return newDeleteOperation(metav1.DeletePropagationBackground, apiVersion, kind, namespace, name)
}

func NewDeleteNonCascadingOperation(apiVersion string, kind string, namespace string, name string) sdkpkg.PatchCollectorOperation {
	return newDeleteOperation(metav1.DeletePropagationOrphan, apiVersion, kind, namespace, name)
}

func newDeleteOperation(propagation metav1.DeletionPropagation, apiVersion, kind, namespace, name string) sdkpkg.PatchCollectorOperation {
	op := &deleteOperation{
		apiVersion:          apiVersion,
		kind:                kind,
		namespace:           namespace,
		name:                name,
		deletionPropagation: propagation,
	}

	return op
}

func NewMergePatchOperation(mergePatch any, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) sdkpkg.PatchCollectorOperation {
	return newPatchOperation(types.MergePatchType, mergePatch, apiVersion, kind, namespace, name, opts...)
}

func NewJSONPatchOperation(jsonpatch any, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) sdkpkg.PatchCollectorOperation {
	return newPatchOperation(types.JSONPatchType, jsonpatch, apiVersion, kind, namespace, name, opts...)
}

func newPatchOperation(patchType types.PatchType, patch any, apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorOption) sdkpkg.PatchCollectorOperation {
	op := &patchOperation{
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

func NewPatchWithJQOperation(jqQuery string, apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) sdkpkg.PatchCollectorOperation {
	return newFilterOperation(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		filter := jq.NewFilter(app.JqLibraryPath)
		return applyJQPatch(jqQuery, filter, u)
	}, apiVersion, kind, namespace, name, opts...)
}

func NewPatchWithMutatingFuncOperation(fn func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion string, kind string, namespace string, name string, opts ...sdkpkg.PatchCollectorOption) sdkpkg.PatchCollectorOperation {
	return newFilterOperation(fn, apiVersion, kind, namespace, name, opts...)
}

func newFilterOperation(fn func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion, kind, namespace, name string, opts ...sdkpkg.PatchCollectorOption) sdkpkg.PatchCollectorOperation {
	op := &patchOperation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		filterFunc: fn,
	}

	for _, opt := range opts {
		opt.Apply(op)
	}

	return op
}
