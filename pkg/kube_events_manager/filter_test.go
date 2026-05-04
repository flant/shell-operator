package kubeeventsmanager

import (
	json "github.com/flant/shell-operator/pkg/utils/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/filter/jq"
)

func TestApplyFilter(t *testing.T) {
	t.Run("filter func with error", func(t *testing.T) {
		uns := &unstructured.Unstructured{Object: map[string]interface{}{"foo": "bar"}}
		_, err := applyFilter(nil, "", filterFuncWithError, uns)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "filterFn (github.com/flant/shell-operator/pkg/kube_events_manager.filterFuncWithError) contains an error:")
	})

	t.Run("nil compiledFilter computes checksum over full object", func(t *testing.T) {
		uns := &unstructured.Unstructured{Object: map[string]interface{}{"foo": "bar"}}
		res, err := applyFilter(nil, "", nil, uns)
		require.NoError(t, err)
		assert.NotEmpty(t, res.Metadata.Checksum)
		assert.Nil(t, res.FilterResult)
		assert.Equal(t, "", res.Metadata.JqFilter)
	})

	t.Run("compiled filter is applied and checksum calculated over result", func(t *testing.T) {
		uns := &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{"replicas": float64(2)}}}
		cf, err := jq.Compile(`.spec`)
		require.NoError(t, err)

		res, err := applyFilter(cf, ".spec", nil, uns)
		require.NoError(t, err)
		assert.Equal(t, ".spec", res.Metadata.JqFilter)
		assert.NotEmpty(t, res.Metadata.Checksum)
		filterStr, ok := res.FilterResult.(string)
		require.True(t, ok)
		assert.Contains(t, filterStr, "replicas")
	})

	t.Run("checksum differs for different compiled filter results", func(t *testing.T) {
		cf, err := jq.Compile(`.metadata.name`)
		require.NoError(t, err)

		uns1 := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "pod-a"}}}
		uns2 := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "pod-b"}}}

		res1, err := applyFilter(cf, ".metadata.name", nil, uns1)
		require.NoError(t, err)
		res2, err := applyFilter(cf, ".metadata.name", nil, uns2)
		require.NoError(t, err)

		assert.NotEqual(t, res1.Metadata.Checksum, res2.Metadata.Checksum)
	})

	t.Run("checksum is same for same compiled filter result", func(t *testing.T) {
		cf, err := jq.Compile(`.metadata.name`)
		require.NoError(t, err)

		uns := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "pod-a"}}}

		res1, err := applyFilter(cf, ".metadata.name", nil, uns)
		require.NoError(t, err)
		res2, err := applyFilter(cf, ".metadata.name", nil, uns)
		require.NoError(t, err)

		assert.Equal(t, res1.Metadata.Checksum, res2.Metadata.Checksum)
	})
}

// TestMonitorConfig_WithJqFilter validates compilation at MonitorConfig construction time.
func TestMonitorConfig_WithJqFilter(t *testing.T) {
	t.Run("valid expression compiles and sets both fields", func(t *testing.T) {
		mc := &MonitorConfig{}
		require.NoError(t, mc.WithJqFilter(`.metadata.labels`))
		assert.Equal(t, `.metadata.labels`, mc.JqFilter)
		assert.NotNil(t, mc.CompiledJqFilter)
	})

	t.Run("empty expression clears compiled filter", func(t *testing.T) {
		mc := &MonitorConfig{}
		require.NoError(t, mc.WithJqFilter(``))
		assert.Equal(t, ``, mc.JqFilter)
		assert.Nil(t, mc.CompiledJqFilter)
	})

	t.Run("invalid expression returns error and leaves config unchanged", func(t *testing.T) {
		mc := &MonitorConfig{}
		err := mc.WithJqFilter(`not valid jq!!!`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compile jqFilter")
	})

	t.Run("compiled filter produces same result as raw ApplyFilter", func(t *testing.T) {
		mc := &MonitorConfig{}
		require.NoError(t, mc.WithJqFilter(`.spec`))

		uns := &unstructured.Unstructured{Object: map[string]interface{}{
			"spec": map[string]interface{}{"replicas": float64(3)},
		}}

		resCompiled, err := applyFilter(mc.CompiledJqFilter, mc.JqFilter, nil, uns)
		require.NoError(t, err)

		interpreted := jq.NewFilter()
		resInterpreted, err := applyFilter(nil, "", nil, uns) // no filter → full checksum
		require.NoError(t, err)

		// Ensure the compiled filter actually applies transformation (not full-object checksum).
		rawInterpreted, err := interpreted.ApplyFilter(`.spec`, uns.UnstructuredContent())
		require.NoError(t, err)
		assert.Equal(t, string(rawInterpreted), resCompiled.FilterResult)

		// And that the unchained path still works.
		assert.NotEqual(t, resCompiled.Metadata.Checksum, resInterpreted.Metadata.Checksum)
	})
}

func filterFuncWithError(_ *unstructured.Unstructured) (interface{}, error) {
	var s []string
	err := json.Unmarshal([]byte("asdasd"), &s)

	return s, err
}
