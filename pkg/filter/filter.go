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

// applyFilter filters object with custom function or jq expression, calculates checksum
// over the result and returns ObjectAndFilterResult. If no filter is provided,
// checksum is calculated over the full JSON representation of the object.
func Run(expression *Expression, filterFn FilterFn, obj *unstructured.Unstructured) (*kemtypes.ObjectAndFilterResult, error) {
	defer trace.StartRegion(context.Background(), "ApplyJqFilter").End()

	result := &kemtypes.ObjectAndFilterResult{
		Object: obj,
	}
	result.Metadata.ResourceId = fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName())

	if expression != nil {
		result.Metadata.JqFilter = expression.Query
	}

	// Apply custom filter function
	if filterFn != nil {
		return applyCustomFilter(filterFn, result, obj)
	}

	// No filter - use raw object JSON
	if expression == nil {
		return applyNoFilter(result, obj)
	}

	// Apply jq expression filter
	return applyJQFilter(expression, result, obj)
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
