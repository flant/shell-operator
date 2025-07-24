package jq

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

func Test_ApplyFilter_SingleDocumentModification(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter, err := CompileJQ(`. + {"status": "active"}`)
	g.Expect(err).Should(BeNil())

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

	jqFilter, err := CompileJQ(`.user.details`)
	g.Expect(err).Should(BeNil())

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

	jqFilter, err := CompileJQ(`.users[] | . + {"status": "active"}`)
	g.Expect(err).Should(BeNil())

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

func Test_CompileJQ_InvalidFilter(t *testing.T) {
	g := NewWithT(t)

	// Test invalid jq syntax
	_, err := CompileJQ(`invalid syntax`)
	g.Expect(err).ShouldNot(BeNil())

	// Test invalid jq function, note: gojq does not fail on compilation for this
	// _, err = CompileJQ(`. | invalid_function`)
	// g.Expect(err).ShouldNot(BeNil())
}

func Test_ApplyFilter_InvalidFunction(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter, err := CompileJQ(`. | invalid_function`)
	g.Expect(err).ShouldNot(BeNil())

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"name": "John"})
	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeNil())
}

func Test_ApplyFilter_NilInputData(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter, err := CompileJQ(`.`)
	g.Expect(err).Should(BeNil())

	result, err := filter.ApplyFilter(jqFilter, nil)
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())
}

func Test_ApplyFilter_EmptyFilter(t *testing.T) {
	g := NewWithT(t)

	_, err := CompileJQ(``)
	g.Expect(err).ShouldNot(BeNil())
}

func Test_ApplyFilter_EmptyResult(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	// Filter that returns no results
	jqFilter, err := CompileJQ(`.nonexistent`)
	g.Expect(err).Should(BeNil())

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"name": "John"})
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())

	var resultMap any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(BeNil()) // Expect result to be nil (empty)
}

func Test_ApplyFilter_PanicSafety(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ApplyFilter panicked: %v", r)
		}
	}()

	jqFilter, err := CompileJQ(`.`)
	g.Expect(err).Should(BeNil())

	// Test with data that could potentially cause a panic
	_, err = filter.ApplyFilter(jqFilter, map[string]any{"key": func() {}})
	g.Expect(err).ShouldNot(BeNil())
}

func Test_ShouldNotMutateInputData(t *testing.T) {
	g := NewWithT(t)

	jqFilter, err := CompileJQ(`.a`)
	require.NoError(t, err)
	filter := NewFilter()
	input := map[string]interface{}{
		"a": "b",
		"c": "d",
	}

	// Create a deep copy of the input for comparison later
	inputBytes, err := json.Marshal(input)
	require.NoError(t, err)
	var originalInput map[string]interface{}
	err = json.Unmarshal(inputBytes, &originalInput)
	require.NoError(t, err)

	_, err = filter.ApplyFilter(jqFilter, input)
	require.NoError(t, err)

	g.Expect(originalInput).Should(Equal(input))

}
