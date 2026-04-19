package jq

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/itchyny/gojq"

	"github.com/flant/shell-operator/pkg/filter"
)

var (
	_ filter.Filter         = (*Filter)(nil)
	_ filter.CompiledFilter = (*CompiledJqFilter)(nil)
)

func NewFilter() *Filter {
	return &Filter{}
}

type Filter struct{}

// ApplyFilter runs jq expression provided in jqFilter with jsonData as input.
func (f *Filter) ApplyFilter(jqFilter string, data map[string]any) ([]byte, error) {
	query, err := gojq.Parse(jqFilter)
	if err != nil {
		return nil, err
	}

	var workData any
	if data == nil {
		workData = nil
	} else {
		workData, err = deepCopyAny(data)
		if err != nil {
			return nil, err
		}
	}

	iter := query.Run(workData)
	return collectResults(iter)
}

func (f *Filter) FilterInfo() string {
	return "jqFilter implementation: using itchyny/gojq"
}

// CompiledJqFilter holds a pre-compiled gojq program. Compile once and reuse
// across many Apply calls to eliminate repeated parse+compile overhead.
type CompiledJqFilter struct {
	code        *gojq.Code
	originalStr string
}

// Compile parses and compiles jqFilter once. The returned *CompiledJqFilter is
// safe for concurrent use and can be reused for every event that carries the
// same filter expression.
func Compile(jqFilter string) (*CompiledJqFilter, error) {
	query, err := gojq.Parse(jqFilter)
	if err != nil {
		return nil, err
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return nil, err
	}

	return &CompiledJqFilter{code: code, originalStr: jqFilter}, nil
}

// Apply executes the pre-compiled jq program against data.
func (c *CompiledJqFilter) Apply(data map[string]any) ([]byte, error) {
	var workData any
	var err error
	if data == nil {
		workData = nil
	} else {
		workData, err = deepCopyAny(data)
		if err != nil {
			return nil, err
		}
	}

	iter := c.code.Run(workData)
	return collectResults(iter)
}

// String returns the original jq filter expression for diagnostics.
func (c *CompiledJqFilter) String() string {
	return c.originalStr
}

// deepCopyAny recursively copies JSON-compatible values (maps, slices, and
// primitives) without going through json.Marshal/Unmarshal. This is
// significantly faster and allocates only the final structure.
func deepCopyAny(input any) (any, error) {
	if input == nil {
		return nil, nil
	}
	return deepCopyValue(input)
}

func deepCopyValue(v any) (any, error) {
	switch val := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(val))
		for k, v := range val {
			copied, err := deepCopyValue(v)
			if err != nil {
				return nil, err
			}
			m[k] = copied
		}
		return m, nil
	case []any:
		s := make([]any, len(val))
		for i, v := range val {
			copied, err := deepCopyValue(v)
			if err != nil {
				return nil, err
			}
			s[i] = copied
		}
		return s, nil
	case string, bool, json.Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return val, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("deepCopyValue: unsupported type %T", v)
	}
}

// collectResults drains a gojq iterator and serialises the results to JSON.
func collectResults(iter gojq.Iter) ([]byte, error) {
	result := make([]any, 0)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			var errGoJq *gojq.HaltError
			if errors.As(err, &errGoJq) && errGoJq.Value() == nil {
				break
			}
			return nil, err
		}
		result = append(result, v)
	}

	switch len(result) {
	case 0:
		return []byte("null"), nil
	case 1:
		return json.Marshal(result[0])
	default:
		return json.Marshal(result)
	}
}
