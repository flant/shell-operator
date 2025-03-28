package jq

import (
	"errors"

	"github.com/itchyny/gojq"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/flant/shell-operator/pkg/filter"
)

var _ filter.Filter = (*Filter)(nil)

func NewFilter() *Filter {
	return &Filter{}
}

type Filter struct{}

// ApplyFilter runs jq expression provided in jqFilter with jsonData as input.
func (f *Filter) ApplyFilter(jqFilter string, jsonData []byte) (string, error) {
	query, err := gojq.Parse(jqFilter)
	if err != nil {
		return "", err
	}

	var data map[string]any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return "", err
	}

	iter := query.Run(data)
	var result string
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
			return "", err
		}
		bytes, err := json.Marshal(v.(map[string]any))
		if err != nil {
			return "", err
		}
		result += string(bytes)
	}

	return result, nil
}

func (f *Filter) FilterInfo() string {
	return "jqFilter implementation: using itchyny/gojq"
}
