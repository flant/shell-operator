package json

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Data generators – mimic the real payloads used by shell-operator.
// ---------------------------------------------------------------------------

func makeSmallMap() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "pod-abc",
			"namespace": "default",
		},
	}
}

func makeMediumMap() map[string]interface{} {
	containers := make([]interface{}, 3)
	for i := range containers {
		containers[i] = map[string]interface{}{
			"name":  fmt.Sprintf("container-%d", i),
			"image": "nginx:1.25",
			"ports": []interface{}{
				map[string]interface{}{"containerPort": float64(8080 + i)},
			},
			"env": []interface{}{
				map[string]interface{}{"name": "ENV_VAR", "value": "val"},
			},
		}
	}
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "pod-abc",
			"namespace": "kube-system",
			"labels": map[string]interface{}{
				"app":     "nginx",
				"version": "v1",
				"team":    "platform",
			},
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": strings.Repeat("x", 200),
			},
		},
		"spec": map[string]interface{}{
			"containers":    containers,
			"restartPolicy": "Always",
			"nodeName":      "worker-01",
		},
		"status": map[string]interface{}{
			"phase": "Running",
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
				map[string]interface{}{"type": "Initialized", "status": "True"},
			},
		},
	}
}

func makeLargeBindingContext(objectCount int) []map[string]interface{} {
	objects := make([]interface{}, objectCount)
	for i := range objects {
		objects[i] = map[string]interface{}{
			"object": map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      fmt.Sprintf("cm-%d", i),
					"namespace": "default",
					"labels": map[string]interface{}{
						"app": "test",
					},
				},
				"data": map[string]interface{}{
					"key1": strings.Repeat("a", 50),
					"key2": strings.Repeat("b", 50),
				},
			},
			"filterResult": map[string]interface{}{
				"name": fmt.Sprintf("cm-%d", i),
			},
		}
	}
	return []map[string]interface{}{
		{
			"binding":    "configmaps",
			"type":       "Synchronization",
			"watchEvent": "Added",
			"objects":    objects,
		},
	}
}

type metricOperation struct {
	Name    string            `json:"name"`
	Value   *float64          `json:"value,omitempty"`
	Buckets []float64         `json:"buckets,omitempty"`
	Labels  map[string]string `json:"labels"`
	Group   string            `json:"group,omitempty"`
	Action  string            `json:"action,omitempty"`
}

func makeMetricOps(count int) []metricOperation {
	ops := make([]metricOperation, count)
	v := float64(1.0)
	for i := range ops {
		ops[i] = metricOperation{
			Name:   fmt.Sprintf("metric_%d", i),
			Value:  &v,
			Labels: map[string]string{"hook": "test", "queue": "main"},
			Action: "set",
		}
	}
	return ops
}

type admissionReview struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Request    admissionRequest  `json:"request"`
	Response   admissionResponse `json:"response,omitempty"`
}

type admissionRequest struct {
	UID       string                 `json:"uid"`
	Kind      map[string]string      `json:"kind"`
	Resource  map[string]string      `json:"resource"`
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Operation string                 `json:"operation"`
	Object    map[string]interface{} `json:"object"`
}

type admissionResponse struct {
	UID     string `json:"uid"`
	Allowed bool   `json:"allowed"`
}

func makeAdmissionReview() admissionReview {
	return admissionReview{
		APIVersion: "admission.k8s.io/v1",
		Kind:       "AdmissionReview",
		Request: admissionRequest{
			UID:       "abc-123-def",
			Kind:      map[string]string{"group": "", "version": "v1", "kind": "Pod"},
			Resource:  map[string]string{"group": "", "version": "v1", "resource": "pods"},
			Name:      "my-pod",
			Namespace: "default",
			Operation: "CREATE",
			Object:    makeMediumMap(),
		},
	}
}

// ---------------------------------------------------------------------------
// Marshal benchmarks
// ---------------------------------------------------------------------------

