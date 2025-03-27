package jq

import (
	"fmt"

	"github.com/itchyny/gojq"

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
	iter := query.Run(jsonData)
	var result string
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if v == nil {
			continue
		}
		if _, ok := v.(error); ok {
			continue
		}
		result += fmt.Sprintf("%v", v)
	}

	return result, nil
}

func (f *Filter) FilterInfo() string {
	return "jqFilter implementation: using itchyny/gojq"
}
