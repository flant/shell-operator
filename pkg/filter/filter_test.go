package filter

import (
	"errors"
	"testing"

	"github.com/flant/shell-operator/pkg/jq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Helper function to create test objects
func newTestObject(data map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: data}
}

// Test RunFn function - custom filter function
func TestRunFn(t *testing.T) {
	t.Run("with custom filter function - success", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "nginx"},
				},
			},
		})

		filterFn := func(o *unstructured.Unstructured) (interface{}, error) {
			return map[string]interface{}{"name": o.GetName()}, nil
		}

		result, err := RunFn(filterFn, obj)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, obj, result.Object)
		assert.NotEmpty(t, result.Metadata.Checksum)
		assert.Equal(t, map[string]interface{}{"name": "test-pod"}, result.FilterResult)
	})

	t.Run("with custom filter function - error", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{"foo": "bar"})
		expectedErr := errors.New("filter failed")

		filterFn := func(_ *unstructured.Unstructured) (interface{}, error) {
			return nil, expectedErr
		}

		result, err := RunFn(filterFn, obj)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "filter function")
		assert.Contains(t, err.Error(), "filter failed")
	})

	t.Run("with custom filter function - invalid JSON result", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{"foo": "bar"})

		// Return something that can't be marshaled to JSON
		filterFn := func(_ *unstructured.Unstructured) (interface{}, error) {
			return make(chan int), nil // channels can't be marshaled
		}

		result, err := RunFn(filterFn, obj)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// Test RunExpression function - jq expression filter
func TestRunExpression(t *testing.T) {
	t.Run("with jq expression - simple filter", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"nodeName": "node-1",
			},
		})

		expr, err := jq.CompileExpression(".spec.nodeName")
		require.NoError(t, err)

		result, err := RunExpression(expr, obj)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, `"node-1"`, string(result.FilterResult.([]byte)))
		assert.NotEmpty(t, result.Metadata.Checksum)
		assert.Equal(t, ".spec.nodeName", result.Metadata.JqFilter)
	})

	t.Run("with jq expression - complex filter", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "nginx", "image": "nginx:1.19"},
					map[string]interface{}{"name": "sidecar", "image": "sidecar:latest"},
				},
			},
		})

		expr, err := jq.CompileExpression(".spec.containers[].name")
		require.NoError(t, err)

		result, err := RunExpression(expr, obj)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, `["nginx","sidecar"]`, string(result.FilterResult.([]byte)))
	})

	t.Run("with jq expression - filter returns null", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"spec": map[string]interface{}{},
		})

		expr, err := jq.CompileExpression(".spec.nonexistent")
		require.NoError(t, err)

		result, err := RunExpression(expr, obj)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "null", string(result.FilterResult.([]byte)))
	})

	t.Run("with jq expression - error", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"spec": "not-an-object",
		})

		expr, err := jq.CompileExpression(".spec.field")
		require.NoError(t, err)

		result, err := RunExpression(expr, obj)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// Test RunPlain function - no filter
func TestRunPlain(t *testing.T) {
	t.Run("no filter - full object checksum", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "kube-system",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		})

		result, err := RunPlain(obj)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, obj, result.Object)
		assert.NotEmpty(t, result.Metadata.Checksum)
		assert.Nil(t, result.FilterResult)
		assert.Equal(t, "kube-system/ConfigMap/test-cm", result.Metadata.ResourceId)
	})

	t.Run("no filter - empty object", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{})

		result, err := RunPlain(obj)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Metadata.Checksum)
	})
}

// Test edge cases and resource ID generation
func TestResourceIdGeneration(t *testing.T) {
	t.Run("standard kubernetes object", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "my-pod",
				"namespace": "default",
			},
		})

		result, err := RunPlain(obj)
		require.NoError(t, err)
		assert.Equal(t, "default/Pod/my-pod", result.Metadata.ResourceId)
	})

	t.Run("cluster-scoped resource", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Node",
			"metadata": map[string]interface{}{
				"name": "node-1",
			},
		})

		result, err := RunPlain(obj)
		require.NoError(t, err)
		assert.Equal(t, "/Node/node-1", result.Metadata.ResourceId)
	})

	t.Run("missing metadata", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Unknown",
		})

		result, err := RunPlain(obj)
		require.NoError(t, err)
		assert.Equal(t, "/Unknown/", result.Metadata.ResourceId)
	})
}
