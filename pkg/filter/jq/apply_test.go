package jq

import (
	json "github.com/flant/shell-operator/pkg/utils/json"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_ApplyFilter_SingleDocumentModification(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter := `. + {"status": "active"}`

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"name": "John", "age": 30})
	g.Expect(err).Should(BeNil())

	var resultMap any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(Equal(map[string]any{"name": "John", "age": 30.0, "status": "active"}))
}

func Test_ApplyFilter_ExtractValuesFromDocument(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter := `.user.details`

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"user": map[string]any{"name": "John", "details": map[string]any{"location": "New York", "occupation": "Developer"}}})
	g.Expect(err).Should(BeNil())

	var resultMap any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(Equal(map[string]any{"location": "New York", "occupation": "Developer"}))
}

func Test_ApplyFilter_MultipleJsonDocumentsInArray(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter := `.users[] | . + {"status": "active"}`

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"users": []any{map[string]any{"name": "John", "status": "inactive"}, map[string]any{"name": "Jane", "status": "inactive"}}})
	g.Expect(err).Should(BeNil())

	var resultMap []any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(HaveLen(2))

	expected := []map[string]any{
		{"name": "John", "status": "active"},
		{"name": "Jane", "status": "active"},
	}

	g.Expect(resultMap).Should(ConsistOf(expected))
}

func Test_ApplyFilter_InvalidFilter(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	// Test invalid jq syntax
	invalidSyntax := `invalid syntax`
	result, err := filter.ApplyFilter(invalidSyntax, map[string]any{"name": "John"})
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeNil())

	// Test invalid jq function
	invalidFunction := `. | invalid_function`
	result, err = filter.ApplyFilter(invalidFunction, map[string]any{"name": "John"})
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeNil())
}

func Test_ApplyFilter_InvalidJson(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter := `.name`

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"name": "John"})
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())

	var resultMap any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).ShouldNot(BeNil())
}

func Test_ApplyFilter_NilInputData(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	result, err := filter.ApplyFilter(`.`, nil)
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())
}

func Test_ApplyFilter_EmptyFilter(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	result, err := filter.ApplyFilter("", map[string]any{"name": "John"})
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeNil())
}

func Test_ApplyFilter_InvalidJsonInDeepCopy(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	// Create invalid JSON data that cannot be marshaled
	invalidData := map[string]any{
		"channel": make(chan int), // channel cannot be marshaled to JSON
	}

	result, err := filter.ApplyFilter(`.`, invalidData)
	g.Expect(err).ShouldNot(BeNil()) // Expect an error due to invalid JSON
	g.Expect(result).Should(BeNil())
}

func Test_ApplyFilter_EmptyResult(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	// Filter that returns no results
	result, err := filter.ApplyFilter(`.nonexistent`, map[string]any{"name": "John"})
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())

	var resultMap any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(BeNil()) // Expect result to be nil (empty)
}

func Test_ApplyFilter_InvalidJqSyntax(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	result, err := filter.ApplyFilter(`invalid syntax`, map[string]any{"name": "John"})
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeNil())
}

func Test_ApplyFilter_InvalidJqFunction(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	result, err := filter.ApplyFilter(`. | invalid_function`, map[string]any{"name": "John"})
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeNil())
}

func Test_ApplyFilter_PanicSafety(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ApplyFilter panicked: %v", r)
		}
	}()

	// Test with data that could potentially cause a panic
	_, err := filter.ApplyFilter(`.`, map[string]any{"key": func() {}})
	g.Expect(err).ShouldNot(BeNil())
}

