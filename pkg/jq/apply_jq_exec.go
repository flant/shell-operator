// +build !cgo cgo,!use_libjq

package jq

// Note: this implementation is enabled by default.

// ApplyJqFilter runs jq expression provided in jqFilter with jsonData as input.
//
// It uses jq as a subprocess.
func ApplyJqFilter(jqFilter string, jsonData []byte, libPath string) (string, error) {
	return jqExec(jqFilter, jsonData, libPath)
}

func JqFilterInfo() string {
	return "Use jq binary for jqFilter"
}
