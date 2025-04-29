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

	var resultMap []any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(HaveLen(1))
	g.Expect(resultMap[0]).Should(Equal(map[string]any{"name": "John", "age": 30.0, "status": "active"}))
}

func Test_ApplyFilter_ExtractValuesFromDocument(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jqFilter := `.user.details`

	result, err := filter.ApplyFilter(jqFilter, map[string]any{"user": map[string]any{"name": "John", "details": map[string]any{"location": "New York", "occupation": "Developer"}}})
	g.Expect(err).Should(BeNil())

	var resultMap []any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(HaveLen(1))
	g.Expect(resultMap[0]).Should(Equal(map[string]any{"location": "New York", "occupation": "Developer"}))
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

	invalidFilter := `. | invalid_function`

	result, err := filter.ApplyFilter(invalidFilter, map[string]any{"name": "John"})
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

	var resultMap []any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).ShouldNot(BeNil())
}

func Test_deepCopy(t *testing.T) {
	g := NewWithT(t)

	original := map[string]any{
		"name": "John",
		"age":  30.0,
		"address": map[string]any{
			"city":  "New York",
			"state": "NY",
		},
	}

	cp := deepCopy(original)

	g.Expect(cp).Should(Equal(original))

	cp["name"] = "Jane"
	cp["address"].(map[string]any)["city"] = "Los Angeles"

	g.Expect(original["name"]).Should(Equal("John"))
	g.Expect(original["address"].(map[string]any)["city"]).Should(Equal("New York"))
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
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())
}

func Test_ApplyFilter_EmptyResult(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	// Filter that returns no results
	result, err := filter.ApplyFilter(`.nonexistent`, map[string]any{"name": "John"})
	g.Expect(err).Should(BeNil())
	g.Expect(result).ShouldNot(BeNil())

	var resultMap []any
	err = json.Unmarshal(result, &resultMap)
	g.Expect(err).Should(BeNil())
	g.Expect(resultMap).Should(HaveLen(1))
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
