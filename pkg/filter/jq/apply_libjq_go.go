//go:build cgo && use_libjq
// +build cgo,use_libjq

package jq

import (
	"fmt"
	"os"

	libjq "github.com/flant/libjq-go"
	"github.com/flant/shell-operator/pkg/filter"
)

var _ filter.Filter = (*Filter)(nil)

func NewFilter(libpath string) *Filter {
	return &Filter{
		Libpath: libpath,
	}
}

type Filter struct {
	Libpath string
}

// Note: add build tag 'use_libjg' to build with libjq-go.

// ApplyJqFilter runs jq expression provided in jqFilter with jsonData as input.
//
// It uses libjq-go or executes jq as a binary if $JQ_EXEC is set to "yes".
func (f *Filter) ApplyFilter(jqFilter string, jsonData []byte) (string, error) {
	// Use jq exec filtering if environment variable is present.
	if os.Getenv("JQ_EXEC") == "yes" {
		return jqExec(jqFilter, jsonData, f.Libpath)
	}

	result, err := libjq.Jq().WithLibPath(f.Libpath).Program(jqFilter).Cached().Run(string(jsonData))
	if err != nil {
		return "", fmt.Errorf("libjq filter '%s': '%s'", jqFilter, err)
	}
	return result, nil
}

func (f *Filter) FilterInfo() string {
	if os.Getenv("JQ_EXEC") == "yes" {
		return "jqFilter implementation: use jq binary from $PATH (JQ_EXEC=yes is set)"
	}
	return "jqFilter implementation: use embedded libjq-go"
}
