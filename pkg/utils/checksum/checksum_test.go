package checksum

import (
	"encoding/json"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type testStruct struct {
	Name  string
	Value int
	Tags  []string
	Meta  map[string]string
}

func TestCalculateStableChecksum_MapStability(t *testing.T) {
	map1 := map[string]interface{}{"a": 1, "b": "hello", "c": true}
	map2 := map[string]interface{}{"b": "hello", "c": true, "a": 1} // Same data, different order

	hash1, err1 := CalculateChecksum(map1)
	require.NoError(t, err1)

	hash2, err2 := CalculateChecksum(map2)
	require.NoError(t, err2)

	assert.Equal(t, hash1, hash2, "Hashes for maps with same data but different key order should be equal")
}

func TestCalculateStableChecksum_MapDifference(t *testing.T) {
	map1 := map[string]interface{}{"a": 1, "b": "hello"}
	map2 := map[string]interface{}{"a": 1, "b": "world"} // Different value for key "b"

	hash1, err1 := CalculateChecksum(map1)
	require.NoError(t, err1)

	hash2, err2 := CalculateChecksum(map2)
	require.NoError(t, err2)

	assert.NotEqual(t, hash1, hash2, "Hashes for maps with different data should not be equal")
}

func TestCalculateStableChecksum_StructStability(t *testing.T) {
	s1 := testStruct{Name: "test", Value: 10, Tags: []string{"tag1", "tag2"}, Meta: map[string]string{"key": "val"}}
	s2 := testStruct{Name: "test", Value: 10, Tags: []string{"tag1", "tag2"}, Meta: map[string]string{"key": "val"}}

	hash1, err1 := CalculateChecksum(s1)
	require.NoError(t, err1)

	hash2, err2 := CalculateChecksum(s2)
	require.NoError(t, err2)

	assert.Equal(t, hash1, hash2, "Hashes for identical structs should be equal")
}

func TestCalculateStableChecksum_StructDifference(t *testing.T) {
	s1 := testStruct{Name: "test", Value: 10}
	s2 := testStruct{Name: "test", Value: 20} // Different value

	hash1, err1 := CalculateChecksum(s1)
	require.NoError(t, err1)

	hash2, err2 := CalculateChecksum(s2)
	require.NoError(t, err2)

	assert.NotEqual(t, hash1, hash2, "Hashes for different structs should not be equal")
}

func TestCalculateStableChecksum_SliceOrder(t *testing.T) {
	slice1 := []string{"a", "b", "c"}
	slice2 := []string{"c", "a", "b"} // Same elements, different order

	hash1, err1 := CalculateChecksum(slice1)
	require.NoError(t, err1)

	hash2, err2 := CalculateChecksum(slice2)
	require.NoError(t, err2)

	assert.NotEqual(t, hash1, hash2, "Hashes for slices with different order should not be equal")
}

func TestCalculateStableChecksum_Slice(t *testing.T) {
	slice1 := []string{"a", "b", "c"}
	slice2 := []string{"a", "b", "c"}

	hash1, err1 := CalculateChecksum(slice1)
	require.NoError(t, err1)

	hash2, err2 := CalculateChecksum(slice2)
	require.NoError(t, err2)

	assert.Equal(t, hash1, hash2, "Hashes for slices with same elements should be equal")
}

func TestCalculateStableChecksum_NilAndEmpty(t *testing.T) {
	hashNil, errNil := CalculateChecksum(nil)
	require.NoError(t, errNil)
	assert.Equal(t, uint64(0), hashNil, "Hash of nil should be 0")

	hashEmptyMap, errEmptyMap := CalculateChecksum(map[string]string{})
	require.NoError(t, errEmptyMap)

	hashOtherEmptyMap, errOtherEmptyMap := CalculateChecksum(map[string]string{})
	require.NoError(t, errOtherEmptyMap)
	assert.Equal(t, hashEmptyMap, hashOtherEmptyMap)
}

func TestCalculateStableChecksum_CycleDetection(t *testing.T) {
	type cyclicStruct struct {
		Name string
		Next *cyclicStruct
	}
	c1 := &cyclicStruct{Name: "c1"}
	c1.Next = c1 // Cycle

	_, err := CalculateChecksum(c1)
	assert.NoError(t, err, "Should not error on cyclic structures")
}

func TestCalculateChecksum_Concurrent(t *testing.T) {
	const goroutines = 16
	const iterations = 1000
	ch := make(chan error, goroutines)

	// Используем разные объекты для разных потоков
	objects := []interface{}{
		map[string]interface{}{"a": 1, "b": "hello", "c": true},
		[]string{"a", "b", "c"},
		testStruct{Name: "test", Value: 10, Tags: []string{"tag1", "tag2"}, Meta: map[string]string{"key": "val"}},
		benchmarkUnstructuredPod,
		benchmarkString,
		benchmarkSlice,
		benchmarkByteSlice,
		benchmarkStructWithBytes,
	}

	for g := 0; g < goroutines; g++ {
		go func(gid int) {
			for i := 0; i < iterations; i++ {
				obj := objects[(gid+i)%len(objects)]
				_, err := CalculateChecksum(obj)
				if err != nil {
					ch <- err
					return
				}
			}
			ch <- nil
		}(g)
	}

	for i := 0; i < goroutines; i++ {
		err := <-ch
		assert.NoError(t, err, "No error expected in concurrent CalculateChecksum")
	}
}

var (
	benchmarkUnstructuredPod = &unstructured.Unstructured{
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
	// A reasonably long string
	benchmarkString = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."

	// A slice with mixed data types
	benchmarkSlice = []interface{}{
		"string_element",
		"string_element_2",
		"string_element_3",
		"string_element_4",
		"string_element_5",
		"string_element_6",
		"string_element_7",
		"string_element_8",
		"string_element_9",
	}

	// A simple byte slice
	benchmarkByteSlice = []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0xED, 0xFA, 0xCE, 0xDE, 0xAD, 0xBE, 0xEF, 0xFE, 0xED, 0xFA, 0xCE}

	// A struct with a byte slice to test direct handling
	benchmarkStructWithBytes = struct {
		ID   int
		Data []byte
	}{
		ID:   123,
		Data: []byte("some important payload that should be hashed directly"),
	}
)

func BenchmarkCalculateChecksum(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CalculateChecksum(benchmarkUnstructuredPod)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateChecksum_String(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CalculateChecksum(benchmarkString)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateChecksum_Slice(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CalculateChecksum(benchmarkSlice)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateChecksum_ByteSlice(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CalculateChecksum(benchmarkByteSlice)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateChecksum_StructWithBytes(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CalculateChecksum(benchmarkStructWithBytes)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJsonAndXxhash(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(benchmarkUnstructuredPod)
		if err != nil {
			b.Fatal(err)
		}
		hasher := xxhash.New()
		hasher.Write(data)
		_ = hasher.Sum64()
	}
}
