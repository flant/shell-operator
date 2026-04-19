package bindingcontext

import (
	"bytes"
	json "github.com/flant/shell-operator/pkg/utils/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	htypes "github.com/flant/shell-operator/pkg/hook/types"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

func makeKubeEventBC() BindingContext {
	bc := BindingContext{
		Binding:    "test-binding",
		Type:       kemtypes.TypeEvent,
		WatchEvent: kemtypes.WatchEventAdded,
		Objects: []kemtypes.ObjectAndFilterResult{
			{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"namespace": "default",
							"name":      "pod-abc",
						},
						"kind": "Pod",
					},
				},
			},
		},
	}
	bc.Metadata.BindingType = htypes.OnKubernetesEvent
	bc.Metadata.Version = "v1"
	return bc
}

func makeSyncBC(count int) BindingContext {
	objects := make([]kemtypes.ObjectAndFilterResult, count)
	for i := range count {
		objects[i] = kemtypes.ObjectAndFilterResult{
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "pod-" + string(rune('a'+i%26)),
					},
					"kind": "Pod",
				},
			},
		}
	}
	bc := BindingContext{
		Binding: "sync-binding",
		Type:    kemtypes.TypeSynchronization,
		Objects: objects,
	}
	bc.Metadata.BindingType = htypes.OnKubernetesEvent
	bc.Metadata.Version = "v1"
	return bc
}

func TestBindingContextList_Json(t *testing.T) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeKubeEventBC()})

	data, err := bcList.Json()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var parsed []map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Len(t, parsed, 1)
	assert.Equal(t, "test-binding", parsed[0]["binding"])
}

func TestBindingContextList_WriteJson(t *testing.T) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeKubeEventBC()})

	var buf bytes.Buffer
	err := bcList.WriteJson(&buf)
	require.NoError(t, err)
	require.NotEmpty(t, buf.Bytes())

	var parsed []map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Len(t, parsed, 1)
	assert.Equal(t, "test-binding", parsed[0]["binding"])
}

func TestBindingContextList_Json_and_WriteJson_produce_equivalent_output(t *testing.T) {
	bcList := ConvertBindingContextList("v1", []BindingContext{
		makeKubeEventBC(),
		makeSyncBC(3),
	})

	jsonBytes, err := bcList.Json()
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, bcList.WriteJson(&buf))

	// Both should parse to the same structure.
	var fromJson, fromWriter interface{}
	require.NoError(t, json.Unmarshal(jsonBytes, &fromJson))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &fromWriter))
	assert.Equal(t, fromJson, fromWriter)
}

func TestBindingContextList_Json_empty(t *testing.T) {
	bcList := BindingContextList{}

	data, err := bcList.Json()
	require.NoError(t, err)

	var parsed []interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Empty(t, parsed)
}

func TestBindingContextList_WriteJson_empty(t *testing.T) {
	bcList := BindingContextList{}

	var buf bytes.Buffer
	require.NoError(t, bcList.WriteJson(&buf))

	var parsed []interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Empty(t, parsed)
}

func TestConvertBindingContextList_OnStartup(t *testing.T) {
	bc := BindingContext{Binding: "onStartup"}
	bc.Metadata.BindingType = htypes.OnStartup

	bcList := ConvertBindingContextList("v1", []BindingContext{bc})
	assert.Len(t, bcList, 1)
	assert.Equal(t, "onStartup", bcList[0]["binding"])
	_, hasType := bcList[0]["type"]
	assert.False(t, hasType)
}

func TestConvertBindingContextList_Schedule(t *testing.T) {
	bc := BindingContext{Binding: "every-5m"}
	bc.Metadata.BindingType = htypes.Schedule

	bcList := ConvertBindingContextList("v1", []BindingContext{bc})
	assert.Len(t, bcList, 1)
	assert.Equal(t, "Schedule", bcList[0]["type"])
}

func TestConvertBindingContextList_KubeEvent(t *testing.T) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeKubeEventBC()})
	assert.Len(t, bcList, 1)
	assert.Equal(t, kemtypes.TypeEvent, bcList[0]["type"])
	assert.Equal(t, "Added", bcList[0]["watchEvent"])
	assert.NotNil(t, bcList[0]["object"])
}

func TestConvertBindingContextList_Synchronization(t *testing.T) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeSyncBC(2)})
	assert.Len(t, bcList, 1)
	assert.Equal(t, kemtypes.TypeSynchronization, bcList[0]["type"])
	objects, ok := bcList[0]["objects"].([]kemtypes.ObjectAndFilterResult)
	require.True(t, ok)
	assert.Len(t, objects, 2)
}

func TestConvertBindingContextList_SynchronizationEmpty(t *testing.T) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeSyncBC(0)})
	assert.Len(t, bcList, 1)
	assert.Equal(t, kemtypes.TypeSynchronization, bcList[0]["type"])
	// Empty objects should be an empty array, not nil.
	objects, ok := bcList[0]["objects"].([]string)
	require.True(t, ok)
	assert.Empty(t, objects)
}

func TestConvertBindingContextList_Group(t *testing.T) {
	bc := BindingContext{Binding: "my-binding"}
	bc.Metadata.BindingType = htypes.OnKubernetesEvent
	bc.Metadata.Group = "my-group"

	bcList := ConvertBindingContextList("v1", []BindingContext{bc})
	assert.Len(t, bcList, 1)
	assert.Equal(t, "Group", bcList[0]["type"])
	assert.Equal(t, "my-group", bcList[0]["groupName"])
}

func BenchmarkBindingContextList_Json(b *testing.B) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeSyncBC(100)})
	b.ResetTimer()
	for range b.N {
		_, _ = bcList.Json()
	}
}

func BenchmarkBindingContextList_WriteJson(b *testing.B) {
	bcList := ConvertBindingContextList("v1", []BindingContext{makeSyncBC(100)})
	var buf bytes.Buffer
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		_ = bcList.WriteJson(&buf)
	}
}
