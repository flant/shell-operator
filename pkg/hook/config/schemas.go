package config

import (
	"encoding/json"
	"fmt"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

var Schemas = map[string]string{
	"v1": `
definitions:
  nameSelector:
    type: object
    additionalProperties: false
    required:
    - matchNames
    properties:
      matchNames:
        type: array
        additionalItems: false
        items:
          type: string
  labelSelector:
    type: object
    additionalProperties: false
    minProperties: 1
    maxProperties: 2
    properties:
      matchLabels:
        type: object
        additionalProperties:
          type: string
      matchExpressions:
        type: array
        items:
          type: object
          additionalProperties: false
          required:
          - key
          - operator
          properties:
            key:
              type: string
            operator:
              type: string
              enum:
              - In
              - NotIn
              - Exists
              - DoesNotExists
            values:
              type: array
              items:
                type: string
  
type: object
additionalProperties: false
required:
- configVersion
minProperties: 2
properties:
  configVersion:
    type: string
    enum:
    - v1
  onStartup:
    title: onStartup binding
    description: |
      the value is the order to sort onStartup hooks
    type: integer
    example: 10    
  schedule:
    title: schedule bindings
    description: |
      configuration of hooks that should run on schedule
    type: array
    additionalItems: false
    minItems: 1
    items:
      type: object
      additionalProperties: false
      required:
      - crontab
      properties:
        name:
          type: string
        crontab:
          type: string
        allowFailure:
          type: bool
          default: false
  kubernetes:
    title: kubernetes event bindings
    type: array
    additionalItems: false
    minItems: 1
    items:
      type: object
      additionalProperties: false
      required:
      - kind
      properties:
        name:
          type: string
        apiVersion:
          type: string
        kind:
          type: string
        watchEvent:
          type: array
          additionalItems: false
          minItems: 1
          items:
            type: string
            enum:
            - Added
            - Modified
            - Deleted
        jqFilter:
          type: string
          example: ".metadata.labels"
        allowFailure:
          type: bool
        resynchronizationPeriod:
          type: string
        nameSelector:
          "$ref": "#/definitions/nameSelector"
        labelSelector:
          "$ref": "#/definitions/labelSelector"
        fieldSelector:
          type: object
          additionalProperties: false
          required:
          - matchExpressions
          properties:
            matchExpressions:
              type: array
              items:
                type: object
                additionalProperties: false
                minProperties: 3
                maxProperties: 3
                properties:
                  field:
                    type: string
                  operator:
                    type: string
                    enum: ["=", "==", "Equals", "!=", "NotEquals"]
                  value:
                    type: string
        namespace:
          type: object
          additionalProperties: false
          minProperties: 1
          maxProperties: 2
          properties:
            nameSelector:
              "$ref": "#/definitions/nameSelector"
            labelSelector:
              "$ref": "#/definitions/labelSelector"
`,
	"v0": `
type: object
additionalProperties: false
minProperties: 1
properties:
  onStartup:
    title: onStartup binding
    description: |
      the value is the order to sort onStartup hooks
    type: integer
  schedule:
    type: object
  onKubernetesEvent:
    type: object
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
