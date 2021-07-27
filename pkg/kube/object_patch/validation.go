package object_patch

import (
	"encoding/json"
	"fmt"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
	"github.com/hashicorp/go-multierror"
	"sigs.k8s.io/yaml"
)

var Schemas = map[string]string{
	"v0": `
definitions:
  common:
    type: object
    properties:
      subresource:
        type: string
  create:
    required:
    - object
    properties:
      object:
        type: object
        additionalProperties: true
        minProperties: 1
  delete:
    type: object
    required:
    - kind
    - name
    properties:
      apiVersion:
        type: string
      kind:
        type: string
      name:
        type: string
  patch:
    type: object
    required:
    - kind
    - name
    properties:
      apiVersion:
        type: string
      kind:
        type: string
      name:
        type: string
      ignoreMissingObject:
        type: boolean

type: object
additionalProperties: false
properties:
  operation: {}
  namespace: {}
  subresource: {}
  apiVersion: {}
  kind: {}
  name: {}
  object: {}
  jsonPatch: {}
  jqFilter: {}
  mergePatch: {}
  ignoreMissingObject: {}

oneOf:
- allOf:
  - properties:
      operation:
        type: string
        enum: ["Create", "CreateOrUpdate", "CreateIfNotExists"]
  - "$ref": "#/definitions/common"
  - "$ref": "#/definitions/create"
- allOf:
  - properties:
      operation:
        type: string
        enum: ["Delete", "DeleteInBackground", "DeleteNonCascading"]
  - "$ref": "#/definitions/common"
  - "$ref": "#/definitions/delete"
- allOf:
  - oneOf:
    - required:
      - operation
      - jqFilter
      properties:
        operation:
          type: string
          enum: ["JQPatch"]
        jqFilter:
          type: string
          minimum: 1
    - required:
      - operation
      - mergePatch
      properties:
        operation:
          type: string
          enum: ["MergePatch"]
        mergePatch:
          type: object
          minProperties: 1
    - required:
      - operation
      - jsonPatch
      properties:
        operation:
          type: string
          enum: ["JSONPatch"]
        jsonPatch:
          type: array
          minItems: 1
          items:
          - type: object
            required: ["op", "path", "value"]
            properties:
              op:
                type: string
                minLength: 1
              path:
                type: string
                minLength: 1
              value: {}
  - "$ref": "#/definitions/common"
  - "$ref": "#/definitions/patch"
`,
}

var SchemasCache = map[string]*spec.Schema{}

// GetSchema returns loaded schema.
func GetSchema(name string) *spec.Schema {
	if s, ok := SchemasCache[name]; ok {
		return s
	}
	if _, ok := Schemas[name]; !ok {
		return nil
	}

	// ignore error because load is guaranteed by tests
	SchemasCache[name], _ = LoadSchema(name)
	return SchemasCache[name]
}

// LoadSchema returns spec.Schema object loaded from yaml in Schemas map.
func LoadSchema(name string) (*spec.Schema, error) {
	yml, err := swag.BytesToYAMLDoc([]byte(Schemas[name]))
	if err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %v", err)
	}
	d, err := swag.YAMLToJSON(yml)
	if err != nil {
		return nil, fmt.Errorf("yaml to json: %v", err)
	}

	s := new(spec.Schema)

	if err := json.Unmarshal(d, s); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}

	err = spec.ExpandSchema(s, s, nil /*new(noopResCache)*/)
	if err != nil {
		return nil, fmt.Errorf("expand schema: %v", err)
	}

	return s, nil
}

// See https://github.com/kubernetes/apiextensions-apiserver/blob/1bb376f70aa2c6f2dec9a8c7f05384adbfac7fbb/pkg/apiserver/validation/validation.go#L47
func ValidateOperationSpec(obj interface{}, s *spec.Schema, rootName string) (multiErr error) {
	if s == nil {
		return fmt.Errorf("validate kubernetes patch spec: schema is not provided")
	}

	validator := validate.NewSchemaValidator(s, nil, rootName, strfmt.Default)

	result := validator.Validate(obj)
	if result.IsValid() {
		return nil
	}

	var allErrs = &multierror.Error{Errors: make([]error, 1)}
	for _, err := range result.Errors {
		allErrs = multierror.Append(allErrs, err)
	}
	// NOTE: no validation errors, but kubernetes patch spec is not valid!
	if allErrs.Len() == 1 {
		allErrs = multierror.Append(allErrs, fmt.Errorf("kubernetes patch spec is not valid"))
	}

	if allErrs.Len() > 1 {
		yamlObj, _ := yaml.Marshal(obj)
		allErrs.Errors[0] = fmt.Errorf("can't validate document:\n%s", yamlObj)
	}

	return allErrs
}
