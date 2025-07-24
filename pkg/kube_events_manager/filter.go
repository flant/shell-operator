package kubeeventsmanager

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"runtime/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/filter"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	"github.com/itchyny/gojq"
)

// applyFilter filters object json representation with jq expression, calculate checksum
// over result and return ObjectAndFilterResult. If jqFilter is empty, no filter
// is required and checksum is calculated over full json representation of the object.
func applyFilter(jqFilter *gojq.Code, fl filter.Filter, filterFn func(obj *unstructured.Unstructured) (result interface{}, err error), obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "ApplyJqFilter").End()

	res := &kemtypes.ObjectAndFilterResult{
		Object: obj,
	}
	res.Metadata.JqFilter = jqFilter
	res.Metadata.ResourceId = resourceIDStore.GetResourceID(obj).String()

	// If filterFn is passed, run it and return result.
	if filterFn != nil {
		filteredObj, err := filterFn(obj)
		if err != nil {
			return nil, fmt.Errorf("filterFn (%s) contains an error: %v", runtime.FuncForPC(reflect.ValueOf(filterFn).Pointer()).Name(), err)
		}

		res.FilterResult = filteredObj
		res.Metadata.Checksum, err = checksum.CalculateChecksum(filteredObj)
		if err != nil {
			return nil, err
		}

		return res, nil
	}

	if jqFilter == nil {
		var err error
		res.Metadata.Checksum, err = checksum.CalculateChecksum(obj)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		var filtered []byte
		filtered, err = fl.ApplyFilter(jqFilter, obj.UnstructuredContent())
		if err != nil {
			return nil, fmt.Errorf("jqFilter: %v", err)
		}

		res.FilterResult = string(filtered)
		res.Metadata.Checksum, err = checksum.CalculateChecksum(filtered)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}
