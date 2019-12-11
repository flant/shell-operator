package hook

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/flant/libjq-go"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMain(m *testing.M) {
	jqDone := make(chan struct{})
	go JqCallLoop(jqDone)

	os.Exit(m.Run())
}

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
		bc          BindingContext
		fn          func()
		jqAsertions [][]string
	}{
		{
			"OnStartup binding",
			BindingContext{
				Binding: "onStartup",
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
			BindingContext{
				Binding:    "kubernetes",
				Type:       "Event",
				WatchEvent: WatchEventAdded,
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]string{
							"namespace": "default",
						},
						"kind": "Pod",
						"name": "pod-qwe",
					},
				},
			},
			func() {
				assert.Len(t, bcList[0], 4)
				assert.Equal(t, "kubernetes", bcList[0]["binding"])
				assert.Equal(t, "Event", bcList[0]["type"])
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
				{`.[0].object.name`, `"pod-qwe"`},
			},
		},
		{
			"kubernetes Synchronization event",
			BindingContext{
				Binding: "kubernetes",
				Type:    "Synchronization",
				Objects: []ObjectAndFilterResult{
					ObjectAndFilterResult{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]string{
									"namespace": "default",
								},
								"kind": "Pod",
								"name": "pod-qwe",
							},
						},
						FilterResult: "",
					},
					ObjectAndFilterResult{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]string{
									"namespace": "default",
								},
								"kind": "Deployment",
								"name": "deployment-test",
							},
						},
						FilterResult: "{\"labels\":{\"label-name\":\"label-value\"}}",
					},
					ObjectAndFilterResult{
						Object: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]string{
									"namespace": "default",
								},
								"kind": "Deployment",
								"name": "deployment-2",
							}},
						FilterResult: "",
					},
				},
			},
			func() {
				assert.Len(t, bcList[0], 3)
				assert.Len(t, bcList[0]["objects"], 3)
				assert.Equal(t, "Synchronization", bcList[0]["type"])
				assert.Equal(t, "kubernetes", bcList[0]["binding"])
				assert.NotContains(t, bcList[0], "object")
				assert.NotContains(t, bcList[0], "filterResult")
			},
			[][]string{
				// JSON dump should contains n fields: binding, type and objects
				{`.[0] | length`, `3`},
				{`.[0].binding`, `"kubernetes"`},
				{`.[0].type`, `"Synchronization"`},

				{`.[0].objects[] | select(.object.name == "pod-qwe") | has("filterResult")`, `false`},
				{`.[0].objects[] | select(.object.name == "deployment-test") | has("filterResult")`, `true`},
				{`.[0].objects[] | select(.object.name == "deployment-test") | .filterResult | fromjson | .labels."label-name"`, `"label-value"`},
				{`.[0].objects[] | select(.object.name == "deployment-2") | has("filterResult")`, `false`},
			},
		},
		{
			"kubernetes Synchronization with empty objects",
			BindingContext{
				Binding: "kubernetes",
				Type:    "Synchronization",
				Objects: []ObjectAndFilterResult{},
			},
			func() {
				assert.Len(t, bcList[0]["objects"], 0)
				assert.Equal(t, "Synchronization", bcList[0]["type"])
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
			bcList = ConvertBindingContextList("v1", []BindingContext{tt.bc})
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
		bc          BindingContext
		fn          func()
		jqAsertions [][]string
	}{
		{
			"OnStartup binding",
			BindingContext{
				Binding: "onStartup",
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
			BindingContext{
				Binding:    "onKubernetesEvent",
				Type:       "Event",
				WatchEvent: WatchEventAdded,
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]string{
							"namespace": "default",
						},
						"kind": "Pod",
						"name": "pod-qwe",
					},
				},
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
				{`.[0].resourceEvent`, `"Added"`},
				{`.[0].resourceNamespace`, `"default"`},
				{`.[0].resourceKind`, `"Pod"`},
				{`.[0].resourceName`, `"pod-qwe"`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bcList = ConvertBindingContextList("v0", []BindingContext{tt.bc})
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
