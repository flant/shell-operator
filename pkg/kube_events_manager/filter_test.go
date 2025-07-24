package kubeeventsmanager

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/filter/jq"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
)

func TestApplyFilter(t *testing.T) {
	t.Run("filter func with error", func(t *testing.T) {
		uns := &unstructured.Unstructured{Object: map[string]interface{}{"foo": "bar"}}
		filter := jq.NewFilter()
		jqFilter, err := jq.CompileJQ("")
		assert.Error(t, err)
		_, err = applyFilter(jqFilter, filter, filterFuncWithError, uns)
		assert.EqualError(t, err, "filterFn (github.com/flant/shell-operator/pkg/kube_events_manager.filterFuncWithError) contains an error: invalid character 'a' looking for beginning of value")
	})
}

func filterFuncWithError(_ *unstructured.Unstructured) (interface{}, error) {
	var s []string
	err := json.Unmarshal([]byte("asdasd"), &s)

	return s, err
}

var benchmarkUnstructuredPod = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod-benchmark",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "my-app",
				"env": "production",
			},
			"annotations": map[string]interface{}{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "8080",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "main-container",
					"image": "nginx:latest",
					"ports": []interface{}{
						map[string]interface{}{
							"containerPort": 80,
						},
					},
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "500m",
							"memory": "1Gi",
						},
						"requests": map[string]interface{}{
							"cpu":    "250m",
							"memory": "512Mi",
						},
					},
				},
			},
			"nodeName": "node-1.example.com",
		},
		"status": map[string]interface{}{
			"phase":  "Running",
			"podIP":  "10.0.0.15",
			"hostIP": "192.168.1.101",
		},
	},
}

func BenchmarkApplyFilterJSON(b *testing.B) {
	fl := jq.NewFilter()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := applyFilter(nil, fl, nil, benchmarkUnstructuredPod)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Test_applyFilter_with_jq_and_no_error(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"field1": "val1",
				"field2": "val2",
			},
		},
	}
	jqFilter, err := jq.CompileJQ(".spec")
	require.NoError(t, err)

	res, err := applyFilter(jqFilter, jq.NewFilter(), nil, obj)
	if assert.NoError(t, err) {
		assert.Equal(t, `{"field1":"val1","field2":"val2"}`, res.FilterResult)
		checksum, _ := utils_checksum.CalculateChecksum(`{"field1":"val1","field2":"val2"}`)
		assert.Equal(t, checksum, res.Metadata.Checksum)
		assert.Equal(t, obj, res.Object)
	}
}

func Test_applyFilter_should_not_mutate_object(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"field1": "val1",
				"field2": "val2",
			},
		},
	}

	t.Run("with jqFilter", func(t *testing.T) {
		jqFilter, err := jq.CompileJQ(".spec")
		require.NoError(t, err)

		objToTest := obj.DeepCopy()
		original := obj.DeepCopy()

		_, err = applyFilter(jqFilter, jq.NewFilter(), nil, objToTest)
		require.NoError(t, err)

		assert.True(t, reflect.DeepEqual(original.Object, objToTest.Object))
	})

	t.Run("with filterFn", func(t *testing.T) {
		// This filterFn does not mutate the object
		filterFn := func(obj *unstructured.Unstructured) (result interface{}, err error) {
			obj.Object["spec"].(map[string]interface{})["field1"] = "val2"
			return obj.Object["spec"], nil
		}

		objToTest := obj.DeepCopy()
		original := obj.DeepCopy()

		_, err := applyFilter(nil, nil, filterFn, objToTest)
		require.NoError(t, err)

		assert.True(t, reflect.DeepEqual(original.Object, objToTest.Object))
	})

	t.Run("with no filter", func(t *testing.T) {
		objToTest := obj.DeepCopy()
		original := obj.DeepCopy()

		_, err := applyFilter(nil, nil, nil, objToTest)
		require.NoError(t, err)

		assert.True(t, reflect.DeepEqual(original.Object, objToTest.Object))
	})

}
