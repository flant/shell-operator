package object_patch

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
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

	Object     map[string]interface{} `json:"object,omitempty" yaml:"object,omitempty"`
	JQFilter   string                 `json:"jqFilter,omitempty" yaml:"jqFilter,omitempty"`
	MergePatch map[string]interface{} `json:"mergePatch,omitempty" yaml:"mergePatch,omitempty"`
	JSONPatch  []interface{}          `json:"jsonPatch,omitempty" yaml:"jsonPatch,omitempty"`

	IgnoreMissingObject bool `json:"ignoreMissingObject" yaml:"ignoreMissingObject"`
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

func ParseOperations(specBytes []byte) ([]*Operation, error) {
	log.Debugf("parsing patcher operations:\n%s", specBytes)

	specs, err := unmarshalFromJSONOrYAML(specBytes)
	if err != nil {
		return nil, err
	}

	var validationErrors = &multierror.Error{}
	var ops = make([]*Operation, 0)
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

// operation is a command for ObjectPatcher. There are 4 types of command:
// - create(update) object. object field must be set.
// - delete object. deletionPropagation should be set.
// - patch object with API call Patch. patchType should be set.
// - filter object. filterFunc should be set.
type Operation struct {
	// Object coordinates for patch and delete.
	apiVersion  string
	kind        string
	namespace   string
	name        string
	subresource string

	// Create options.
	object         interface{}
	ignoreIfExists bool
	updateIfExists bool

	// Patch options.
	filterFunc          func(*unstructured.Unstructured) (*unstructured.Unstructured, error)
	patchType           types.PatchType
	patch               interface{}
	ignoreMissingObject bool

	// Delete options.
	deletionPropagation metav1.DeletionPropagation
}

func (op *Operation) applyOptions(options ...OperationOption) {
	for _, option := range options {
		option(op)
	}
}

func (op *Operation) isCreate() bool {
	return op.object != nil
}

func (op *Operation) isDelete() bool {
	return op.deletionPropagation != ""
}

func (op *Operation) isPatch() bool {
	return op.patchType != ""
}

func (op *Operation) isFilter() bool {
	return op.filterFunc != nil
}

func (op *Operation) Description() string {
	if op.isCreate() {
		return "Create object"
	}
	if op.isDelete() {
		return fmt.Sprintf("Delete object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
	}
	if op.isPatch() {
		return fmt.Sprintf("Patch object %s/%s/%s/%s using %s patch", op.apiVersion, op.kind, op.namespace, op.name, op.patchType)
	}
	if op.isFilter() {
		return fmt.Sprintf("Filter object %s/%s/%s/%s", op.apiVersion, op.kind, op.namespace, op.name)
	}
	return "Unknown operation"
}

//		operationError = o.Create(&unstructured.Unstructured{Object: spec.Object},

func NewFromOperationSpec(spec OperationSpec) *Operation {
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
		return NewFilterPatchOperation(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return applyJQPatch(spec.JQFilter, u)
		},
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			WithIgnoreMissingObject(spec.IgnoreMissingObject),
		)
	case MergePatch:
		return NewMergePatchOperation(spec.MergePatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			WithIgnoreMissingObject(spec.IgnoreMissingObject),
		)
		//mergePatch, err := json.Marshal(spec.MergePatch)
		//if err != nil {
		//	applyErrors = multierror.Append(applyErrors, err)
		//	continue
		//}
		//operationError = o.Patch(spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
		//	WithSubresource(spec.Subresource),
		//	UseMergePatch(mergePatch),
		//	WithIgnoreMissingObject(spec.IgnoreMissingObject),
		//)
	case JSONPatch:
		return NewJSONPatchOperation(spec.JSONPatch,
			spec.ApiVersion, spec.Kind, spec.Namespace, spec.Name,
			WithSubresource(spec.Subresource),
			WithIgnoreMissingObject(spec.IgnoreMissingObject),
		)
		//jsonPatch, err := json.Marshal(spec.JSONPatch)
	}

	// Should not be reached!
	return nil
}

func NewCreateOperation(obj interface{}, options ...OperationOption) *Operation {
	op := &Operation{
		object: obj,
	}
	op.applyOptions(options...)
	return op
}

func NewDeleteOperation(apiVersion, kind, namespace, name string, options ...OperationOption) *Operation {
	op := &Operation{
		apiVersion:          apiVersion,
		kind:                kind,
		namespace:           namespace,
		name:                name,
		deletionPropagation: metav1.DeletePropagationForeground,
	}
	op.applyOptions(options...)
	return op
}

func NewMergePatchOperation(mergePatch interface{}, apiVersion, kind, namespace, name string, options ...OperationOption) *Operation {
	op := &Operation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		patch:      mergePatch,
		patchType:  types.MergePatchType,
	}
	op.applyOptions(options...)
	return op
}

func NewJSONPatchOperation(jsonPatch interface{}, apiVersion, kind, namespace, name string, options ...OperationOption) *Operation {
	op := &Operation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		patch:      jsonPatch,
		patchType:  types.JSONPatchType,
	}
	op.applyOptions(options...)
	return op
}

func NewFilterPatchOperation(filter func(*unstructured.Unstructured) (*unstructured.Unstructured, error), apiVersion, kind, namespace, name string, options ...OperationOption) *Operation {
	op := &Operation{
		apiVersion: apiVersion,
		kind:       kind,
		namespace:  namespace,
		name:       name,
		filterFunc: filter,
	}
	op.applyOptions(options...)
	return op
}
