//go:build !cgo || (cgo && !use_libjq)
// +build !cgo cgo,!use_libjq

package jq

import "github.com/flant/shell-operator/pkg/filter"

var _ filter.Filter = (*Filter)(nil)

func NewFilter(libpath string) *Filter {
	return &Filter{
		Libpath: libpath,
	}
}

type Filter struct {
	Libpath string
}

// ApplyJqFilter runs jq expression provided in jqFilter with jsonData as input.
//
// It uses jq as a subprocess.
func (f *Filter) ApplyFilter(jqFilter string, jsonData []byte) (string, error) {
	return jqExec(jqFilter, jsonData, f.Libpath)
}

func (f *Filter) FilterInfo() string {
	return "jqFilter implementation: use jq binary from $PATH"
}
