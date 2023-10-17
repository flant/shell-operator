package kube_events_manager

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestApplyFilter(t *testing.T) {
	t.Run("filter func with error", func(t *testing.T) {
		uns := &unstructured.Unstructured{Object: map[string]interface{}{"foo": "bar"}}
		_, err := applyFilter("", filterFuncWithError, uns)
		assert.EqualError(t, err, "filterFn (github.com/flant/shell-operator/pkg/kube_events_manager.filterFuncWithError) contains an error: invalid character 'a' looking for beginning of value")
	})
}

func filterFuncWithError(_ *unstructured.Unstructured) (result interface{}, err error) {
	var s []string

	err = json.Unmarshal([]byte("asdasd"), &s)

	return s, err
}