func Test_deepCopyAny(t *testing.T) {
	g := NewWithT(t)

	// Test copying a map preserves types
	inputMap := map[string]any{"foo": "bar", "num": 42}
	copyMap, err := deepCopyAny(inputMap)
	g.Expect(err).Should(BeNil())
	g.Expect(copyMap).Should(Equal(map[string]any{"foo": "bar", "num": 42}))
	g.Expect(copyMap).ShouldNot(BeIdenticalTo(inputMap))

	// Test copying a slice preserves types
	inputSlice := []any{"a", 1, true}
	copySlice, err := deepCopyAny(inputSlice)
	g.Expect(err).Should(BeNil())
	g.Expect(copySlice).Should(Equal([]any{"a", 1, true}))
	g.Expect(copySlice).ShouldNot(BeIdenticalTo(inputSlice))

	// Test copying nil
	copyNil, err := deepCopyAny(nil)
	g.Expect(err).Should(BeNil())
	g.Expect(copyNil).Should(BeNil())

	// Test copying a value with unsupported type
	inputInvalid := map[string]any{"ch": make(chan int)}
	copyInvalid, err := deepCopyAny(inputInvalid)
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(copyInvalid).Should(BeNil())
}

func Test_deepCopyAny_NestedMap(t *testing.T) {
	g := NewWithT(t)

	input := map[string]any{
		"metadata": map[string]any{
			"name":      "my-pod",
			"namespace": "default",
			"labels":    map[string]any{"app": "test"},
		},
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}

	result, err := deepCopyAny(input)
	g.Expect(err).Should(BeNil())

	resultMap := result.(map[string]any)
	g.Expect(resultMap["metadata"]).Should(Equal(input["metadata"]))

	// Verify it's a true deep copy: mutating the copy must not affect the original.
	resultMeta := resultMap["metadata"].(map[string]any)
	resultMeta["name"] = "mutated"
	g.Expect(input["metadata"].(map[string]any)["name"]).Should(Equal("my-pod"))
}

func Test_deepCopyAny_NestedSlice(t *testing.T) {
	g := NewWithT(t)

	input := []any{
		map[string]any{"name": "a"},
		map[string]any{"name": "b"},
	}

	result, err := deepCopyAny(input)
	g.Expect(err).Should(BeNil())

	resultSlice := result.([]any)
	g.Expect(resultSlice).Should(HaveLen(2))

	// Mutate copy, verify original is untouched.
	resultSlice[0].(map[string]any)["name"] = "mutated"
	g.Expect(input[0].(map[string]any)["name"]).Should(Equal("a"))
}

func Test_deepCopyAny_NumericTypes(t *testing.T) {
	g := NewWithT(t)

	input := map[string]any{
		"int":     42,
		"int64":   int64(100),
		"float64": 3.14,
		"bool":    true,
		"string":  "hello",
	}

	result, err := deepCopyAny(input)
	g.Expect(err).Should(BeNil())
	g.Expect(result).Should(Equal(input))
}

func BenchmarkDeepCopyAny(b *testing.B) {
	input := map[string]any{
		"metadata": map[string]any{
			"name":      "my-pod",
			"namespace": "default",
			"labels":    map[string]any{"app": "test", "env": "prod"},
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "main", "image": "nginx:latest"},
				map[string]any{"name": "sidecar", "image": "envoy:latest"},
			},
			"replicas": float64(3),
		},
	}
	b.ResetTimer()
	for range b.N {
		_, _ = deepCopyAny(input)
	}
}

// ---- Compile / CompiledJqFilter tests ----

func Test_Compile_ValidExpression(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.metadata.name`)
	g.Expect(err).Should(BeNil())
	g.Expect(cf).ShouldNot(BeNil())
	g.Expect(cf.String()).Should(Equal(`.metadata.name`))
}

func Test_Compile_InvalidExpression(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`this is not jq`)
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(cf).Should(BeNil())
}

func Test_Compile_EmptyExpression(t *testing.T) {
	g := NewWithT(t)

	// Empty string is not valid jq.
	cf, err := Compile(``)
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(cf).Should(BeNil())
}

func Test_CompiledJqFilter_Apply_SingleDocumentModification(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`. + {"status": "active"}`)
	g.Expect(err).Should(BeNil())

	result, err := cf.Apply(map[string]any{"name": "Alice", "age": 25})
	g.Expect(err).Should(BeNil())

	var got any
	g.Expect(json.Unmarshal(result, &got)).Should(BeNil())
	g.Expect(got).Should(Equal(map[string]any{"name": "Alice", "age": float64(25), "status": "active"}))
}

func Test_CompiledJqFilter_Apply_ExtractField(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.metadata.labels`)
	g.Expect(err).Should(BeNil())

	input := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{"app": "foo", "env": "prod"},
		},
	}
	result, err := cf.Apply(input)
	g.Expect(err).Should(BeNil())

	var got any
	g.Expect(json.Unmarshal(result, &got)).Should(BeNil())
	g.Expect(got).Should(Equal(map[string]any{"app": "foo", "env": "prod"}))
}

