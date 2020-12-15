// +build cgo

package jq

import (
	"fmt"
	"os"

	. "github.com/flant/libjq-go"
)

// runJqFilter uses libjq-go filter if CGO is enabled and
// uses exec filter if $JQ_EXEC is set to "yes".
func runJqFilter(jqFilter string, jsonData []byte, libPath string) (result string, err error) {
	if os.Getenv("JQ_EXEC") == "yes" {
		return jqFilterExec(jqFilter, jsonData, libPath)
	}
	return jqFilterLibJqGo(jqFilter, jsonData, libPath)
}

func jqFilterLibJqGo(jqFilter string, jsonData []byte, libPath string) (result string, err error) {
	result, err = Jq().WithLibPath(libPath).Program(jqFilter).Cached().Run(string(jsonData))
	if err != nil {
		return "", fmt.Errorf("libjq filter '%s': '%s'", jqFilter, err)
	}
	return
}
