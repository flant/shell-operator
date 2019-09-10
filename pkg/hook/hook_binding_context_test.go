package hook

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/stretchr/testify/assert"
)

func Test_BindingContext_Convert(t *testing.T) {
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
		fmt.Printf("bindingContext: %v", string(data))
	}
}
