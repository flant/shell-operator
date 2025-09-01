package object_patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8yaml "sigs.k8s.io/yaml"

	"github.com/flant/kube-client/manifest"
	"github.com/flant/shell-operator/pkg/filter"
)

func unmarshalFromJSONOrYAML(specs []byte) ([]OperationSpec, error) {
	fromJsonSpecs, err := unmarshalFromJson(specs)
	if err != nil {
		return unmarshalFromYaml(specs)
	}

	return fromJsonSpecs, nil
}

func unmarshalFromJson(jsonSpecs []byte) ([]OperationSpec, error) {
	var specSlice []OperationSpec

	dec := json.NewDecoder(bytes.NewReader(jsonSpecs))
	for {
		var doc OperationSpec
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		specSlice = append(specSlice, doc)
	}

	return specSlice, nil
}

func unmarshalFromYaml(yamlSpecs []byte) ([]OperationSpec, error) {
	var specSlice []OperationSpec

	dec := yaml.NewDecoder(bytes.NewReader(yamlSpecs))
	for {
		var doc OperationSpec
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		specSlice = append(specSlice, doc)
	}

	return specSlice, nil
}

func applyJQPatch(jqFilter string, fl filter.Filter, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	filterResult, err := fl.ApplyFilter(jqFilter, obj.UnstructuredContent())
	if err != nil {
		return nil, fmt.Errorf("failed to apply jqFilter:\n%sto Object:\n%s\n"+
			"error: %s", jqFilter, obj, err)
	}

	retObj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(filterResult, nil, retObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filterResult:\n%s\nto Unstructured Object\nerror: %s", filterResult, err)
	}

	return retObj, nil
}

func generateSubresources(subresource string) []string {
	if subresource != "" {
		return []string{subresource}
	}

	return nil
}

func toUnstructured(obj any) (*unstructured.Unstructured, error) {
	switch v := obj.(type) {
	case []byte:
		mft, err := manifest.NewFromYAML(string(v))
		if err != nil {
			return nil, err
		}
		return mft.Unstructured(), nil
	case string:
		mft, err := manifest.NewFromYAML(v)
		if err != nil {
			return nil, err
		}
		return mft.Unstructured(), nil
	case map[string]any:
		return &unstructured.Unstructured{Object: v}, nil
	default:
		objectContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("convert to unstructured: %v", err)
		}
		return &unstructured.Unstructured{Object: objectContent}, nil
	}
}

func convertPatchToBytes(patch any) ([]byte, error) {
	var err error
	var intermediate any
	switch v := patch.(type) {
	case []byte:
		err = k8yaml.Unmarshal(v, &intermediate)
	case string:
		err = k8yaml.Unmarshal([]byte(v), &intermediate)
	default:
		intermediate = v
	}
	if err != nil {
		return nil, err
	}

	// Try to encode to JSON.
	var patchBytes []byte
	patchBytes, err = json.Marshal(intermediate)
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}
