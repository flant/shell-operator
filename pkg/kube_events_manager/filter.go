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
	"github.com/itchyny/gojq"
)

// applyFilter runs jq expression and returns an object.
func applyFilter(jqFilter *gojq.Code, fl filter.Filter, filterFn func(obj *unstructured.Unstructured) (result interface{}, err error), obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "ApplyJqFilter").End()

	res := &kemtypes.ObjectAndFilterResult{
		Object: obj,
	}
	res.Metadata.JqFilter = jqFilter
	res.Metadata.ResourceId = resourceIDStore.GetResourceID(obj).String()

	var objectForFilter interface{}

	if filterFn != nil {
		filteredObj, err := filterFn(obj)
		if err != nil {
			return nil, fmt.Errorf("filterFn (%s) contains an error: %v", runtime.FuncForPC(reflect.ValueOf(filterFn).Pointer()).Name(), err)
		}
		res.FilterResult = filteredObj
		objectForFilter = filteredObj
	} else if jqFilter != nil {
		filteredBytes, err := fl.ApplyFilter(jqFilter, obj.UnstructuredContent())
		if err != nil {
			return nil, fmt.Errorf("jqFilter: %v", err)
		}
		res.FilterResult = string(filteredBytes)
		objectForFilter = filteredBytes
	} else {
		// No filters, use the original object.
		objectForFilter = obj
	}

	res.FilterResult = objectForFilter

	return res, nil
}
