package hook

import (
	"testing"

	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/stretchr/testify/assert"

	. "github.com/flant/libjq-go"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func JqEqual(t *testing.T, input []byte, program string, expected string) {
	res, err := Jq().Program(program).Run(string(input))
	if assert.NoError(t, err) {
		assert.Equal(t, expected, res, "jq: '%s', json was '%s'", program, string(input))
	}
}

// Test conversion of BindingContext for v1, also test json marshal of v1 binding contexts.
func Test_ConvertBindingContextList_v1(t *testing.T) {
	var bcList BindingContextList
	var bcJson []byte

	tests := []struct {
		name        string
		bc          func() BindingContext
		fn          func()
		jqAsertions [][]string
	}{
		{
			"OnStartup binding",
			func() BindingContext {
				bc := BindingContext{
					Binding: "onStartup",
				}
				bc.Metadata.BindingType = OnStartup
				return bc
			},
			func() {
				assert.Equal(t, "onStartup", bcList[0]["binding"])
			},
			[][]string{
				{`.[0].binding`, `"onStartup"`},
				{`.[0] | length`, `1`},
			},
		},
		{
			"kubernetes Event binding",
			func() BindingContext {
				bc := BindingContext{
					Binding:    "kubernetes",
					Type:       TypeEvent,
					WatchEvent: WatchEventAdded,
					Objects: []ObjectAndFilterResult{
						{
							Object: &unstructured.Unstructured{
								Object: map[string]interface{}{
									"metadata": map[string]interface{}{
										"namespace": "default",
										"name":      "pod-qwe",
									},
									"kind": "Pod",
								},
							},
						},
					},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				return bc
			},

			func() {
				assert.Len(t, bcList[0], 4)
				assert.Equal(t, "kubernetes", bcList[0]["binding"])
				assert.Equal(t, TypeEvent, bcList[0]["type"])
				assert.Equal(t, "Added", bcList[0]["watchEvent"])
				assert.IsType(t, &unstructured.Unstructured{}, bcList[0]["object"])
				assert.Contains(t, bcList[0]["object"].(*unstructured.Unstructured).Object, "metadata")

			},
			[][]string{
				// JSON dump should has only 4 fields: binding, type, watchEvent and object.
				{`.[0] | length`, `4`},
				{`.[0].binding`, `"kubernetes"`},
				{`.[0].type`, `"Event"`},
				{`.[0].watchEvent`, `"Added"`},
				{`.[0].object.metadata.namespace`, `"default"`},
				{`.[0].object.metadata.name`, `"pod-qwe"`},
			},
		},
		{
			"kubernetes Synchronization event",
			func() BindingContext {
				bc := BindingContext{
					Binding: "kubernetes",
					Type:    TypeSynchronization,
					Objects: []ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent

				// object without jqfilter should not have filterResult field
				obj := ObjectAndFilterResult{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "pod-qwe",
							},
							"kind": "Pod",
						},
					},
					FilterResult: "asd",
				}
				obj.Metadata.JqFilter = ""
				bc.Objects = append(bc.Objects, obj)

				// object with jqfilter should have filterResult field
				obj = ObjectAndFilterResult{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "deployment-test",
							},
							"kind": "Deployment",
						},
					},
					FilterResult: "{\"labels\":{\"label-name\":\"label-value\"}}",
				}
				obj.Metadata.JqFilter = ".metadata.labels"
				bc.Objects = append(bc.Objects, obj)

				// object with jqfilter and with empty result should have filterResult field
				obj = ObjectAndFilterResult{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "deployment-2",
							},
							"kind": "Deployment",
						}},
					FilterResult: `""`,
				}
				obj.Metadata.JqFilter = ".metadata.labels"
				bc.Objects = append(bc.Objects, obj)

				return bc
			},
			func() {
				assert.Len(t, bcList[0], 3)
				assert.Len(t, bcList[0]["objects"], 3)
				assert.Equal(t, TypeSynchronization, bcList[0]["type"])
				assert.Equal(t, "kubernetes", bcList[0]["binding"])
				assert.NotContains(t, bcList[0], "object")
				assert.NotContains(t, bcList[0], "filterResult")
			},
			[][]string{
				// JSON dump should contains n fields: binding, type and objects
				{`.[0] | length`, `3`},
				{`.[0].binding`, `"kubernetes"`},
				{`.[0].type`, `"Synchronization"`},

				{`.[0].objects[] | select(.object.metadata.name == "pod-qwe") | has("filterResult")`, `false`},
				{`.[0].objects[] | select(.object.metadata.name == "deployment-test") | has("filterResult")`, `true`},
				{`.[0].objects[] | select(.object.metadata.name == "deployment-test") | .filterResult.labels."label-name"`, `"label-value"`},
				{`.[0].objects[] | select(.object.metadata.name == "deployment-2") | has("filterResult")`, `true`},
			},
		},
		{
			"kubernetes Synchronization with empty objects",
			func() BindingContext {
				bc := BindingContext{
					Binding: "kubernetes",
					Type:    TypeSynchronization,
					Objects: []ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				return bc
			},
			func() {
				assert.Len(t, bcList[0]["objects"], 0)
				assert.Equal(t, TypeSynchronization, bcList[0]["type"])
				assert.Equal(t, "kubernetes", bcList[0]["binding"])
				assert.NotContains(t, bcList[0], "object")
				assert.NotContains(t, bcList[0], "filterResult")
			},
			[][]string{
				// JSON dump should contains n fields: binding, type and objects
				{`.[0] | length`, `3`},
				{`.[0].binding`, `"kubernetes"`},
				{`.[0].type`, `"Synchronization"`},
				// objects should be an empty array
				{`.[0] | has("objects")`, `true`},
				{`.[0].objects | length`, `0`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bcList = ConvertBindingContextList("v1", []BindingContext{tt.bc()})
			assert.Len(t, bcList, 1)

			var err error
			bcJson, err = bcList.Json()
			assert.NoError(t, err)

			for _, jqAssertion := range tt.jqAsertions {
				JqEqual(t, bcJson, jqAssertion[0], jqAssertion[1])
			}

			tt.fn()
		})
	}
}

