package jq

// ApplyJqFilter runs jq expression provided in jqFilter with jsonData as input.
//
// It uses libjq-go when CGO is enabled and executes jq binary
// if CGO is disabled or JQ_EXEC variable is set to "yes".
func ApplyJqFilter(jqFilter string, jsonData []byte, libPath string) (string, error) {
	return runJqFilter(jqFilter, jsonData, libPath)
}
