// +build !cgo

package jq

// runJqFilter runs exec filter if CGO is disabled.
func runJqFilter(jqFilter string, jsonData []byte, libPath string) (result string, err error) {
	return jqFilterExec(jqFilter, jsonData, libPath)
}
