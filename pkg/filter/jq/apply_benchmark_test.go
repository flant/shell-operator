package jq

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func setupNodeList(count int) []map[string]any {
	var node map[string]any
	content, _ := os.ReadFile("testdata/test.yaml")
	yaml.Unmarshal(content, &node)

	nodeslice := make([]map[string]any, 0, count)

	for range count {
		nodeslice = append(nodeslice, node)
	}

	return nodeslice
}

func benchmarkApplyFilter(b *testing.B, filter string, objs []map[string]any) {
	f := NewFilter()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, obj := range objs {
			// res, _ := f.ApplyFilter(filter, obj)
			// fmt.Println(string(res))
			f.ApplyFilter(filter, obj)
		}
	}
}

func BenchmarkApplyFilter_dot_10(b *testing.B) {
	benchmarkApplyFilter(b, ".", setupNodeList(10))
}
func BenchmarkApplyFilter_dot_100(b *testing.B) {
	benchmarkApplyFilter(b, ".", setupNodeList(100))
}
func BenchmarkApplyFilter_dot_1000(b *testing.B) {
	benchmarkApplyFilter(b, ".", setupNodeList(1000))
}

func BenchmarkApplyFilter_easy_10(b *testing.B) {
	benchmarkApplyFilter(b, ".metadata.name", setupNodeList(10))
}

func BenchmarkApplyFilter_easy_100(b *testing.B) {
	benchmarkApplyFilter(b, ".metadata.name", setupNodeList(100))
}

func BenchmarkApplyFilter_easy_1000(b *testing.B) {
	benchmarkApplyFilter(b, ".metadata.name", setupNodeList(1000))
}
