package jq

import (
	"encoding/json"
	"errors"

	"github.com/itchyny/gojq"

	"github.com/flant/shell-operator/pkg/filter"
)

var _ filter.Filter = (*Filter)(nil)

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

func (f *Filter) FilterInfo() string {
	return "jqFilter implementation: using itchyny/gojq"
}

func deepCopyAny(input any) (any, error) {
	if input == nil {
		return nil, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var output any
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}
