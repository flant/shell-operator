package object_patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/jq"
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

func applyJQPatch(jqFilter string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	objBytes, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}

	filterResult, err := jq.ApplyJqFilter(jqFilter, objBytes, app.JqLibraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to apply jqFilter:\n%sto Object:\n%s\n"+
			"error: %s", jqFilter, obj, err)
	}

	var retObj = &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode([]byte(filterResult), nil, retObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filterResult:\n%s\nto Unstructured Object\nerror: %s", filterResult, err)
	}

	return retObj, nil
}

func generateSubresources(subresource string) (ret []string) {
	if subresource != "" {
		ret = append(ret, subresource)
	}

	return
}