func Test_CompiledJqFilter_Apply_MultipleResults(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.users[] | .name`)
	g.Expect(err).Should(BeNil())

	input := map[string]any{
		"users": []any{
			map[string]any{"name": "Alice"},
			map[string]any{"name": "Bob"},
		},
	}
	result, err := cf.Apply(input)
	g.Expect(err).Should(BeNil())

	var got []any
	g.Expect(json.Unmarshal(result, &got)).Should(BeNil())
	g.Expect(got).Should(ConsistOf("Alice", "Bob"))
}

func Test_CompiledJqFilter_Apply_NullResult(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.nonexistent`)
	g.Expect(err).Should(BeNil())

	result, err := cf.Apply(map[string]any{"name": "Alice"})
	g.Expect(err).Should(BeNil())
	g.Expect(result).Should(Equal([]byte("null")))
}

func Test_CompiledJqFilter_Apply_NilInput(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.`)
	g.Expect(err).Should(BeNil())

	result, err := cf.Apply(nil)
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())
}

func Test_CompiledJqFilter_Apply_RuntimeError(t *testing.T) {
	g := NewWithT(t)

	// .foo on a non-object (null) causes a runtime jq error.
	cf, err := Compile(`.foo`)
	g.Expect(err).Should(BeNil())

	// Passing nil as input means workData == nil; trying .foo on null returns null.
	result, err := cf.Apply(nil)
	g.Expect(err).Should(BeNil())
	g.Expect(result).Should(Equal([]byte("null")))
}

func Test_CompiledJqFilter_Apply_UnmarshalableInput(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.`)
	g.Expect(err).Should(BeNil())

	_, err = cf.Apply(map[string]any{"ch": make(chan int)})
	g.Expect(err).ShouldNot(BeNil())
}

// Test_CompiledJqFilter_Reuse verifies that the same compiled filter can be
// applied to different inputs and produces correct independent results.
func Test_CompiledJqFilter_Reuse(t *testing.T) {
	g := NewWithT(t)

	cf, err := Compile(`.spec.replicas`)
	g.Expect(err).Should(BeNil())

	inputs := []map[string]any{
		{"spec": map[string]any{"replicas": float64(1)}},
		{"spec": map[string]any{"replicas": float64(3)}},
		{"spec": map[string]any{"replicas": float64(5)}},
	}
	expected := []float64{1, 3, 5}

	for i, input := range inputs {
		result, err := cf.Apply(input)
		g.Expect(err).Should(BeNil(), "input index %d", i)

		var got float64
		g.Expect(json.Unmarshal(result, &got)).Should(BeNil(), "input index %d", i)
		g.Expect(got).Should(Equal(expected[i]), "input index %d", i)
	}
}

// Test_Compile_ProducesIdenticalResultsToApplyFilter verifies that the
// compiled path and the interpreted path yield identical output.
func Test_Compile_ProducesIdenticalResultsToApplyFilter(t *testing.T) {
	g := NewWithT(t)

	filterStr := `.metadata | {name, namespace}`
	input := map[string]any{
		"metadata": map[string]any{
			"name":      "my-pod",
			"namespace": "default",
			"labels":    map[string]any{"app": "foo"},
		},
	}

	interpreted := NewFilter()
	resultInterpreted, err := interpreted.ApplyFilter(filterStr, input)
	g.Expect(err).Should(BeNil())

	cf, err := Compile(filterStr)
	g.Expect(err).Should(BeNil())
	resultCompiled, err := cf.Apply(input)
	g.Expect(err).Should(BeNil())

	g.Expect(resultCompiled).Should(Equal(resultInterpreted))
}
