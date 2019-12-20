package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_ObjectAndFilterResult_ToJson(t *testing.T) {
	obj := &ObjectAndFilterResult{
		FilterResult: `{"spec":"asd"}`,
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]string{
					"namespace": "default",
				},
				"kind": "Pod",
				"name": "pod-qwe",
			},
		},
	}
	obj.Metadata.JqFilter = ".spec"

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	//fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, "filterResult")
}

func Test_ObjectAndFilterResult_ToJson_EmptyFilterResult(t *testing.T) {
	obj := &ObjectAndFilterResult{
		FilterResult: ``,
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]string{
					"namespace": "default",
				},
				"kind": "Pod",
				"name": "pod-qwe",
			},
		},
	}
	obj.Metadata.JqFilter = ".spec"

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	//fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, `"filterResult":null`)
}

func Test_ObjectAndFilterResult_ToJson_NullFilterResult(t *testing.T) {
	obj := &ObjectAndFilterResult{
		FilterResult: `null`,
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]string{
					"namespace": "default",
				},
				"kind": "Pod",
				"name": "pod-qwe",
			},
		},
	}
	obj.Metadata.JqFilter = ".spec"

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	//fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, `"filterResult":null`)
}

func Test_ObjectAndFilterResult_ToJson_NoFilterResult(t *testing.T) {
	obj := &ObjectAndFilterResult{
		FilterResult: `{"spec":"asd"}`,
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]string{
					"namespace": "default",
				},
				"kind": "Pod",
				"name": "pod-qwe",
			},
		},
	}
	obj.Metadata.JqFilter = ""

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	//fmt.Printf("%s\n", jsonStr)
	assert.NotContains(t, jsonStr, "filterResult")
}