func BenchmarkMarshal_SmallMap_GoJSON(b *testing.B) {
	in := makeSmallMap()
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkMarshal_SmallMap_StdJSON(b *testing.B) {
	in := makeSmallMap()
	b.ResetTimer()
	for range b.N {
		_, _ = stdjson.Marshal(in)
	}
}

func BenchmarkMarshal_MediumMap_GoJSON(b *testing.B) {
	in := makeMediumMap()
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkMarshal_MediumMap_StdJSON(b *testing.B) {
	in := makeMediumMap()
	b.ResetTimer()
	for range b.N {
		_, _ = stdjson.Marshal(in)
	}
}

func BenchmarkMarshal_LargeBindingCtx50_GoJSON(b *testing.B) {
	in := makeLargeBindingContext(50)
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkMarshal_LargeBindingCtx50_StdJSON(b *testing.B) {
	in := makeLargeBindingContext(50)
	b.ResetTimer()
	for range b.N {
		_, _ = stdjson.Marshal(in)
	}
}

func BenchmarkMarshal_LargeBindingCtx500_GoJSON(b *testing.B) {
	in := makeLargeBindingContext(500)
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkMarshal_LargeBindingCtx500_StdJSON(b *testing.B) {
	in := makeLargeBindingContext(500)
	b.ResetTimer()
	for range b.N {
		_, _ = stdjson.Marshal(in)
	}
}

func BenchmarkMarshal_Struct_GoJSON(b *testing.B) {
	in := makeMetricOps(10)
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkMarshal_Struct_StdJSON(b *testing.B) {
	in := makeMetricOps(10)
	b.ResetTimer()
	for range b.N {
		_, _ = stdjson.Marshal(in)
	}
}

func BenchmarkMarshal_AdmissionReview_GoJSON(b *testing.B) {
	in := makeAdmissionReview()
	b.ResetTimer()
	for range b.N {
		_, _ = Marshal(in)
	}
}

func BenchmarkMarshal_AdmissionReview_StdJSON(b *testing.B) {
	in := makeAdmissionReview()
	b.ResetTimer()
	for range b.N {
		_, _ = stdjson.Marshal(in)
	}
}

// ---------------------------------------------------------------------------
// Unmarshal benchmarks
// ---------------------------------------------------------------------------

func BenchmarkUnmarshal_SmallMap_GoJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeSmallMap())
	b.ResetTimer()
	for range b.N {
		var v map[string]interface{}
		_ = Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_SmallMap_StdJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeSmallMap())
	b.ResetTimer()
	for range b.N {
		var v map[string]interface{}
		_ = stdjson.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_MediumMap_GoJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeMediumMap())
	b.ResetTimer()
	for range b.N {
		var v map[string]interface{}
		_ = Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_MediumMap_StdJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeMediumMap())
	b.ResetTimer()
	for range b.N {
		var v map[string]interface{}
		_ = stdjson.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_LargeBindingCtx50_GoJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeLargeBindingContext(50))
	b.ResetTimer()
	for range b.N {
		var v []map[string]interface{}
		_ = Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_LargeBindingCtx50_StdJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeLargeBindingContext(50))
	b.ResetTimer()
	for range b.N {
		var v []map[string]interface{}
		_ = stdjson.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_LargeBindingCtx500_GoJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeLargeBindingContext(500))
	b.ResetTimer()
	for range b.N {
		var v []map[string]interface{}
		_ = Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_LargeBindingCtx500_StdJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeLargeBindingContext(500))
	b.ResetTimer()
	for range b.N {
		var v []map[string]interface{}
		_ = stdjson.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_Struct_GoJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeMetricOps(10))
	b.ResetTimer()
	for range b.N {
		var v []metricOperation
		_ = Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_Struct_StdJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeMetricOps(10))
	b.ResetTimer()
	for range b.N {
		var v []metricOperation
		_ = stdjson.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_AdmissionReview_GoJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeAdmissionReview())
	b.ResetTimer()
	for range b.N {
		var v admissionReview
		_ = Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_AdmissionReview_StdJSON(b *testing.B) {
	data, _ := stdjson.Marshal(makeAdmissionReview())
	b.ResetTimer()
	for range b.N {
		var v admissionReview
		_ = stdjson.Unmarshal(data, &v)
	}
}

// ---------------------------------------------------------------------------
// Encoder benchmarks (streaming write, like BindingContextList.WriteJson)
// ---------------------------------------------------------------------------

func BenchmarkEncoder_LargeBindingCtx500_GoJSON(b *testing.B) {
	in := makeLargeBindingContext(500)
	var buf bytes.Buffer
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		enc := NewEncoder(&buf)
		_ = enc.Encode(in)
	}
}

func BenchmarkEncoder_LargeBindingCtx500_StdJSON(b *testing.B) {
	in := makeLargeBindingContext(500)
	var buf bytes.Buffer
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		enc := stdjson.NewEncoder(&buf)
		_ = enc.Encode(in)
	}
}

// ---------------------------------------------------------------------------
// Decoder benchmarks (streaming read, like MetricOperationsFromReader)
// ---------------------------------------------------------------------------

func BenchmarkDecoder_MetricOps_GoJSON(b *testing.B) {
	ops := makeMetricOps(20)
	data, _ := stdjson.Marshal(ops)
	b.ResetTimer()
	for range b.N {
		dec := NewDecoder(bytes.NewReader(data))
		var v []metricOperation
		_ = dec.Decode(&v)
	}
}

func BenchmarkDecoder_MetricOps_StdJSON(b *testing.B) {
	ops := makeMetricOps(20)
	data, _ := stdjson.Marshal(ops)
	b.ResetTimer()
	for range b.N {
		dec := stdjson.NewDecoder(bytes.NewReader(data))
		var v []metricOperation
		_ = dec.Decode(&v)
	}
}
