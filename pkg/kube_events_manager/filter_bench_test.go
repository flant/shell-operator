package kubeeventsmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/filter/jq"
)

func BenchmarkApplyFilter_JQ(b *testing.B) {
	// Prepare data for benchmark
	// smoke test that all is ok
	t := &testing.T{}
	uns := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":            "test",
			"namespace":       "default",
			"resourceVersion": "1",
		},
		"spec": map[string]interface{}{
			"prop": "value",
		},
	}}
	jqFilter := ".spec"
	filter := jq.NewFilter()

	// smoke test for jqFilter
	res, err := applyFilter(jqFilter, filter, nil, uns)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.JSONEq(t, `{"prop":"value"}`, res.FilterResult.(string))

	b.ResetTimer()
	b.ReportAllocs()

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = applyFilter(jqFilter, filter, nil, uns)
	}
}

func BenchmarkApplyFilter_Func(b *testing.B) {
	// Prepare data for benchmark
	// smoke test that all is ok
	t := &testing.T{}
	uns := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":            "test",
			"namespace":       "default",
			"resourceVersion": "1",
		},
		"spec": map[string]interface{}{
			"prop": "value",
		},
	}}
	filter := jq.NewFilter()
	filterFn := func(obj *unstructured.Unstructured) (interface{}, error) {
		return obj.Object["spec"], nil
	}

	// smoke test for filterFn
	res, err := applyFilter("", filter, filterFn, uns)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, map[string]interface{}{"prop": "value"}, res.FilterResult)

	b.ResetTimer()
	b.ReportAllocs()

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = applyFilter("", filter, filterFn, uns)
	}
}
