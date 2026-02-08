package jq

import (
	"encoding/json"
	"errors"

	"github.com/itchyny/gojq"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Expression struct {
	*gojq.Code
	query string
}

func (e *Expression) Query() string {
	return e.query
}

// ExecuteJQ executes jq expression and collect results
func ExecuteJQ(expression *Expression, obj *unstructured.Unstructured) ([]byte, error) {
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
		query: expression,
	}, nil
}

func Info() string {
	return "jq implementation: using itchyny/gojq"
}
