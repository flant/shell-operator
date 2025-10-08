package filter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"runtime/trace"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	"github.com/itchyny/gojq"
)

type FilterFn func(obj *unstructured.Unstructured) (result interface{}, err error)

type Expression struct {
	*gojq.Code
	Query string
}

// Run filters an object with a custom function or jq expression, calculates checksum
// over the result and returns ObjectAndFilterResult.
//
// Filter precedence (highest to lowest):
// 1. Custom filter function (filterFn) - if provided, takes precedence over jq expression
// 2. JQ expression (expression) - used when filterFn is nil but expression is provided
// 3. No filter - when both filterFn and expression are nil, uses raw object JSON
//
// The function calculates a checksum over the filtered result (or full object if no filter)
// and populates metadata including resource ID and jq filter query (if applicable).
func Run(expression *Expression, filterFn FilterFn, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "ApplyJqFilter").End()

	// Initialize result with object and resource ID
	result := &kemtypes.ObjectAndFilterResult{
		Object: obj,
	}
	result.Metadata.ResourceId = fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName())

	// Set JQ filter in metadata if expression is provided (even if custom filter takes precedence)
	if expression != nil {
		result.Metadata.JqFilter = expression.Query
	}

	// Apply filters based on precedence: custom filter > jq expression > no filter
	switch {
	case filterFn != nil:
		// Custom filter function takes highest precedence
		return applyCustomFilter(filterFn, result, obj)

	case expression != nil:
		// JQ expression filter when no custom filter is provided
		return applyJQFilter(expression, result, obj)

	default:
		// No filter - use raw object JSON when both filterFn and expression are nil
		return applyNoFilter(result, obj)
	}
}

func applyCustomFilter(filterFn FilterFn, result *kemtypes.ObjectAndFilterResult, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	filteredObj, err := filterFn(obj)
	if err != nil {
		funcName := runtime.FuncForPC(reflect.ValueOf(filterFn).Pointer()).Name()
		return nil, fmt.Errorf("filterFn (%s) contains an error: %v", funcName, err)
	}

	filteredBytes, err := json.Marshal(filteredObj)
	if err != nil {
		return nil, err
	}

	result.FilterResult = filteredObj
	result.Metadata.Checksum = utils_checksum.CalculateChecksum(string(filteredBytes))
	return result, nil
}

func applyNoFilter(result *kemtypes.ObjectAndFilterResult, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	result.Metadata.Checksum = utils_checksum.CalculateChecksum(string(data))
	return result, nil
}

func applyJQFilter(expression *Expression, result *kemtypes.ObjectAndFilterResult, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	filtered, err := RunJQ(expression, obj.DeepCopy())
	if err != nil {
		return nil, fmt.Errorf("jqFilter: %v", err)
	}

	filteredStr := string(filtered)
	result.FilterResult = filteredStr
	result.Metadata.Checksum = utils_checksum.CalculateChecksum(filteredStr)
	return result, nil
}

func RunJQ(expression *Expression, obj *unstructured.Unstructured) ([]byte, error) {
	// Execute jq expression and collect results
	iter := expression.Run(obj.UnstructuredContent())
	var results []any

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		// Handle errors from jq execution
		if err, ok := v.(error); ok {
			// HaltError with nil value means graceful termination
			var haltErr *gojq.HaltError
			if errors.As(err, &haltErr) && haltErr.Value() == nil {
				break
			}
			return nil, err
		}

		results = append(results, v)
	}

	// Marshal results based on count
	switch len(results) {
	case 0:
		return []byte("null"), nil
	case 1:
		return json.Marshal(results[0])
	default:
		return json.Marshal(results)
	}
}

func CompileExpression(expression string) (*Expression, error) {
	parsedQuery, err := gojq.Parse(expression)
	if err != nil {
		return nil, err
	}
	compiledQuery, err := gojq.Compile(parsedQuery)
	if err != nil {
		return nil, err
	}

	return &Expression{
		Code:  compiledQuery,
		Query: expression,
	}, nil
}

func Info() string {
	return "Filter implementation: using itchyny/gojq"
}
