package kubeeventsmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"runtime/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/filter"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
)

// applyFilter filters object json representation with a pre-compiled jq expression,
// calculates checksum over the result and returns ObjectAndFilterResult.
// If compiledFilter is nil, no jq filtering is applied and checksum is calculated
// over full json representation of the object.
// jqFilterStr is stored in result metadata for informational purposes only.
func applyFilter(compiledFilter filter.CompiledFilter, jqFilterStr string, filterFn func(obj *unstructured.Unstructured) (result interface{}, err error), obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "ApplyJqFilter").End()

	res := &kemtypes.ObjectAndFilterResult{
		Object: obj,
	}
	res.Metadata.JqFilter = jqFilterStr
	res.Metadata.ResourceId = resourceId(obj)

	// If filterFn is passed, run it and return result.
	if filterFn != nil {
		filteredObj, err := filterFn(obj)
		if err != nil {
			return nil, fmt.Errorf("filterFn (%s) contains an error: %v", runtime.FuncForPC(reflect.ValueOf(filterFn).Pointer()).Name(), err)
		}

		filteredBytes, err := json.Marshal(filteredObj)
		if err != nil {
			return nil, err
		}

		res.FilterResult = filteredObj
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(string(filteredBytes))

		return res, nil
	}

	// Render obj to JSON text to apply jq filter.
	if compiledFilter == nil {
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(string(data))
	} else {
		filtered, err := compiledFilter.Apply(obj.UnstructuredContent())
		if err != nil {
			return nil, fmt.Errorf("jqFilter: %v", err)
		}

		res.FilterResult = string(filtered)
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(string(filtered))
	}

	return res, nil
}
