package hook

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
)

func Test_BindingContext_Convert_V1(t *testing.T) {
	bc := BindingContext{
		Binding: "kubernetes",
		Type:    "Synchronization",
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
	}

	bc1 := ConvertBindingContextListV1([]BindingContext{bc})
	assert.Len(t, bc1, 1)
	bc1_0 := bc1[0]
	assert.Len(t, bc1_0.Objects, 2)
	assert.Equal(t, "Synchronization", bc1_0.Type)
	assert.Equal(t, "kubernetes", bc1_0.Binding)
	assert.Len(t, bc1_0.Object, 0)
	assert.Equal(t, "", bc1_0.FilterResult)

	data, err := json.Marshal(bc1_0)
	if assert.NoError(t, err) {
		t.Logf("bindingContext: %v", string(data))
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
	if assert.NoError(t, err) {
		t.Logf("bindingContext: %v", string(data))
	}
}
