package hook

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

// Test conversion of BindingContext for v1, also test json marshal of v1 binding contexts.
func Test_BindingContext_Convert_V1(t *testing.T) {
	var bc BindingContextV1
	var jsonDump map[string]interface{}

	tests := []struct {
		name string
		bc   BindingContext
		fn   func()
	}{
		{
			"non kubernetes binding",
			BindingContext{
				Binding: "onStartup",
			},
			func() {
				assert.Equal(t, "onStartup", bc.Binding)
				assert.Equal(t, "onStartup", jsonDump["binding"])
				assert.Len(t, jsonDump, 1)
			},
		},
		{
			"kubernetes Event binding",
			BindingContext{
				Binding:    "kubernetes",
				Type:       "Event",
				WatchEvent: kube_events_manager.WatchEventAdded,
				Object: map[string]interface{}{
					"metadata": map[string]string{
						"namespace": "default",
					},
					"kind": "Pod",
					"name": "pod-qwe",
				},
				Kind:      "Pod",
				Name:      "pod-qwe",
				Namespace: "default",
			},
			func() {
				assert.Equal(t, "kubernetes", bc.Binding)
				assert.Equal(t, "Event", bc.Type)
				assert.Contains(t, bc.WatchEvent, "Added")
				assert.Contains(t, bc.Object, "metadata")

				// test json marshal
				assert.Equal(t, "kubernetes", jsonDump["binding"])
				assert.NotContains(t, jsonDump, "objects")
				assert.Contains(t, jsonDump, "watchEvent")
				assert.Equal(t, jsonDump["watchEvent"], "Added")
				assert.Contains(t, jsonDump, "object")
				assert.NotContains(t, jsonDump, "filterResult")
			},
		},
		{
			"kubernetes Synchronization binding",
			BindingContext{
				Binding:    "kubernetes",
				Type:       "Synchronization",
				WatchEvent: "Added",
				Objects: []interface{}{
					kube_events_manager.ObjectAndFilterResult{
						Object: map[string]interface{}{
							"metadata": map[string]string{
								"namespace": "default",
							},
							"kind": "Pod",
							"name": "pod-qwe",
						},
						FilterResult: "",
					},
					kube_events_manager.ObjectAndFilterResult{
						Object: map[string]interface{}{
							"metadata": map[string]string{
								"namespace": "default",
							},
							"kind": "Deployment",
							"name": "deployment-test",
						},
						FilterResult: "{\"labels\":{\"label-name\":\"label-value\"}}",
					},
				},
			},
			func() {
				assert.Len(t, bc.Objects, 2)
				assert.Equal(t, "Synchronization", bc.Type)
				assert.Equal(t, "kubernetes", bc.Binding)
				assert.Len(t, bc.Object, 0)
				assert.Equal(t, "", bc.FilterResult)

				// test json marshal
				assert.Equal(t, "kubernetes", jsonDump["binding"])
				assert.NotContains(t, jsonDump, "object")
				assert.NotContains(t, jsonDump, "filterResult")
				assert.NotContains(t, jsonDump, "watchEvent")
			},
		},
		{
			"kubernetes empty Synchronization binding",
			BindingContext{
				Binding: "kubernetes",
				Type:    "Synchronization",
				Objects: []interface{}{},
			},
			func() {
				assert.Len(t, bc.Objects, 0)
				assert.Equal(t, "Synchronization", bc.Type)
				assert.Equal(t, "kubernetes", bc.Binding)
				assert.Len(t, bc.Object, 0)
				assert.Equal(t, "", bc.FilterResult)

				// test json marshal
				assert.Equal(t, "kubernetes", jsonDump["binding"])
				assert.Equal(t, "Synchronization", jsonDump["type"])

				assert.NotContains(t, jsonDump, "object")
				assert.NotContains(t, jsonDump, "filterResult")
				assert.NotContains(t, jsonDump, "watchEvent")

				assert.Contains(t, jsonDump, "objects")
				assert.Len(t, jsonDump["objects"], 0)
				assert.Equal(t, []interface{}{}, jsonDump["objects"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonDump = make(map[string]interface{}, 0)
			bcList := ConvertBindingContextListV1([]BindingContext{tt.bc})
			assert.Len(t, bcList, 1)
			bc = bcList[0]
			jsonData, err := json.Marshal(bc.Map())
			assert.NoError(t, err)
			err = json.Unmarshal(jsonData, &jsonDump)
			//fmt.Printf("bc.Map(): %#v\njsonData: %s\njsonDump: %#v\n", bc.Map(), string(jsonData), jsonDump)
			assert.NoError(t, err)
			tt.fn()
		})
	}
}

func Test_BindingContext_Convert_V0(t *testing.T) {
	bc := BindingContext{
		Binding:    "onKubernetesEvent",
		Type:       "Event",
		WatchEvent: kube_events_manager.WatchEventAdded,
		Object: map[string]interface{}{
			"metadata": map[string]string{
				"namespace": "default",
			},
			"kind": "Pod",
			"name": "pod-qwe",
		},
		Kind:      "Pod",
		Name:      "pod-qwe",
		Namespace: "default",
	}

	bc0 := ConvertBindingContextListV0([]BindingContext{bc})
	assert.Len(t, bc0, 1)
	bc0_0 := bc0[0]
	assert.Equal(t, "onKubernetesEvent", bc0_0.Binding)
	assert.Equal(t, "pod-qwe", bc0_0.ResourceName)
	assert.Equal(t, "Pod", bc0_0.ResourceKind)
	assert.Equal(t, "default", bc0_0.ResourceNamespace)
	assert.Equal(t, "add", bc0_0.ResourceEvent)

	data, err := json.Marshal(bc0_0)
	if !assert.NoError(t, err) {
		t.Logf("bindingContext: %v", string(data))
	}
}
