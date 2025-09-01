package jq

import (
	"encoding/json"
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

	// Test copying a map
	inputMap := map[string]any{"foo": "bar", "num": 42}
	copyMap, err := deepCopyAny(inputMap)
	g.Expect(err).Should(BeNil())
	g.Expect(copyMap).Should(Equal(map[string]any{"foo": "bar", "num": float64(42)}))
	g.Expect(copyMap).ShouldNot(BeIdenticalTo(inputMap))

	// Test copying a slice
	inputSlice := []any{"a", 1, true}
	copySlice, err := deepCopyAny(inputSlice)
	g.Expect(err).Should(BeNil())
	g.Expect(copySlice).Should(Equal([]any{"a", float64(1), true}))
	g.Expect(copySlice).ShouldNot(BeIdenticalTo(inputSlice))

	// Test copying nil
	copyNil, err := deepCopyAny(nil)
	g.Expect(err).Should(BeNil())
	g.Expect(copyNil).Should(BeNil())

	// Test copying a value that cannot be marshaled to JSON
	inputInvalid := map[string]any{"ch": make(chan int)}
	copyInvalid, err := deepCopyAny(inputInvalid)
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(copyInvalid).Should(BeNil())
}
