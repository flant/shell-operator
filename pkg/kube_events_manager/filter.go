package kube_events_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"runtime/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/jq"
	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
)

// applyFilter filters object json representation with jq expression, calculate checksum
// over result and return ObjectAndFilterResult. If jqFilter is empty, no filter
// is required and checksum is calculated over full json representation of the object.
func applyFilter(jqFilter string, filterFn func(obj *unstructured.Unstructured) (result interface{}, err error), obj *unstructured.Unstructured) (*ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "ApplyJqFilter").End()

	res := &ObjectAndFilterResult{
		Object: obj,
	}
	res.Metadata.JqFilter = jqFilter
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
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	if jqFilter == "" {
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(string(data))
	} else {
		var err error
		var filtered string
		filtered, err = jq.ApplyJqFilter(jqFilter, data, app.JqLibraryPath)
		if err != nil {
			return nil, fmt.Errorf("jqFilter: %v", err)
		}

		res.FilterResult = filtered
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(filtered)
	}

	return res, nil
}
