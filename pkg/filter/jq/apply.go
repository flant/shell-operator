package jq

import (
	"errors"
	"maps"

	"github.com/itchyny/gojq"

	"github.com/flant/shell-operator/pkg/filter"
)

var _ filter.Filter = (*Filter)(nil)

func NewFilter() *Filter {
	return &Filter{}
}

type Filter struct{}

// ApplyFilter runs jq expression provided in jqFilter with jsonData as input.
func (f *Filter) ApplyFilter(jqFilter string, data map[string]any) (map[string]any, error) {
	query, err := gojq.Parse(jqFilter)
	if err != nil {
		return nil, err
	}

	iter := query.Run(data)
	result := make(map[string]any)
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
		if resultMap, ok := v.(map[string]any); ok {
			maps.Copy(result, resultMap)
		}
	}

	return result, nil
}

func (f *Filter) FilterInfo() string {
	return "jqFilter implementation: using itchyny/gojq"
}
