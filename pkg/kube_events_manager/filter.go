package kube_events_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/jq"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// ApplyFilter filters object json representation with jq expression, calculate checksum
// over result and return ObjectAndFilterResult. If jqFilter is empty, no filter
// is required and checksum is calculated over full json representation of the object.
func ApplyFilter(jqFilter string, filterFn func(obj *unstructured.Unstructured) (result string, err error), obj *unstructured.Unstructured) (*ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.TODO(), "ApplyJqFilter").End()

	res := &ObjectAndFilterResult{
		Object: obj,
	}
	res.Metadata.JqFilter = jqFilter
	res.Metadata.ResourceId = ResourceId(obj)

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	res.ObjectBytes = int64(len(data))

	// If filterFn is passed, run it and return result.
	if filterFn != nil {
		var err error
		var filtered string
		filtered, err = filterFn(obj)
		if err != nil {
			return nil, fmt.Errorf("filterFn: %v", err)
		}
		res.FilterResult = filtered
		res.Metadata.Checksum = utils_checksum.CalculateChecksum(filtered)
		return res, nil
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
