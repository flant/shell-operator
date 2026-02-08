package jq

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Helper function to create test objects
func newTestObject(data map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: data}
}

// Test ExecuteJQ function
func TestExecuteJQ(t *testing.T) {
	t.Run("simple selection", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"name": "test",
			"age":  42,
		})

		expr, err := CompileExpression(".name")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, `"test"`, string(result))
	})

	t.Run("multiple results", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"items": []interface{}{1, 2, 3},
		})

		expr, err := CompileExpression(".items[]")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "[1,2,3]", string(result))
	})

	t.Run("no results - returns null", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{})

		expr, err := CompileExpression("empty")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "null", string(result))
	})

	t.Run("single null result", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"value": nil,
		})

		expr, err := CompileExpression(".value")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "null", string(result))
	})

	t.Run("complex object transformation", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"users": []interface{}{
				map[string]interface{}{"name": "Alice", "age": 30},
				map[string]interface{}{"name": "Bob", "age": 25},
			},
		})

		expr, err := CompileExpression(".users | map({name, older: (.age > 26)})")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)

		var parsed []map[string]interface{}
		err = json.Unmarshal(result, &parsed)
		require.NoError(t, err)
		assert.Len(t, parsed, 2)
		assert.Equal(t, "Alice", parsed[0]["name"])
		assert.Equal(t, true, parsed[0]["older"])
		assert.Equal(t, "Bob", parsed[1]["name"])
		assert.Equal(t, false, parsed[1]["older"])
	})

	t.Run("array construction", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"a": 1,
			"b": 2,
		})

		expr, err := CompileExpression("[.a, .b, (.a + .b)]")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "[1,2,3]", string(result))
	})

	t.Run("object construction", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"firstName": "John",
			"lastName":  "Doe",
		})

		expr, err := CompileExpression("{fullName: (.firstName + \" \" + .lastName)}")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.JSONEq(t, `{"fullName":"John Doe"}`, string(result))
	})

	t.Run("type error", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"value": "string",
		})

		expr, err := CompileExpression(".value.field")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("halt error with nil value - graceful termination", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"items": []interface{}{1, 2, 3},
		})

		// limit(0) produces halt error with nil value
		expr, err := CompileExpression(".items[] | limit(0; .)")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "null", string(result))
	})

	t.Run("division by zero", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"value": 10,
		})

		expr, err := CompileExpression(".value / 0")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("nested array and object", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"data": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"tags": []interface{}{"a", "b", "c"},
					},
					map[string]interface{}{
						"tags": []interface{}{"x", "y"},
					},
				},
			},
		})

		expr, err := CompileExpression(".data.items[].tags[]")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, `["a","b","c","x","y"]`, string(result))
	})

	t.Run("conditional expression", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"status": "active",
		})

		expr, err := CompileExpression(`if .status == "active" then "running" else "stopped" end`)
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, `"running"`, string(result))
	})

	t.Run("boolean values", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"enabled":  true,
			"disabled": false,
		})

		expr, err := CompileExpression("[.enabled, .disabled]")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "[true,false]", string(result))
	})

	t.Run("numeric operations", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"a": 10,
			"b": 3,
		})

		expr, err := CompileExpression("{sum: (.a + .b), diff: (.a - .b), prod: (.a * .b)}")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.JSONEq(t, `{"sum":13,"diff":7,"prod":30}`, string(result))
	})

	t.Run("string operations", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"text": "Hello, World!",
		})

		expr, err := CompileExpression(".text | ascii_upcase")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, `"HELLO, WORLD!"`, string(result))
	})

	t.Run("empty object", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{})

		expr, err := CompileExpression(".")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, "{}", string(result))
	})

	t.Run("special characters in strings", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"text": "Line 1\nLine 2\tTabbed",
		})

		expr, err := CompileExpression(".text")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)

		var text string
		err = json.Unmarshal(result, &text)
		require.NoError(t, err)
		assert.Equal(t, "Line 1\nLine 2\tTabbed", text)
	})

	t.Run("unicode characters", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"emoji":    "🚀✨🎉",
			"cyrillic": "Привет",
		})

		expr, err := CompileExpression("{emoji, cyrillic}")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.JSONEq(t, `{"emoji":"🚀✨🎉","cyrillic":"Привет"}`, string(result))
	})

	t.Run("kubernetes object filtering", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
				},
			},
		})

		expr, err := CompileExpression(".metadata.name")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, `"test-pod"`, string(result))
	})

	t.Run("kubernetes object complex filtering", func(t *testing.T) {
		obj := newTestObject(map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
					map[string]interface{}{
						"name":  "sidecar",
						"image": "sidecar:latest",
					},
				},
			},
		})

		expr, err := CompileExpression(".spec.containers[].name")
		require.NoError(t, err)

		result, err := ExecuteJQ(expr, obj)
		require.NoError(t, err)
		assert.Equal(t, `["nginx","sidecar"]`, string(result))
	})
}

