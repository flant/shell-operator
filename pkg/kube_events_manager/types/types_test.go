package types

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/flant/shell-operator/pkg/filter/jq"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//nolint:unparam
func newObj(kind, ns, name, filterResult string) *ObjectAndFilterResult {
	return &ObjectAndFilterResult{
		FilterResult: filterResult,
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": ns,
					"name":      name,
				},
				"kind": kind,
			},
		},
	}
}

func Test_ObjectAndFilterResult_ToJson(t *testing.T) {
	obj := newObj("Pod", "default", "pod-qwe", `{"spec":"asd"}`)
	var err error
	obj.Metadata.JqFilter, err = jq.CompileJQ(".spec")
	assert.NoError(t, err)

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	// fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, "filterResult")
}

func Test_ObjectAndFilterResult_ToJson_EmptyFilterResult(t *testing.T) {
	obj := newObj("Pod", "default", "pod-qwe", ``)
	var err error
	obj.Metadata.JqFilter, err = jq.CompileJQ(".spec")
	assert.NoError(t, err)

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	// fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, `"filterResult":null`)
}

func Test_ObjectAndFilterResult_ToJson_NullFilterResult(t *testing.T) {
	obj := newObj("Pod", "default", "pod-qwe", `null`)
	var err error
	obj.Metadata.JqFilter, err = jq.CompileJQ(".spec")
	assert.NoError(t, err)

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	// fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, `"filterResult":null`)
}

func Test_ObjectAndFilterResult_ToJson_Empty_JQFilter_Has_FilterResult(t *testing.T) {
	obj := newObj("Pod", "default", "pod-qwe", `{"spec":"asd"}`)
	obj.Metadata.JqFilter = nil

	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	jsonStr := string(data)
	// fmt.Printf("%s\n", jsonStr)
	assert.Contains(t, jsonStr, "filterResult")
}

func Test_Sort_ByNamespaceAndName(t *testing.T) {
	inputObjs := []ObjectAndFilterResult{
		*newObj("Pod", "default", "pod-qwe", ``),
		*newObj("Pod", "kube-system", "kube-proxy-lh65x", ``),
		*newObj("Pod", "kube-system", "kube-proxy-rkrr7", ``),
		*newObj("Pod", "kube-system", "kindnet-bcpzg", ``),
		*newObj("Pod", "default", "registry-proxy-dtcq6", ``),
		*newObj("Pod", "kube-system", "coredns-54ff9cd656-lx49b", ``),
		*newObj("Pod", "kube-system", "kube-apiserver-control-plane", ``),
	}

	sort.Sort(ByNamespaceAndName(inputObjs))

	assert.True(t, sort.IsSorted(ByNamespaceAndName(inputObjs)))

	assert.Equal(t, "pod-qwe", inputObjs[0].Object.GetName())
	assert.Equal(t, "registry-proxy-dtcq6", inputObjs[1].Object.GetName())
	assert.Equal(t, "coredns-54ff9cd656-lx49b", inputObjs[2].Object.GetName())
	assert.Equal(t, "kindnet-bcpzg", inputObjs[3].Object.GetName())
	assert.Equal(t, "kube-apiserver-control-plane", inputObjs[4].Object.GetName())
	assert.Equal(t, "kube-proxy-lh65x", inputObjs[5].Object.GetName())
	assert.Equal(t, "kube-proxy-rkrr7", inputObjs[6].Object.GetName())
}
