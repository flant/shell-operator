package json

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshal_Struct(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	in := sample{Name: "test", Value: 42}

	got, err := Marshal(in)
	require.NoError(t, err)

	want, err := json.Marshal(in)
	require.NoError(t, err)

	assert.JSONEq(t, string(want), string(got))
}

func TestMarshal_Map(t *testing.T) {
	in := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "pod-1",
			"namespace": "default",
		},
		"kind":       "Pod",
		"apiVersion": "v1",
	}

	got, err := Marshal(in)
	require.NoError(t, err)

	want, err := json.Marshal(in)
	require.NoError(t, err)

	assert.JSONEq(t, string(want), string(got))
}

func TestMarshal_Nil(t *testing.T) {
	got, err := Marshal(nil)
	require.NoError(t, err)
	assert.Equal(t, "null", string(got))
}

func TestMarshalIndent(t *testing.T) {
	in := map[string]string{"a": "b"}

	got, err := MarshalIndent(in, "", "  ")
	require.NoError(t, err)

	want, err := json.MarshalIndent(in, "", "  ")
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got))
}

func TestUnmarshal_Struct(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := `{"name":"hello","value":99}`

	var got sample
	require.NoError(t, Unmarshal([]byte(data), &got))
	assert.Equal(t, "hello", got.Name)
	assert.Equal(t, 99, got.Value)
}

func TestUnmarshal_Map(t *testing.T) {
	data := `{"key":"val","nested":{"a":1}}`

	var got map[string]interface{}
	require.NoError(t, Unmarshal([]byte(data), &got))
	assert.Equal(t, "val", got["key"])
	nested, ok := got["nested"].(map[string]interface{})
	require.True(t, ok)
	assert.InDelta(t, float64(1), nested["a"], 0.001)
}

func TestUnmarshal_InvalidJSON(t *testing.T) {
	var v interface{}
	err := Unmarshal([]byte("not-json"), &v)
	assert.Error(t, err)
}

func TestNewEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	require.NoError(t, enc.Encode(map[string]int{"x": 1}))

	var got map[string]int
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, 1, got["x"])
}

func TestNewDecoder(t *testing.T) {
	input := `{"name":"test","count":3}`
	dec := NewDecoder(strings.NewReader(input))

	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	var got sample
	require.NoError(t, dec.Decode(&got))
	assert.Equal(t, "test", got.Name)
	assert.Equal(t, 3, got.Count)
}

func TestNewDecoder_Stream(t *testing.T) {
	input := `{"a":1}
{"a":2}
{"a":3}
`
	dec := NewDecoder(strings.NewReader(input))

	var results []int
	for {
		var v map[string]int
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		results = append(results, v["a"])
	}
	assert.Equal(t, []int{1, 2, 3}, results)
}

func TestValid(t *testing.T) {
	assert.True(t, Valid([]byte(`{"key": "value"}`)))
	assert.True(t, Valid([]byte(`[1,2,3]`)))
	assert.True(t, Valid([]byte(`"hello"`)))
	assert.False(t, Valid([]byte(`{invalid`)))
	assert.False(t, Valid([]byte(``)))
}

func TestNumber(t *testing.T) {
	data := `{"n": 12345678901234567890}`

	dec := NewDecoder(strings.NewReader(data))
	dec.UseNumber()

	var got map[string]interface{}
	require.NoError(t, dec.Decode(&got))

	num, ok := got["n"].(Number)
	require.True(t, ok)
	assert.Equal(t, "12345678901234567890", num.String())
}

func TestMarshal_CustomMarshaler(t *testing.T) {
	v := customType{Val: "hello"}
	got, err := Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, `"custom:hello"`, string(got))
}

type customType struct {
	Val string
}

func (c customType) MarshalJSON() ([]byte, error) {
	return Marshal("custom:" + c.Val)
}

func TestRoundTrip_Slice(t *testing.T) {
	original := []map[string]interface{}{
		{"binding": "test", "type": "Event"},
		{"binding": "sync", "type": "Synchronization"},
	}

	data, err := Marshal(original)
	require.NoError(t, err)

	var decoded []map[string]interface{}
	require.NoError(t, Unmarshal(data, &decoded))

	assert.Len(t, decoded, 2)
	assert.Equal(t, "test", decoded[0]["binding"])
	assert.Equal(t, "Synchronization", decoded[1]["type"])
}

func TestRoundTrip_NestedUnstructured(t *testing.T) {
	original := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "my-config",
			"namespace": "kube-system",
			"labels": map[string]interface{}{
				"app": "test",
			},
		},
		"data": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	data, err := Marshal(original)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, Unmarshal(data, &decoded))

	assert.Equal(t, "v1", decoded["apiVersion"])
	meta, ok := decoded["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "my-config", meta["name"])
	labels, ok := meta["labels"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test", labels["app"])
}

func TestEncoder_MultipleValues(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	require.NoError(t, enc.Encode(map[string]int{"a": 1}))
	require.NoError(t, enc.Encode(map[string]int{"b": 2}))

	dec := NewDecoder(&buf)
	var v1, v2 map[string]int
	require.NoError(t, dec.Decode(&v1))
	require.NoError(t, dec.Decode(&v2))
	assert.Equal(t, 1, v1["a"])
	assert.Equal(t, 2, v2["b"])
}

func BenchmarkMarshal_Map(b *testing.B) {
	in := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "pod-1",
			"namespace": "default",
			"labels":    map[string]interface{}{"app": "test", "version": "v1"},
		},
		"kind":       "Pod",
		"apiVersion": "v1",
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "main", "image": "nginx:latest"},
			},
		},
	}
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkUnmarshal_Map(b *testing.B) {
	data := []byte(`{"metadata":{"name":"pod-1","namespace":"default","labels":{"app":"test","version":"v1"}},"kind":"Pod","apiVersion":"v1","spec":{"containers":[{"name":"main","image":"nginx:latest"}]}}`)
	b.ResetTimer()
	for range b.N {
		var v map[string]interface{}
		_ = Unmarshal(data, &v)
	}
}
