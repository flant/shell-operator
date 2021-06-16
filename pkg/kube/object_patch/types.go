package object_patch

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

	// FilterFunc makes sense only for OperationType==Filter, mutate objects in go hooks
	FilterFunc func(*unstructured.Unstructured) (*unstructured.Unstructured, error) `json:"filterFunc,omitempty" yaml:"filterFunc,omitempty"`
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

	Filter OperationType = "Filter"
)
