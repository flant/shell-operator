package binding_context

import (
	"testing"

	. "github.com/flant/libjq-go"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/flant/shell-operator/pkg/hook/types"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

func JqEqual(t *testing.T, input []byte, program string, expected string) {
	// nolint:typecheck // Ignore false positive: undeclared name: `Jq`.
	res, err := Jq().Program(program).Run(string(input))
	if assert.NoError(t, err) {
		assert.Equal(t, expected, res, "jq: '%s', json was '%s'", program, string(input))
	}
}

// Test conversion of BindingContext for v1, also test json marshal of v1 binding contexts.
func Test_ConvertBindingContextList_v1(t *testing.T) {
	g := NewWithT(t)

	var bcList BindingContextList
	var bcJson []byte

	tests := []struct {
		name         string
		bc           func() []BindingContext
		fn           func()
		jqAssertions [][]string
	}{
		{
			"OnStartup binding",
			func() []BindingContext {
				bc := BindingContext{
					Binding: "onStartup",
				}
				bc.Metadata.BindingType = OnStartup
				return []BindingContext{bc}
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
			func() []BindingContext {
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
				return []BindingContext{bc}
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
			func() []BindingContext {
				bc := BindingContext{
					Binding: "kubernetes",
					Type:    TypeSynchronization,
					Objects: []ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent

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
					FilterResult: `{"labels":{"label-name":"label-value"}}`,
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
						},
					},
					FilterResult: `""`,
				}
				obj.Metadata.JqFilter = ".metadata.labels"
				bc.Objects = append(bc.Objects, obj)

				return []BindingContext{bc}
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
			"binding context with group",
			func() []BindingContext {
				bcs := []BindingContext{}

				bc := BindingContext{
					Binding:    "monitor_pods",
					Type:       TypeEvent,
					WatchEvent: WatchEventAdded,
					Objects:    []ObjectAndFilterResult{},
					Snapshots:  map[string][]ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				bc.Metadata.Group = "pods"
				bc.Metadata.IncludeSnapshots = []string{"monitor_pods"}

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
				}
				bc.Objects = append(bc.Objects, obj)
				bc.Snapshots["monitor_pods"] = append(bc.Snapshots["monitor_pods"], obj)
				bcs = append(bcs, bc)

				bc = BindingContext{
					Binding:    "monitor_pods",
					Type:       TypeEvent,
					WatchEvent: WatchEventAdded,
					Objects:    []ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent

				obj = ObjectAndFilterResult{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "pod-qwe",
							},
							"kind": "Pod",
						},
					},
				}
				bc.Objects = append(bc.Objects, obj)

				bcs = append(bcs, bc)
				return bcs
			},
			func() {
				g.Expect(bcList[0]).Should(HaveKey("binding"))
				g.Expect(bcList[0]).Should(HaveKey("snapshots"))
				g.Expect(bcList[0]).Should(HaveKey("type"))
				g.Expect(bcList[0]).ShouldNot(HaveKey("objects"))
			},
			[][]string{
				{`. | length`, `2`},

				// grouped binding context contains binding, type and snapshots
				{`.[0] | length`, `3`}, // Only 3 fields
				{`.[0].snapshots | has("monitor_pods")`, `true`},
				{`.[0].snapshots."monitor_pods" | length`, `1`},
				{`.[0].binding`, `"pods"`},
				{`.[0].type`, `"Group"`},

				// JSON dump should has only 4 fields: binding, type, watchEvent and object.
				{`.[1] | length`, `4`},
				{`.[1].binding`, `"monitor_pods"`},
				{`.[1].type`, `"Event"`},
				{`.[1].watchEvent`, `"Added"`},
				{`.[1].object.metadata.namespace`, `"default"`},
				{`.[1].object.metadata.name`, `"pod-qwe"`},
			},
		},
		{
			"grouped Synchronization",
			func() []BindingContext {
				bcs := []BindingContext{}

				bc := BindingContext{
					Binding:   "monitor_config_maps",
					Type:      TypeSynchronization,
					Objects:   []ObjectAndFilterResult{},
					Snapshots: map[string][]ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				bc.Metadata.Group = "pods"
				bc.Metadata.IncludeSnapshots = []string{"monitor_config_maps"}
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
				bc.Objects = append(bc.Objects, obj)
				bc.Snapshots["monitor_config_maps"] = append(bc.Snapshots["monitor_config_maps"], obj)
				bcs = append(bcs, bc)

				bc = BindingContext{
					Binding: "monitor_pods",
					Type:    TypeSynchronization,
					Objects: []ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				// object without jqfilter should not have filterResult field
				obj = ObjectAndFilterResult{
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
				bc.Objects = append(bc.Objects, obj)

				bcs = append(bcs, bc)
				return bcs
			},
			func() {
				g.Expect(bcList).Should(HaveLen(2))

				g.Expect(bcList[0]).Should(HaveLen(3))
				g.Expect(bcList[0]).Should(HaveKey("binding"))
				g.Expect(bcList[0]["binding"]).Should(Equal("pods"))
				g.Expect(bcList[0]).Should(HaveKey("type"))
				g.Expect(bcList[0]["type"]).Should(Equal("Group"))
				g.Expect(bcList[0]).Should(HaveKey("snapshots"))
				g.Expect(bcList[0]["snapshots"]).Should(HaveLen(1))

				g.Expect(bcList[1]).Should(HaveLen(3))
				g.Expect(bcList[1]).Should(HaveKey("binding"))
				g.Expect(bcList[1]).Should(HaveKey("type"))
				g.Expect(bcList[1]["type"]).Should(Equal(TypeSynchronization))
				g.Expect(bcList[1]).Should(HaveKey("objects"))
				g.Expect(bcList[1]["objects"]).Should(HaveLen(1))
			},
			[][]string{
				{`. | length`, `2`},

				// grouped binding context contains binding==group, type and snapshots
				{`.[0] | length`, `3`}, // Only 3 fields
				{`.[0].binding`, `"pods"`},
				{`.[0].type`, `"Group"`},
				{`.[0].snapshots | has("monitor_config_maps")`, `true`},
				{`.[0].snapshots."monitor_config_maps" | length`, `1`},

				// usual Synchronization has 3 fields: binding, type and objects.
				{`.[1] | length`, `3`},
				{`.[1].binding`, `"monitor_pods"`},
				{`.[1].type`, `"Synchronization"`},
				{`.[1].objects | length`, `1`},
				{`.[1].objects[0].object.metadata.namespace`, `"default"`},
				{`.[1].objects[0].object.metadata.name`, `"pod-qwe"`},
			},
		},
		{
			"kubernetes Synchronization with empty objects",
			func() []BindingContext {
				bc := BindingContext{
					Binding: "kubernetes",
					Type:    TypeSynchronization,
					Objects: []ObjectAndFilterResult{},
				}
				bc.Metadata.BindingType = OnKubernetesEvent
				return []BindingContext{bc}
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
			bcList = ConvertBindingContextList("v1", tt.bc())
			// assert.Len(t, bcList, 1)

			var err error
			bcJson, err = bcList.Json()
			assert.NoError(t, err)

			for _, jqAssertion := range tt.jqAssertions {
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