// Test conversion of BindingContext for v1, also test json marshal of v1 binding contexts.
func Test_ConvertBindingContextList_v0(t *testing.T) {
	var bcList BindingContextList
	var bcJson []byte

	tests := []struct {
		name        string
		bc          func() BindingContext
		fn          func()
		jqAsertions [][]string
	}{
		{
			"OnStartup binding",
			func() BindingContext {
				bc := BindingContext{
					Binding: "onStartup",
				}
				bc.Metadata.BindingType = OnStartup
				return bc
			},
			func() {
				assert.Equal(t, "onStartup", bcList[0]["binding"])
			},
			[][]string{
				{`.[0].binding`, `"onStartup"`},
				{`.[0] | length`, `1`},
			},
		},
		{
			"onKubernetesEvent binding",
			func() BindingContext {
				bc := BindingContext{
					Binding:    "onKubernetesEvent",
					Type:       "Event",
					WatchEvent: WatchEventAdded,
					Objects: []ObjectAndFilterResult{
						{
							Object: &unstructured.Unstructured{
								Object: map[string]interface{}{
									"metadata": map[string]interface{}{
										"namespace": "default",
										"name":      "pod-qwe",
									},
									"kind": "Pod",
								},
							},
						},
					},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				return bc
			},

			func() {
				assert.Len(t, bcList[0], 5)
				assert.Equal(t, "onKubernetesEvent", bcList[0]["binding"])
				assert.Equal(t, "add", bcList[0]["resourceEvent"])
				assert.Equal(t, "default", bcList[0]["resourceNamespace"])
				assert.Equal(t, "Pod", bcList[0]["resourceKind"])
				assert.Equal(t, "pod-qwe", bcList[0]["resourceName"])
			},
			[][]string{
				// JSON dump should has only 4 fields: binding, type, watchEvent and object.
				{`.[0] | length`, `5`},
				{`.[0].binding`, `"onKubernetesEvent"`},
				{`.[0].resourceEvent`, `"add"`},
				{`.[0].resourceNamespace`, `"default"`},
				{`.[0].resourceKind`, `"Pod"`},
				{`.[0].resourceName`, `"pod-qwe"`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bcList = ConvertBindingContextList("v0", []BindingContext{tt.bc()})
			assert.Len(t, bcList, 1)

			var err error
			bcJson, err = bcList.Json()
			assert.NoError(t, err)

			for _, jqAssertion := range tt.jqAsertions {
				JqEqual(t, bcJson, jqAssertion[0], jqAssertion[1])
			}

			tt.fn()
		})
	}
}
