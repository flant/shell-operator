package object_patch

import (
	"fmt"

	log "github.com/deckhouse/deckhouse/go_lib/log"
	"github.com/hashicorp/go-multierror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// A JSON and YAML representation of the operation for shell hooks
type OperationSpec struct {
	Operation   OperationType `json:"operation" yaml:"operation"`
	ApiVersion  string        `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind        string        `json:"kind,omitempty" yaml:"kind,omitempty"`
	Namespace   string        `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name        string        `json:"name,omitempty" yaml:"name,omitempty"`
	Subresource string        `json:"subresource,omitempty" yaml:"subresource,omitempty"`

	Object     interface{} `json:"object,omitempty" yaml:"object,omitempty"`
	JQFilter   string      `json:"jqFilter,omitempty" yaml:"jqFilter,omitempty"`
	MergePatch interface{} `json:"mergePatch,omitempty" yaml:"mergePatch,omitempty"`
	JSONPatch  interface{} `json:"jsonPatch,omitempty" yaml:"jsonPatch,omitempty"`

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
		case *filterOperation:
			if operation.subresource == "/status" && operation.ignoreHookError {
				patchStatusOperations = append(patchStatusOperations, operation)
			}
		case *patchOperation:
			if operation.subresource == "/status" && operation.ignoreHookError {
				patchStatusOperations = append(patchStatusOperations, operation)
			}
		}
	}

	return patchStatusOperations
}

func ParseOperations(specBytes []byte) ([]Operation, error) {
	log.Debugf("parsing patcher operations:\n%s", specBytes)

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
// - createOperation to create or update object via Create and Update API calls. Unstructured, map[string]interface{} or runtime.Object is required.
//
// - deleteOperation to delete object via Delete API call. deletionPropagation should be set, default is Foregound.
//
// - patchOperation to modify object via Patch API call. patchType should be set. patch can be string, []byte or map[string]interface{}
//
// - filterOperation to modify object via Get-filter-Update process. filterFunc should be set.
type Operation interface {
	Description() string
}

type createOperation struct {
	object      interface{}
	subresource string

	ignoreIfExists bool
	updateIfExists bool
}

func (op *createOperation) Description() string {
	return "Create object"
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

type patchOperation struct {
	// Object coordinates for patch and delete.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Patch options.
	patchType           types.PatchType
	patch               interface{}
	ignoreMissingObject bool
	ignoreHookError     bool
}

func (op *patchOperation) Description() string {
	return fmt.Sprintf("Patch object %s/%s/%s/%s using %s patch", op.apiVersion, op.kind, op.namespace, op.name, op.patchType)
}

type filterOperation struct {
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

func (op *filterOperation) Description() string {
	return fmt.Sprintf("Filter object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
}

func NewFromOperationSpec(spec OperationSpec) Operation {
	switch spec.Operation {
	case Create:
		return NewCreateOperation(spec.Object,
			WithSubresource(spec.Subresource))
	case CreateIfNotExists:
		return NewCreateOperation(spec.Object,
			WithSubresource(spec.Subresource),
			IgnoreIfExists())
	case CreateOrUpdate:
		return NewCreateOperation(spec.Object,
			WithSubresource(spec.Subresource),
			UpdateIfExists())
	case Delete:
		return NewDeleteOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource))
	case DeleteInBackground:
		return NewDeleteOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			InBackground())
	case DeleteNonCascading:
		return NewDeleteOperation(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			NonCascading())
	case JQPatch:
		return NewFilterPatchOperation(
			func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				return applyJQPatch(spec.JQFilter, u)
			},
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			WithIgnoreMissingObject(spec.IgnoreMissingObject),
			WithIgnoreHookError(spec.IgnoreHookError),
		)
	case MergePatch:
		return NewMergePatchOperation(spec.MergePatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			WithIgnoreMissingObject(spec.IgnoreMissingObject),
			WithIgnoreHookError(spec.IgnoreHookError),
		)
	case JSONPatch:
		return NewJSONPatchOperation(spec.JSONPatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			WithIgnoreMissingObject(spec.IgnoreMissingObject),
			WithIgnoreHookError(spec.IgnoreHookError),
		)
	}

	// Should not be reached!
	return nil
}

func NewCreateOperation(obj interface{}, options ...CreateOption) Operation {
	op := &createOperation{
		object: obj,
	}
	for _, option := range options {
		option.applyToCreate(op)
	}
	return op
}

func NewDeleteOperation(apiVersion, kind, namespace, name string, options ...DeleteOption) Operation {
	op := &deleteOperation{
		apiVersion:          apiVersion,
		kind:                kind,
		namespace:           namespace,
		name:                name,
		deletionPropagation: metav1.DeletePropagationForeground,
	}
	for _, option := range options {
		option.applyToDelete(op)
	}
	return op
}

func NewMergePatchOperation(mergePatch interface{}, apiVersion, kind, namespace, name string, options ...PatchOption) Operation {
	op := &patchOperation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		patch:      mergePatch,
		patchType:  types.MergePatchType,
	}
	for _, option := range options {
		option.applyToPatch(op)
	}
	return op
}

func NewJSONPatchOperation(jsonPatch interface{}, apiVersion, kind, namespace, name string, options ...PatchOption) Operation {
	op := &patchOperation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		patch:      jsonPatch,
		patchType:  types.JSONPatchType,
	}
	for _, option := range options {
		option.applyToPatch(op)
	}
	return op
}

func NewFilterPatchOperation(filter func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion, kind, namespace, name string, options ...FilterOption) Operation {
	op := &filterOperation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		filterFunc: filter,
	}
	for _, option := range options {
		option.applyToFilter(op)
	}
	return op
}
