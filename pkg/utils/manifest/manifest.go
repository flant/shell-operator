package manifest

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type Manifest map[string]interface{}

func getFieldString(m map[string]interface{}, field string) string {
	if obj, ok := m[field]; ok {
		if res, isString := obj.(string); isString {
			return res
		}
	}
	return ""
}

func NewManifest(apiVersion, kind, name string) Manifest {
	return Manifest(map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
	})
}

func NewManifestFromYaml(yamlOrJson string) (Manifest, error) {
	var m Manifest
	err := yaml.Unmarshal([]byte(yamlOrJson), &m)
	if err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func MustManifestFromYaml(yamlOrJson string) Manifest {
	m, err := NewManifestFromYaml(yamlOrJson)
	if err != nil {
		panic(err)
	}
	return m
}

func (m Manifest) Id() string {
	return fmt.Sprintf("%s/%s/%s", m.Namespace("default"), m.Kind(), m.Name())
}

// HasBasicFields is true if Template has version, kind and metadata.name fields
func (m Manifest) HasBasicFields() bool {
	_, hasKind := m["kind"]
	_, hasVersion := m["apiVersion"]
	if !hasKind || !hasVersion {
		return false
	}

	_, hasName := m.Metadata()["name"]
	return hasName
}

func (m Manifest) Kind() string {
	return getFieldString(m, "kind")
}

func (m Manifest) ApiVersion() string {
	return getFieldString(m, "apiVersion")
}

func (m Manifest) Name() string {
	return getFieldString(m.Metadata(), "name")
}

func (m Manifest) Namespace(defaultNs string) string {
	ns := getFieldString(m.Metadata(), "namespace")
	if ns != "" {
		return ns
	}
	return defaultNs
}

func (m Manifest) Metadata() map[string]interface{} {
	_, hasMetadata := m["metadata"]
	if hasMetadata {
		meta, ok := m["metadata"].(map[string]interface{})
		if ok {
			return meta
		}
	}

	return map[string]interface{}{}
}

func (m Manifest) SetNamespace(ns string) {
	m.Metadata()["namespace"] = ns
}

func (m Manifest) ToUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: m,
	}
}
