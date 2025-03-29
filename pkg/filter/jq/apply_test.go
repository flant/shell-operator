package jq

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_ApplyFilter_SingleDocumentModification(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jsonData := []byte(`{"name": "John", "age": 30}`)
	jqFilter := `. + {"status": "active"}`

	result, err := filter.ApplyFilter(jqFilter, jsonData)

	g.Expect(err).Should(BeNil())
	g.Expect(result).Should(MatchJSON(`{"name": "John", "age": 30, "status": "active"}`))
}

func Test_ApplyFilter_ExtractValuesFromDocument(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jsonData := []byte(`{"user": {"name": "John", "details": {"location": "New York", "occupation": "Developer"}}}`)
	jqFilter := `.user.details`

	result, err := filter.ApplyFilter(jqFilter, jsonData)

	g.Expect(err).Should(BeNil())
	g.Expect(result).Should(MatchJSON(`{"location": "New York", "occupation": "Developer"}`))
}

func Test_ApplyFilter_MultipleJsonDocumentsInArray(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jsonData := []byte(`{"users": [{"name": "John", "status": "inactive"}, {"name": "Jane", "status": "inactive"}]}`)
	jqFilter := `.users[] | . + {"status": "active"}`

	result, err := filter.ApplyFilter(jqFilter, jsonData)

	g.Expect(err).Should(BeNil())
	g.Expect(result).Should(Equal(`{"name":"John","status":"active"}{"name":"Jane","status":"active"}`))
}

func Test_ApplyFilter_InvalidFilter(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	jsonData := []byte(`{"name": "John"}`)
	invalidFilter := `. | invalid_function`

	result, err := filter.ApplyFilter(invalidFilter, jsonData)

	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeEmpty())
}

func Test_ApplyFilter_InvalidJson(t *testing.T) {
	g := NewWithT(t)
	filter := NewFilter()

	invalidJson := []byte(`{"name": "John" invalid_json`)
	jqFilter := `.name`

	result, err := filter.ApplyFilter(jqFilter, invalidJson)

	g.Expect(err).ShouldNot(BeNil())
	g.Expect(result).Should(BeEmpty())
}
