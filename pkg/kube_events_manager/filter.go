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
	// we will add Object to the result later, because here we use objects from sync.Pool
	res := &kemtypes.ObjectAndFilterResult{}
	res.Metadata.JqFilter = jqFilter
	res.Metadata.ResourceId = resourceIDStore.GetResourceID(obj).String()

	switch {
	case filterFn != nil:
		filteredObj, err := filterFn(obj)
		if err != nil {
			return nil, fmt.Errorf("filterFn (%s) contains an error: %v", runtime.FuncForPC(reflect.ValueOf(filterFn).Pointer()).Name(), err)
		}
		res.FilterResult = filteredObj
	case jqFilter != nil:
		filteredBytes, err := fl.ApplyFilter(jqFilter, obj.UnstructuredContent())
		if err != nil {
			return nil, fmt.Errorf("jqFilter: %v", err)
		}
		res.FilterResult = string(filteredBytes)
	}

	return res, nil
}
