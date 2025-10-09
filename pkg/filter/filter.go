/*
Package filter provides functionality for transforming and filtering objects
from plain kubernetes unstructured objects to structured objects, that shell-operator can work with.
  - Supports JQ Expressions
  - Supports Custom Filter Functions
  - Supports Plain Filter Functions

Enriches objects with metadata and calculates a checksum.
*/
package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"runtime/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/flant/shell-operator/pkg/jq"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
)

type FilterFn func(obj *unstructured.Unstructured) (result interface{}, err error)

// RunFn runs a filter function on an object and returns an ObjectAndFilterResult.
func RunFn(filterFn FilterFn, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "FilterRunFn").End()

	filtered, err := filterFn(obj)
	if err != nil {
		return nil, fmt.Errorf("filter function (%s) execution error: %v", funcName(filterFn), err)
	}

	filteredBytes, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}

	return &kemtypes.ObjectAndFilterResult{
		Object: obj,
		Metadata: kemtypes.ObjectAndFilterResultMetadata{
			ResourceId: resourceID(obj),
			Checksum:   utils_checksum.CalculateChecksum(string(filteredBytes)),
		},
		FilterResult: filtered,
	}, nil
}

// RunExpression runs a jq expression on an object and returns an ObjectAndFilterResult.
func RunExpression(expression *jq.Expression, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "FilterRunExpression").End()

	filtered, err := jq.ExecuteJQ(expression, obj)
	if err != nil {
		return nil, fmt.Errorf("jq expression execution error: %v", err)
	}

	return &kemtypes.ObjectAndFilterResult{
		Object: obj,
		Metadata: kemtypes.ObjectAndFilterResultMetadata{
			ResourceId: resourceID(obj),
			Checksum:   utils_checksum.CalculateChecksum(string(filtered)),
			JqFilter:   expression.Query(),
		},
		FilterResult: filtered,
	}, nil
}

// RunPlain runs NO filter function on an object and returns an ObjectAndFilterResult.
func RunPlain(obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	// TODO: json operation could be avoided, we can caclculate checksum from the object itself
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return &kemtypes.ObjectAndFilterResult{
		Object: obj,
		Metadata: kemtypes.ObjectAndFilterResultMetadata{
			ResourceId: resourceID(obj),
			Checksum:   utils_checksum.CalculateChecksum(string(data)),
		},
	}, nil
}

func resourceID(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName())
}

func funcName(fn FilterFn) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}