// Test CompileExpression function
func TestCompileExpression(t *testing.T) {
	t.Run("valid simple expression", func(t *testing.T) {
		expr, err := CompileExpression(".field")
		require.NoError(t, err)
		assert.NotNil(t, expr)
		assert.NotNil(t, expr.Code)
		assert.Equal(t, ".field", expr.Query())
	})

	t.Run("valid complex expression", func(t *testing.T) {
		expr, err := CompileExpression(`.items[] | select(.type == "pod") | .metadata.name`)
		require.NoError(t, err)
		assert.NotNil(t, expr)
		assert.NotNil(t, expr.Code)
	})

	t.Run("invalid expression - syntax error", func(t *testing.T) {
		expr, err := CompileExpression(".field[")
		assert.Error(t, err)
		assert.Nil(t, expr)
	})

	t.Run("invalid expression - incomplete", func(t *testing.T) {
		expr, err := CompileExpression(".")
		require.NoError(t, err)
		assert.NotNil(t, expr)
	})

	t.Run("empty expression", func(t *testing.T) {
		expr, err := CompileExpression("")
		assert.Error(t, err)
		assert.Nil(t, expr)
	})

	t.Run("expression with function", func(t *testing.T) {
		expr, err := CompileExpression(".items | length")
		require.NoError(t, err)
		assert.NotNil(t, expr)
	})

	t.Run("expression with pipe", func(t *testing.T) {
		expr, err := CompileExpression(".data | keys | sort")
		require.NoError(t, err)
		assert.NotNil(t, expr)
	})

	t.Run("expression with map", func(t *testing.T) {
		expr, err := CompileExpression(".items | map(.name)")
		require.NoError(t, err)
		assert.NotNil(t, expr)
	})

	t.Run("expression with select", func(t *testing.T) {
		expr, err := CompileExpression(`.items[] | select(.status == "active")`)
		require.NoError(t, err)
		assert.NotNil(t, expr)
	})

	t.Run("expression with multiple pipes", func(t *testing.T) {
		expr, err := CompileExpression(".items[] | select(.type == \"pod\") | .metadata.name | length")
		require.NoError(t, err)
		assert.NotNil(t, expr)
	})
}

// Test Expression struct methods
func TestExpression(t *testing.T) {
	t.Run("Query method returns original expression", func(t *testing.T) {
		originalQuery := ".metadata.name"
		expr, err := CompileExpression(originalQuery)
		require.NoError(t, err)
		assert.Equal(t, originalQuery, expr.Query())
	})

	t.Run("Code field is set correctly", func(t *testing.T) {
		expr, err := CompileExpression(".field")
		require.NoError(t, err)
		assert.NotNil(t, expr.Code)
	})
}

// Test Info function
func TestInfo(t *testing.T) {
	t.Run("returns correct implementation info", func(t *testing.T) {
		info := Info()
		assert.Equal(t, "jq implementation: using itchyny/gojq", info)
	})
}
