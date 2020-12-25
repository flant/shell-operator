package object_patch

type OperationSpec struct {
	Operation   OperationType `json:"operation" yaml:"operation"`
	ApiVersion  string        `json:"apiVersion" yaml:"apiVersion"`
	Kind        string        `json:"kind" yaml:"kind"`
	Namespace   string        `json:"namespace" yaml:"namespace"`
	Name        string        `json:"name" yaml:"name"`
	Subresource string        `json:"subresource" yaml:"subresource"`

	Object     map[string]interface{} `json:"object" yaml:"object"`
	JQFilter   string                 `json:"jqFilter" yaml:"jqFilter"`
	MergePatch map[string]interface{} `json:"mergePatch" yaml:"mergePatch"`
	JSONPatch  []interface{}          `json:"jsonPatch" yaml:"jsonPatch"`
}

type OperationType string

const (
	CreateOrUpdate OperationType = "CreateOrUpdate"
	Create         OperationType = "Create"

	Delete             OperationType = "Delete"
	DeleteInBackground OperationType = "DeleteInBackground"
	DeleteNonCascading OperationType = "DeleteNonCascading"

	JQPatch    OperationType = "JQFilter"
	MergePatch OperationType = "MergePatch"
	JSONPatch  OperationType = "JSONPatch"
)
