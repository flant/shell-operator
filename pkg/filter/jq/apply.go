package jq

import (
	"errors"
	"fmt"

	"github.com/itchyny/gojq"

	"github.com/flant/shell-operator/pkg/filter"
	json "github.com/flant/shell-operator/pkg/utils/json"
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

	workData, err := cloneInput(data)
	if err != nil {
		return nil, err
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
//
// gojq mutates its input map for some queries (e.g. update assignment). To keep
// the caller's data (typically the SharedIndexInformer's cached object) untouched,
// the input is cloned via a JSON marshal/unmarshal round-trip. This is functionally
// equivalent to a deep copy, allocates only the final structure, and avoids
// having to maintain a hand-written deep-copy that is aware of every Go type
// that may appear in an unstructured object.
func (c *CompiledJqFilter) Apply(data map[string]any) ([]byte, error) {
	workData, err := cloneInput(data)
	if err != nil {
		return nil, err
	}

	iter := c.code.Run(workData)
	return collectResults(iter)
}

// String returns the original jq filter expression for diagnostics.
func (c *CompiledJqFilter) String() string {
	return c.originalStr
}

// cloneInput returns a fresh JSON-compatible value that is safe to pass to
// gojq.Code.Run / gojq.Query.Run. Going through a JSON round-trip is the
// cheapest correctness-preserving way to detach the value from the caller's
// memory, given that the typical input is already JSON-shaped (parsed from a
// Kubernetes watch response).
func cloneInput(data map[string]any) (any, error) {
	if data == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("clone jq input: marshal: %w", err)
	}

	var out any
	if err := json.Unmarshal(bytes, &out); err != nil {
		return nil, fmt.Errorf("clone jq input: unmarshal: %w", err)
	}
	return out, nil
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
