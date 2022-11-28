// +build cgo,use_libjq

package jq

import (
	"fmt"
	"os"

	. "github.com/flant/libjq-go"
)

// Note: add build tag 'use_libjg' to build with libjq-go.

// ApplyJqFilter runs jq expression provided in jqFilter with jsonData as input.
//
// It uses libjq-go or executes jq as a binary if $JQ_EXEC is set to "yes".
func ApplyJqFilter(jqFilter string, jsonData []byte, libPath string) (string, error) {
	// Use jq exec filtering if environment variable is present.
	if os.Getenv("JQ_EXEC") == "yes" {
		return jqExec(jqFilter, jsonData, libPath)
	}

	result, err := Jq().WithLibPath(libPath).Program(jqFilter).Cached().Run(string(jsonData))
	if err != nil {
		return "", fmt.Errorf("libjq filter '%s': '%s'", jqFilter, err)
	}
	return result, nil
}

func JqFilterInfo() string {
	if os.Getenv("JQ_EXEC") == "yes" {
		return "JQ_EXEC is set, use jq binary for jqFilter"
	}
	return "Use libjq-go for jqFilter"
}
